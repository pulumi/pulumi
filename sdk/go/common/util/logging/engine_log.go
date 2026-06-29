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
	"io"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// engineLogHandler forwards log records to the engine over gRPC.
type engineLogHandler struct {
	ctx      context.Context
	engine   pulumirpc.EngineClient
	fallback slog.Handler
}

// newEngineLogHandler dials the engine at address and returns a handler that
// forwards records to it, along with the connection to close on shutdown.
func newEngineLogHandler(address string) (slog.Handler, io.Closer, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return engineLogHandler{
		ctx:      context.Background(),
		engine:   pulumirpc.NewEngineClient(conn),
		fallback: stderrJSONHandler(),
	}, conn, nil
}

func (h engineLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h engineLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if _, err := h.engine.Log(h.ctx, &pulumirpc.LogRequest{
		Severity: slogLevelToLogSeverity(r.Level),
		Message:  r.Message,
	}); err != nil {
		return h.fallback.Handle(ctx, r)
	}
	return nil
}

func (h engineLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h engineLogHandler) WithGroup(string) slog.Handler { return h }

func stderrJSONHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: LevelTrace})
}

func slogLevelToLogSeverity(level slog.Level) pulumirpc.LogSeverity {
	switch {
	case level >= slog.LevelError:
		return pulumirpc.LogSeverity_ERROR
	case level >= slog.LevelWarn:
		return pulumirpc.LogSeverity_WARNING
	case level >= slog.LevelInfo:
		return pulumirpc.LogSeverity_INFO
	default:
		return pulumirpc.LogSeverity_DEBUG
	}
}
