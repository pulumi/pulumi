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

package neo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// EventStreamer is the subset of *client.Client we depend on for the SSE event stream and
// for posting CLI tool result user events back to the Neo task. It is an interface so the
// loop can be unit-tested without a live HTTP backend.
type EventStreamer interface {
	StreamNeoTaskEvents(ctx context.Context, orgName, taskID string) (<-chan []byte, <-chan error, error)
	PostNeoTaskUserEvent(ctx context.Context, orgName, taskID string, body any) error
}

// Session glues the SSE event stream, the local Executor, and the Pulumi Cloud client
// together.
type Session struct {
	Client   EventStreamer
	Executor *Executor
	OrgName  string
	TaskID   string
	// Log receives single-line status messages so the caller can render them however it
	// likes (stderr today, a TUI tomorrow). nil disables logging.
	Log io.Writer
}

// Run drives the loop. It blocks until ctx is cancelled or the SSE stream errors out.
func (s *Session) Run(ctx context.Context) error {
	events, errs, err := s.Client.StreamNeoTaskEvents(ctx, s.OrgName, s.TaskID)
	if err != nil {
		return fmt.Errorf("opening event stream: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case streamErr, ok := <-errs:
			if ok && streamErr != nil {
				return streamErr
			}
		case raw, ok := <-events:
			if !ok {
				return nil
			}
			if err := s.handleEvent(ctx, raw); err != nil {
				return err
			}
		}
	}
}

func (s *Session) handleEvent(ctx context.Context, raw []byte) error {
	var env ConsoleEventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		s.logf("warning: skipping malformed Neo console event: %v", err)
		return nil
	}
	// We only care about agent → user events. User input echoes are ignored.
	if env.Type != consoleEventAgentResponse || len(env.EventBody) == 0 {
		return nil
	}

	var head BackendEventHeader
	if err := json.Unmarshal(env.EventBody, &head); err != nil {
		s.logf("warning: skipping malformed backend event: %v", err)
		return nil
	}
	if head.Type != backendEventCliToolRequest {
		return nil
	}

	var req CliToolRequest
	if err := json.Unmarshal(env.EventBody, &req); err != nil {
		return fmt.Errorf("decoding cli_tool_request: %w", err)
	}
	s.runBatch(ctx, req)
	return nil
}

func (s *Session) runBatch(ctx context.Context, req CliToolRequest) {
	for _, call := range req.ToolCalls {
		s.logf("→ %s", call.Name)
	}

	items := s.Executor.Execute(ctx, req.ToolCalls)
	result := CliToolResult{
		Type:        userEventCliToolResult,
		Timestamp:   time.Now().UTC(),
		EntityDiff:  EntityDiff{Add: []any{}, Remove: []any{}},
		ToolResults: items,
	}
	if err := s.Client.PostNeoTaskUserEvent(ctx, s.OrgName, s.TaskID, result); err != nil {
		s.logf("error: posting cli_tool_result: %v", err)
	}
}

func (s *Session) logf(format string, args ...any) {
	if s.Log == nil {
		return
	}
	fmt.Fprintf(s.Log, format+"\n", args...)
}
