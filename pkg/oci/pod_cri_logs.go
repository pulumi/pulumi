// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oci

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"time"
)

// The CRI log format. containerd writes each container's output to a file, one framed line per
// newline, shaped:
//
//	<RFC3339Nano timestamp> <stream> <tag> <content>
//
// where stream is "stdout" or "stderr" and tag is "F" (a full line — content ends here) or "P" (a
// partial line — content continues in the following line(s) of the same stream). This is the
// kubelet/CRI on-disk convention. CRI has no log-read RPC, so reading a container's output means
// reading this file and undoing the framing — the honest asterisk to the "engine needs only the
// endpoint" story: it also needs the pod log directory mounted.
const (
	criStreamStdout = "stdout"
	criStreamStderr = "stderr"
	criTagFull      = "F"
	criTagPartial   = "P"
)

// criLogPollInterval is how often the follow-mode tailer re-reads a log file after reaching its
// current end, waiting for the container to produce more.
const criLogPollInterval = 100 * time.Millisecond

// deframeCRILogLine parses one framed CRI log line (without its trailing newline) into its
// content, stream, and full/partial flag. ok is false for a line that is not in the expected
// framing — the caller passes such lines through unchanged rather than dropping output it does not
// recognize.
//
// Only the first three spaces are structural (timestamp, stream, tag); everything after the third
// space is content verbatim, so content keeps any spaces of its own.
func deframeCRILogLine(line []byte) (content, stream []byte, full, ok bool) {
	// timestamp
	sp := bytes.IndexByte(line, ' ')
	if sp < 0 {
		return nil, nil, false, false
	}
	rest := line[sp+1:]
	// stream
	sp = bytes.IndexByte(rest, ' ')
	if sp < 0 {
		return nil, nil, false, false
	}
	stream = rest[:sp]
	rest = rest[sp+1:]
	// tag, then content (the remainder). A line may carry an empty content with no trailing
	// space ("<ts> stdout F"), so a missing third space means empty content, not a parse failure.
	var tag []byte
	if sp = bytes.IndexByte(rest, ' '); sp < 0 {
		tag, content = rest, nil
	} else {
		tag, content = rest[:sp], rest[sp+1:]
	}

	if !bytes.Equal(stream, []byte(criStreamStdout)) && !bytes.Equal(stream, []byte(criStreamStderr)) {
		return nil, nil, false, false
	}
	switch {
	case bytes.Equal(tag, []byte(criTagFull)):
		full = true
	case bytes.Equal(tag, []byte(criTagPartial)):
		full = false
	default:
		return nil, nil, false, false
	}
	return content, stream, full, true
}

// criLogStream is the io.ReadCloser ContainerLogs hands back. A goroutine tails the log file,
// de-frames each line, and writes the reconstructed content to a pipe; Read consumes it. Closing
// cancels the tailer and unblocks the goroutine's next write.
type criLogStream struct {
	reader *io.PipeReader
	cancel context.CancelFunc
}

func (s *criLogStream) Read(p []byte) (int, error) { return s.reader.Read(p) }

func (s *criLogStream) Close() error {
	s.cancel()
	return s.reader.Close()
}

// newCRILogStream begins tailing the log file at path and returns a reader over its de-framed
// content. It returns immediately without opening the file: the tailer opens it (retrying while it
// does not yet exist, in follow mode, since a just-started container may not have produced its log
// yet) so a caller that attaches right after StartContainer does not race the file into existence.
// When follow is false it reads to the current end and stops; when true it keeps reading until the
// context is cancelled or the reader is closed.
func newCRILogStream(ctx context.Context, path string, follow bool) (io.ReadCloser, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	pr, pw := io.Pipe()
	go tailCRILog(streamCtx, path, follow, pw)
	return &criLogStream{reader: pr, cancel: cancel}, nil
}

// tailCRILog opens and tails the log file, writing de-framed content to pw. It owns pw and always
// closes it. Full lines (and unrecognized lines) are terminated with a newline; partial-line
// content is written without one so the logical line reassembles across P continuations — exactly
// what a line-oriented consumer such as scrapeServingPort expects.
func tailCRILog(ctx context.Context, path string, follow bool, pw *io.PipeWriter) {
	f, err := openCRILog(ctx, path, follow)
	if err != nil {
		_ = pw.CloseWithError(err)
		return
	}
	defer f.Close()

	var pending []byte // bytes read but not yet split into a complete line
	buf := make([]byte, 4096)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				nl := bytes.IndexByte(pending, '\n')
				if nl < 0 {
					break
				}
				if err := writeDeframed(pw, pending[:nl]); err != nil {
					return // the reader was closed
				}
				pending = pending[nl+1:]
			}
		}
		if readErr == io.EOF {
			if !follow {
				_ = pw.Close()
				return
			}
			select {
			case <-ctx.Done():
				_ = pw.Close()
				return
			case <-time.After(criLogPollInterval):
			}
			continue
		}
		if readErr != nil {
			_ = pw.CloseWithError(readErr)
			return
		}
	}
}

// writeDeframed writes one raw log line's de-framed content to pw, reconstructing the logical-line
// newline. It returns an error only if pw is closed (the reader went away), which is the tailer's
// signal to stop.
func writeDeframed(pw *io.PipeWriter, rawLine []byte) error {
	content, _, full, ok := deframeCRILogLine(rawLine)
	out := content
	if !ok {
		out = rawLine // pass an unframed line through untouched
	}
	if len(out) > 0 {
		if _, err := pw.Write(out); err != nil {
			return err
		}
	}
	if full || !ok {
		if _, err := pw.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// openCRILog opens the log file. In follow mode it retries while the file does not yet exist,
// because a container's log file may appear a moment after it is started; it gives up when the
// context is cancelled.
func openCRILog(ctx context.Context, path string, follow bool) (*os.File, error) {
	for {
		f, err := os.Open(path)
		if err == nil {
			return f, nil
		}
		if !follow || !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(criLogPollInterval):
		}
	}
}
