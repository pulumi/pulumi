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

package auth

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

const (
	helperMessageBuffer = 128
	helperLineLimit     = 4096
)

type helperOutput struct {
	mu         sync.Mutex
	sink       func(string)
	terminalMu sync.Mutex
	terminal   io.Writer
}

var defaultHelperOutput = &helperOutput{}

// CaptureHelperStderr routes CLI helper diagnostics to a renderer until cleanup is called.
func CaptureHelperStderr() (<-chan string, func()) {
	return defaultHelperOutput.capture()
}

func (w *helperOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	if w.sink != nil {
		w.sink(string(p))
		w.mu.Unlock()
		return len(p), nil
	}
	w.mu.Unlock()
	return len(p), w.writeTerminal(string(p))
}

func (w *helperOutput) writeTerminal(message string) error {
	w.terminalMu.Lock()
	defer w.terminalMu.Unlock()
	terminal := w.terminal
	if terminal == nil {
		terminal = os.Stderr
	}
	_, err := io.WriteString(terminal, logging.FilterString(message))
	return err
}

// capture buffers diagnostics without making helper completion depend on the renderer.
func (w *helperOutput) capture() (<-chan string, func()) {
	messages := make(chan string, helperMessageBuffer)
	omitted := 0
	omission := func() string {
		return fmt.Sprintf("[credential helper: omitted %d diagnostic messages while the display was busy]\n", omitted)
	}
	w.mu.Lock()
	previous := w.sink
	w.sink = func(message string) {
		if omitted > 0 {
			select {
			case messages <- omission():
				omitted = 0
			default:
				omitted++
				return
			}
		}
		select {
		case messages <- message:
		default:
			omitted++
		}
	}
	w.mu.Unlock()
	return messages, sync.OnceFunc(func() {
		w.mu.Lock()
		w.sink = previous
		close(messages)
		var notice string
		if omitted > 0 {
			notice = omission()
		}
		w.mu.Unlock()

		// The renderer has stopped. Flush queued diagnostics without holding up helper writes.
		for message := range messages {
			_ = w.writeTerminal(message)
		}
		_ = w.writeTerminal(notice)
	})
}

// helperLines assembles stderr lines from one invocation. Overlong lines are omitted, so a
// truncated secret cannot bypass the display's line-based redaction.
type helperLines struct {
	output  io.Writer
	pending []byte
	tooLong bool
}

func (w *helperLines) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		line, rest, complete := bytes.Cut(p, []byte{'\n'})
		if len(w.pending)+len(line) > helperLineLimit {
			w.tooLong = true
		}
		if !w.tooLong {
			w.pending = append(w.pending, line...)
		}
		if complete {
			w.pending = append(w.pending, '\n')
			w.flush()
		}
		p = rest
	}
	return n, nil
}

func (w *helperLines) flush() {
	if w.tooLong {
		fmt.Fprintln(w.output, "[credential helper: omitted diagnostic line exceeding 4096 bytes]")
	} else if len(w.pending) > 0 {
		_, _ = w.output.Write(w.pending)
	}
	w.pending = w.pending[:0]
	w.tooLong = false
}
