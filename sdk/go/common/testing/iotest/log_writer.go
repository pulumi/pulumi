// Copyright 2016-2023, Pulumi Corporation.
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

// Package iotest provides testing utilities for code that uses the io package.
package iotest

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

// logWriter is an io.Writer that writes to a testing.T.
type logWriter struct {
	// t holds the subset of testing.TB
	// that we are allowed to use.
	//
	// We're not storing testing.TB directly to ensure that
	// we don't accidentally use other log methods.
	t interface {
		Logf(string, ...interface{})
		Helper()
	}

	prefix string

	// Holds buffered text for the next write or flush
	// if we haven't yet seen a newline.
	buff bytes.Buffer
	mu   sync.Mutex // guards buff
}

var _ io.Writer = (*logWriter)(nil)

// LogWriter builds and returns an io.Writer that
// writes messages to the given testing.TB.
// It ensures that each line is logged separately.
//
// Any trailing buffered text that does not end with a newline
// is flushed when the test finishes.
//
// The returned writer is safe for concurrent use
// from multiple parallel tests.
func LogWriter(t testing.TB) io.Writer {
	return LogWriterPrefixed(t, "")
}

// LogWriterPrefixed is a variant of LogWriter
// that prepends the given prefix to each line.
func LogWriterPrefixed(t testing.TB, prefix string) io.Writer {
	w := logWriter{t: t, prefix: prefix}
	t.Cleanup(w.flush)
	return &w
}

func (w *logWriter) Write(bs []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.t.Helper() // so that the log message points to the caller

	// t.Logf adds a newline so we should not write bs as-is.
	// Instead, we'll call t.Log one line at a time.
	//
	// To handle the case when Write is called with a partial line,
	// we use a buffer.
	total := len(bs)
	for len(bs) > 0 {
		idx := bytes.IndexByte(bs, '\n')
		if idx < 0 {
			// No newline. Buffer it for later.
			w.buff.Write(bs)
			break
		}

		var line []byte
		line, bs = bs[:idx], bs[idx+1:]

		if w.buff.Len() == 0 {
			// Nothing buffered from a prior partial write.
			// This is the majority case.
			w.t.Logf("%s%s", w.prefix, line)
			continue
		}

		// There's a prior partial write. Join and flush.
		w.buff.Write(line)
		w.t.Logf("%s%s", w.prefix, w.buff.String())
		w.buff.Reset()
	}
	return total, nil
}

// flush flushes buffered text, even if it doesn't end with a newline.
func (w *logWriter) flush() {
	if w.buff.Len() > 0 {
		w.t.Logf("%s%s", w.prefix, w.buff.String())
		w.buff.Reset()
	}
}
