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
	// UIEvents, when non-nil, receives parsed events for the bubbletea TUI to display.
	// The Session closes this channel when Run() exits.
	UIEvents chan<- UIEvent
}

// Run drives the loop. It blocks until ctx is cancelled or the SSE stream errors out.
// If UIEvents is set, it is closed when Run returns.
func (s *Session) Run(ctx context.Context) error {
	if s.UIEvents != nil {
		defer close(s.UIEvents)
	}

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
	// Forward user input echoes to the TUI so the user's messages are visible.
	if env.Type == consoleEventUserInput && len(env.EventBody) > 0 {
		s.forwardUserInputToUI(env.EventBody)
		return nil
	}

	// We only care about agent → user events for tool dispatch.
	if env.Type != consoleEventAgentResponse || len(env.EventBody) == 0 {
		return nil
	}

	// Forward all backend events to the TUI (if connected) before tool dispatch.
	s.forwardToUI(env.EventBody)

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
		// No CLI-side work to do. If the agent marked this as its final message,
		// the turn is complete and the TUI can re-enable input.
		if msg.IsFinal {
			sendUI(s.UIEvents, UITaskIdle{})
		}
		return nil
	}
	return s.runBatch(ctx, cliCalls)
}

func (s *Session) runBatch(ctx context.Context, calls []apitype.AgentBackendEventToolCall) error {
	items := make([]apitype.AgentUserEventToolResultItem, 0, len(calls))
	for _, call := range calls {
		sendUI(s.UIEvents, UIToolStarted{Name: call.Name, Args: call.Args})

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

		result := s.invokeToolCall(ctx, call)
		items = append(items, result)

		sendUI(s.UIEvents, UIToolCompleted{Name: call.Name, Args: call.Args, IsError: result.IsError})
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

// sendUI sends a UIEvent to the TUI channel if it is connected.
// Uses a non-blocking send so the session loop is never blocked by a slow TUI.
func sendUI(ch chan<- UIEvent, evt UIEvent) {
	if ch == nil {
		return
	}
	select {
	case ch <- evt:
	default:
	}
}

// forwardToUI parses a backend event body and sends the appropriate UIEvent to the TUI.
// This is called for every agentResponse envelope so the TUI can display all event types,
// not just the assistant_message tool calls that the session loop acts on.
func (s *Session) forwardToUI(eventBody json.RawMessage) {
	if s.UIEvents == nil {
		return
	}

	var head apitype.AgentBackendEventHeader
	if err := json.Unmarshal(eventBody, &head); err != nil {
		return
	}

	switch head.Type {
	case backendEventAssistantMessage:
		var msg apitype.AgentBackendEventAssistantMessage
		if err := json.Unmarshal(eventBody, &msg); err != nil {
			return
		}
		sendUI(s.UIEvents, UIAssistantMessage{Content: msg.Content, IsFinal: msg.IsFinal})
	case backendEventExecToolCallProgress:
		var p apitype.AgentBackendEventExecToolCallProgress
		if err := json.Unmarshal(eventBody, &p); err != nil {
			return
		}
		sendUI(s.UIEvents, UIToolProgress{Name: p.Name, Message: p.Content})
	case backendEventError:
		var e apitype.AgentBackendEventError
		if err := json.Unmarshal(eventBody, &e); err != nil {
			return
		}
		sendUI(s.UIEvents, UIError{Message: e.Message})
	case backendEventWarning:
		var w apitype.AgentBackendEventWarning
		if err := json.Unmarshal(eventBody, &w); err != nil {
			return
		}
		sendUI(s.UIEvents, UIWarning{Message: w.Message})
	case backendEventCancelled:
		sendUI(s.UIEvents, UICancelled{})
	case backendEventUserApprovalRequest:
		var a apitype.AgentBackendEventUserApprovalRequest
		if err := json.Unmarshal(eventBody, &a); err != nil {
			return
		}
		sendUI(s.UIEvents, UIApprovalRequest{
			ApprovalID:  a.ID,
			Message:     a.Message,
			Sensitivity: a.Sensitivity,
		})
	}
	// Server-side exec_tool_call and tool_response events describe tools the agent
	// runtime executes; the CLI-run equivalents are emitted directly from runBatch.
}

// forwardUserInputToUI parses a userInput event body and sends a UIUserMessage to the TUI.
func (s *Session) forwardUserInputToUI(eventBody json.RawMessage) {
	if s.UIEvents == nil {
		return
	}

	var evt apitype.AgentUserEventUserMessage
	if err := json.Unmarshal(eventBody, &evt); err != nil {
		return
	}
	if evt.Content != "" {
		sendUI(s.UIEvents, UIUserMessage{Content: evt.Content})
	}
}
