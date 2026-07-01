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
	acpID string
	// pc creates the Neo task and backs the Session event loop (needs the
	// concrete client). poster is the same client viewed through a narrow
	// interface so user-event posting can be faked in tests; both point at the
	// one cloud client.
	pc           *client.Client
	poster       userEventPoster
	updater      neoTaskUpdater
	orgName      string
	projectName  string
	stackRefName string
	cwd          string
	handlers     map[string]ToolHandler
	client       acp.Client

	mu      sync.Mutex
	taskID  string
	started bool
	// permissionMode and planMode are the config-option selections that feed the
	// Neo task. They are mutated by SetConfigOption and read when the task is
	// created (start) — permissionMode also live-PATCHes a running task, while
	// planMode is fixed once the task exists. Default zero values mean the
	// hardcoded baseline: full-access permissions, plan mode off.
	permissionMode client.NeoPermissionMode
	planMode       bool
	activeTurn     chan turnResult
}

// userEventPoster posts user events (chat messages, approvals, cancels) back to
// a Neo task. *client.Client satisfies it; tests fake it.
type userEventPoster interface {
	PostNeoTaskUserEvent(ctx context.Context, orgName, taskID string, body any) error
}

// neoTaskUpdater PATCHes a live Neo task's mode settings. *client.Client
// satisfies it; tests fake it. Used to push a read-only toggle to a task that
// already exists (SetConfigOption).
type neoTaskUpdater interface {
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
	s.mu.Lock()
	permissionMode := s.permissionMode
	if permissionMode == "" {
		permissionMode = client.NeoPermissionModeDefault
	}
	planMode := s.planMode
	s.mu.Unlock()

	resp, err := createNeoTaskWithEntityRetry(baseCtx, s.pc, s.orgName, prompt, s.stackRefName, s.projectName,
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
		}, nil)
	if err != nil {
		return err
	}

	sessCtx, cancel := context.WithCancel(baseCtx)
	uiCh := make(chan UIEvent, 64)

	s.mu.Lock()
	s.taskID = resp.TaskID
	s.started = true
	s.mu.Unlock()

	session := &Session{
		Client:   s.pc,
		Handlers: s.handlers,
		OrgName:  s.orgName,
		TaskID:   resp.TaskID,
		UIEvents: uiCh,
	}
	// When Run returns (stream ended or ctx cancelled) tear down the derived
	// context so the pump stops too.
	go func() {
		defer cancel()
		_ = session.Run(sessCtx)
	}()
	go s.pump(sessCtx, uiCh)
	return nil
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
	defer q.close()
	go s.drainPumpQueue(ctx, q)

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

// drainPumpQueue performs queued pump actions in order until the queue is closed.
// It is the only place that writes session/update notifications, so a slow editor
// stalls it alone and never the pump that feeds it.
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
	_ = s.poster.PostNeoTaskUserEvent(ctx, s.orgName, taskID, apitype.AgentUserEventUserConfirmation{
		Type:       "user_confirmation",
		ApprovalID: e.ApprovalID,
		Approved:   approved,
	})
}
