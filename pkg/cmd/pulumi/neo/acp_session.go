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
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// acpSession is the per-session state an ACP session carries: the resolved
// Pulumi Cloud client and task target, the working directory, the tool
// handlers, and the editor client used to stream updates. The Neo task and its
// event loop are created lazily on the first prompt (matching the TUI).
type acpSession struct {
	acpID        string
	api          neoSessionAPI
	orgName      string
	projectName  string
	stackRefName string
	cwd          string
	handlers     map[string]ToolHandler
	client       acp.Client

	// startMu serializes task creation (start) with config-option changes
	// (setConfigOption), spanning their network calls. Without it a mode change
	// could land between start reading the modes and the task becoming
	// PATCHable — stored and advertised, but never applied to the task. mu stays
	// the short-lived field lock; never acquire startMu while holding mu.
	startMu sync.Mutex

	mu      sync.Mutex
	taskID  string
	started bool
	// ended records that the Session event loop has exited (runErr says why;
	// nil means a clean stream end or connection teardown). Set by the start
	// goroutine when Run returns. Once ended, new prompts are rejected and the
	// pump's teardown resolves any in-flight turn.
	ended  bool
	runErr error
	// permissionMode and planMode are the config-option selections that feed the
	// Neo task. They are mutated by SetConfigOption and read when the task is
	// created (start) — permissionMode also live-PATCHes a running task, while
	// planMode is fixed once the task exists. Default zero values mean the
	// hardcoded baseline: full-access permissions, plan mode off.
	permissionMode client.NeoPermissionMode
	planMode       bool
	activeTurn     chan turnResult
}

// neoSessionAPI is the slice of the cloud client an ACP session drives its Neo
// task through: creating the task, streaming its events, posting user events
// (chat messages, approvals, cancels), and PATCHing a live task's mode
// settings. *client.Client satisfies it; tests fake it.
type neoSessionAPI interface {
	EventStreamer
	neoTaskCreator
	UpdateNeoTask(ctx context.Context, orgName, taskID string, opts client.UpdateNeoTaskOptions) error
}

// turnResult is how the event pump signals the waiting Prompt call that the
// current turn ended: with a stop reason, or a fatal error.
type turnResult struct {
	reason acp.StopReason
	err    error
}

// start creates the Neo task with the first prompt as the initial message and
// launches the long-lived Session event loop plus the translation pump. Both run
// on a context derived from the connection lifetime so they survive past the
// prompt request that started them.
func (s *acpSession) start(baseCtx context.Context, prompt string) error {
	// Hold startMu across the whole creation window so a concurrent
	// setConfigOption either happens-before (its values are read below) or
	// happens-after (it sees started and PATCHes the live task).
	s.startMu.Lock()
	defer s.startMu.Unlock()

	s.mu.Lock()
	permissionMode := s.permissionMode
	if permissionMode == "" {
		permissionMode = client.NeoPermissionModeDefault
	}
	planMode := s.planMode
	s.mu.Unlock()

	// If the backend drops the attached stack (createNeoTaskWithEntityRetry's
	// fallback), collect the warning here — uiCh doesn't exist yet — and queue
	// it below so it reaches the editor, mirroring the TUI.
	var warnings []string
	resp, err := createNeoTaskWithEntityRetry(baseCtx, s.api, s.orgName, prompt, s.stackRefName, s.projectName,
		client.CreateNeoTaskOptions{
			ToolExecutionMode: "cli",
			// Manual approval routes every gated tool call to the editor as an ACP
			// permission request; balanced/auto would resolve them server-side and
			// bypass the editor. It is fixed (not a config option) for that reason.
			ApprovalMode: client.NeoApprovalModeManual,
			// PermissionMode (read-only) and PlanMode come from the editor's config
			// option selections; see the `permission` and `plan` ACP config options.
			PermissionMode: permissionMode,
			PlanMode:       planMode,
		}, func(originalErr error) {
			warnings = append(warnings,
				entityDroppedWarning(s.orgName, s.projectName, s.stackRefName, originalErr))
		})
	if err != nil {
		return err
	}

	sessCtx, cancel := context.WithCancel(baseCtx)
	uiCh := make(chan UIEvent, 64)
	// Queue creation-time warnings ahead of any stream events so the pump
	// forwards them to the editor first.
	for _, w := range warnings {
		sendUI(uiCh, UIWarning{Message: w})
	}

	s.mu.Lock()
	s.taskID = resp.TaskID
	s.started = true
	s.mu.Unlock()

	session := &Session{
		Client:   s.api,
		Handlers: s.handlers,
		OrgName:  s.orgName,
		TaskID:   resp.TaskID,
		UIEvents: uiCh,
	}
	// When Run returns (stream ended, failed, or ctx cancelled), record that the
	// event loop is gone — new prompts are rejected from then on — and tear down
	// the derived context so the pump stops too and resolves any in-flight turn
	// (see pump's teardown).
	go func() {
		defer cancel()
		err := session.Run(sessCtx)
		s.mu.Lock()
		s.ended, s.runErr = true, err
		s.mu.Unlock()
	}()
	go s.pump(sessCtx, uiCh)
	return nil
}

// runTurn runs one prompt turn: on the first prompt it creates the Neo task and
// starts the event loop (start); on later prompts it posts the user's message.
// Either way it blocks until the turn ends (a final assistant message,
// cancellation, or a fatal error) and reports the stop reason. baseCtx is the
// connection-lifetime context the session's background loops run on; ctx is the
// prompt request's own context.
func (s *acpSession) runTurn(ctx, baseCtx context.Context, text string) (acp.PromptResult, error) {
	// Register this turn before any work so the pump can signal it the moment
	// the turn ends, even for a very fast turn. ACP drives one prompt at a time
	// per session; reject an overlapping prompt rather than overwrite activeTurn,
	// which would orphan the prior waiter (the pump only ever signals the latest).
	done := make(chan turnResult, 1)
	s.mu.Lock()
	if s.ended {
		// The Session event loop is gone (stream failure or clean end); a new
		// turn could never complete, so fail it up front.
		s.mu.Unlock()
		return acp.PromptResult{}, s.endedError()
	}
	if s.activeTurn != nil {
		s.mu.Unlock()
		return acp.PromptResult{}, fmt.Errorf("a prompt turn is already in progress for session %q", s.acpID)
	}
	s.activeTurn = done
	needStart := !s.started
	taskID := s.taskID
	s.mu.Unlock()

	var startErr error
	if needStart {
		startErr = s.start(baseCtx, text)
	} else {
		startErr = s.api.PostNeoTaskUserEvent(ctx, s.orgName, taskID,
			apitype.AgentUserEventUserMessage{Type: "user_message", Content: text})
	}
	if startErr != nil {
		s.mu.Lock()
		s.activeTurn = nil
		s.mu.Unlock()
		return acp.PromptResult{}, startErr
	}

	select {
	case <-ctx.Done():
		return acp.PromptResult{}, ctx.Err()
	case tr := <-done:
		if tr.err != nil {
			return acp.PromptResult{}, tr.err
		}
		return acp.PromptResult{StopReason: tr.reason}, nil
	}
}

// cancel posts a cancel user event for the session's task, if one is running.
// The turn ends with StopCancelled once the backend acknowledges.
func (s *acpSession) cancel(ctx context.Context) error {
	s.mu.Lock()
	started, taskID := s.started, s.taskID
	s.mu.Unlock()
	if !started || taskID == "" {
		return nil
	}
	return s.api.PostNeoTaskUserEvent(ctx, s.orgName, taskID, apitype.AgentUserEventCancel{Type: "user_cancel"})
}

// endedError describes why the session can no longer run turns: the event loop
// exited with runErr, or ended without error (a clean stream end or connection
// teardown).
func (s *acpSession) endedError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runErr != nil {
		return fmt.Errorf("Neo session ended: %w", s.runErr)
	}
	return errors.New("Neo session has ended")
}

// pump translates Session UIEvents into ordered actions (session/update
// notifications, permission requests, turn resolutions) and hands them to a
// dedicated writer goroutine. It runs until the session context is cancelled or
// the event channel closes.
//
// Draining uiCh and writing to the editor are deliberately decoupled. uiCh is a
// best-effort channel: Session.sendUI drops events when it is full (fine for the
// TUI, which only renders them). But the pump also derives the turn-boundary
// signals that resolve a waiting Prompt from this stream, and those must not be
// lost. If the pump wrote session/update notifications inline it would block on
// a slow editor stdout, let uiCh back up, and a dropped boundary event would
// hang Prompt until cancellation. Routing everything through an unbounded,
// ordered queue lets the pump keep draining uiCh at all times, while preserving
// the order the agent produced events (so updates still precede the turn result).
func (s *acpSession) pump(ctx context.Context, uiCh <-chan UIEvent) {
	q := newPumpQueue()
	go s.drainPumpQueue(ctx, q)
	defer func() {
		// Teardown must resolve a waiting Prompt: the event loop is gone, so no
		// turn-boundary event will ever arrive. Queue a final turn result — a
		// no-op if the turn already resolved (finishTurn without a waiter does
		// nothing) — then close the queue, which still delivers everything
		// queued ahead of it before the drain stops.
		q.push(pumpAction{finish: &turnResult{err: s.endedError()}})
		q.close()
	}()

	tracker := toolTracker{cwd: s.cwd}
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-uiCh:
			if !ok {
				return
			}
			switch e := evt.(type) {
			case UIApprovalRequest:
				req := e
				q.push(pumpAction{approval: &req})
			case UICancelled:
				q.push(pumpAction{finish: &turnResult{reason: acp.StopCancelled}})
			case UIError:
				q.push(pumpAction{finish: &turnResult{err: errors.New(e.Message)}})
			case UITaskIdle:
				q.push(pumpAction{finish: &turnResult{reason: acp.StopEndTurn}})
			default:
				if update, send := tracker.translate(evt); send {
					q.push(pumpAction{notify: update})
				}
			}
		}
	}
}

// drainPumpQueue performs queued pump actions in order until the queue is closed
// and drained. It is the only writer of stream-derived session/update
// notifications, so a slow editor stalls it alone and never the pump that feeds
// it. (requestPermission — already off this goroutine — sends its
// config_option_update directly: that update is ordered by the permission
// round-trip it follows, not by the event stream, and jsonrpc2 serializes
// concurrent writes.)
func (s *acpSession) drainPumpQueue(ctx context.Context, q *pumpQueue) {
	for {
		a, ok := q.pop()
		if !ok {
			return
		}
		switch {
		case a.notify != nil:
			s.notify(ctx, a.notify)
		case a.approval != nil:
			// Run the editor round-trip off the writer so later actions keep
			// flowing while the user decides.
			go s.requestPermission(ctx, *a.approval)
		case a.finish != nil:
			s.finishTurn(*a.finish)
		}
	}
}

// notify sends a session/update notification for this session, dropping errors:
// a failed notification means the editor has gone away, which the connection
// teardown handles.
func (s *acpSession) notify(ctx context.Context, update acp.SessionUpdate) {
	_ = s.client.Notify(ctx, "session/update", acp.SessionNotification{SessionID: s.acpID, Update: update})
}

// finishTurn delivers the turn's outcome to the waiting Prompt call, if any. The
// channel is buffered so this never blocks the pump.
func (s *acpSession) finishTurn(tr turnResult) {
	s.mu.Lock()
	ch := s.activeTurn
	s.activeTurn = nil
	s.mu.Unlock()
	if ch != nil {
		ch <- tr
	}
}

// requestPermission forwards a Neo approval request to the editor as an ACP
// permission request and relays the decision back to the backend as a user
// confirmation. A failed/denied request is treated as a rejection.
func (s *acpSession) requestPermission(ctx context.Context, e UIApprovalRequest) {
	title := e.Message
	if title == "" {
		title = e.ToolName
	}
	var res acp.RequestPermissionResult
	approved := false
	if err := s.client.Call(ctx, "session/request_permission", acp.RequestPermissionParams{
		SessionID: s.acpID,
		ToolCall:  acp.PermissionToolCall{ToolCallID: e.ApprovalID, Title: title},
		Options:   acp.ApprovalOptions(),
	}, &res); err == nil {
		approved = res.Approved()
	}

	// Approving an exit_plan_mode request exits plan mode server-side (the
	// PlanModeTracker clears in lockstep). Reflect that to the editor with a
	// config_option_update so its plan-mode indicator follows along.
	if approved && e.ApprovalType == approvalTypePlanExit && s.noteExitedPlanMode() {
		s.notify(ctx, acp.ConfigOptionUpdate{ConfigOptions: s.configOptionsSnapshot()})
	}

	s.mu.Lock()
	taskID := s.taskID
	s.mu.Unlock()
	_ = s.api.PostNeoTaskUserEvent(ctx, s.orgName, taskID, apitype.AgentUserEventUserConfirmation{
		Type:       "user_confirmation",
		ApprovalID: e.ApprovalID,
		Approved:   approved,
	})
}
