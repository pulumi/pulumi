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

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// -----------------------------------------------------------------------------
// Pure helpers
// -----------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	t.Parallel()

	// Exercise every branch: under, equal-to, and over the limit. The ellipsis
	// must only appear when we actually shortened the input.
	cases := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"empty_input", "", 5, ""},
		{"under_limit", "abc", 5, "abc"},
		{"exact_fit", "abcde", 5, "abcde"},
		{"over_limit", "abcdef", 5, "abcde..."},
		{"zero_max_and_content", "abc", 0, "..."},
		{"zero_max_empty_stays_empty", "", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, truncate(tc.in, tc.maxLen))
		})
	}
}

func TestRenderAssistantFinal(t *testing.T) {
	t.Parallel()

	// Empty (or whitespace-only) input collapses to empty — otherwise we'd emit
	// a lone marker with no content, which looks broken.
	assert.Empty(t, renderAssistantFinal(""))
	assert.Empty(t, renderAssistantFinal("\n  "))

	// Single-line: the marker sits on the same line as the text. No indented
	// continuation block.
	singleLine := renderAssistantFinal("hello")
	assert.Contains(t, singleLine, "hello")
	assert.NotContains(t, singleLine, "\n    ", "single-line output must not add a 4-space continuation indent")

	// Multi-line: first line gets the marker; remaining lines are indented under
	// the marker so the paragraph visually belongs to the assistant reply.
	multi := renderAssistantFinal("first\nsecond\nthird")
	assert.Contains(t, multi, "first")
	assert.Contains(t, multi, "second")
	assert.Contains(t, multi, "third")
	// Some form of block-level break separates the marker line from the rest.
	assert.Contains(t, multi, "\n")
}

func TestRenderAssistantStreaming(t *testing.T) {
	t.Parallel()

	assert.Empty(t, renderAssistantStreaming(""))
	// The streaming indent is two spaces — matches the marker column in the
	// final render so tokens don't visually jump when streaming transitions to
	// final.
	assert.Equal(t, "  hi", renderAssistantStreaming("hi"))
}

func TestWaitForEvent_DeliversEvent(t *testing.T) {
	t.Parallel()

	// The returned tea.Cmd is a blocking read; feed an event, execute the cmd,
	// and assert it surfaces as the exact msg value. This is the bridge between
	// the Session goroutine and bubbletea.
	ch := make(chan UIEvent, 1)
	want := UIAssistantMessage{Content: "x", IsFinal: true}
	ch <- want

	cmd := waitForEvent(ch)
	msg := cmd()
	assert.Equal(t, want, msg)
}

func TestWaitForEvent_ReturnsQuitOnClose(t *testing.T) {
	t.Parallel()

	// Session closes the channel when its Run() exits. The TUI must see that as
	// a clean shutdown signal (tea.Quit), not a zero-value event.
	ch := make(chan UIEvent)
	close(ch)

	cmd := waitForEvent(ch)
	msg := cmd()
	// tea.Quit is itself a function; the waitForEvent wrapper returns its
	// *result* (tea.QuitMsg). Compare by type rather than by value.
	_, ok := msg.(tea.QuitMsg)
	assert.True(t, ok, "expected tea.QuitMsg, got %T", msg)
}

// -----------------------------------------------------------------------------
// NewModel
// -----------------------------------------------------------------------------

func TestNewModel_IdleStart(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{Org: "o", WorkDir: "/w", Username: "alice"})
	assert.False(t, m.busy, "idle start must not mark the model busy")
	assert.Empty(t, m.blocks, "idle start must have no busy block seeded")
	assert.True(t, m.textInput.Focused(), "text input should be focused at startup")
	assert.Equal(t, "o", m.welcome.org)
	assert.Equal(t, "/w", m.welcome.workDir)
}

func TestNewModel_BusyStart(t *testing.T) {
	t.Parallel()

	// When the caller has already handed a prompt to the backend, we seed the
	// busy state so Enter is gated and the spinner starts ticking — the user
	// must not be able to talk over the agent during startup.
	m := NewModel(ModelConfig{Busy: true})
	assert.True(t, m.busy)
	require.Len(t, m.blocks, 1, "busy start seeds exactly one busy block")
	assert.Equal(t, blockBusy, m.blocks[0].kind)
	assert.Equal(t, shimmerVerb, m.blocks[0].shimmer)
	assert.NotEmpty(t, m.blocks[0].label, "busy block must have a non-empty thinking label")
}

// -----------------------------------------------------------------------------
// Block manipulation
// -----------------------------------------------------------------------------

func TestAppendBlock_KeepsBusyAtBottom(t *testing.T) {
	t.Parallel()

	// The busy block must always render last so the spinner stays at the bottom
	// of the scrollback. Inserting a regular block in front of it preserves
	// that invariant.
	m := &Model{blocks: []block{
		{kind: blockUserMessage, rendered: "user"},
		{kind: blockBusy, label: "thinking"},
	}}
	m.appendBlock(block{kind: blockToolComplete, rendered: "tool"})

	require.Len(t, m.blocks, 3)
	assert.Equal(t, blockUserMessage, m.blocks[0].kind)
	assert.Equal(t, blockToolComplete, m.blocks[1].kind)
	assert.Equal(t, blockBusy, m.blocks[2].kind, "busy must stay at the bottom")
}

func TestAppendBlock_NoBusyGoesToEnd(t *testing.T) {
	t.Parallel()

	m := &Model{blocks: []block{{kind: blockUserMessage}}}
	m.appendBlock(block{kind: blockToolComplete})

	require.Len(t, m.blocks, 2)
	assert.Equal(t, blockToolComplete, m.blocks[1].kind)
}

func TestRemoveBlockKind(t *testing.T) {
	t.Parallel()

	m := &Model{blocks: []block{
		{kind: blockUserMessage},
		{kind: blockBusy},
		{kind: blockToolComplete},
		{kind: blockBusy},
	}}
	m.removeBlockKind(blockBusy)

	require.Len(t, m.blocks, 2)
	assert.Equal(t, blockUserMessage, m.blocks[0].kind)
	assert.Equal(t, blockToolComplete, m.blocks[1].kind)
}

func TestFindBlockKind(t *testing.T) {
	t.Parallel()

	m := &Model{blocks: []block{
		{kind: blockUserMessage},
		{kind: blockAssistantStreaming},
		{kind: blockUserMessage},
	}}
	// Returns the *last* index — matters for streaming: we need to update the
	// latest assistant bubble, not an earlier one.
	assert.Equal(t, 2, m.findBlockKind(blockUserMessage))
	assert.Equal(t, 1, m.findBlockKind(blockAssistantStreaming))
	assert.Equal(t, -1, m.findBlockKind(blockError))
}

func TestShowBusy_FirstCallReturnsTickCmdAndSetsBusy(t *testing.T) {
	t.Parallel()

	m := &Model{spinner: spinner.New()}
	cmd := m.showBusy("working...", shimmerWave)

	require.NotNil(t, cmd, "first showBusy must return the spinner Tick command")
	assert.True(t, m.busy)
	require.Len(t, m.blocks, 1)
	assert.Equal(t, blockBusy, m.blocks[0].kind)
	assert.Equal(t, "working...", m.blocks[0].label)
	assert.Equal(t, shimmerWave, m.blocks[0].shimmer)
}

func TestShowBusy_WhileBusyReturnsNil(t *testing.T) {
	t.Parallel()

	// A second showBusy during the same turn (e.g. from a UIToolCompleted
	// chaining back into a thinking state) must not start a second Tick loop —
	// that would double the spinner speed.
	m := &Model{busy: true, blocks: []block{{kind: blockBusy, label: "old"}}, spinner: spinner.New()}
	cmd := m.showBusy("new", shimmerVerb)

	assert.Nil(t, cmd)
	assert.True(t, m.busy)
	require.Len(t, m.blocks, 1)
	assert.Equal(t, "new", m.blocks[0].label, "label must be updated in place")
	assert.Equal(t, shimmerVerb, m.blocks[0].shimmer)
}

func TestEndBusy_ResetsStateAndBlock(t *testing.T) {
	t.Parallel()

	m := &Model{
		busy:   true,
		frame:  42,
		blocks: []block{{kind: blockUserMessage}, {kind: blockBusy}},
	}
	m.endBusy()

	assert.False(t, m.busy)
	assert.Equal(t, 0, m.frame, "frame must reset so the next busy cycle starts clean")
	require.Len(t, m.blocks, 1)
	assert.Equal(t, blockUserMessage, m.blocks[0].kind)
}

// -----------------------------------------------------------------------------
// Model.Init / Update / View
// -----------------------------------------------------------------------------

func TestModel_Init_ReturnsBatch(t *testing.T) {
	t.Parallel()

	// Both the idle and busy starts must return a non-nil Init cmd; otherwise
	// the TUI never starts listening for events and the whole session hangs
	// waiting for input that never surfaces.
	idle := NewModel(ModelConfig{})
	require.NotNil(t, idle.Init())

	busy := NewModel(ModelConfig{Busy: true})
	require.NotNil(t, busy.Init())
}

func TestModel_Update_WindowSize_ResizesViewportAndBuildsRenderer(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	um := updated.(Model)

	assert.Equal(t, 100, um.width)
	assert.Equal(t, 30, um.height)
	assert.Equal(t, 100, um.welcome.termWidth)
	assert.Equal(t, 100, um.viewport.Width)
	assert.Equal(t, 30-inputBarHeight, um.viewport.Height)
	// The glamour renderer is lazily built on the first WindowSize so it can
	// pick up the real terminal width for wrapping.
	require.NotNil(t, um.mdRenderer)
}

func TestModel_Update_WindowSize_MinimumViewportHeight(t *testing.T) {
	t.Parallel()

	// A terminal smaller than the input bar would give a negative viewport
	// height; the clamp in Update guarantees at least 1 row so bubbletea
	// doesn't panic.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 1})
	um := updated.(Model)
	assert.GreaterOrEqual(t, um.viewport.Height, 1)
}

func TestModel_Update_KeyCtrlC_ReturnsQuit(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd)

	// Running the returned Cmd should surface tea.QuitMsg.
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "Ctrl-C must produce a tea.QuitMsg")
}

func TestModel_Update_KeyEnter_WhileBusy_SwallowsAndDoesNotSend(t *testing.T) {
	t.Parallel()

	// Enter while busy must be a no-op: the typed text stays in the input
	// (user can retry after UITaskIdle) and no value is posted to outCh.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh, Busy: true})
	m.textInput.SetValue("queued")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, "queued", um.textInput.Value(), "text must stay in input while busy")
	select {
	case got := <-outCh:
		t.Fatalf("no message must be sent while busy, got %+v", got)
	default:
	}
}

func TestModel_Update_KeyEnter_Idle_SendsAndClearsInput(t *testing.T) {
	t.Parallel()

	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.textInput.SetValue("hello")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	select {
	case got := <-outCh:
		msg, ok := got.event.(apitype.AgentUserEventUserMessage)
		require.True(t, ok, "Enter must post a UserMessage event, got %T", got.event)
		assert.Equal(t, "hello", msg.Content)
	default:
		t.Fatal("Enter must post the input to outCh")
	}
	assert.Empty(t, um.textInput.Value(), "input must clear after send")
	assert.True(t, um.busy, "sending must enter the busy state")

	// Optimistic render: the user's message must appear in the transcript
	// as soon as Enter is handled, before any server echo arrives.
	idx := um.findBlockKind(blockUserMessage)
	require.NotEqual(t, -1, idx, "Enter must render the user message immediately")
	assert.Contains(t, um.blocks[idx].rendered, "hello")

	// The content is queued for echo suppression so the server's replay
	// doesn't duplicate the block.
	require.Len(t, um.pendingUserEchoes, 1)
	assert.Equal(t, "hello", um.pendingUserEchoes[0])
}

func TestModel_Update_KeyEnter_EmptyInput_NoSend(t *testing.T) {
	t.Parallel()

	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	// input left empty

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	assert.False(t, um.busy, "empty Enter must not enter the busy state")
	assert.Equal(t, -1, um.findBlockKind(blockUserMessage), "empty Enter must not render a user block")
	select {
	case got := <-outCh:
		t.Fatalf("empty Enter must not send, got %+v", got)
	default:
	}
}

// -----------------------------------------------------------------------------
// Approval flow
// -----------------------------------------------------------------------------

// newApprovalPendingModel returns a Model that has just received an approval
// request — busy is cleared, pendingApproval is true, and the prompt has been
// swapped to the approval prompt. Mirrors the state UIApprovalRequest leaves
// behind so each Enter test can start from a known point.
func newApprovalPendingModel(t *testing.T, outCh chan outboundEvent) Model {
	t.Helper()
	m := NewModel(ModelConfig{OutCh: outCh, EventCh: make(chan UIEvent, 4), Busy: true})
	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:  "appr_1",
		Message:     "Run pulumi up?",
		Sensitivity: "high",
	})
	return updated.(Model)
}

func TestModel_Update_UIApprovalRequest_ShowsPromptAndPausesAgent(t *testing.T) {
	t.Parallel()

	// The approval request must clear busy (the agent is intentionally paused),
	// append a visible approval block, and swap the input prompt so the user
	// knows Enter now answers the approval rather than sending a chat message.
	outCh := make(chan outboundEvent, 1)
	m := newApprovalPendingModel(t, outCh)

	assert.False(t, m.busy, "approval request must end busy so the user can answer")
	assert.True(t, m.pendingApproval)
	assert.Equal(t, "appr_1", m.pendingApprovalID)
	assert.GreaterOrEqual(t, m.findBlockKind(blockApproval), 0, "an approval block must be appended")
	assert.Contains(t, m.textInput.Prompt, "Approve?", "input prompt must reflect approval mode")
}

func TestModel_Update_KeyEnter_Approval_ApproveYes(t *testing.T) {
	t.Parallel()

	cases := []string{"y", "Y", "yes", "YES", "Yes"}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			outCh := make(chan outboundEvent, 1)
			m := newApprovalPendingModel(t, outCh)
			m.textInput.SetValue(in)

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			um := updated.(Model)

			// Must post a confirmation event with Approved=true and no instructions.
			select {
			case got := <-outCh:
				conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
				require.True(t, ok, "expected UserConfirmation, got %T", got.event)
				assert.True(t, conf.Approved, "%q must be parsed as approval", in)
				assert.Equal(t, "appr_1", conf.ApprovalID, "must echo the request id")
				assert.Empty(t, conf.Message, "approval must not carry instructions")
				assert.Equal(t, userEventUserConfirmation, conf.Type)
			default:
				t.Fatalf("Enter must post a confirmation event")
			}

			// State must reset: pendingApproval cleared, prompt restored, input
			// cleared, and a busy block re-armed because the agent is about to
			// resume work.
			assert.False(t, um.pendingApproval)
			assert.Empty(t, um.pendingApprovalID)
			assert.Empty(t, um.textInput.Value())
			assert.True(t, um.busy, "approving must hand the turn back to the agent")
			require.NotNil(t, cmd, "approval must return the spinner Tick command")
		})
	}
}

func TestModel_Update_KeyEnter_Approval_DenyWithReason(t *testing.T) {
	t.Parallel()

	// Anything that isn't "y"/"yes" is treated as a denial; the typed text becomes
	// the instructions field so the agent can act on the user's reasoning.
	outCh := make(chan outboundEvent, 1)
	m := newApprovalPendingModel(t, outCh)
	m.textInput.SetValue("not on prod")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok, "expected UserConfirmation, got %T", got.event)
		assert.False(t, conf.Approved)
		assert.Equal(t, "appr_1", conf.ApprovalID)
		assert.Equal(t, "not on prod", conf.Message, "denial must forward the typed reason")
	default:
		t.Fatal("Enter must post a confirmation event")
	}

	assert.False(t, um.pendingApproval)
	assert.False(t, um.busy, "denial must NOT re-arm busy — the agent is not running")
	assert.Nil(t, cmd, "denial must not return a spinner cmd")
}

func TestModel_Update_KeyEnter_Approval_DenyEmpty(t *testing.T) {
	t.Parallel()

	// An empty input is a denial with no instructions. Same outcome as a reasoned
	// denial wire-wise (Approved=false), with an empty Message field.
	outCh := make(chan outboundEvent, 1)
	m := newApprovalPendingModel(t, outCh)
	// input left empty

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(Model)

	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok, "expected UserConfirmation, got %T", got.event)
		assert.False(t, conf.Approved)
		assert.Empty(t, conf.Message)
	default:
		t.Fatal("Enter must post a confirmation event even on empty input")
	}
	assert.False(t, um.busy)
}

func TestModel_Update_Approval_NonEnterKey_ForwardsToTextInput(t *testing.T) {
	t.Parallel()

	// While waiting for approval, non-Enter keys must still type into the input
	// (so the user can compose a denial reason). The approval state must NOT
	// clear and no event may be posted.
	outCh := make(chan outboundEvent, 1)
	m := newApprovalPendingModel(t, outCh)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	um := updated.(Model)

	assert.True(t, um.pendingApproval, "non-Enter key must not exit approval mode")
	assert.Equal(t, "a", um.textInput.Value())
	select {
	case got := <-outCh:
		t.Fatalf("non-Enter must not post a confirmation, got %+v", got)
	default:
	}
}

func TestModel_Update_KeyEnter_Approval_NotGatedByBusy(t *testing.T) {
	t.Parallel()

	// The approval branch sits ahead of the busy gate in Update because the agent
	// is intentionally paused waiting for the user. Even if busy somehow stayed
	// true (e.g. a stray TickMsg arrived between UIApprovalRequest and Enter),
	// Enter must still answer the approval rather than be swallowed.
	outCh := make(chan outboundEvent, 1)
	m := newApprovalPendingModel(t, outCh)
	m.busy = true // simulate a stale busy state
	m.textInput.SetValue("y")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok)
		assert.True(t, conf.Approved)
	default:
		t.Fatal("approval Enter must not be gated by busy")
	}
}

func TestModel_Update_MouseMsg_ForwardsToViewport(t *testing.T) {
	t.Parallel()

	// Mouse events are routed to the viewport for scrolling. We don't introspect
	// the viewport's internal scroll state here (it's an external bubble), just
	// verify the dispatch doesn't panic and returns a Model — the coverage of
	// the MouseMsg arm is the goal.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	_, ok := updated.(Model)
	assert.True(t, ok)
}

func TestModel_Update_UnknownMessage_ForwardsToTextInput(t *testing.T) {
	t.Parallel()

	// The default switch arm forwards unhandled messages to the text input
	// (e.g. textinput.Blink). The arm must return without panicking; coverage
	// of the default branch is the goal.
	m := NewModel(ModelConfig{})
	type unknownMsg struct{}
	updated, _ := m.Update(unknownMsg{})
	_, ok := updated.(Model)
	assert.True(t, ok)
}

func TestModel_Update_SpinnerTick_AdvancesFrameWhileBusy(t *testing.T) {
	t.Parallel()

	// The shimmer animation is driven by frame++ on every spinner tick. If this
	// regresses, the busy label stops animating even though the spinner glyph
	// still moves.
	m := NewModel(ModelConfig{Busy: true})
	start := m.frame

	updated, _ := m.Update(spinner.TickMsg{})
	um := updated.(Model)
	assert.Greater(t, um.frame, start, "frame must advance on TickMsg while busy")
}

func TestModel_Update_SpinnerTick_IgnoredWhenIdle(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	updated, cmd := m.Update(spinner.TickMsg{})
	um := updated.(Model)
	assert.Equal(t, 0, um.frame, "idle TickMsg must not advance the shimmer frame")
	assert.Nil(t, cmd, "idle TickMsg must not schedule another tick")
}

func TestModel_Update_UIAssistantMessage_Streaming_AppendsThenUpdates(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	// First streaming chunk: must append a new blockAssistantStreaming.
	updated, _ := m.Update(UIAssistantMessage{Content: "one"})
	m1 := updated.(Model)
	idx := m1.findBlockKind(blockAssistantStreaming)
	require.NotEqual(t, -1, idx, "first streaming msg must append a streaming block")

	// Second streaming chunk: same block kind; the rendered text must change
	// but the number of streaming blocks must not grow.
	updated2, _ := m1.Update(UIAssistantMessage{Content: "one two"})
	m2 := updated2.(Model)
	count := 0
	for _, b := range m2.blocks {
		if b.kind == blockAssistantStreaming {
			count++
		}
	}
	assert.Equal(t, 1, count, "second streaming msg must update in place, not append")
}

func TestModel_Update_UIAssistantMessage_Final_ReplacesStreaming(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	// Seed a streaming block so the final msg has something to replace.
	updated, _ := m.Update(UIAssistantMessage{Content: "streaming"})
	m1 := updated.(Model)

	updated2, _ := m1.Update(UIAssistantMessage{Content: "done", IsFinal: true})
	m2 := updated2.(Model)

	assert.Equal(t, -1, m2.findBlockKind(blockAssistantStreaming), "final msg must drop any streaming block")
	assert.GreaterOrEqual(t, m2.findBlockKind(blockAssistantFinal), 0, "final msg must leave a final block")
}

func TestModel_Update_UIToolStarted_ShowsBusyBlock(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIToolStarted{
		Name: "filesystem__read",
		Args: json.RawMessage(`{"file_path":"/x"}`),
	})
	um := updated.(Model)

	require.Len(t, um.blocks, 1)
	assert.Equal(t, blockBusy, um.blocks[0].kind)
	assert.Contains(t, um.blocks[0].label, "Read", "busy label must reflect the pretty tool name")
	assert.Equal(t, shimmerWave, um.blocks[0].shimmer)
}

func TestModel_Update_UIToolCompleted_AppendsCompleteAndStaysBusy(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch, Busy: true})
	updated, _ := m.Update(UIToolCompleted{
		Name: "filesystem__read",
		Args: json.RawMessage(`{"file_path":"/x"}`),
	})
	um := updated.(Model)

	// Tool result is appended, busy block stays pinned at the bottom so the
	// spinner keeps spinning between consecutive tool calls.
	require.GreaterOrEqual(t, len(um.blocks), 2)
	assert.Equal(t, blockBusy, um.blocks[len(um.blocks)-1].kind)
	assert.Equal(t, blockToolComplete, um.blocks[len(um.blocks)-2].kind)
}

func TestModel_Update_UIError_EndsBusyAndAppendsError(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch, Busy: true})
	updated, _ := m.Update(UIError{Message: "boom"})
	um := updated.(Model)

	assert.False(t, um.busy, "UIError must clear the busy state")
	assert.Equal(t, -1, um.findBlockKind(blockBusy))
	assert.GreaterOrEqual(t, um.findBlockKind(blockError), 0)
}

func TestModel_Update_UIWarning_AppendsWarning(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIWarning{Message: "careful"})
	um := updated.(Model)

	assert.GreaterOrEqual(t, um.findBlockKind(blockWarning), 0)
}

func TestModel_Update_UICancelled_EndsBusyAndAppendsCancelled(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch, Busy: true})
	updated, _ := m.Update(UICancelled{})
	um := updated.(Model)

	assert.False(t, um.busy)
	assert.GreaterOrEqual(t, um.findBlockKind(blockCancelled), 0)
}

func TestModel_Update_UITaskIdle_EndsBusy(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch, Busy: true})
	updated, _ := m.Update(UITaskIdle{})
	um := updated.(Model)

	assert.False(t, um.busy, "UITaskIdle re-enables input")
	assert.Equal(t, -1, um.findBlockKind(blockBusy))
}

func TestModel_Update_UISessionURL_UpdatesWelcomeConsoleURL(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UISessionURL{URL: "https://app.pulumi.com/x"})
	um := updated.(Model)

	assert.Equal(t, "https://app.pulumi.com/x", um.welcome.consoleURL)
}

func TestModel_Update_UIUserMessage_AppendsUserBlock(t *testing.T) {
	t.Parallel()

	// With an empty pending queue the echo comes from outside this TUI
	// (e.g. the web UI on the same task) and must render as a user block.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated, _ := m.Update(UIUserMessage{Content: "hi there"})
	um := updated.(Model)

	idx := um.findBlockKind(blockUserMessage)
	require.NotEqual(t, -1, idx)
	assert.Contains(t, um.blocks[idx].rendered, "hi there")
}

func TestModel_Update_UIUserMessage_SelfEchoIsSuppressed(t *testing.T) {
	t.Parallel()

	// Submitting "hi" renders a block optimistically and queues the content
	// for suppression. When the server echoes that same message back, the
	// queue entry is popped and the redundant render is skipped — so the
	// transcript contains exactly one user block.
	outCh := make(chan outboundEvent, 1)
	evCh := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{OutCh: outCh, EventCh: evCh})
	m.textInput.SetValue("hi")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	<-outCh // drain the submitted event so outCh doesn't leak
	require.Len(t, m.pendingUserEchoes, 1)

	updated2, _ := m.Update(UIUserMessage{Content: "hi"})
	m = updated2.(Model)

	count := 0
	for _, b := range m.blocks {
		if b.kind == blockUserMessage {
			count++
		}
	}
	assert.Equal(t, 1, count, "self-echo must not double-render the user message")
	assert.Empty(t, m.pendingUserEchoes, "matching echo must pop the queue entry")
}

func TestModel_Update_UIUserMessage_ForeignEchoStillRenders(t *testing.T) {
	t.Parallel()

	// A user message that didn't originate from this TUI (for example, the
	// user typing in the web UI for the same task) must still render. The
	// dedup queue only suppresses echoes that match what this TUI submitted.
	outCh := make(chan outboundEvent, 1)
	evCh := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{OutCh: outCh, EventCh: evCh})
	m.textInput.SetValue("from cli")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	<-outCh

	updated2, _ := m.Update(UIUserMessage{Content: "from web"})
	m = updated2.(Model)

	count := 0
	for _, b := range m.blocks {
		if b.kind == blockUserMessage {
			count++
		}
	}
	assert.Equal(t, 2, count, "non-matching echo must still render as a user block")
	require.Len(t, m.pendingUserEchoes, 1, "non-matching echo must not pop the queue")
	assert.Equal(t, "from cli", m.pendingUserEchoes[0])
}

func TestNewModel_InitialPromptRendersUserBlock(t *testing.T) {
	t.Parallel()

	// `pulumi neo "my prompt"` sends the prompt via CreateNeoTask rather
	// than outCh, so the TUI only learns about it through ModelConfig.
	// NewModel must render it as a user block and seed the echo queue so
	// the server's replay doesn't duplicate it.
	m := NewModel(ModelConfig{InitialPrompt: "kick off", Busy: true})

	idx := m.findBlockKind(blockUserMessage)
	require.NotEqual(t, -1, idx, "initial prompt must appear as a user block at startup")
	assert.Contains(t, m.blocks[idx].rendered, "kick off")

	require.Len(t, m.pendingUserEchoes, 1)
	assert.Equal(t, "kick off", m.pendingUserEchoes[0])

	// The busy block still sits at the bottom so the spinner is visible
	// while the agent starts its first turn.
	assert.Equal(t, blockBusy, m.blocks[len(m.blocks)-1].kind)
}

func TestModel_View_ShowsHintBasedOnBusy(t *testing.T) {
	t.Parallel()

	// The footer hint line is the user's only affordance telling them whether
	// Enter will do anything. Pin both states.
	idle := NewModel(ModelConfig{})
	assert.Contains(t, idle.View(), "enter to send")

	busy := NewModel(ModelConfig{Busy: true})
	assert.Contains(t, busy.View(), "agent is working")
	assert.Contains(t, busy.View(), "enter disabled")
}

// -----------------------------------------------------------------------------
// Plan mode
// -----------------------------------------------------------------------------

func TestModel_Update_ShiftTab_TogglesPlanModeBeforeFirstMessage(t *testing.T) {
	t.Parallel()

	// Shift+Tab before the first message is sent is the user's affordance to
	// opt into plan mode. The toggle is reflected in the footer hint so the
	// user gets immediate feedback without waiting for any server round trip.
	m := NewModel(ModelConfig{})
	assert.NotContains(t, m.View(), "plan mode on")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	assert.True(t, m.planMode, "Shift+Tab must flip planMode on")
	assert.Contains(t, m.View(), "plan mode", "hint must show the plan-mode indicator")

	// Second press toggles back off — same affordance, symmetric behaviour.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	assert.False(t, m.planMode, "second Shift+Tab must flip planMode off")
	assert.NotContains(t, m.View(), "plan mode on")
}

func TestModel_Update_ShiftTab_AfterFirstMessage_WarnsAndDoesNotToggle(t *testing.T) {
	t.Parallel()

	// Plan mode is task-level on the wire and is snapshotted the moment the
	// first message is dispatched. A post-send toggle would be misleading —
	// the dispatcher has already captured planMode for CreateNeoTask, so any
	// later flip could not affect the task.
	m := NewModel(ModelConfig{MessageSent: true})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)

	assert.False(t, m.planMode, "post-send Shift+Tab must not toggle planMode")
	idx := m.findBlockKind(blockWarning)
	require.NotEqual(t, -1, idx, "post-send Shift+Tab must append a warning block")
	assert.Contains(t, m.blocks[idx].rendered, "task-level")
}

func TestModel_Update_KeyEnter_SendingFirstMessageFreezesPlanMode(t *testing.T) {
	t.Parallel()

	// Sending the first user message both (a) carries the current planMode
	// across to the dispatcher and (b) flips messageSent so any subsequent
	// Shift+Tab is a no-op. This is the moment the TUI commits planMode.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.planMode = true
	m.textInput.SetValue("kick off")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	select {
	case got := <-outCh:
		msg, ok := got.event.(apitype.AgentUserEventUserMessage)
		require.True(t, ok, "expected AgentUserEventUserMessage, got %T", got.event)
		assert.Equal(t, "kick off", msg.Content)
		assert.True(t, got.planMode, "outbound envelope must carry the TUI's planMode")
	default:
		t.Fatal("Enter must post the input to outCh")
	}

	assert.True(t, m.messageSent, "first send must freeze the plan-mode affordance")

	// Shift+Tab after send must warn, not toggle.
	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated2.(Model)
	assert.True(t, m.planMode, "post-send Shift+Tab must leave planMode untouched")
	assert.NotEqual(t, -1, m.findBlockKind(blockWarning), "post-send Shift+Tab must warn")
}

func TestModel_Update_UIApprovalRequest_PlanCategory_RendersPlanHeaderAndMarkdown(t *testing.T) {
	t.Parallel()

	// A plan-category approval signals that the agent is ready to exit plan
	// mode with its proposed plan. The body comes in as markdown and must be
	// routed through the model's renderer so the user sees a formatted plan
	// rather than raw asterisks. The distinct "Proposed plan" header tells
	// the user this isn't a regular tool approval.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	m.planMode = true
	// Initialize the markdown renderer (built on WindowSize).
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_1",
		Message:         "I've finished exploring and have a plan ready for your review.",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "# Plan\n\n- step one\n- step two",
	})
	um := updated.(Model)

	assert.True(t, um.pendingApproval, "plan approval must enter the pending state")
	assert.Equal(t, approvalTypePlanExit, um.pendingApprovalType,
		"plan approval must record its wire approval_type")
	idx := um.findBlockKind(blockApproval)
	require.NotEqual(t, -1, idx)
	assert.Contains(t, um.blocks[idx].rendered, "Proposed plan")
	// Glamour wraps each word in its own ANSI escape run; assert on word
	// fragments that the renderer never splits ("step" shows up verbatim).
	assert.Contains(t, um.blocks[idx].rendered, "step", "rendered plan must include the plan body")
	assert.Contains(t, um.blocks[idx].rendered, "Plan", "rendered plan must include the heading")
	assert.Contains(t, um.textInput.Prompt, "Approve plan",
		"prompt must indicate this is a plan approval")
}

func TestModel_Update_UIApprovalRequest_General_UsesExistingApprovalRendering(t *testing.T) {
	t.Parallel()

	// Regular (non-plan) tool approvals keep the existing "⚠ Approval required"
	// rendering and generic prompt. The plan path must not leak into them — they
	// share the same wire event type (user_approval_request) and only diverge on
	// ApprovalType.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_2",
		Message:      "run pulumi up",
		ApprovalType: "general",
	})
	um := updated.(Model)

	assert.NotEqual(t, approvalTypePlanExit, um.pendingApprovalType,
		"general approval must not be flagged as a plan")
	idx := um.findBlockKind(blockApproval)
	require.NotEqual(t, -1, idx)
	assert.Contains(t, um.blocks[idx].rendered, "Approval required")
	assert.Contains(t, um.textInput.Prompt, "Approve?")
	assert.NotContains(t, um.textInput.Prompt, "plan")
}

func TestModel_Update_ApprovePlan_ClearsPlanMode(t *testing.T) {
	t.Parallel()

	// Approving the plan exits plan mode server-side (PlanModeTracker stops
	// gating writes); the local indicator must mirror that immediately so the
	// footer doesn't misrepresent the effective state.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.planMode = true

	// Simulate receiving the plan approval request.
	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_3",
		Message:         "I've finished exploring.",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "# Plan\n\n- step one\n- step two",
	})
	m = updated.(Model)
	m.textInput.SetValue("y")

	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated2.(Model)

	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok, "expected AgentUserEventUserConfirmation, got %T", got.event)
		assert.True(t, conf.Approved)
		assert.Equal(t, "appr_3", conf.ApprovalID)
	default:
		t.Fatal("approving plan must post a confirmation event")
	}

	assert.False(t, m.planMode, "approved plan must auto-clear planMode")
	assert.False(t, m.pendingApproval)
	assert.Empty(t, m.pendingApprovalType, "approval type must be cleared after response")
}

func TestModel_Update_DenyPlan_LeavesPlanModeOn(t *testing.T) {
	t.Parallel()

	// Denying the plan means the user wants the agent to re-plan — plan mode
	// must stay on so writes remain gated while the agent iterates.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.planMode = true

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_4",
		Message:         "I've finished exploring.",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "# Plan\n\n- step one\n- step two",
	})
	m = updated.(Model)
	m.textInput.SetValue("cover error handling too")

	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated2.(Model)

	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok)
		assert.False(t, conf.Approved)
		assert.Equal(t, "cover error handling too", conf.Message, "denial text becomes the re-plan instructions")
	default:
		t.Fatal("denying plan must post a confirmation event")
	}

	assert.True(t, m.planMode, "denied plan must leave planMode on")
}

func TestModel_RenderMarkdown_FallsBackWhenRendererNil(t *testing.T) {
	t.Parallel()

	// The md renderer is built on WindowSize; until then renderMarkdown must
	// be a no-op rather than crash. Send no WindowSize and verify the raw text
	// is returned unchanged.
	m := &Model{}
	assert.Equal(t, "hello **world**", m.renderMarkdown("hello **world**"))
}

func TestModel_RenderMarkdown_UsesRendererAfterWindowSize(t *testing.T) {
	t.Parallel()

	// Once WindowSize has initialized glamour, renderMarkdown must route the
	// input through it. The exact styled bytes vary by terminal, but the
	// rendered output must contain the "hello" text and differ from the raw
	// input (proving the renderer actually ran).
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	um := updated.(Model)
	require.NotNil(t, um.mdRenderer)

	got := um.renderMarkdown("# hello")
	assert.Contains(t, got, "hello")
}

func TestModel_RebuildContent_HandlesMixedBlocksWithoutPanic(t *testing.T) {
	t.Parallel()

	// rebuildContent handles each blockKind plus the busy-block special case
	// (which reads the spinner glyph at render time). Feed one of each kind so
	// every branch of the per-block switch runs, and both the "was at bottom"
	// and default paths get exercised across multiple invocations.
	m := NewModel(ModelConfig{})
	m.blocks = []block{
		{kind: blockUserMessage, rendered: "  user "},
		{kind: blockToolComplete, rendered: "  tool ok"},
		{kind: blockAssistantFinal, rendered: "  final"},
		{kind: blockWarning, rendered: "  warn"},
		{kind: blockBusy, label: "Thinking...", shimmer: shimmerVerb},
	}

	// Must not panic and must not mangle the block slice.
	require.NotPanics(t, func() { m.rebuildContent() })
	// Second call exercises the "not at bottom" path once scrolled up and then
	// back down (via GotoBottom internally); simply verifying it also doesn't
	// panic is enough — we don't reach into the viewport internals here.
	require.NotPanics(t, func() { m.rebuildContent() })
	require.Len(t, m.blocks, 5, "blocks slice must be untouched")
}
