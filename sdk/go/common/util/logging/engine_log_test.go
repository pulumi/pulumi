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

package logging

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// fakeEngine is a minimal EngineServer that records the Log requests it receives.
type fakeEngine struct {
	pulumirpc.UnimplementedEngineServer

	mu   sync.Mutex
	logs []loggedMessage
}

type loggedMessage struct {
	severity pulumirpc.LogSeverity
	message  string
}

func (e *fakeEngine) Log(_ context.Context, req *pulumirpc.LogRequest) (*emptypb.Empty, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = append(e.logs, loggedMessage{severity: req.Severity, message: req.Message})
	return &emptypb.Empty{}, nil
}

func (e *fakeEngine) snapshot() []loggedMessage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]loggedMessage(nil), e.logs...)
}

// serveFakeEngine starts the fake engine on a local port and returns it with its
// address.
func serveFakeEngine(t *testing.T) (*fakeEngine, string) {
	t.Helper()

	engine := &fakeEngine{}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	pulumirpc.RegisterEngineServer(srv, engine)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	return engine, lis.Addr().String()
}

func TestSlogLevelToLogSeverity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level slog.Level
		want  pulumirpc.LogSeverity
	}{
		{LevelTrace, pulumirpc.LogSeverity_DEBUG},
		{slog.LevelDebug, pulumirpc.LogSeverity_DEBUG},
		{slog.LevelInfo, pulumirpc.LogSeverity_INFO},
		{slog.LevelWarn, pulumirpc.LogSeverity_WARNING},
		{slog.LevelError, pulumirpc.LogSeverity_ERROR},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, slogLevelToLogSeverity(c.level))
	}
}

// TestEngineLogHandlerFallsBackOnError verifies that when the engine rejects a
// log, the record is emitted to the fallback handler rather than dropped.
func TestEngineLogHandlerFallsBackOnError(t *testing.T) {
	t.Parallel()

	conn, err := grpc.NewClient("127.0.0.1:1", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	fallback := &recordingHandler{}
	h := engineLogHandler{
		ctx:      t.Context(),
		engine:   pulumirpc.NewEngineClient(conn),
		fallback: fallback,
	}

	rec := slog.NewRecord(time.Time{}, slog.LevelError, "boom", 0)
	require.NoError(t, h.Handle(t.Context(), rec))

	assert.Equal(t, []string{"boom"}, fallback.messages())
}

// TestInitLoggingForwardsToEngine verifies the end-to-end wiring: when logtostderr
// is set and an engine address is given, InitLogging installs the engine handler
// as primary, Infof-style messages are formatted before forwarding, and each level
// maps to the matching severity. Without an engine address, nothing is forwarded.
//
//nolint:paralleltest // mutates the global slog default logger
func TestInitLoggingForwardsToEngine(t *testing.T) {
	engine, addr := serveFakeEngine(t)
	restoreLogging(t)

	InitLogging(true /*logToStderr*/, 0, false, addr)

	Infof("hello %s", "world")
	Warningf("careful %d", 2)
	Errorf("boom")

	assert.Equal(t, []loggedMessage{
		{pulumirpc.LogSeverity_INFO, "hello world"},
		{pulumirpc.LogSeverity_WARNING, "careful 2"},
		{pulumirpc.LogSeverity_ERROR, "boom"},
	}, engine.snapshot())
}

//nolint:paralleltest // mutates the global slog default logger
func TestInitLoggingDoesNotForwardWithoutLogToStderr(t *testing.T) {
	engine, addr := serveFakeEngine(t)
	restoreLogging(t)

	// No logtostderr, so the engine handler is not installed even with an address.
	InitLogging(false, 0, false, addr)
	Errorf("boom")

	assert.Empty(t, engine.snapshot())
}

// restoreLogging snapshots and restores the global handler state mutated by
// InitLogging so tests don't leak handlers into one another.
func restoreLogging(t *testing.T) {
	t.Helper()
	handlerMu.Lock()
	savedPrimary, savedConn := primary, engineLogConn
	handlerMu.Unlock()
	t.Cleanup(func() {
		handlerMu.Lock()
		defer handlerMu.Unlock()
		if engineLogConn != nil && engineLogConn != savedConn {
			engineLogConn.Close() //nolint:errcheck
		}
		primary, engineLogConn = savedPrimary, savedConn
		rebuildLogger()
	})
}

// recordingHandler is a slog.Handler that records the messages it handles.
type recordingHandler struct {
	mu   sync.Mutex
	msgs []string
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, r.Message)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }

func (h *recordingHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.msgs...)
}
