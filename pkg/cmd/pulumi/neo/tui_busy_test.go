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
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// TestIsFinalUIEvent_MirrorsBackendRule — the TUI-side classifier needs to
// stay in lockstep with isFinalBackendEvent, including the CLI-tool-calls
// exception.
func TestIsFinalUIEvent_MirrorsBackendRule(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ev   UIEvent
		want bool
	}{
		{"assistant_message_streaming", UIAssistantMessage{IsFinal: false}, false},
		{"assistant_message_final_no_cli_work", UIAssistantMessage{IsFinal: true}, true},
		{"assistant_message_final_with_cli_work", UIAssistantMessage{IsFinal: true, HasPendingCLIWork: true}, false},
		{"approval_request_final", UIApprovalRequest{}, true},
		{"cancelled_final", UICancelled{}, true},
		{"error_final", UIError{}, true},
		{"task_idle_final", UITaskIdle{}, true},
		{"tool_started_non_final", UIToolStarted{Name: "x__y"}, false},
		{"tool_progress_non_final", UIToolProgress{Name: "x__y"}, false},
		{"tool_completed_non_final", UIToolCompleted{Name: "x__y"}, false},
		{"warning_non_final", UIWarning{Message: "careful"}, false},
		{"user_message_non_final", UIUserMessage{Content: "hi"}, false},
		{"awaiting_approvals_non_final", UIAwaitingApprovals{}, false},
		{"context_compression_non_final", UIContextCompression{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isFinalUIEvent(tc.ev))
		})
	}
}

// TestBusy_NoFlickerOnCLIToolHandoff is the key regression test the declarative
// busy rule was written to pass. Under the old imperative rule, an
// assistant_message with is_final=true and queued CLI tool calls hid the
// spinner before the first UIToolStarted arrived. Walk the full golden
// sequence (submit → hand-off → tool × 2 → truly-final) and assert the
// spinner stays on throughout, only flipping off at the end.
func TestBusy_NoFlickerOnCLIToolHandoff(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 16)
	model := tea.Model(NewModel(ModelConfig{EventCh: ch, Busy: true}))
	require.True(t, model.(Model).busy, "NewModel with Busy:true must seed busy state")

	// Step 1: the agent's first message is the CLI-work hand-off. is_final is
	// true but there's a queued CLI tool call — spinner must stay on.
	model, _ = model.Update(UIAssistantMessage{IsFinal: true, HasPendingCLIWork: true, Content: ""})
	assert.True(t, model.(Model).busy, "hand-off assistant_message with pending CLI work must leave spinner on")

	// Step 2: session starts running the first tool. Label switches to a tool label.
	model, _ = model.Update(UIToolStarted{Name: "filesystem__read", Args: json.RawMessage(`{"file_path":"/x"}`)})
	assert.True(t, model.(Model).busy)

	// Step 3: progress update from the tool.
	model, _ = model.Update(UIToolProgress{Name: "filesystem__read", Message: "reading"})
	assert.True(t, model.(Model).busy)

	// Step 4: first tool completes; inter-tool gap shows the thinking label.
	model, _ = model.Update(UIToolCompleted{Name: "filesystem__read", Args: json.RawMessage(`{"file_path":"/x"}`)})
	assert.True(t, model.(Model).busy)

	// Step 5: agent decides to run a second tool — another hand-off arrives first.
	model, _ = model.Update(UIAssistantMessage{IsFinal: true, HasPendingCLIWork: true, Content: ""})
	assert.True(t, model.(Model).busy, "second hand-off must not clear busy either")

	model, _ = model.Update(UIToolStarted{Name: "filesystem__write", Args: json.RawMessage(`{"file_path":"/y"}`)})
	assert.True(t, model.(Model).busy)

	model, _ = model.Update(UIToolCompleted{Name: "filesystem__write", Args: json.RawMessage(`{"file_path":"/y"}`)})
	assert.True(t, model.(Model).busy)

	// Step 6: agent's truly-final message — no pending CLI work. Spinner drops.
	model, _ = model.Update(UIAssistantMessage{IsFinal: true, HasPendingCLIWork: false, Content: "done"})
	m := model.(Model)
	assert.False(t, m.busy, "truly-final assistant_message clears busy")
	assert.Equal(t, -1, m.findBlockKind(blockBusy))
	assert.GreaterOrEqual(t, m.findBlockKind(blockAssistantFinal), 0)
}

// TestBusy_NonFinalStreamingKeepsSpinnerOn — under the old rule, a non-final
// assistant_message explicitly removed the busy block. The declarative rule
// keeps the spinner on throughout streaming so there is no gap before the
// next tool or the truly-final message.
func TestBusy_NonFinalStreamingKeepsSpinnerOn(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	model := tea.Model(NewModel(ModelConfig{EventCh: ch, Busy: true}))

	model, _ = model.Update(UIAssistantMessage{Content: "thinking...", IsFinal: false})
	assert.True(t, model.(Model).busy, "non-final assistant_message must leave spinner on")
	m := model.(Model)
	assert.GreaterOrEqual(t, m.findBlockKind(blockAssistantStreaming), 0, "streaming content renders as a streaming block")
	assert.GreaterOrEqual(t, m.findBlockKind(blockBusy), 0, "spinner block is still present")

	// More streaming text — still non-final, still busy.
	model, _ = model.Update(UIAssistantMessage{Content: "thinking harder", IsFinal: false})
	assert.True(t, model.(Model).busy)

	// Truly final — spinner off.
	model, _ = model.Update(UIAssistantMessage{Content: "thinking harder. Done.", IsFinal: true})
	assert.False(t, model.(Model).busy)
}

// TestBusy_CancellingSubstate — pressing ESC while busy posts user_cancel
// upstream, sets the cancelling flag, and rewrites the busy label to
// "Cancelling...". The flag clears on the next final event (here:
// UICancelled).
func TestBusy_CancellingSubstate(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	outCh := make(chan outboundEvent, 4)
	model := tea.Model(NewModel(ModelConfig{EventCh: ch, OutCh: outCh, Busy: true}))

	// Press ESC mid-turn.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := model.(Model)
	assert.True(t, m.cancelling, "ESC must set the cancelling flag")
	assert.True(t, m.busy, "spinner stays on while we wait for the backend to confirm")

	// user_cancel was posted upstream.
	select {
	case ev := <-outCh:
		c, ok := ev.event.(apitype.AgentUserEventCancel)
		require.True(t, ok, "ESC must post an AgentUserEventCancel")
		assert.Equal(t, userEventUserCancel, c.Type)
	default:
		t.Fatal("ESC did not post any user event")
	}

	// Busy block's label reflects the cancelling state.
	idx := m.findBlockKind(blockBusy)
	require.NotEqual(t, -1, idx)
	assert.Equal(t, "Cancelling...", m.blocks[idx].label)

	// Any non-final event while cancelling keeps the cancelling label, not the
	// thinking label or a tool label.
	model, _ = model.Update(UIToolStarted{Name: "filesystem__read"})
	m = model.(Model)
	idx = m.findBlockKind(blockBusy)
	require.NotEqual(t, -1, idx)
	assert.Equal(t, "Cancelling...", m.blocks[idx].label, "cancelling overrides tool labels too")
	assert.True(t, m.cancelling, "still cancelling until a final event arrives")

	// Backend acknowledges the cancel → spinner off, cancelling flag clears.
	model, _ = model.Update(UICancelled{})
	m = model.(Model)
	assert.False(t, m.busy)
	assert.False(t, m.cancelling, "cancelling must clear on the final event")
	assert.Equal(t, -1, m.findBlockKind(blockBusy))
}

// TestBusy_EscIgnoredWhenIdle — ESC while the TUI is idle is a no-op. We
// don't want to post a spurious user_cancel.
func TestBusy_EscIgnoredWhenIdle(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	outCh := make(chan outboundEvent, 4)
	model := tea.Model(NewModel(ModelConfig{EventCh: ch, OutCh: outCh}))
	assert.False(t, model.(Model).busy)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := model.(Model)
	assert.False(t, m.cancelling)
	assert.False(t, m.busy)

	select {
	case <-outCh:
		t.Fatal("ESC when idle must not post a user event")
	default:
	}
}

// TestBusy_EscIgnoredWhileApprovalPending — the agent is already paused
// waiting for us during an approval; ESC should be a no-op here.
func TestBusy_EscIgnoredWhileApprovalPending(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	outCh := make(chan outboundEvent, 4)
	model := tea.Model(NewModel(ModelConfig{EventCh: ch, OutCh: outCh, Busy: true}))

	// Simulate an approval request arriving.
	model, _ = model.Update(UIApprovalRequest{ApprovalID: "a1", Message: "ok?"})
	require.True(t, model.(Model).pendingApproval)

	// ESC must not post anything.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.(Model).cancelling)
	select {
	case <-outCh:
		t.Fatal("ESC during pending approval must not post a user event")
	default:
	}
}

// TestBusy_UnopinionatedEventsWhenIdle — warnings, session URLs, foreign
// user messages arriving while the TUI is idle must NOT spin up the
// indicator. These events fall through labelForUIEvent's default branch;
// under an earlier draft of the rule they would have triggered a
// "Thinking..." spin, which is wrong when no task is running.
func TestBusy_UnopinionatedEventsWhenIdle(t *testing.T) {
	t.Parallel()

	for _, ev := range []UIEvent{
		UIWarning{Message: "careful"},
		UISessionURL{URL: "https://example.invalid"},
		UIUserMessage{Content: "from another client"},
	} {
		ch := make(chan UIEvent, 4)
		model := tea.Model(NewModel(ModelConfig{EventCh: ch}))
		model, _ = model.Update(ev)
		assert.False(t, model.(Model).busy, "event %T must not start the spinner when idle", ev)
	}
}
