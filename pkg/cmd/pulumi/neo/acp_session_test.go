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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// capturedNotif is one session/update the fake client received.
type capturedNotif struct {
	method string
	params any
}

// fakeACPClient implements acp.Client, recording notifications and serving a
// canned permission outcome for Call.
type fakeACPClient struct {
	mu            sync.Mutex
	notifications []capturedNotif
	callMethod    string
	callParams    any
	permResult    acp.RequestPermissionResult
	callErr       error
}

func (c *fakeACPClient) Notify(_ context.Context, method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, capturedNotif{method: method, params: params})
	return nil
}

func (c *fakeACPClient) Call(_ context.Context, method string, params, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callMethod, c.callParams = method, params
	if c.callErr != nil {
		return c.callErr
	}
	if r, ok := result.(*acp.RequestPermissionResult); ok {
		*r = c.permResult
	}
	return nil
}

func TestPumpForwardsUpdatesAndEndsTurn(t *testing.T) {
	t.Parallel()

	fc := &fakeACPClient{}
	done := make(chan turnResult, 1)
	s := &acpSession{acpID: "sess_x", client: fc, activeTurn: done}
	uiCh := make(chan UIEvent, 8)
	go s.pump(t.Context(), uiCh)

	uiCh <- UIAssistantMessage{Content: "hello"}
	uiCh <- UIToolStarted{Name: "shell__exec"}
	uiCh <- UITaskIdle{}

	select {
	case tr := <-done:
		require.NoError(t, tr.err)
		assert.Equal(t, acp.StopEndTurn, tr.reason)
	case <-time.After(2 * time.Second):
		t.Fatal("turn did not finish")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.Len(t, fc.notifications, 2)
	for _, n := range fc.notifications {
		assert.Equal(t, "session/update", n.method)
	}
}

// gatedClient is an acp.Client whose Notify blocks until gate is closed,
// simulating an editor that is slow to read stdout. It counts completed
// notifications so a test can assert none were lost.
type gatedClient struct {
	gate chan struct{}
	mu   sync.Mutex
	n    int
}

func (c *gatedClient) Notify(_ context.Context, _ string, _ any) error {
	<-c.gate
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil
}

func (c *gatedClient) Call(context.Context, string, any, any) error { return nil }

func (c *gatedClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// TestPumpKeepsDrainingWhileEditorIsSlow is the regression guard for the
// decoupling between draining UIEvents and writing to the editor. A slow editor
// (Notify blocked) must not stall the pump: it keeps accepting events into an
// unbounded queue, so Session.sendUI never fills uiCh and never drops a
// turn-boundary event. The turn still resolves only after the editor drains its
// updates, and no notification is lost.
func TestPumpKeepsDrainingWhileEditorIsSlow(t *testing.T) {
	t.Parallel()

	fc := &gatedClient{gate: make(chan struct{})}
	done := make(chan turnResult, 1)
	s := &acpSession{acpID: "sess_x", client: fc, activeTurn: done}

	// Unbuffered: a successful send proves the pump actually received the event
	// rather than it sitting in a channel buffer.
	uiCh := make(chan UIEvent)
	go s.pump(t.Context(), uiCh)

	const n = 100
	for i := range n {
		select {
		case uiCh <- UIAssistantMessage{Content: "x"}:
		case <-time.After(2 * time.Second):
			t.Fatalf("pump stopped draining at event %d while the editor was slow", i)
		}
	}
	uiCh <- UITaskIdle{}

	// The turn can't end before its updates reach the (still-blocked) editor.
	select {
	case <-done:
		t.Fatal("turn resolved before the editor drained its updates")
	case <-time.After(50 * time.Millisecond):
	}

	close(fc.gate) // editor catches up
	select {
	case tr := <-done:
		require.NoError(t, tr.err)
		assert.Equal(t, acp.StopEndTurn, tr.reason)
	case <-time.After(2 * time.Second):
		t.Fatal("turn did not finish after the editor caught up")
	}
	assert.Equal(t, n, fc.count(), "every notification should be delivered, none dropped")
}

func TestPumpBoundaryReasons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event UIEvent
		check func(t *testing.T, tr turnResult)
	}{
		{"cancelled", UICancelled{}, func(t *testing.T, tr turnResult) {
			require.NoError(t, tr.err)
			assert.Equal(t, acp.StopCancelled, tr.reason)
		}},
		{"error", UIError{Message: "boom"}, func(t *testing.T, tr turnResult) {
			require.Error(t, tr.err)
			assert.Contains(t, tr.err.Error(), "boom")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			done := make(chan turnResult, 1)
			s := &acpSession{acpID: "sess_x", client: &fakeACPClient{}, activeTurn: done}
			uiCh := make(chan UIEvent, 1)
			go s.pump(t.Context(), uiCh)
			uiCh <- tt.event
			select {
			case tr := <-done:
				tt.check(t, tr)
			case <-time.After(2 * time.Second):
				t.Fatal("turn did not finish")
			}
		})
	}
}

// TestPumpTeardownResolvesActiveTurn is the regression guard for a dying event
// loop: when Session.Run exits without ever emitting a turn-boundary event
// (e.g. the stream failed), the pump's teardown must still resolve the waiting
// Prompt — with the loop's error — instead of leaving it blocked forever.
func TestPumpTeardownResolvesActiveTurn(t *testing.T) {
	t.Parallel()

	fc := &fakeACPClient{}
	done := make(chan turnResult, 1)
	s := &acpSession{acpID: "sess_x", client: fc, activeTurn: done}
	ctx, cancel := context.WithCancel(t.Context())
	uiCh := make(chan UIEvent)
	go s.pump(ctx, uiCh)

	// Simulate the Run goroutine: record why the loop died, then cancel the
	// session context, exactly in that order.
	s.mu.Lock()
	s.ended, s.runErr = true, errors.New("event stream lost")
	s.mu.Unlock()
	cancel()

	select {
	case tr := <-done:
		require.Error(t, tr.err)
		assert.ErrorContains(t, tr.err, "event stream lost")
	case <-time.After(2 * time.Second):
		t.Fatal("session teardown did not resolve the waiting turn")
	}
}

// TestRunTurnFirstPromptCreatesTaskAndCompletes drives a whole first turn
// through the real start path against a fake backend: the task is created with
// the session's configured modes, the event loop connects to the scripted
// stream, the assistant's message reaches the editor as a session/update, and
// the final message resolves the turn with end_turn.
func TestRunTurnFirstPromptCreatesTaskAndCompletes(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{stream: make(chan client.NeoStreamEvent, 8)}
	fc := &fakeACPClient{}
	s := &acpSession{
		acpID:          "sess_x",
		api:            fp,
		orgName:        "acme",
		projectName:    "proj",
		stackRefName:   "dev",
		client:         fc,
		handlers:       map[string]ToolHandler{},
		baseCtx:        t.Context(),
		permissionMode: client.NeoPermissionModeReadOnly,
		planMode:       true,
	}
	// Queue the turn-ending final assistant message so the event loop finds it
	// as soon as it connects.
	fp.stream <- client.NeoStreamEvent{Data: mustAgentResponseEnvelope(t, apitype.AgentBackendEventAssistantMessage{
		Type:    backendEventAssistantMessage,
		Content: "done",
		IsFinal: true,
	})}

	res, err := s.runTurn(t.Context(), "build it")
	require.NoError(t, err)
	assert.Equal(t, acp.StopEndTurn, res.StopReason)

	fp.mu.Lock()
	require.Len(t, fp.created, 1)
	created := fp.created[0]
	fp.mu.Unlock()
	assert.Equal(t, "build it", created.content)
	assert.Equal(t, "dev", created.stackName)
	assert.Equal(t, "proj", created.projectName)
	assert.Equal(t, "cli", created.opts.ToolExecutionMode)
	assert.Equal(t, client.NeoApprovalModeManual, created.opts.ApprovalMode,
		"approval stays manual so gated calls surface as editor permission requests")
	assert.Equal(t, client.NeoPermissionModeReadOnly, created.opts.PermissionMode)
	assert.True(t, created.opts.PlanMode)

	s.mu.Lock()
	assert.True(t, s.started)
	assert.Equal(t, "task_1", s.taskID)
	s.mu.Unlock()

	// The assistant's message was written to the editor before the turn
	// resolved (the pump orders updates ahead of the turn result).
	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.NotEmpty(t, fc.notifications)
	assert.Equal(t, "session/update", fc.notifications[0].method)
}

// TestStartDefaultsModes verifies the hardcoded baseline reaches task creation
// when the editor never changed a config option: full-access permissions, plan
// mode off.
func TestStartDefaultsModes(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{}
	s := &acpSession{acpID: "sess_x", api: fp, orgName: "acme", client: &fakeACPClient{}, baseCtx: t.Context()}

	require.NoError(t, s.start("hello"))

	fp.mu.Lock()
	defer fp.mu.Unlock()
	require.Len(t, fp.created, 1)
	assert.Equal(t, client.NeoPermissionModeDefault, fp.created[0].opts.PermissionMode)
	assert.False(t, fp.created[0].opts.PlanMode)
}

// TestRunTurnCreateErrorReleasesTurn: a failed task creation must propagate the
// error, leave the session unstarted, and release the turn slot so the editor
// can retry the prompt.
func TestRunTurnCreateErrorReleasesTurn(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{createErr: errors.New("task quota exceeded")}
	s := &acpSession{acpID: "sess_x", api: fp, orgName: "acme", client: &fakeACPClient{}, baseCtx: t.Context()}

	_, err := s.runTurn(t.Context(), "hello")
	require.ErrorContains(t, err, "task quota exceeded")

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.False(t, s.started, "a failed creation must not mark the session started")
	assert.Nil(t, s.activeTurn, "a failed start must release the turn slot for a retry")
}

// TestRunTurnPostErrorReleasesTurn: on a later prompt, a failed user_message
// post must propagate the error and release the turn slot so the editor can
// retry — the documented cleanup in runTurn's start-error path.
func TestRunTurnPostErrorReleasesTurn(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{postErr: errors.New("backend rejected the message")}
	s := &acpSession{
		acpID: "sess_x", api: fp, orgName: "acme", client: &fakeACPClient{},
		started: true, taskID: "task_9",
	}

	_, err := s.runTurn(t.Context(), "again")
	require.ErrorContains(t, err, "backend rejected the message")

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Nil(t, s.activeTurn, "a failed post must release the turn slot for a retry")
}

// TestRunTurnSurvivesEventLoopPanic: a panic anywhere on the session event
// loop goroutine (here: while connecting the event stream, but tool handlers
// run there too) is recovered into runErr instead of crashing the process —
// the waiting turn resolves with the panic error and later prompts fail fast
// with the same cause.
func TestRunTurnSurvivesEventLoopPanic(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{streamPanic: true}
	s := &acpSession{acpID: "sess_x", api: fp, orgName: "acme", client: &fakeACPClient{}, baseCtx: t.Context()}

	_, err := s.runTurn(t.Context(), "hello")
	require.ErrorContains(t, err, "panic in Neo session event loop")
	require.ErrorContains(t, err, "stream exploded")

	_, err = s.runTurn(t.Context(), "again")
	require.ErrorContains(t, err, "panic in Neo session event loop")
}

// TestRunTurnConcurrentPromptsExactlyOneWins races two prompts on a started
// session: exactly one must register the turn (and complete once the pump
// resolves it), the other must be rejected without posting to the backend.
func TestRunTurnConcurrentPromptsExactlyOneWins(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{}
	s := &acpSession{
		acpID: "sess_x", api: fp, orgName: "acme", client: &fakeACPClient{},
		started: true, taskID: "task_9",
	}

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := s.runTurn(t.Context(), "hi")
			errs <- err
		}()
	}

	// The loser fails fast; the winner blocks until the turn is resolved, so the
	// first error to arrive must be the rejection.
	select {
	case err := <-errs:
		require.ErrorContains(t, err, "already in progress")
	case <-time.After(2 * time.Second):
		t.Fatal("neither prompt returned; expected one to be rejected immediately")
	}

	// Resolve the winner's turn the way the pump would, once its post landed.
	require.Eventually(t, func() bool {
		fp.mu.Lock()
		defer fp.mu.Unlock()
		return len(fp.posted) == 1
	}, 2*time.Second, 5*time.Millisecond, "the winning prompt should post exactly one user_message")
	s.finishTurn(turnResult{reason: acp.StopEndTurn})

	select {
	case err := <-errs:
		require.NoError(t, err, "the winning prompt should complete once the turn resolves")
	case <-time.After(2 * time.Second):
		t.Fatal("winning prompt did not resolve after finishTurn")
	}
}

// TestStartEntityDroppedWarningReachesEditor: when the backend rejects the
// attached stack and task creation falls back to no stack context, the editor
// must be told — as a message-stream warning ahead of any stream events.
func TestStartEntityDroppedWarningReachesEditor(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{rejectStack: true}
	fc := &fakeACPClient{}
	s := &acpSession{
		acpID: "sess_x", api: fp, orgName: "acme", projectName: "proj", stackRefName: "dev",
		client: fc, handlers: map[string]ToolHandler{}, baseCtx: t.Context(),
	}

	require.NoError(t, s.start("hello"))

	// The task was still created, just without the stack attached.
	fp.mu.Lock()
	require.Len(t, fp.created, 1)
	assert.Empty(t, fp.created[0].stackName)
	fp.mu.Unlock()

	require.Eventually(t, func() bool {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		for _, n := range fc.notifications {
			sn, ok := n.params.(acp.SessionNotification)
			if !ok {
				continue
			}
			if chunk, ok := sn.Update.(acp.AgentMessageChunk); ok &&
				strings.Contains(chunk.Content.Text, "could not attach stack acme/proj/dev") {
				return true
			}
		}
		return false
	}, 2*time.Second, 5*time.Millisecond, "the stack-dropped warning should reach the editor")
}

// TestRunTurnLaterPromptPostsUserMessage: once the task exists, a prompt posts a
// user_message instead of creating a task, and resolves when the turn ends.
func TestRunTurnLaterPromptPostsUserMessage(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{}
	s := &acpSession{
		acpID: "sess_x", api: fp, orgName: "acme", client: &fakeACPClient{},
		started: true, taskID: "task_9",
	}

	turnDone := make(chan struct{})
	var res acp.PromptResult
	var err error
	go func() {
		defer close(turnDone)
		res, err = s.runTurn(t.Context(), "again")
	}()

	// Wait for the post, then resolve the turn the way the pump would.
	require.Eventually(t, func() bool {
		fp.mu.Lock()
		defer fp.mu.Unlock()
		return len(fp.posted) == 1
	}, 2*time.Second, 5*time.Millisecond, "second prompt should post a user_message")
	s.finishTurn(turnResult{reason: acp.StopEndTurn})
	<-turnDone

	require.NoError(t, err)
	assert.Equal(t, acp.StopEndTurn, res.StopReason)

	fp.mu.Lock()
	defer fp.mu.Unlock()
	assert.Empty(t, fp.created, "no second task is created")
	msg, ok := fp.posted[0].(apitype.AgentUserEventUserMessage)
	require.True(t, ok)
	assert.Equal(t, "user_message", msg.Type)
	assert.Equal(t, "again", msg.Content)
}

func TestRequestPermissionRelaysDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		outcome      acp.PermissionOutcome
		callErr      error
		wantApproved bool
	}{
		{"allow", acp.PermissionOutcome{Outcome: "selected", OptionID: "allow"}, nil, true},
		{"reject", acp.PermissionOutcome{Outcome: "selected", OptionID: "reject"}, nil, false},
		{"cancelled", acp.PermissionOutcome{Outcome: "cancelled"}, nil, false},
		{"call error", acp.PermissionOutcome{}, errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fc := &fakeACPClient{permResult: acp.RequestPermissionResult{Outcome: tt.outcome}, callErr: tt.callErr}
			fp := &fakeTaskAPI{}
			s := &acpSession{acpID: "sess_x", client: fc, api: fp, orgName: "acme", taskID: "task_1"}

			s.requestPermission(t.Context(), UIApprovalRequest{ApprovalID: "appr_1", Message: "run it?"})

			fc.mu.Lock()
			assert.Equal(t, "session/request_permission", fc.callMethod)
			fc.mu.Unlock()

			fp.mu.Lock()
			defer fp.mu.Unlock()
			require.Len(t, fp.posted, 1)
			conf, ok := fp.posted[0].(apitype.AgentUserEventUserConfirmation)
			require.True(t, ok)
			assert.Equal(t, "appr_1", conf.ApprovalID)
			assert.Equal(t, tt.wantApproved, conf.Approved)
		})
	}
}
