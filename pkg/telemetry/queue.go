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
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
)

const (
	walExtension    = ".jsonl"
	maxBatch        = 100
	orphanDelay     = 2 * time.Second
	staleEmptyAge   = time.Minute
	shutdownTimeout = 5 * time.Second
	requestTimeout  = 10 * time.Second
)

type queue struct {
	dir        string
	invocation string
	url        string
	userAgent  string
	ci         bool
	pulse      time.Duration
	client     *http.Client

	sendMu sync.Mutex

	mu      sync.Mutex
	pending []json.RawMessage
	wal     *os.File
	held    bool
	closed  bool

	stop            chan struct{}
	cancel          context.CancelFunc
	shutdownTimeout time.Duration
}

func newQueue(dir, invocation, url, userAgent string, ci bool, pulse time.Duration) *queue {
	return &queue{
		dir:        dir,
		invocation: invocation,
		url:        strings.TrimSuffix(url, "/") + "/api/telemetry/v1",
		userAgent:  userAgent,
		ci:         ci,
		pulse:      pulse,
		client:     &http.Client{Timeout: requestTimeout},
		stop:       make(chan struct{}),
		cancel:     func() {},

		shutdownTimeout: shutdownTimeout,
	}
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
		case <-q.stop:
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
		if err := q.post(ctx, batch); err != nil {
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
	close(q.stop)
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
	close(q.stop)
	q.pending = nil
	if q.wal != nil {
		logCleanupErr(fsutil.UnlockFile(q.wal))
		logCleanupErr(q.wal.Close())
		logCleanupErr(os.Remove(q.wal.Name()))
		q.wal = nil
	}
}

func (q *queue) post(ctx context.Context, events []json.RawMessage) error {
	body, err := json.Marshal(struct {
		Events []json.RawMessage `json:"events"`
	}{events})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", q.userAgent)

	resp, err := q.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode < 300:
		return nil
	case resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout &&
		resp.StatusCode != http.StatusTooManyRequests:
		slog.DebugContext(ctx, "telemetry rejected", "status", resp.StatusCode)
		return nil
	default:
		return fmt.Errorf("telemetry endpoint returned %s", resp.Status)
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
		if err := q.post(ctx, batch); err != nil {
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
