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

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

const (
	// walExtension is the extension of the write-ahead logs in the queue's directory.
	walExtension = ".jsonl"
	// maxBatch is the most events sent at once.
	maxBatch = 100
	// orphanDelay is how long an invocation runs before it sends events other invocations left behind, so that
	// short-lived invocations don't spend time on them.
	orphanDelay = 2 * time.Second
	// staleEmptyAge is how old an empty write-ahead log must be before it is removed, so that a log another
	// invocation has just created isn't removed before that invocation locks it.
	staleEmptyAge = time.Minute
	// shutdownTimeout is how long shutdown waits to send pending events in CI.
	shutdownTimeout = 5 * time.Second
)

// queue collects an invocation's events and sends them in batches. Outside CI it also keeps them in the
// invocation's write-ahead log, so that a later invocation can send whatever this one didn't.
type queue struct {
	// dir holds the write-ahead logs of all invocations.
	dir        string
	invocation string
	// ci means there is no write-ahead log; shutdown sends pending events instead, since a CI machine is unlikely
	// to run the CLI again.
	ci bool
	// pulse is how often pending events are sent.
	pulse time.Duration
	send  sendFunc

	// sendMu makes sure only one flush sends at a time.
	sendMu sync.Mutex

	mu sync.Mutex
	// pending are the events that haven't been sent yet.
	pending []json.RawMessage
	wal     *os.File
	// held keeps events from being sent or written to the write-ahead log until the invocation's backend is known
	// to be eligible.
	held   bool
	closed bool

	cancel          context.CancelFunc
	shutdownTimeout time.Duration
}

func newQueue(dir, invocation string, ci bool, pulse time.Duration, send sendFunc) *queue {
	q := &queue{
		dir:        dir,
		invocation: invocation,
		ci:         ci,
		pulse:      pulse,
		send:       send,
		held:       true,
		cancel:     func() {},

		shutdownTimeout: shutdownTimeout,
	}
	if !ci {
		q.wal = openWAL(dir, invocation)
	}
	return q
}

func (q *queue) add(line json.RawMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.pending = append(q.pending, line)
	if q.wal != nil && !q.held {
		if _, err := fmt.Fprintf(q.wal, "%s\n", line); err != nil {
			slog.Debug("could not write telemetry event", "err", err)
		}
	}
}

// openWAL creates and locks this invocation's write-ahead log, the file events are written to before they are
// sent. If the CLI exits before sending them, a later invocation finds the unlocked file and sends them instead.
func openWAL(dir, invocation string) *os.File {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Debug("could not create telemetry directory", "err", err)
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, invocation+walExtension),
		os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		slog.Debug("could not create telemetry log", "err", err)
		return nil
	}
	if !fsutil.TryLockFile(f) {
		logCleanupErr(f.Close())
		return nil
	}
	return f
}

// rewrite replaces the contents of a write-ahead log in place. Writing a new file and renaming it over the log
// would drop the lock that keeps other invocations from sending its events, and Windows can't rename over a locked
// file. If the write fails, the events are still pending in memory and this invocation keeps trying to send them.
func rewrite(f *os.File, lines []json.RawMessage) error {
	var buf bytes.Buffer
	for _, line := range lines {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := f.Write(buf.Bytes())
	return err
}

func (q *queue) run(ctx context.Context) {
	ticker := time.NewTicker(q.pulse)
	defer ticker.Stop()
	orphans := time.NewTimer(orphanDelay)
	defer orphans.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := q.flush(ctx); err != nil {
				slog.DebugContext(ctx, "could not send telemetry", "err", err)
			}
		case <-orphans.C:
			if q.isHeld() {
				orphans.Reset(orphanDelay)
			} else {
				q.sendOrphans(ctx)
			}
		}
	}
}

func (q *queue) flush(ctx context.Context) error {
	q.sendMu.Lock()
	defer q.sendMu.Unlock()
	for {
		q.mu.Lock()
		if q.held {
			q.mu.Unlock()
			return nil
		}
		batch := q.pending[:min(len(q.pending), maxBatch)]
		q.mu.Unlock()
		if len(batch) == 0 {
			return nil
		}
		if err := q.send(ctx, batch); err != nil {
			return err
		}
		q.mu.Lock()
		if len(q.pending) >= len(batch) {
			q.pending = q.pending[len(batch):]
			if q.wal != nil {
				logCleanupErr(rewrite(q.wal, q.pending))
			}
		}
		q.mu.Unlock()
	}
}

func (q *queue) shutdown(ctx context.Context) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.releaseLocked()
	q.closed = true
	q.mu.Unlock()
	q.cancel()

	if q.ci {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), q.shutdownTimeout)
		defer cancel()
		if err := q.flush(ctx); err != nil {
			slog.DebugContext(ctx, "could not send telemetry", "err", err)
		}
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.wal == nil {
		return
	}
	empty := len(q.pending) == 0
	if empty {
		logCleanupErr(rewrite(q.wal, nil))
	}
	logCleanupErr(fsutil.UnlockFile(q.wal))
	logCleanupErr(q.wal.Close())
	if empty {
		logCleanupErr(os.Remove(q.wal.Name()))
	}
	q.wal = nil
}

func (q *queue) isHeld() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.held
}

func (q *queue) release() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.releaseLocked()
	}
}

func (q *queue) releaseLocked() {
	if !q.held {
		return
	}
	q.held = false
	if q.wal != nil {
		logCleanupErr(rewrite(q.wal, q.pending))
	}
}

func (q *queue) discard() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.cancel()
	q.pending = nil
	if q.wal != nil {
		logCleanupErr(fsutil.UnlockFile(q.wal))
		logCleanupErr(q.wal.Close())
		logCleanupErr(os.Remove(q.wal.Name()))
		q.wal = nil
	}
}

func (q *queue) sendOrphans(ctx context.Context) {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, walExtension) || name == q.invocation+walExtension {
			continue
		}
		q.sendOrphan(ctx, filepath.Join(q.dir, name))
	}
}

func (q *queue) sendOrphan(ctx context.Context, path string) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return
	}
	if info, err := f.Stat(); err != nil || (info.Size() == 0 && time.Since(info.ModTime()) <= staleEmptyAge) ||
		!fsutil.TryLockFile(f) {
		logCleanupErr(f.Close())
		return
	}
	done := false
	defer func() {
		logCleanupErr(fsutil.UnlockFile(f))
		logCleanupErr(f.Close())
		if done {
			logCleanupErr(os.Remove(path))
		}
	}()

	data, err := io.ReadAll(f)
	if err != nil {
		return
	}
	var lines []json.RawMessage
	for line := range bytes.Lines(data) {
		line = bytes.TrimSpace(line)
		if json.Valid(line) {
			lines = append(lines, line)
		}
	}
	for len(lines) > 0 {
		batch := lines[:min(len(lines), maxBatch)]
		if err := q.send(ctx, batch); err != nil {
			slog.DebugContext(ctx, "could not send telemetry", "err", err)
			break
		}
		lines = lines[len(batch):]
	}
	done = rewrite(f, lines) == nil && len(lines) == 0
}

func logCleanupErr(err error) {
	if err != nil {
		slog.Debug("telemetry cleanup failed", "err", err)
	}
}
