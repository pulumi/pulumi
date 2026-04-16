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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// ToolHandler executes a single named method on a Neo CLI-local tool. The method is the
// part of the agent's full tool name after the "<server>__" prefix; args is the raw JSON
// arguments object. The returned value is JSON-encoded into the AgentUserEventToolResultItem
// content.
type ToolHandler interface {
	Invoke(ctx context.Context, method string, args json.RawMessage) (any, error)
}

// EventStreamer is the subset of *client.Client we depend on for the SSE event stream and
// for posting CLI tool result user events back to the Neo task. It is an interface so the
// loop can be unit-tested without a live HTTP backend.
type EventStreamer interface {
	StreamNeoTaskEvents(ctx context.Context, orgName, taskID string) (<-chan client.NeoStreamEvent, error)
	PostNeoTaskUserEvent(ctx context.Context, orgName, taskID string, body any) error
}

// Session glues the SSE event stream, the local tool handlers, and the Pulumi Cloud client
// together.
type Session struct {
	Client   EventStreamer
	Handlers map[string]ToolHandler
	OrgName  string
	TaskID   string
	// Log receives single-line status messages so the caller can render them however it
	// likes (stderr today, a TUI tomorrow). nil disables logging.
	Log io.Writer
}

// Run drives the loop. It blocks until ctx is cancelled or the SSE stream errors out.
func (s *Session) Run(ctx context.Context) error {
	stream, err := s.Client.StreamNeoTaskEvents(ctx, s.OrgName, s.TaskID)
	if err != nil {
		return fmt.Errorf("opening event stream: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-stream:
			if !ok {
				return nil
			}
			if evt.Err != nil {
				return evt.Err
			}
			if err := s.handleEvent(ctx, evt.Data); err != nil {
				return err
			}
		}
	}
}

func (s *Session) handleEvent(ctx context.Context, raw []byte) error {
	var env apitype.AgentConsoleEvent
	if err := json.Unmarshal(raw, &env); err != nil {
		s.logf("warning: skipping malformed Neo console event: %v", err)
		return nil
	}
	// We only care about agent → user events. User input echoes are ignored.
	if env.Type != consoleEventAgentResponse || len(env.EventBody) == 0 {
		return nil
	}

	var head apitype.AgentBackendEventHeader
	if err := json.Unmarshal(env.EventBody, &head); err != nil {
		s.logf("warning: skipping malformed backend event: %v", err)
		return nil
	}
	if head.Type != backendEventAssistantMessage {
		return nil
	}

	var msg apitype.AgentBackendEventAssistantMessage
	if err := json.Unmarshal(env.EventBody, &msg); err != nil {
		return fmt.Errorf("decoding assistantMessage: %w", err)
	}

	// Only the cli-marked calls are ours to execute. Cloud-marked calls in the same
	// message are handled by the agent runtime — touching them would double-execute
	// the tool. If no calls are cli-marked there is nothing to do and posting an empty
	// tool_result would confuse the agent (which is not paused waiting for us).
	cliCalls := make([]apitype.AgentBackendEventToolCall, 0, len(msg.ToolCalls))
	for _, call := range msg.ToolCalls {
		if call.ExecutionMode == toolExecutionModeCLI {
			cliCalls = append(cliCalls, call)
		}
	}
	if len(cliCalls) == 0 {
		return nil
	}
	return s.runBatch(ctx, cliCalls)
}

func (s *Session) runBatch(ctx context.Context, calls []apitype.AgentBackendEventToolCall) error {
	items := make([]apitype.AgentUserEventToolResultItem, 0, len(calls))
	for _, call := range calls {
		s.logf("→ %s", call.Name)

		// The agent runtime relies on exec_tool_call to transition the call into its
		// "running" state. If this post fails, the agent will believe the tool never
		// started, so any tool_result we'd send later would be rejected or mis-attributed.
		// Abort the batch and let the session loop surface the error.
		execEvt := apitype.AgentUserEventExecToolCall{
			Type:       userEventExecToolCall,
			ToolCallID: call.ToolCallID,
			Name:       call.Name,
		}
		if err := s.Client.PostNeoTaskUserEvent(ctx, s.OrgName, s.TaskID, execEvt); err != nil {
			return fmt.Errorf("posting exec_tool_call for %q: %w", call.Name, err)
		}

		items = append(items, s.invokeToolCall(ctx, call))
	}

	result := apitype.AgentUserEventToolResult{
		Type:        userEventToolResult,
		ToolResults: items,
	}
	if err := s.Client.PostNeoTaskUserEvent(ctx, s.OrgName, s.TaskID, result); err != nil {
		s.logf("error: posting tool_result: %v", err)
	}
	return nil
}

// invokeToolCall dispatches a single tool call to the appropriate handler by splitting
// the tool name on "__" into server and method. Errors are returned as
// AgentUserEventToolResultItem with IsError=true rather than propagated, so the agent can
// retry or report.
func (s *Session) invokeToolCall(
	ctx context.Context, call apitype.AgentBackendEventToolCall,
) apitype.AgentUserEventToolResultItem {
	res := apitype.AgentUserEventToolResultItem{ToolCallID: call.ToolCallID, Name: call.Name}

	server, method, ok := strings.Cut(call.Name, "__")
	if !ok {
		res.IsError = true
		res.Content = map[string]string{"error": fmt.Sprintf("tool name %q is missing the server prefix", call.Name)}
		return res
	}
	handler, ok := s.Handlers[server]
	if !ok {
		res.IsError = true
		res.Content = map[string]string{"error": fmt.Sprintf("tool %q is not available in CLI mode", server)}
		return res
	}
	value, err := handler.Invoke(ctx, method, call.Args)
	if err != nil {
		res.IsError = true
		res.Content = map[string]string{"error": err.Error()}
		return res
	}
	res.Content = value
	return res
}

func (s *Session) logf(format string, args ...any) {
	if s.Log == nil {
		return
	}
	fmt.Fprintf(s.Log, format+"\n", args...)
}
