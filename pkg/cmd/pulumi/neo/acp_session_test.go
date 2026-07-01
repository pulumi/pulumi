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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
			fp := &fakePoster{}
			s := &acpSession{acpID: "sess_x", client: fc, poster: fp, orgName: "acme", taskID: "task_1"}

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
