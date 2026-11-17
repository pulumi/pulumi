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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeSender struct {
	mu      sync.Mutex
	batches [][]json.RawMessage
	err     error
	release chan struct{}
}

func (s *fakeSender) send(ctx context.Context, events []json.RawMessage) error {
	s.mu.Lock()
	s.batches = append(s.batches, events)
	err, release := s.err, s.release
	s.mu.Unlock()
	if release != nil {
		<-release
	}
	return err
}

func (s *fakeSender) sent() [][]json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]json.RawMessage{}, s.batches...)
}

func newTestQueue(t *testing.T, dir, invocation string, send sendFunc) *queue {
	q := newQueue(dir, invocation, false, time.Hour, send)
	t.Cleanup(q.discard)
	return q
}

func readWAL(t *testing.T, q *queue) string {
	q.mu.Lock()
	defer q.mu.Unlock()
	info, err := q.wal.Stat()
	require.NoError(t, err)
	data := make([]byte, info.Size())
	_, err = q.wal.ReadAt(data, 0)
	if !errors.Is(err, io.EOF) {
		require.NoError(t, err)
	}
	return string(data)
}

func testEvent(i int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"facts":{"exit-code":%d}}`, i))
}

func TestHeldUntilReleased(t *testing.T) {
	t.Parallel()

	s := &fakeSender{}
	q := newTestQueue(t, t.TempDir(), "self", s.send)
	q.add(testEvent(0))

	require.NoError(t, q.flush(t.Context()))
	require.Empty(t, s.sent())
	require.Empty(t, readWAL(t, q))

	q.release()
	require.Equal(t, string(testEvent(0))+"\n", readWAL(t, q))

	require.NoError(t, q.flush(t.Context()))
	require.Len(t, s.sent(), 1)
	require.Empty(t, readWAL(t, q))
}

func TestFlushBatches(t *testing.T) {
	t.Parallel()

	s := &fakeSender{}
	q := newTestQueue(t, t.TempDir(), "self", s.send)
	q.release()
	for i := range 2*maxBatch + 50 {
		q.add(testEvent(i))
	}

	require.NoError(t, q.flush(t.Context()))

	sent := s.sent()
	sizes := make([]int, 0, len(sent))
	for _, batch := range sent {
		sizes = append(sizes, len(batch))
	}
	require.Equal(t, []int{maxBatch, maxBatch, 50}, sizes)
	require.Empty(t, q.pending)
	require.Empty(t, readWAL(t, q))
}

func TestFlushKeepsPendingOnError(t *testing.T) {
	t.Parallel()

	s := &fakeSender{err: errors.New("unavailable")}
	q := newTestQueue(t, t.TempDir(), "self", s.send)
	q.release()
	q.add(testEvent(0))

	require.ErrorContains(t, q.flush(t.Context()), "unavailable")
	require.Len(t, q.pending, 1)
	require.Equal(t, string(testEvent(0))+"\n", readWAL(t, q))
}

func TestFlushAfterDiscard(t *testing.T) {
	t.Parallel()

	s := &fakeSender{release: make(chan struct{})}
	q := newTestQueue(t, t.TempDir(), "self", s.send)
	q.release()
	q.add(testEvent(0))

	flushed := make(chan error, 1)
	go func() { flushed <- q.flush(t.Context()) }()
	require.Eventually(t, func() bool { return len(s.sent()) == 1 }, 10*time.Second, 10*time.Millisecond)

	q.discard()
	close(s.release)

	require.NoError(t, <-flushed)
	require.Empty(t, q.pending)
}

func TestOrphanSkipsCorruptLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	wal := filepath.Join(dir, "crashed"+walExtension)
	require.NoError(t, os.WriteFile(wal, []byte(`{"facts":{"ci":true}}`+"\n"+`{"facts":{"ci":`), 0o600))

	s := &fakeSender{}
	newTestQueue(t, dir, "other", s.send).sendOrphans(t.Context())

	require.Len(t, s.sent(), 1)
	require.Len(t, s.sent()[0], 1)
	require.NoFileExists(t, wal)
}

func TestOrphanEmptyLogs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh"+walExtension)
	stale := filepath.Join(dir, "stale"+walExtension)
	require.NoError(t, os.WriteFile(fresh, nil, 0o600))
	require.NoError(t, os.WriteFile(stale, nil, 0o600))
	old := time.Now().Add(-2 * staleEmptyAge)
	require.NoError(t, os.Chtimes(stale, old, old))

	s := &fakeSender{}
	newTestQueue(t, dir, "other", s.send).sendOrphans(t.Context())

	require.Empty(t, s.sent())
	require.FileExists(t, fresh)
	require.NoFileExists(t, stale)
}

func TestOrphanKeptWhenSendFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	wal := filepath.Join(dir, "crashed"+walExtension)
	require.NoError(t, os.WriteFile(wal, []byte(`{"facts":{"ci":true}}`+"\n"), 0o600))

	s := &fakeSender{err: errors.New("unavailable")}
	newTestQueue(t, dir, "other", s.send).sendOrphans(t.Context())

	require.Len(t, s.sent(), 1)
	data, err := os.ReadFile(wal)
	require.NoError(t, err)
	require.Equal(t, `{"facts":{"ci":true}}`+"\n", string(data))
}
