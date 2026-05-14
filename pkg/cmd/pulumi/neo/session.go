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
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// Reconnect tuning. Backoff doubles per consecutive failure up to reconnectMaxBackoff;
// the total budget resets on every successfully-delivered event. Vars (not consts) so
// tests can override them.
var (
	reconnectInitialBackoff = 1 * time.Second
	reconnectMaxBackoff     = 30 * time.Second
	reconnectTotalBudget    = 5 * time.Minute
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
//
// lastEventID is the SSE id of the last event the caller successfully processed; the
// service replays only events with sequence greater than that id, so a reconnect resumes
// losslessly. Pass "" for the initial connection.
type EventStreamer interface {
	StreamNeoTaskEvents(ctx context.Context, orgName, taskID, lastEventID string) (<-chan client.NeoStreamEvent, error)
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
	// The caller owns the channel — Session.Run never closes it. The channel has
	// other writers (dispatchUserEvents, createTask, the pulumi sink), and closing
	// from here races them (pulumi/pulumi-service#42773).
	UIEvents chan<- UIEvent
}

// Run drives the loop. It blocks until ctx is cancelled (clean shutdown, returns nil),
// the stream ends cleanly (returns nil), or an unrecoverable error occurs (returns the
// error). Mid-stream network drops are reopened silently with Last-Event-ID so the
// server replays missed events; the user sees no signal unless the retry budget is
// exhausted.
func (s *Session) Run(ctx context.Context) error {
	var (
		lastEventID string
		failures    int
		deadline    time.Time
	)

	for {
		stream, err := s.Client.StreamNeoTaskEvents(ctx, s.OrgName, s.TaskID, lastEventID)
		if err != nil {
			// Never-yet-connected or non-transient error: surface the original
			// open failure so e.g. auth/not-found don't get masked by silent retry.
			if lastEventID == "" || !isTransientStreamError(err) {
				return fmt.Errorf("opening event stream: %w", err)
			}
		} else {
			gotEvent, drainErr := s.drainStream(ctx, stream, &lastEventID)
			if gotEvent {
				// Progress means the connection works; reset the budget so a flaky
				// link doesn't burn 5 minutes' worth of attempts on intermittent drops.
				failures = 0
				deadline = time.Time{}
			}
			if drainErr == nil {
				return nil
			}
			if !isTransientStreamError(drainErr) {
				return drainErr
			}
			err = drainErr
		}

		if deadline.IsZero() {
			deadline = time.Now().Add(reconnectTotalBudget)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("event stream reconnect budget exhausted: %w", err)
		}
		failures++
		if !sleepCtx(ctx, backoffDelay(failures)) {
			return nil
		}
	}
}

// drainStream reads events until the channel closes, ctx is cancelled, or an error
// event arrives. It updates *lastEventID for each event that carries an `id:`. The
// first return reports whether any non-error event was delivered.
func (s *Session) drainStream(
	ctx context.Context, stream <-chan client.NeoStreamEvent, lastEventID *string,
) (bool, error) {
	gotEvent := false
	for {
		select {
		case <-ctx.Done():
			return gotEvent, nil
		case evt, ok := <-stream:
			if !ok {
				return gotEvent, nil
			}
			if evt.Err != nil {
				return gotEvent, evt.Err
			}
			if evt.ID != "" {
				*lastEventID = evt.ID
			}
			gotEvent = true
			if err := s.handleEvent(ctx, evt.Data); err != nil {
				return gotEvent, err
			}
		}
	}
}

// isTransientStreamError reports whether err is a transport-level failure worth a
// reconnect attempt. Conservative on purpose: unrecognised errors (handler bugs,
// decode errors) propagate so they aren't masked by silent retries.
func isTransientStreamError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return !errors.Is(ue.Err, context.Canceled)
	}
	var oe *net.OpError
	return errors.As(err, &oe)
}

// backoffDelay returns the wait before the Nth (1-based) reconnect attempt: exponential
// up to reconnectMaxBackoff, plus up to 25% jitter to desynchronise flapping clients.
func backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := reconnectInitialBackoff << (attempt - 1)
	if d <= 0 || d > reconnectMaxBackoff {
		d = reconnectMaxBackoff
	}
	// #nosec G404 -- jitter is a desynchronization signal, not a secret.
	return d + time.Duration(rand.Int64N(int64(d/4)+1))
}

// sleepCtx waits for d or until ctx is cancelled. Returns false if cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
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
		// Handlers may return a partial value alongside an error (e.g. shell
		// timeout). Prefer that value so the agent sees what was captured.
		if value != nil {
			res.Content = value
		} else {
			res.Content = map[string]string{"error": err.Error()}
		}
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
		sendUI(s.UIEvents, UIAssistantMessage{
			Content:           msg.Content,
			IsFinal:           msg.IsFinal,
			HasPendingCLIWork: msg.IsFinal && hasPendingCLIToolCalls(msg.ToolCalls),
		})
		// todo__TodoWrite is cloud-marked, so it never reaches runBatch / the
		// UIToolStarted path — forward the args directly as a UITodoList.
		for _, tc := range msg.ToolCalls {
			if tc.Name != toolNameTodoWrite {
				continue
			}
			if items, ok := parseTodoWriteArgs(tc.Args); ok {
				sendUI(s.UIEvents, UITodoList{Items: items})
			}
		}
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
		var req apitype.AgentBackendEventUserApprovalRequest
		if err := json.Unmarshal(eventBody, &req); err != nil {
			return
		}
		sendUI(s.UIEvents, UIApprovalRequest{
			ApprovalID:      req.ApprovalID,
			Message:         req.Message,
			Sensitivity:     req.Sensitivity,
			ApprovalType:    req.ApprovalType,
			PlanDescription: req.Context.PlanDescription,
			ToolName:        req.Context.ToolName,
		})
	case backendEventAwaitingApprovals:
		sendUI(s.UIEvents, UIAwaitingApprovals{})
	case backendEventContextCompression:
		var c apitype.AgentBackendEventContextCompression
		if err := json.Unmarshal(eventBody, &c); err != nil {
			return
		}
		sendUI(s.UIEvents, UIContextCompression{Status: c.Status})
	case backendEventToolResponse,
		userEventExecToolCall, // server-side echo of a tool running (same discriminator as the CLI-posted event)
		backendEventChangeEntities,
		backendEventSetTaskName:
		// Intentionally ignored: the TUI has no dedicated rendering for these,
		// and the declarative busy rule does not need a heartbeat — the spinner
		// Tick is self-perpetuating while busy, and m.cancelling persists across
		// events until a final one arrives.
	}
}

// forwardUserInputToUI parses a userInput event body and routes it to the TUI:
// user_message → UIUserMessage (echo of a chat message, possibly from another
// client) and user_confirmation → UIApprovalResolved (the cloud's signal that a
// pending approval has been settled, either by the user clicking approve in the
// console or by the auto-approval handler under ApprovalMode=auto/balanced).
func (s *Session) forwardUserInputToUI(eventBody json.RawMessage) {
	if s.UIEvents == nil {
		return
	}

	// Peek at the inner type; we reuse AgentBackendEventHeader because it's just
	// a Type field and the JSON shape on the user-input side matches.
	var head apitype.AgentBackendEventHeader
	if err := json.Unmarshal(eventBody, &head); err != nil {
		return
	}

	switch head.Type {
	case userEventUserMessage:
		var evt apitype.AgentUserEventUserMessage
		if err := json.Unmarshal(eventBody, &evt); err != nil {
			return
		}
		if evt.Content != "" {
			sendUI(s.UIEvents, UIUserMessage{Content: evt.Content})
		}
	case userEventUserConfirmation:
		var evt apitype.AgentUserEventUserConfirmation
		if err := json.Unmarshal(eventBody, &evt); err != nil {
			return
		}
		sendUI(s.UIEvents, UIApprovalResolved{
			ApprovalID: evt.ApprovalID,
			Approved:   evt.Approved,
		})
	}
}
