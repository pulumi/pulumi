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
	w := logWriter{t: t}
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
			w.t.Logf("%s", line)
			continue
		}

		// There's a prior partial write. Join and flush.
		w.buff.Write(line)
		w.t.Logf("%s", w.buff.String())
		w.buff.Reset()
	}
	return total, nil
}

// flush flushes buffered text, even if it doesn't end with a newline.
func (w *logWriter) flush() {
	if w.buff.Len() > 0 {
		w.t.Logf("%s", w.buff.String())
	}
}
