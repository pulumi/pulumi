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
	"reflect"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/acarl005/stripansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// collectPrintln walks a tea.Cmd (potentially a Batch) and returns the bodies
// of every tea.Println-produced message inside. tea.Println builds an
// unexported printLineMessage{messageBody string}, so we identify it by type
// name and read the field via reflection. This is the only way to assert
// "what would have been written to terminal scrollback" from a unit test.
//
// Update arms typically bundle tea.Println cmds with a waitForEvent cmd that
// blocks forever on the event channel — running each leaf with a short
// timeout in a goroutine sidesteps that without the test having to know
// which cmds are "safe" to invoke.
func collectPrintln(cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}
	msg, ok := runCmd(cmd)
	if !ok {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []string
		for _, c := range batch {
			out = append(out, collectPrintln(c)...)
		}
		return out
	}
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Struct && v.Type().Name() == "printLineMessage" {
		if f := v.FieldByName("messageBody"); f.IsValid() && f.Kind() == reflect.String {
			return []string{f.String()}
		}
	}
	return nil
}

// runFirstFlushTick finds and runs the firstFlushReadyMsg tick from the
// first WindowSizeMsg's cmd batch. Waits longer than runCmd because the
// production tick fires at 50ms (the runCmd timeout). tea.Batch returns the
// sole cmd directly when only one is queued, so handle both shapes.
func runFirstFlushTick(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg := cmd()
	if tick, ok := msg.(firstFlushReadyMsg); ok {
		return tick
	}
	batch, ok := msg.(tea.BatchMsg)
	require.True(t, ok, "expected firstFlushReadyMsg or a batch, got %T", msg)
	for _, c := range batch {
		done := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { done <- c() }(c)
		select {
		case m := <-done:
			if tick, ok := m.(firstFlushReadyMsg); ok {
				return tick
			}
		case <-time.After(time.Second):
		}
	}
	t.Fatal("did not find firstFlushReadyMsg in the WindowSizeMsg batch")
	return nil
}

// runCmd invokes cmd in a goroutine and returns its result if it produces one
// within a short window. waitForEvent and similar blocking cmds time out and
// are reported as "no message" — collectPrintln then ignores them.
func runCmd(cmd tea.Cmd) (tea.Msg, bool) {
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case m := <-done:
		return m, true
	case <-time.After(50 * time.Millisecond):
		return nil, false
	}
}

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
		{kind: blockAssistantFinal},
		{kind: blockUserMessage},
	}}
	// Returns the *last* index — matters when multiple blocks of the same
	// kind exist and we need to act on the most recent one.
	assert.Equal(t, 2, m.findBlockKind(blockUserMessage))
	assert.Equal(t, 1, m.findBlockKind(blockAssistantFinal))
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

func TestModel_Update_WindowSize_TracksDimensionsAndBuildsRenderer(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	um := updated.(Model)

	assert.Equal(t, 100, um.width)
	assert.Equal(t, 30, um.height)
	// welcome.termWidth tracks liveWidth() = terminal width minus a 4-col
	// margin so the banner fills the available width.
	assert.Equal(t, 96, um.welcome.termWidth)
	// The glamour renderer is lazily built on the first WindowSize so it can
	// pick up the real terminal width for wrapping.
	require.NotNil(t, um.mdRenderer)
	assert.True(t, um.sizeReceived, "first WindowSize must flip sizeReceived")
}

func TestModel_Update_WindowSize_TinyTerminalDoesNotPanic(t *testing.T) {
	t.Parallel()

	// View must not panic on a tiny terminal.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 1})
	um := updated.(Model)
	require.NotPanics(t, func() { _ = um.viewString() })
}

func TestModel_Update_KeyCtrlC_TwoPressesQuit(t *testing.T) {
	t.Parallel()

	// First Ctrl+C arms the "press again to exit" prompt without quitting,
	// matching the cancel-vs-quit semantics requested in pulumi-service#42029.
	// Only a second Ctrl+C in a row produces tea.QuitMsg.
	m := NewModel(ModelConfig{})
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um := updated.(Model)
	assert.True(t, um.ctrlCArmed, "first Ctrl+C must arm the second-press-to-exit prompt")
	if cmd != nil {
		if msg := cmd(); msg != nil {
			_, isQuit := msg.(tea.QuitMsg)
			assert.False(t, isQuit, "first Ctrl+C must not quit")
		}
	}
	assert.Contains(t, um.viewString(), "Press Ctrl+C again to exit",
		"footer must surface the second-press-to-exit hint")

	// Second Ctrl+C in a row quits.
	_, cmd = um.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "second consecutive Ctrl+C must produce a tea.QuitMsg")
}

func TestModel_Update_KeyCtrlC_FirstPressCancelsWhenBusy(t *testing.T) {
	t.Parallel()

	// First Ctrl+C while the agent is mid-turn must mirror ESC: post a
	// user_cancel upstream and flip the cancelling flag, while still arming
	// the second-press-to-exit prompt.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh, Busy: true})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um := updated.(Model)
	assert.True(t, um.ctrlCArmed, "Ctrl+C must arm the exit prompt")
	assert.True(t, um.cancelling, "Ctrl+C while busy must trigger cancellation")
	assert.True(t, um.busy, "spinner stays on until the backend confirms")

	select {
	case ev := <-outCh:
		c, ok := ev.event.(apitype.AgentUserEventCancel)
		require.True(t, ok, "Ctrl+C must post an AgentUserEventCancel, got %T", ev.event)
		assert.Equal(t, userEventUserCancel, c.Type)
	default:
		t.Fatal("Ctrl+C while busy did not post a cancel event")
	}

	idx := um.findBlockKind(blockBusy)
	require.NotEqual(t, -1, idx)
	assert.Equal(t, "Cancelling...", um.blocks[idx].label)
}

func TestModel_Update_KeyCtrlC_OtherKeyDisarms(t *testing.T) {
	t.Parallel()

	// A keystroke between the two Ctrl+C presses resets the gate. Without
	// this, an idle session could be exited by a Ctrl+C now and another one
	// minutes later — the user would have lost the "press again" context.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um := updated.(Model)
	require.True(t, um.ctrlCArmed)

	updated, _ = um.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	um = updated.(Model)
	assert.False(t, um.ctrlCArmed, "any other key must disarm the exit prompt")
	assert.NotContains(t, um.viewString(), "Press Ctrl+C again to exit")

	// Another Ctrl+C now arms again rather than quitting.
	_, cmd := um.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			_, isQuit := msg.(tea.QuitMsg)
			assert.False(t, isQuit, "Ctrl+C after disarm must not quit")
		}
	}
}

func TestModel_Update_KeyCtrlC_TimeoutDisarms(t *testing.T) {
	t.Parallel()

	// The "press again to exit" gate must auto-disarm after a brief window so
	// it doesn't silently linger across a long idle. The first press schedules
	// a ctrlCDisarmMsg tagged with the current arm gen; receiving that msg
	// while still armed at the same gen flips ctrlCArmed back off.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um := updated.(Model)
	require.True(t, um.ctrlCArmed)
	gen := um.ctrlCArmGen

	updated2, _ := um.Update(ctrlCDisarmMsg{gen: gen})
	um2 := updated2.(Model)
	assert.False(t, um2.ctrlCArmed, "disarm tick at the current gen must clear the arm")
	assert.NotContains(t, um2.viewString(), "Press Ctrl+C again to exit")
}

func TestModel_Update_KeyCtrlC_StaleDisarmIgnored(t *testing.T) {
	t.Parallel()

	// A disarm tick from an earlier arm cycle must not clear a fresh arm.
	// Mechanism: arm, type a key (disarms locally and leaves the old tick
	// in flight), arm again (gen advances). Delivering the old tick now
	// must be a no-op because its gen no longer matches.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um := updated.(Model)
	staleGen := um.ctrlCArmGen

	updated, _ = um.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	um = updated.(Model)
	require.False(t, um.ctrlCArmed)

	updated, _ = um.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um = updated.(Model)
	require.True(t, um.ctrlCArmed)
	require.Greater(t, um.ctrlCArmGen, staleGen, "re-arm must advance the generation")

	updated, _ = um.Update(ctrlCDisarmMsg{gen: staleGen})
	um = updated.(Model)
	assert.True(t, um.ctrlCArmed, "stale-gen disarm tick must not clear a fresh arm")
}

func TestModel_Update_KeyCtrlZ_Suspends(t *testing.T) {
	t.Parallel()

	// Ctrl+Z must hand the shell back via standard job control (SIGTSTP).
	// Bubbletea models this as the Suspend command, which resolves to a
	// tea.SuspendMsg. It must work mid-turn, so assert it both idle and busy.
	t.Run("idle", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{})
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
		require.NotNil(t, cmd)
		_, ok := cmd().(tea.SuspendMsg)
		assert.True(t, ok, "Ctrl+Z must produce a tea.SuspendMsg")
	})

	t.Run("busy", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{})
		m.busy = true
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
		require.NotNil(t, cmd)
		_, ok := cmd().(tea.SuspendMsg)
		assert.True(t, ok, "Ctrl+Z must suspend even while the agent is busy")
	})
}

func TestModel_Update_KeyCtrlD_BehavesLikeCtrlC(t *testing.T) {
	t.Parallel()

	// Ctrl+D is wired as an alias for Ctrl+C — same two-press quit gate, same
	// cancel-when-busy semantics — so users who reach for either binding to
	// exit get the same behaviour.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	um := updated.(Model)
	assert.True(t, um.ctrlCArmed, "first Ctrl+D must arm the second-press-to-exit prompt")
	assert.Contains(t, um.viewString(), "Press Ctrl+C again to exit",
		"footer hint must surface even when the first press was Ctrl+D")

	_, cmd := um.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "second consecutive Ctrl+D must produce a tea.QuitMsg")

	// Also verify the cross-binding case: Ctrl+C followed by Ctrl+D quits.
	m2 := NewModel(ModelConfig{})
	updated2, _ := m2.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	um2 := updated2.(Model)
	require.True(t, um2.ctrlCArmed)
	_, cmd2 := um2.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	require.NotNil(t, cmd2)
	_, ok = cmd2().(tea.QuitMsg)
	assert.True(t, ok, "Ctrl+C then Ctrl+D must quit just like two Ctrl+Cs")
}

func TestModel_Update_KeyCtrlD_FirstPressCancelsWhenBusy(t *testing.T) {
	t.Parallel()

	// Ctrl+D while busy mirrors Ctrl+C: post a user_cancel upstream, flip
	// the cancelling flag, and arm the second-press-to-exit prompt.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh, Busy: true})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	um := updated.(Model)
	assert.True(t, um.ctrlCArmed)
	assert.True(t, um.cancelling, "Ctrl+D while busy must trigger cancellation")

	select {
	case ev := <-outCh:
		c, ok := ev.event.(apitype.AgentUserEventCancel)
		require.True(t, ok, "Ctrl+D must post an AgentUserEventCancel, got %T", ev.event)
		assert.Equal(t, userEventUserCancel, c.Type)
	default:
		t.Fatal("Ctrl+D while busy did not post a cancel event")
	}
}

func TestModel_View_BusyHintMentionsCancelKeys(t *testing.T) {
	t.Parallel()

	// Per pulumi-service#42029 the busy footer must surface both ways to
	// abort a turn so the affordance isn't hidden behind a key the user has
	// to discover.
	busy := NewModel(ModelConfig{Busy: true})
	view := busy.viewString()
	assert.Contains(t, view, "esc")
	assert.Contains(t, view, "ctrl+c")
	assert.Contains(t, view, "cancel")
}

func TestModel_Update_KeyEnter_WhileBusy_SwallowsAndDoesNotSend(t *testing.T) {
	t.Parallel()

	// Enter while busy must be a no-op: the typed text stays in the input
	// (user can retry after UITaskIdle) and no value is posted to outCh.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh, Busy: true})
	m.textInput.SetValue("queued")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	um := updated.(Model)

	assert.Nil(t, cmd)
	assert.Equal(t, "queued", um.textInput.Value(), "text must stay in input while busy")
	select {
	case got := <-outCh:
		t.Fatalf("no message must be sent while busy, got %+v", got)
	default:
	}
}

func TestModel_Update_ShiftEnter_InsertsNewlineDoesNotSend(t *testing.T) {
	t.Parallel()

	// Shift+Enter goes through the textarea's InsertNewline binding rather
	// than the bare-Enter branch in Update. Nothing posts to outCh; the
	// value gains a newline.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.textInput.SetValue("hello")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	um := updated.(Model)

	select {
	case got := <-outCh:
		t.Fatalf("shift+enter must not send; got %#v", got)
	default:
	}
	assert.Contains(t, um.textInput.Value(), "\n",
		"shift+enter must insert a newline into the textarea")
	assert.False(t, um.busy, "shift+enter must not enter the busy state")
}

func TestModel_Update_ShiftEnter_GrowsUnboundedWithContent(t *testing.T) {
	t.Parallel()

	// MaxHeight=0 must let the textarea grow without capping logical lines —
	// the default MaxHeight=99 silently swallows Enter past the cap.
	m := NewModel(ModelConfig{})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated0.(Model)

	const newlines = 15
	for range newlines {
		updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
		m = updated.(Model)
	}

	assert.Equal(t, newlines, strings.Count(m.textInput.Value(), "\n"))
	assert.Equal(t, newlines+1, m.textInput.Height())
}

func TestModel_Update_KeyEnter_BackslashSuffixInsertsNewline(t *testing.T) {
	t.Parallel()

	// Trailing backslash rewrites Enter from submit to newline so users on
	// terminals that can't distinguish Shift+Enter still have a way to add
	// a line. Backslash is stripped; nothing posts to outCh.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.textInput.SetValue("hello\\")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	um := updated.(Model)

	select {
	case got := <-outCh:
		t.Fatalf("backslash-Enter must not send; got %#v", got)
	default:
	}
	assert.Equal(t, "hello\n", um.textInput.Value(),
		"backslash must be stripped and replaced with a newline")
	assert.False(t, um.busy, "backslash-Enter must not enter the busy state")
}

func TestModel_Update_KeyEnter_Idle_SendsAndClearsInput(t *testing.T) {
	t.Parallel()

	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.textInput.SetValue("hello")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	um := updated.(Model)

	assert.False(t, um.busy, "empty Enter must not enter the busy state")
	assert.Equal(t, -1, um.findBlockKind(blockUserMessage), "empty Enter must not render a user block")
	select {
	case got := <-outCh:
		t.Fatalf("empty Enter must not send, got %+v", got)
	default:
	}
}

func TestModel_Update_KeyEnter_QuitOrExit_ClosesSession(t *testing.T) {
	t.Parallel()

	// Per pulumi-service#42477, typing `quit` or `exit` and pressing Enter
	// must cleanly close the TUI, complementing Ctrl+C / Ctrl+D for users
	// who reach for shell-style commands first. Match is case-insensitive
	// and tolerates surrounding whitespace; nothing must be posted to outCh.
	cases := []string{"quit", "exit", "QUIT", "Exit", "  quit  ", "  EXIT\t"}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			outCh := make(chan outboundEvent, 1)
			m := NewModel(ModelConfig{OutCh: outCh})
			m.textInput.SetValue(input)

			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			require.NotNil(t, cmd, "quit/exit Enter must return a command")
			_, ok := cmd().(tea.QuitMsg)
			assert.True(t, ok, "quit/exit Enter must produce a tea.QuitMsg")

			select {
			case got := <-outCh:
				t.Fatalf("quit/exit must not be posted as a chat message, got %+v", got)
			default:
			}
		})
	}
}

func TestModel_Update_KeyEnter_QuitSubstring_StillSends(t *testing.T) {
	t.Parallel()

	// Strict whole-input match: a message that merely contains the word
	// "quit" must still be sent as a normal user message.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh})
	m.textInput.SetValue("quit the deploy")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	um := updated.(Model)

	select {
	case got := <-outCh:
		msg, ok := got.event.(apitype.AgentUserEventUserMessage)
		require.True(t, ok, "Enter must post a UserMessage event, got %T", got.event)
		assert.Equal(t, "quit the deploy", msg.Content)
	default:
		t.Fatal("a message containing the word 'quit' must still be sent")
	}
	assert.True(t, um.busy, "sending must enter the busy state")
}

func TestModel_Update_KeyEnter_QuitWhileBusy_DoesNotQuit(t *testing.T) {
	t.Parallel()

	// While the agent is mid-turn, Enter is swallowed wholesale: the typed
	// "quit" stays in the input and the session keeps running. Users who
	// genuinely want to bail mid-turn use Ctrl+C (twice).
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh, Busy: true})
	m.textInput.SetValue("quit")

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd, "Enter on 'quit' while busy must not produce a quit command")
}

func TestModel_Update_KeyRune_TypesAndDoesNotScrollViewport(t *testing.T) {
	t.Parallel()

	// Regression for pulumi/pulumi-service#42025: pressing plain letters
	// (u/d/f/b/j/k) or space must type into the input rather than getting
	// intercepted as scroll keys.
	for _, r := range []rune{'u', 'd', 'f', 'b', 'j', 'k', ' '} {
		t.Run(string(r), func(t *testing.T) {
			t.Parallel()

			m := NewModel(ModelConfig{})
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
			um := updated.(Model)

			updated, _ = um.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			um = updated.(Model)

			assert.Equal(t, string(r), um.textInput.Value(), "rune must reach the text input")
		})
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
	assert.GreaterOrEqual(t, m.findBlockKind(blockApprovalGeneral), 0, "an approval block must be appended")
	assert.Contains(t, m.approvalPromptText, "Approve?", "input prompt must reflect approval mode")
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

			updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
}

func TestModel_Update_KeyEnter_Approval_DenyEmpty(t *testing.T) {
	t.Parallel()

	// An empty input is a denial with no instructions. Same outcome as a reasoned
	// denial wire-wise (Approved=false), with an empty Message field.
	outCh := make(chan outboundEvent, 1)
	m := newApprovalPendingModel(t, outCh)
	// input left empty

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
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

	_, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok)
		assert.True(t, conf.Approved)
	default:
		t.Fatal("approval Enter must not be gated by busy")
	}
}

func TestModel_Update_UnknownMessage_ForwardsToTextInput(t *testing.T) {
	t.Parallel()

	// The default switch arm forwards unhandled messages to the text input
	// (e.g. textarea.Blink). The arm must return without panicking; coverage
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

// TestModel_Update_UIAssistantMessage_CommitsEachContentMessage pins the
// "no chunking" assumption: every assistant_message with non-empty content
// — final or not — commits its own blockAssistantFinal to scrollback.
func TestModel_Update_UIAssistantMessage_CommitsEachContentMessage(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})

	updated, _ := m.Update(UIAssistantMessage{Content: "first turn"})
	m1 := updated.(Model)
	updated2, _ := m1.Update(UIAssistantMessage{Content: "second turn"})
	m2 := updated2.(Model)
	updated3, _ := m2.Update(UIAssistantMessage{Content: "final reply", IsFinal: true})
	m3 := updated3.(Model)

	finals := 0
	for _, b := range m3.blocks {
		if b.kind == blockAssistantFinal {
			finals++
		}
	}
	assert.Equal(t, 3, finals, "three messages with content must produce three final blocks")
}

// TestModel_Update_UIAssistantMessage_EmptyContentSkipsCommit guards the
// empty-content branch: a tool-call-only message (no text) must not produce
// a phantom final block.
func TestModel_Update_UIAssistantMessage_EmptyContentSkipsCommit(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	updated, _ := m.Update(UIAssistantMessage{Content: "", IsFinal: true})
	um := updated.(Model)

	assert.Equal(t, -1, um.findBlockKind(blockAssistantFinal),
		"empty content must not commit a final block")
}

// TestModel_Update_UIAssistantMessage_NewTurn_CommitsPriorTurn is a
// regression for pulumi-service#42775: two consecutive non-final messages
// must each reach scrollback. Previously the second silently overwrote
// the first.
func TestModel_Update_UIAssistantMessage_NewTurn_CommitsPriorTurn(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})

	updated, _ := m.Update(UIAssistantMessage{Content: "I've explored the project."})
	m1 := updated.(Model)
	updated2, cmd := m1.Update(UIAssistantMessage{Content: "Got it — keep the existing bucket."})
	m2 := updated2.(Model)

	var raws []string
	for _, b := range m2.blocks {
		if b.kind == blockAssistantFinal {
			raws = append(raws, b.raw)
		}
	}
	assert.Equal(t, []string{
		"I've explored the project.",
		"Got it — keep the existing bucket.",
	}, raws, "each non-final turn with content must commit its own final block")

	// And both must reach the user via tea.Println, not just sit in m.blocks.
	printed := strings.Join(collectPrintln(cmd), "\n")
	assert.Contains(t, printed, "Got it — keep the existing bucket.",
		"second turn must reach scrollback via tea.Println")
}

// TestModel_Update_UIAssistantMessage_HandoffCommitsToScrollback is a
// regression for a bug where hand-off messages (IsFinal=true,
// HasPendingCLIWork=true) carrying assistant commentary were dropped before
// reaching scrollback. Each hand-off must commit its own final block.
func TestModel_Update_UIAssistantMessage_HandoffCommitsToScrollback(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch, Busy: true})

	// First hand-off: a complete utterance preceding a tool call.
	updated, _ := m.Update(UIAssistantMessage{
		IsFinal: true, HasPendingCLIWork: true, Content: "I'll read the file",
	})
	m1 := updated.(Model)

	idx := m1.findBlockKind(blockAssistantFinal)
	require.GreaterOrEqual(t, idx, 0, "hand-off must commit a final assistant block")
	assert.Equal(t, "I'll read the file", m1.blocks[idx].raw)

	// Second hand-off after the tool runs: must add a second committed block,
	// not overwrite the first.
	updated2, _ := m1.Update(UIAssistantMessage{
		IsFinal: true, HasPendingCLIWork: true, Content: "Now editing it",
	})
	m2 := updated2.(Model)

	finals := 0
	for _, b := range m2.blocks {
		if b.kind == blockAssistantFinal {
			finals++
		}
	}
	assert.Equal(t, 2, finals, "two hand-offs must produce two committed final blocks")
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
	updated, cmd := m.Update(UISessionURL{URL: "https://app.pulumi.com/x"})
	um := updated.(Model)

	assert.Equal(t, "https://app.pulumi.com/x", um.welcome.consoleURL)

	// The session URL is also dropped to terminal scrollback so it survives
	// re-renders. It must arrive wrapped in an OSC 8 hyperlink — without the
	// escape, supporting terminals show plain text and the user can't click
	// through to the task in the console.
	prints := collectPrintln(cmd)
	require.NotEmpty(t, prints, "UISessionURL must emit a tea.Println line")
	joined := strings.Join(prints, "\n")
	assert.Contains(t, joined, "\x1b]8;;https://app.pulumi.com/x\x1b\\",
		"session URL must be wrapped in an OSC 8 hyperlink opener")
	assert.Contains(t, joined, "https://app.pulumi.com/x")
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	assert.Contains(t, idle.viewString(), "enter to send")

	busy := NewModel(ModelConfig{Busy: true})
	assert.Contains(t, busy.viewString(), "agent is working")
	assert.Contains(t, busy.viewString(), "enter disabled")
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
	assert.NotContains(t, m.viewString(), "plan mode on")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = updated.(Model)
	assert.True(t, m.planMode, "Shift+Tab must flip planMode on")
	assert.Contains(t, m.viewString(), "plan mode", "hint must show the plan-mode indicator")

	// Second press toggles back off — same affordance, symmetric behaviour.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = updated.(Model)
	assert.False(t, m.planMode, "second Shift+Tab must flip planMode off")
	assert.NotContains(t, m.viewString(), "plan mode on")
}

func TestModel_Update_ShiftTab_AfterFirstMessage_WarnsAndDoesNotToggle(t *testing.T) {
	t.Parallel()

	// Plan mode is task-level on the wire and is snapshotted the moment the
	// first message is dispatched. A post-send toggle would be misleading —
	// the dispatcher has already captured planMode for CreateNeoTask, so any
	// later flip could not affect the task.
	m := NewModel(ModelConfig{MessageSent: true})

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
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

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	updated2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
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
	idx := um.findBlockKind(blockApprovalPlan)
	require.NotEqual(t, -1, idx)
	assert.Contains(t, um.blocks[idx].rendered, "Proposed plan")
	// Glamour wraps each word in its own ANSI escape run; assert on word
	// fragments that the renderer never splits ("step" shows up verbatim).
	assert.Contains(t, um.blocks[idx].rendered, "step", "rendered plan must include the plan body")
	assert.Contains(t, um.blocks[idx].rendered, "Plan", "rendered plan must include the heading")
	assert.Contains(t, um.approvalPromptText, "Approve plan",
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
	idx := um.findBlockKind(blockApprovalGeneral)
	require.NotEqual(t, -1, idx)
	assert.Contains(t, um.blocks[idx].rendered, "Approval required")
	assert.Contains(t, um.approvalPromptText, "Approve?")
	assert.NotContains(t, um.approvalPromptText, "plan")
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

	updated2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	updated2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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

func TestModel_Update_UIApprovalRequest_AskUser_RendersAsQuestion(t *testing.T) {
	t.Parallel()

	// The agent's ux__ask_user tool reuses user_approval_request to ask
	// clarifying questions. The TUI must NOT render this as an approval —
	// no warning header, no "Approve?" prompt.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_q",
		Message:      "Which region should I deploy to?",
		ApprovalType: "general",
		ToolName:     "ux__ask_user",
	})
	um := updated.(Model)

	assert.True(t, um.pendingIsQuestion, "ask_user must set pendingIsQuestion")
	assert.Equal(t, -1, um.findBlockKind(blockApprovalGeneral),
		"ask_user must NOT route to the general-approval block")
	idx := um.findBlockKind(blockQuestion)
	require.NotEqual(t, -1, idx, "ask_user must commit a blockQuestion")
	rendered := um.blocks[idx].rendered
	assert.NotContains(t, rendered, "Approval required",
		"question rendering must not use the approval-required header")
	assert.NotContains(t, rendered, "⚠",
		"question rendering must not use the warning glyph")
	assert.Contains(t, rendered, "Question",
		"question rendering must include a question header")
	assert.Contains(t, rendered, "Which region should I deploy to?",
		"question body must be rendered verbatim")
	assert.Contains(t, um.approvalPromptText, "Your answer",
		"prompt must invite a free-form answer, not an approval")
	assert.NotContains(t, um.approvalPromptText, "Approve",
		"prompt must not say 'Approve?' for a question")
}

func TestModel_Update_UIApprovalRequest_AskUser_BareToolName(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_q2",
		Message:      "Pick a runtime.",
		ApprovalType: "general",
		ToolName:     "ask_user",
	})
	um := updated.(Model)

	assert.True(t, um.pendingIsQuestion)
	assert.NotEqual(t, -1, um.findBlockKind(blockQuestion))
}

func TestModel_Update_UIApprovalRequest_AskUser_PlanExitWinsOverToolName(t *testing.T) {
	t.Parallel()

	// approval_type "plan_exit" is the highest-priority discriminator: a
	// hypothetical plan_exit request that also carries a ux__ask_user tool
	// name must render as a plan, never a question.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_p",
		Message:         "intro",
		ApprovalType:    approvalTypePlanExit,
		ToolName:        "ux__ask_user", // ignored when approval_type is plan_exit
		PlanDescription: "# Plan\n\n- step",
	})
	um := updated.(Model)

	assert.False(t, um.pendingIsQuestion, "plan_exit must not be treated as a question")
	assert.Equal(t, -1, um.findBlockKind(blockQuestion),
		"plan_exit must not produce a blockQuestion")
	assert.NotEqual(t, -1, um.findBlockKind(blockApprovalPlan),
		"plan_exit must produce a blockApprovalPlan")
}

func TestModel_Update_UIApprovalRequest_GeneralWithoutAskUser_StillApproval(t *testing.T) {
	t.Parallel()

	// Sanity check that real "general" approvals (no ask_user tool name)
	// still take the existing approval path. Guards against a regression
	// where the new branch over-matches.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_real",
		Message:      "Run pulumi up?",
		ApprovalType: "general",
		// no ToolName at all — represents a real approval-gated tool
	})
	um := updated.(Model)

	assert.False(t, um.pendingIsQuestion)
	assert.NotEqual(t, -1, um.findBlockKind(blockApprovalGeneral))
	assert.Contains(t, um.approvalPromptText, "Approve?")
}

func TestModel_Update_AnswerQuestion_SendsConfirmationWithAnswer(t *testing.T) {
	t.Parallel()

	// Pressing Enter on a question must send the typed text as the answer.
	// Wire reply is Approved=false, Message=<answer> — the backend's
	// ask_user tool wrapper converts ok=false+instructions into a
	// tool_response delivering the answer to the agent.
	outCh := make(chan outboundEvent, 1)
	evCh := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{OutCh: outCh, EventCh: evCh})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_q3",
		Message:      "Which region?",
		ApprovalType: "general",
		ToolName:     "ux__ask_user",
	})
	m = updated.(Model)
	require.True(t, m.pendingIsQuestion)

	answer := "us-west-2 with a hot spare in eu-central-1"
	m.textInput.SetValue(answer)

	updated2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated2.(Model)

	select {
	case got := <-outCh:
		conf, ok := got.event.(apitype.AgentUserEventUserConfirmation)
		require.True(t, ok, "expected AgentUserEventUserConfirmation, got %T", got.event)
		assert.False(t, conf.Approved,
			"answer is sent with Approved=false to match the backend ask_user wrapper contract")
		assert.Equal(t, "appr_q3", conf.ApprovalID)
		assert.Equal(t, answer, conf.Message,
			"the typed text must be sent verbatim as instructions")
	default:
		t.Fatal("answering a question must post a confirmation event")
	}

	assert.False(t, m.pendingApproval, "submitting an answer must clear pendingApproval")
	assert.False(t, m.pendingIsQuestion, "pendingIsQuestion must be cleared on submit")
	assert.Empty(t, m.pendingApprovalType)

	idx := m.findBlockKind(blockAnswerSubmitted)
	require.NotEqual(t, -1, idx, "submitting must commit a blockAnswerSubmitted")
	rendered := m.blocks[idx].rendered
	assert.Contains(t, rendered, "Answered",
		"the post-submit block must read 'Answered', not 'Denied'")
	assert.NotContains(t, rendered, "Denied",
		"the post-submit block must NOT read 'Denied' for a question answer")
	assert.Contains(t, rendered, answer)
}

func TestModel_Update_AnswerQuestion_EmptyInputIsNoOp(t *testing.T) {
	t.Parallel()

	// Empty input + Enter must not produce an outbound event and must
	// leave the question pending.
	outCh := make(chan outboundEvent, 1)
	evCh := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{OutCh: outCh, EventCh: evCh})

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_q4",
		Message:      "Pick a region.",
		ApprovalType: "general",
		ToolName:     "ux__ask_user",
	})
	m = updated.(Model)

	updated2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated2.(Model)

	select {
	case got := <-outCh:
		t.Fatalf("empty Enter must not post a confirmation event; got %#v", got.event)
	default:
	}

	assert.True(t, m.pendingApproval, "empty Enter must leave the question pending")
	assert.True(t, m.pendingIsQuestion, "pendingIsQuestion must remain set")
}

func TestQuestionWrapsToTerminalWidth(t *testing.T) {
	t.Parallel()

	// Long question bodies wrap rather than clipping at the viewport edge.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = updated0.(Model)

	long := strings.Repeat("word ", 25) // ~125 chars
	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr_q5",
		Message:      long,
		ApprovalType: "general",
		ToolName:     "ux__ask_user",
	})
	um := updated.(Model)

	idx := um.findBlockKind(blockQuestion)
	require.NotEqual(t, -1, idx)
	widths := visibleLines(um.blocks[idx].rendered)
	require.Greater(t, len(widths), 1,
		"long question body must wrap; got: %q", um.blocks[idx].rendered)
	for i, w := range widths {
		assert.LessOrEqual(t, w, 40, "line %d exceeds terminal width: width=%d", i, w)
	}
}

func TestModel_Update_UITodoList_OutsidePlanModeCommitsBlock(t *testing.T) {
	t.Parallel()

	// Outside plan mode every TodoWrite is rendered immediately so the user
	// sees status flips ("step completed") and edits land in scrollback.
	m := NewModel(ModelConfig{})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	updated, cmd := m.Update(UITodoList{Items: []UITodoItem{
		{Content: "set up project", Index: 0, Status: "completed"},
		{Content: "add lambda", Index: 1, Status: "in_progress"},
		{Content: "run preview", Index: 2, Status: "pending"},
	}})
	um := updated.(Model)

	idx := um.findBlockKind(blockTodoList)
	require.NotEqual(t, -1, idx, "TodoWrite outside plan mode must commit a blockTodoList")
	rendered := um.blocks[idx].rendered
	assert.Contains(t, rendered, "TODO")
	assert.Contains(t, rendered, "[x] ")
	assert.Contains(t, rendered, "[~] ")
	assert.Contains(t, rendered, "[ ] ")
	// Index ordering is load-bearing: the agent communicates "next step"
	// implicitly via priority order.
	assert.Less(t, strings.Index(rendered, "set up project"), strings.Index(rendered, "add lambda"))
	assert.Less(t, strings.Index(rendered, "add lambda"), strings.Index(rendered, "run preview"))

	printed := collectPrintln(cmd)
	require.Len(t, printed, 1, "outside plan mode the TODO block must be printed to scrollback")
	assert.Contains(t, printed[0], "set up project")
}

func TestModel_Update_UITodoList_InPlanModeBuffersAndDoesNotCommit(t *testing.T) {
	t.Parallel()

	// In plan mode the list is held back so it can be folded into the
	// upcoming Proposed plan block — committing now would split the visual
	// unit the user picked the embedded layout for.
	m := NewModel(ModelConfig{})
	m.planMode = true
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	updated, cmd := m.Update(UITodoList{Items: []UITodoItem{
		{Content: "draft plan", Index: 0, Status: "pending"},
	}})
	um := updated.(Model)

	assert.Equal(t, -1, um.findBlockKind(blockTodoList),
		"plan-mode TodoWrite must not commit a standalone block")
	require.Len(t, um.pendingTodos, 1)
	assert.Equal(t, "draft plan", um.pendingTodos[0].Content)
	assert.Empty(t, collectPrintln(cmd), "plan-mode TodoWrite must not write to scrollback yet")
}

func TestModel_Update_PlanExitFoldsBufferedTodosIntoPlanBlock(t *testing.T) {
	t.Parallel()

	// The buffered list must surface as a Tasks: subsection inside the
	// single Proposed plan block — that's the layout the user chose so the
	// plan and the tasks read as one unit in scrollback.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	m.planMode = true
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	updated1, _ := m.Update(UITodoList{Items: []UITodoItem{
		{Content: "alpha task", Index: 0, Status: "pending"},
		{Content: "beta task", Index: 1, Status: "pending"},
	}})
	m = updated1.(Model)

	updated2, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_plan",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "# Plan\n\n- step one",
	})
	um := updated2.(Model)

	assert.Empty(t, um.pendingTodos, "buffered todos must be drained on plan_exit")
	idx := um.findBlockKind(blockApprovalPlan)
	require.NotEqual(t, -1, idx)
	rendered := um.blocks[idx].rendered
	assert.Contains(t, rendered, "Proposed plan")
	assert.Contains(t, rendered, "Tasks:")
	assert.Contains(t, rendered, "alpha task")
	assert.Contains(t, rendered, "beta task")
	// Single block — no separate blockTodoList ever materialized.
	assert.Equal(t, -1, um.findBlockKind(blockTodoList),
		"plan-exit must not produce a separate TODO block")
}

func TestModel_Update_PlanModeUITodoList_OnlyLatestSnapshotIsRendered(t *testing.T) {
	t.Parallel()

	// Several TodoWrites can arrive in plan mode (the agent drafts and
	// revises). The plan_exit block must reflect only the most recent
	// snapshot so users don't see superseded work.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	m.planMode = true
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	updated1, _ := m.Update(UITodoList{Items: []UITodoItem{
		{Content: "stale entry", Index: 0, Status: "pending"},
	}})
	m = updated1.(Model)
	updated2, _ := m.Update(UITodoList{Items: []UITodoItem{
		{Content: "latest entry", Index: 0, Status: "pending"},
	}})
	m = updated2.(Model)

	updated3, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_plan2",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "Plan body.",
	})
	um := updated3.(Model)

	idx := um.findBlockKind(blockApprovalPlan)
	require.NotEqual(t, -1, idx)
	rendered := um.blocks[idx].rendered
	assert.Contains(t, rendered, "latest entry")
	assert.NotContains(t, rendered, "stale entry",
		"superseded list entries must not leak into the plan block")
}

func TestModel_Update_PlanExitWithoutBufferedTodosRendersCleanPlan(t *testing.T) {
	t.Parallel()

	// When no TodoWrite has fired during plan mode, the plan block must
	// render exactly as before — no spurious Tasks: header attached.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	m.planMode = true
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr_plan3",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "# Plan\n\nbody only",
	})
	um := updated.(Model)

	idx := um.findBlockKind(blockApprovalPlan)
	require.NotEqual(t, -1, idx)
	rendered := um.blocks[idx].rendered
	assert.Contains(t, rendered, "Proposed plan")
	assert.NotContains(t, rendered, "Tasks:",
		"a plan with no buffered todos must not advertise an empty Tasks: section")
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

// -----------------------------------------------------------------------------
// Text reflow: wrapping and resize re-render
// -----------------------------------------------------------------------------

// visibleLines returns each line's ANSI-stripped visible width.
func visibleLines(rendered string) []int {
	lines := strings.Split(rendered, "\n")
	widths := make([]int, len(lines))
	for i, l := range lines {
		widths[i] = lipgloss.Width(l)
	}
	return widths
}

func TestWarningWrapsToTerminalWidth(t *testing.T) {
	t.Parallel()

	// Long warning must wrap rather than clip at the viewport edge.
	msg := "this is an intentionally long warning message that must span multiple lines when the terminal is narrow"
	got := renderIndented(warningStyle, 40, "⚠ "+msg)

	widths := visibleLines(got)
	require.Greater(t, len(widths), 1, "long warning must wrap to multiple lines; got: %q", got)
	for i, w := range widths {
		assert.LessOrEqual(t, w, 40, "line %d exceeds terminal width: width=%d", i, w)
	}
	assert.Contains(t, got, "warning", "wrapped output must still contain the message body")
}

func TestUserBubbleWrapsToTerminalWidth(t *testing.T) {
	t.Parallel()

	m := &Model{width: 40}
	long := strings.Repeat("word ", 25) // ~125 chars
	got := m.renderUserBubble(long)

	require.Contains(t, got, "\n", "long bubble must contain a newline (wrapped): %q", got)
	for i, w := range visibleLines(got) {
		assert.LessOrEqual(t, w, 40, "bubble line %d exceeds terminal width: width=%d", i, w)
	}
}

func TestUserBubbleDoesNotPadShortMessages(t *testing.T) {
	t.Parallel()

	// Short messages hug content so the bubble looks like a chat bubble,
	// not a full-width card.
	m := &Model{width: 80}
	got := m.renderUserBubble("hi")

	widths := visibleLines(got)
	require.Len(t, widths, 1, "short bubble should render on a single line: %q", got)
	assert.Less(t, widths[0], 20, "short bubble line should hug content, not fill terminal; got width=%d", widths[0])
}

func TestUserBubbleWrapsAtWideTerminal(t *testing.T) {
	t.Parallel()

	// At a wide terminal the bubble must wrap against liveWidth (m.width-4),
	// not raw m.width — otherwise wrapped lines sit on the terminal wrap
	// column and desync the inline-renderer line accounting.
	const termWidth = 200
	m := &Model{width: termWidth}
	long := strings.Repeat("word ", 60) // ~300 chars, forces wrap
	got := m.renderUserBubble(long)

	widths := visibleLines(got)
	require.Greater(t, len(widths), 1, "long bubble at wide terminal must wrap; got: %q", got)
	for i, w := range widths {
		assert.LessOrEqual(t, w, m.liveWidth(), "bubble line %d sits past liveWidth: width=%d", i, w)
	}
}

func TestLiveWidth_Boundaries(t *testing.T) {
	t.Parallel()

	// liveWidth contract: wide terminals back off by a 4-col cushion so
	// rendered content stays inside the wrap column; at or below the
	// minUsableWidth threshold (40) we hand back the raw width so something
	// still renders on a degenerate terminal.
	cases := []struct {
		width int
		want  int
	}{
		{width: 0, want: 0},
		{width: 1, want: 1},
		{width: 40, want: 40},  // boundary: not > minUsableWidth, no cushion
		{width: 41, want: 37},  // first width that gets the cushion
		{width: 80, want: 76},  // typical narrow terminal
		{width: 100, want: 96}, // matches the welcome.termWidth assertion
		{width: 200, want: 196},
	}
	for _, tc := range cases {
		m := &Model{width: tc.width}
		assert.Equal(t, tc.want, m.liveWidth(), "liveWidth(width=%d)", tc.width)
	}
}

func TestResizeReRendersWidthDependentBlocks(t *testing.T) {
	t.Parallel()

	// Blocks cached at event time must re-wrap on resize, not stay at the
	// old width forever.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})

	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	long := strings.Repeat("word ", 25)
	updated1, _ := m.Update(UIWarning{Message: long})
	m = updated1.(Model)

	idx := m.findBlockKind(blockWarning)
	require.NotEqual(t, -1, idx)
	widthsAt80 := visibleLines(m.blocks[idx].rendered)
	for i, w := range widthsAt80 {
		assert.LessOrEqual(t, w, 80, "line %d at width 80 exceeds: width=%d", i, w)
	}

	updated2, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = updated2.(Model)

	widthsAt40 := visibleLines(m.blocks[idx].rendered)
	assert.Greater(t, len(widthsAt40), len(widthsAt80),
		"resize to 40 cols must produce more lines than the 80-col render; 80=%d lines, 40=%d lines",
		len(widthsAt80), len(widthsAt40))
	for i, w := range widthsAt40 {
		assert.LessOrEqual(t, w, 40, "line %d at width 40 exceeds: width=%d", i, w)
	}
}

func TestRenderBlock_SkipsWidthIndependentBlocks(t *testing.T) {
	t.Parallel()

	// Empty-raw kinds (blockBusy, blockToolComplete) keep their pre-styled
	// rendered string untouched on resize.
	m := &Model{width: 40}
	b := block{kind: blockToolComplete, rendered: "  ⏺ Read(\"/x\")"}
	m.renderBlock(&b)
	assert.Equal(t, "  ⏺ Read(\"/x\")", b.rendered, "empty raw must be a no-op")
}

func TestModel_LiveView_OnlyShowsLiveBlocks(t *testing.T) {
	t.Parallel()

	// In inline mode, View() draws only the live frame: the busy spinner, an
	// in-flight streaming assistant, and an open pulumi op. Committed blocks
	// (user messages, tool completions, finals, warnings) live in terminal
	// scrollback and must not appear in View(). Render one of each kind so
	// the filter is exercised, and check that committed kinds are absent
	// from the visible View string.
	m := NewModel(ModelConfig{})
	m.blocks = []block{
		{kind: blockUserMessage, rendered: "USERSCROLLBACK"},
		{kind: blockToolComplete, rendered: "TOOLCOMPLETESCROLLBACK"},
		{kind: blockAssistantFinal, rendered: "FINALSCROLLBACK"},
		{kind: blockWarning, rendered: "WARNSCROLLBACK"},
		{kind: blockBusy, label: "Thinking...", shimmer: shimmerVerb},
	}

	view := m.viewString()
	// Live blocks are visible. Shimmer styles each char with its own ANSI
	// run, so strip escapes before substring-matching the label.
	assert.Contains(t, stripansi.Strip(view), "Thinking", "busy label must appear in View")
	// Committed blocks are NOT visible — they were emitted to scrollback.
	assert.NotContains(t, view, "USERSCROLLBACK")
	assert.NotContains(t, view, "TOOLCOMPLETESCROLLBACK")
	assert.NotContains(t, view, "FINALSCROLLBACK")
	assert.NotContains(t, view, "WARNSCROLLBACK")
	require.Len(t, m.blocks, 5, "View must not mutate the blocks slice")
}

func TestApprovalGeneralWrapsToTerminalWidth(t *testing.T) {
	t.Parallel()

	// Long approval message wraps rather than clipping at the viewport edge.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = updated0.(Model)

	long := strings.Repeat("word ", 25) // ~125 chars
	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr",
		Message:      long,
		ApprovalType: "general",
	})
	um := updated.(Model)

	idx := um.findBlockKind(blockApprovalGeneral)
	require.NotEqual(t, -1, idx)
	widths := visibleLines(um.blocks[idx].rendered)
	require.Greater(t, len(widths), 1,
		"long approval body must wrap to multiple lines; got: %q", um.blocks[idx].rendered)
	for i, w := range widths {
		assert.LessOrEqual(t, w, 40, "line %d exceeds terminal width: width=%d", i, w)
	}
}

func TestApprovalPlanReflowsOnResize(t *testing.T) {
	t.Parallel()

	// Plan markdown must re-render through glamour on resize instead of
	// staying pinned to the width it arrived at.
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated0.(Model)

	longParagraph := strings.Repeat("alpha ", 30) // ~180 chars, glamour-wrappable
	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID:      "appr",
		Message:         "intro",
		ApprovalType:    approvalTypePlanExit,
		PlanDescription: "# Plan\n\n" + longParagraph,
	})
	m = updated.(Model)

	idx := m.findBlockKind(blockApprovalPlan)
	require.NotEqual(t, -1, idx)
	linesAt80 := len(visibleLines(m.blocks[idx].rendered))

	updated2, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = updated2.(Model)

	linesAt40 := len(visibleLines(m.blocks[idx].rendered))
	assert.Greater(t, linesAt40, linesAt80,
		"resize to 40 cols must produce more wrapped lines than the 80-col render; 80=%d lines, 40=%d lines",
		linesAt80, linesAt40)
}

func TestApprovalChoiceDenialWrapsToTerminalWidth(t *testing.T) {
	t.Parallel()

	// Denial text can be up to textinput's 4096-char limit; must wrap.
	outCh := make(chan outboundEvent, 1)
	evCh := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{OutCh: outCh, EventCh: evCh})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	m = updated0.(Model)

	updated1, _ := m.Update(UIApprovalRequest{
		ApprovalID:   "appr",
		Message:      "run something",
		ApprovalType: "general",
	})
	m = updated1.(Model)
	m.textInput.SetValue(strings.Repeat("because ", 20)) // ~160 chars of denial text

	updated2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated2.(Model)
	<-outCh

	idx := m.findBlockKind(blockApprovalChoice)
	require.NotEqual(t, -1, idx)
	widths := visibleLines(m.blocks[idx].rendered)
	require.Greater(t, len(widths), 1,
		"long denial must wrap to multiple lines; got: %q", m.blocks[idx].rendered)
	for i, w := range widths {
		assert.LessOrEqual(t, w, 40, "line %d exceeds terminal width: width=%d", i, w)
	}
}

func TestApprovalChoiceApprovedStaysSingleLine(t *testing.T) {
	t.Parallel()

	// Approved carries its verdict in block.approved, not raw — the raw==""
	// early-exit must not short-circuit this kind.
	m := &Model{width: 80}
	b := block{kind: blockApprovalChoice, approved: true}
	m.renderBlock(&b)

	widths := visibleLines(b.rendered)
	require.Len(t, widths, 1, "approved verdict must render on a single line: %q", b.rendered)
	assert.Contains(t, b.rendered, "Approved")
}

func TestModel_LiveView_PulumiOpLiveVsCommitted(t *testing.T) {
	t.Parallel()

	// In inline mode, an open pulumi op block (done==false) belongs in the
	// live frame above the input. Once UIPulumiEnd fires it flips to done==true
	// and its rendered string is committed to scrollback via tea.Println — at
	// that point it must drop out of View(). A future refactor that flips
	// isLiveKind's pulumi predicate would either double-print the block (live
	// frame + scrollback) or drop the running summary entirely.
	m := NewModel(ModelConfig{})
	m.blocks = []block{
		{kind: blockPulumiOp, rendered: "PULUMI_OPEN_LIVE", pulumi: &pulumiBlockState{done: false}},
		{kind: blockPulumiOp, rendered: "PULUMI_DONE_SCROLLBACK", pulumi: &pulumiBlockState{done: true}},
	}

	view := m.viewString()
	assert.Contains(t, view, "PULUMI_OPEN_LIVE",
		"open pulumi op (done=false) must appear in the live frame")
	assert.NotContains(t, view, "PULUMI_DONE_SCROLLBACK",
		"finalized pulumi op (done=true) must not appear in View — it was committed to scrollback")
}

func TestModel_Update_FirstWindowSize_EmitsWelcomeAndInitialPromptToScrollback(t *testing.T) {
	t.Parallel()

	// NewModel queues an InitialPrompt as a committed user-message block but
	// can't emit it yet — the welcome banner needs the real terminal width
	// first. The first WindowSizeMsg is the moment we know the width, so it
	// must (a) tea.Println the welcome banner and (b) tea.Println every
	// pre-seeded committed block. Subsequent WindowSizeMsgs must NOT re-emit;
	// otherwise resizing the terminal would stack duplicate banners into
	// scrollback.
	m := NewModel(ModelConfig{InitialPrompt: "deploy prod"})
	require.False(t, m.sizeReceived, "fresh model has not received a size yet")

	updated, sizeCmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	um := updated.(Model)
	assert.True(t, um.sizeReceived, "first WindowSize must flip sizeReceived")

	// The welcome and pre-seeded prompt are deferred behind a tea.Tick (see
	// firstFlushReadyMsg). Drive the tick manually so the test exercises the
	// post-defer scrollback emission rather than the empty pre-tick path.
	updated2, cmd := um.Update(runFirstFlushTick(t, sizeCmd))
	um = updated2.(Model)

	printed := collectPrintln(cmd)
	welcomeMatches := 0
	promptMatches := 0
	for _, line := range printed {
		if strings.Contains(line, "Pulumi Neo") {
			welcomeMatches++
		}
		if strings.Contains(line, "deploy prod") {
			promptMatches++
		}
	}
	assert.Equal(t, 1, welcomeMatches, "welcome banner must reach scrollback exactly once; got: %v", printed)
	assert.Equal(t, 1, promptMatches, "InitialPrompt user block must reach scrollback exactly once; got: %v", printed)

	// A subsequent WindowSizeMsg is just a resize — no new scrollback.
	_, cmd2 := um.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	printed2 := collectPrintln(cmd2)
	for _, line := range printed2 {
		assert.NotContains(t, line, "Pulumi Neo", "second resize must not re-emit the welcome banner")
		assert.NotContains(t, line, "deploy prod", "second resize must not re-emit the initial-prompt block")
	}
}

func TestModel_Update_UIPulumiEnd_CommitsRenderedToScrollback(t *testing.T) {
	t.Parallel()

	// UIPulumiEnd flips the open pulumi op block to done==true (live →
	// committed) and emits its rendered form via tea.Println so the final
	// summary lives in terminal scrollback. Without this emission, a finished
	// pulumi run would silently disappear from view (it leaves the live frame
	// the moment done flips, but never gets printed above).
	ch := make(chan UIEvent, 4)
	m := NewModel(ModelConfig{EventCh: ch})
	updated0, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated0.(Model)

	updated1, _ := m.Update(UIPulumiStart{
		ToolName: "pulumi__pulumi_preview", StackName: "dev", IsPreview: true,
	})
	m = updated1.(Model)
	updated2, _ := m.Update(UIPulumiResource{
		ToolName: "pulumi__pulumi_preview",
		Op:       deploy.OpCreate,
		URN:      "urn:pulumi:dev::p::aws:s3/Bucket::b",
		Type:     "aws:s3/Bucket",
		Status:   "planned",
	})
	m = updated2.(Model)

	_, endCmd := m.Update(UIPulumiEnd{
		ToolName: "pulumi__pulumi_preview",
		Counts:   display.ResourceChanges{deploy.OpCreate: 1},
		Elapsed:  "1.2s",
	})

	printed := collectPrintln(endCmd)
	matches := 0
	for _, line := range printed {
		if strings.Contains(line, "PulumiPreview") {
			matches++
		}
	}
	assert.GreaterOrEqual(t, matches, 1,
		"UIPulumiEnd must tea.Println the rendered pulumi block; got: %v", printed)
}

func TestModel_WrapPlain_TinyWidthShortCircuits(t *testing.T) {
	t.Parallel()

	// wrapPlain backs off to identity when liveWidth <= 4 — wordwrap.String
	// with a non-positive boundary has historically panicked or produced
	// off-by-one breakage in some reflow versions. The guard exists so a
	// pathologically narrow terminal doesn't crash the TUI.
	long := "a fairly long sentence that would otherwise wrap"

	m := NewModel(ModelConfig{})
	m.width = 1 // liveWidth returns m.width directly when at or below minUsableWidth
	require.LessOrEqual(t, m.liveWidth(), 4, "test relies on tiny liveWidth path")
	assert.Equal(t, long, m.wrapPlain(long), "tiny-width path must return input verbatim")

	// Sanity: at a normal width, wrapPlain inserts at least one newline so
	// we know the short-circuit isn't accidentally absorbing every input.
	m.width = 100
	wrapped := m.wrapPlain(strings.Repeat("word ", 30))
	assert.Contains(t, wrapped, "\n", "normal-width path must wrap into multiple lines")
}

// -----------------------------------------------------------------------------
// Conversation spacing (issue #42472)
// -----------------------------------------------------------------------------

// TestPrintlnBlock_FirstEmissionIsBare guards the welcome-banner case: the
// first scrollback emission ever must NOT carry a leading blank line, so the
// session opens with the banner pinned to the top of the transcript.
func TestPrintlnBlock_FirstEmissionIsBare(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	cmd := m.printlnBlock("hello")
	require.True(t, m.hasEmittedScrollback, "first call must flip the latch")

	got := collectPrintln(cmd)
	require.Len(t, got, 1)
	assert.Equal(t, "hello", got[0], "first emission must be exactly the body, no leading newline")
}

// TestPrintlnBlock_SubsequentEmissionsLeadByNewline pins the spacing rule for
// every block after the first: a single leading "\n" so each block is
// visually separated from whatever sits above it in scrollback.
func TestPrintlnBlock_SubsequentEmissionsLeadByNewline(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	_ = m.printlnBlock("welcome") // burn the first-emission slot

	cmd := m.printlnBlock("hello")
	got := collectPrintln(cmd)
	require.Len(t, got, 1)
	assert.Equal(t, "\nhello", got[0],
		"subsequent emissions must start with a single \\n so blocks have a blank-line gap")
}

// TestTranscriptSpacing_FullSequence drives a representative session through
// the model and asserts every committed scrollback emission after the welcome
// carries exactly one leading "\n". This is the regression test for #42472:
// without per-block spacing, blocks render directly under each other.
func TestTranscriptSpacing_FullSequence(t *testing.T) {
	t.Parallel()

	ch := make(chan UIEvent, 16)
	m := tea.Model(NewModel(ModelConfig{EventCh: ch}))

	// First WindowSize defers the welcome behind a tea.Tick (firstFlushReadyMsg).
	// Drive the tick so we observe the actual scrollback emission.
	var sizeCmd tea.Cmd
	m, sizeCmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	var welcomeCmd tea.Cmd
	m, welcomeCmd = m.Update(runFirstFlushTick(t, sizeCmd))
	welcomePrinted := collectPrintln(welcomeCmd)
	require.NotEmpty(t, welcomePrinted, "first WindowSize must emit the welcome banner")
	assert.False(t, strings.HasPrefix(welcomePrinted[0], "\n"),
		"welcome banner must be the bare first emission, with no leading blank line")

	// Drive a sequence of events that each commit a block to scrollback.
	// UIUserMessage is a foreign message (no matching pendingUserEcho) so it
	// renders. UIAssistantMessage with IsFinal=true commits a final block.
	// UIWarning and UIError commit warning and error blocks. The combined
	// transcript must have a blank line between every consecutive pair.
	events := []UIEvent{
		UIUserMessage{Content: "hello from web ui"},
		UIAssistantMessage{Content: "hi back", IsFinal: true},
		UIToolStarted{Name: "shell__exec"},
		UIToolCompleted{Name: "shell__exec"},
		UIWarning{Message: "watch out"},
		UIError{Message: "boom"},
	}

	followUpPrinted := make([]string, 0, len(events))
	for _, ev := range events {
		var cmd tea.Cmd
		m, cmd = m.Update(ev)
		followUpPrinted = append(followUpPrinted, collectPrintln(cmd)...)
	}

	require.NotEmpty(t, followUpPrinted, "events should have produced at least one Println")
	for i, body := range followUpPrinted {
		assert.True(t, strings.HasPrefix(body, "\n"),
			"emission #%d after the welcome must start with \\n; got %q", i, body)
		assert.False(t, strings.HasPrefix(body, "\n\n"),
			"emission #%d must not double up the gap (only one leading \\n); got %q", i, body)
	}
}

// TestView_LeadingBlankLine_Idle and _Busy both lock in the spacing rule: a
// blank gap line sits above the live frame so the last committed block in
// scrollback is separated from the input zone, but the spinner and the
// prompt stay flush so the busy indicator reads as part of the input.
func TestView_LeadingBlankLine_Idle(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	m.width = 80
	m.height = 24
	require.Empty(t, m.blocks, "test relies on an idle model with no blocks")

	view := m.viewString()
	assert.True(t, strings.HasPrefix(view, "\n"),
		"View() must start with a blank line so the prompt is separated from the last scrollback block; got: %q", view)
	// After the leading blank, the next thing must be the prompt — not a
	// second blank line. The textarea wraps the "❯ " prompt in ANSI escapes,
	// so strip them before doing the prefix check.
	stripped := stripansi.Strip(view)
	assert.True(t, strings.HasPrefix(stripped, "\n❯ "),
		"idle View() must put the prompt immediately after the leading blank; got: %q", stripped)
}

func TestView_LeadingBlankLine_Busy_SpinnerFlushWithPrompt(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	m.width = 80
	m.height = 24
	m.blocks = []block{{kind: blockBusy, label: "Thinking...", shimmer: shimmerVerb}}

	view := stripansi.Strip(m.viewString())
	require.True(t, strings.HasPrefix(view, "\n"),
		"View() must start with a blank line above the live frame; got: %q", view)

	// Spinner appears in the live frame, prompt below it, with NO blank line
	// between them — the spinner reads as part of the input zone.
	idx := strings.Index(view, "❯ ")
	require.Greater(t, idx, 0, "prompt must appear after the live frame")
	above := view[:idx]
	assert.False(t, strings.HasSuffix(above, "\n\n"),
		"there must be NO blank line between the busy spinner and the prompt — they read as one zone; got: %q", above)
	assert.Contains(t, above, "Thinking",
		"busy spinner label must appear above the prompt")
}

// TestLiveView_BlankBetweenLiveBlocks pins the inter-block gap inside the live
// frame when an open pulumi op is pending under the busy spinner.
func TestLiveView_BlankBetweenLiveBlocks(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	m.width = 80
	m.blocks = []block{
		{kind: blockPulumiOp, rendered: "PULUMI_LIVE", pulumi: &pulumiBlockState{done: false}},
		{kind: blockBusy, label: "Thinking...", shimmer: shimmerVerb},
	}

	live := stripansi.Strip(m.liveView())
	require.Contains(t, live, "PULUMI_LIVE")
	require.Contains(t, live, "Thinking")

	// The pulumi-op → busy pair must be separated by a blank line. We can't
	// anchor on exact line counts (the busy line and rendered string may wrap
	// or embed ANSI), so check that "\n\n" appears between them.
	pulumiIdx := strings.Index(live, "PULUMI_LIVE")
	thinkingIdx := strings.Index(live, "Thinking")
	require.Less(t, pulumiIdx, thinkingIdx)
	assert.Contains(t, live[pulumiIdx:thinkingIdx], "\n\n",
		"pulumi-op → busy gap must be a full blank line")
}

func TestModel_Update_CtrlA_CyclesApprovalMode(t *testing.T) {
	t.Parallel()

	// Ctrl+A advances the approval mode through manual → balanced → auto →
	// manual. Pre-first-message there is no PATCH dispatch — the value lives
	// purely in the TUI snapshot until the user sends their first message.
	outCh := make(chan outboundEvent, 4)
	m := NewModel(ModelConfig{
		OutCh:               outCh,
		InitialApprovalMode: client.NeoApprovalModeManual,
	})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoApprovalModeBalanced, m.approvalMode)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoApprovalModeAuto, m.approvalMode)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoApprovalModeManual, m.approvalMode,
		"third Ctrl+A must wrap back to manual")

	// Pre-message toggles must not put anything on outCh — the snapshot is
	// only committed on the first user_message.
	select {
	case ev := <-outCh:
		t.Fatalf("pre-message Ctrl+A must not dispatch an outbound event, got %#v", ev)
	default:
	}
}

func TestModel_Update_CtrlA_AfterFirstMessage_DispatchesUpdate(t *testing.T) {
	t.Parallel()

	// Once the first message has been sent, Ctrl+A must both flip the local
	// snapshot AND dispatch an outboundEvent.update so the runNeo dispatcher
	// PATCHes the live task. Without that, cloud ApprovalHandler would still
	// see the original mode and prompt at the wrong cadence.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{
		OutCh:               outCh,
		MessageSent:         true,
		TaskCreated:         true,
		InitialApprovalMode: client.NeoApprovalModeManual,
	})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoApprovalModeBalanced, m.approvalMode)

	// The keypress only schedules the debounce tick; nothing is on outCh yet.
	select {
	case ev := <-outCh:
		t.Fatalf("Ctrl+A must defer the dispatch behind the debounce tick, got %#v", ev)
	default:
	}

	// Deliver the debounce tick with the current gen — that's what tea.Tick
	// would do after modeToggleDebounce — and assert the PATCH lands.
	updated, _ = m.Update(approvalDebounceTickMsg{gen: m.approvalDebounceGen})
	_ = updated

	select {
	case got := <-outCh:
		require.NotNil(t, got.update, "the debounce tick must dispatch an update")
		require.NotNil(t, got.update.ApprovalMode, "update must carry the new approvalMode")
		assert.Equal(t, client.NeoApprovalModeBalanced, *got.update.ApprovalMode)
		assert.Nil(t, got.update.PermissionMode,
			"approval-mode toggle must not include permissionMode")
	default:
		t.Fatal("the debounce tick at the current gen must dispatch")
	}
}

func TestModel_Update_CtrlA_RapidPressesCollapseToOneDispatch(t *testing.T) {
	t.Parallel()

	// Three Ctrl+A presses in quick succession (faster than the debounce
	// window) must collapse to a single PATCH carrying the FINAL mode value.
	// Each press advances the debounce gen, so only the last-scheduled tick
	// fires the dispatch — the earlier two are gen-mismatched and silently
	// dropped.
	outCh := make(chan outboundEvent, 4)
	m := NewModel(ModelConfig{
		OutCh:               outCh,
		MessageSent:         true,
		TaskCreated:         true,
		InitialApprovalMode: client.NeoApprovalModeManual,
	})

	// Three presses: manual → balanced → auto → manual.
	for range 3 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
		m = updated.(Model)
	}
	require.Equal(t, client.NeoApprovalModeManual, m.approvalMode,
		"three presses must wrap back to manual")
	require.Equal(t, 3, m.approvalDebounceGen, "each press advances the debounce gen")

	// Stale ticks from the first two presses arrive — both must no-op.
	updated, _ := m.Update(approvalDebounceTickMsg{gen: 1})
	m = updated.(Model)
	updated, _ = m.Update(approvalDebounceTickMsg{gen: 2})
	m = updated.(Model)
	select {
	case ev := <-outCh:
		t.Fatalf("stale-gen ticks must not dispatch, got %#v", ev)
	default:
	}

	// The latest tick fires the dispatch with the final mode value.
	updated, _ = m.Update(approvalDebounceTickMsg{gen: 3})
	_ = updated

	select {
	case got := <-outCh:
		require.NotNil(t, got.update)
		require.NotNil(t, got.update.ApprovalMode)
		assert.Equal(t, client.NeoApprovalModeManual, *got.update.ApprovalMode,
			"the collapsed dispatch must carry the FINAL mode after all presses")
	default:
		t.Fatal("the current-gen tick must dispatch")
	}

	// And no further events on outCh.
	select {
	case ev := <-outCh:
		t.Fatalf("only one dispatch must land for three rapid presses, got extra %#v", ev)
	default:
	}
}

func TestModel_Update_ApprovalDebounceTick_StaleGenDoesNotDispatch(t *testing.T) {
	t.Parallel()

	// Defensive: a debounce tick whose gen no longer matches must be a no-op
	// even when there's no other pending press. This guards the case where a
	// late tea.Tick fires after the user has already moved past the toggle.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{
		OutCh:               outCh,
		MessageSent:         true,
		TaskCreated:         true,
		InitialApprovalMode: client.NeoApprovalModeAuto,
	})
	m.approvalDebounceGen = 5

	updated, _ := m.Update(approvalDebounceTickMsg{gen: 4})
	_ = updated
	select {
	case ev := <-outCh:
		t.Fatalf("stale-gen tick must not dispatch, got %#v", ev)
	default:
	}
}

func TestModel_Update_CtrlA_DuringTaskCreationIsNoop(t *testing.T) {
	t.Parallel()

	// Between Enter and the task URL arriving (the create-task in-flight
	// window), Ctrl+A would race the dispatcher: getTaskID() returns "" and
	// the update event is silently dropped. The fix swallows the keypress
	// instead so the status bar never lies about what the cloud is enforcing.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{
		OutCh:               outCh,
		MessageSent:         true,
		TaskCreated:         false,
		InitialApprovalMode: client.NeoApprovalModeManual,
	})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)

	assert.Equal(t, client.NeoApprovalModeManual, m.approvalMode,
		"Ctrl+A during the create-task window must not change the local mode")
	select {
	case ev := <-outCh:
		t.Fatalf("Ctrl+A during the create-task window must not dispatch, got %#v", ev)
	default:
	}
}

func TestModel_Update_UISessionURL_UnfreezesPostMessageToggles(t *testing.T) {
	t.Parallel()

	// UISessionURL is the dispatcher's signal that CreateNeoTask succeeded —
	// from that moment on, Ctrl+A / Ctrl+R must work again. The status-bar
	// indicator also flips here.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{
		OutCh:               outCh,
		EventCh:             make(chan UIEvent, 4),
		MessageSent:         true,
		InitialApprovalMode: client.NeoApprovalModeManual,
	})
	require.False(t, m.taskCreated, "precondition: task not yet created")

	updated, _ := m.Update(UISessionURL{URL: "https://app.pulumi.com/x/neo/tasks/t1"})
	m = updated.(Model)
	require.True(t, m.taskCreated, "UISessionURL must flip taskCreated")

	// Toggle now works — schedules a debounced PATCH.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoApprovalModeBalanced, m.approvalMode)

	// Fire the debounce tick so the PATCH actually lands.
	updated, _ = m.Update(approvalDebounceTickMsg{gen: m.approvalDebounceGen})
	_ = updated
	select {
	case got := <-outCh:
		require.NotNil(t, got.update)
	default:
		t.Fatal("debounce tick after UISessionURL must dispatch an update")
	}
}

func TestModel_Update_CtrlR_TogglesPermissionMode(t *testing.T) {
	t.Parallel()

	// Ctrl+R flips read-only on and off. Like Ctrl+A, pre-message it's a
	// purely local flip; post-message it dispatches an update.
	outCh := make(chan outboundEvent, 4)
	m := NewModel(ModelConfig{
		OutCh:                 outCh,
		InitialPermissionMode: client.NeoPermissionModeDefault,
	})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoPermissionModeReadOnly, m.permissionMode)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoPermissionModeDefault, m.permissionMode,
		"second Ctrl+R must flip back to default")

	select {
	case ev := <-outCh:
		t.Fatalf("pre-message Ctrl+R must not dispatch, got %#v", ev)
	default:
	}
}

func TestModel_Update_CtrlR_AfterFirstMessage_DispatchesUpdate(t *testing.T) {
	t.Parallel()

	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{
		OutCh:                 outCh,
		MessageSent:           true,
		TaskCreated:           true,
		InitialPermissionMode: client.NeoPermissionModeDefault,
	})

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m = updated.(Model)
	assert.Equal(t, client.NeoPermissionModeReadOnly, m.permissionMode)

	// Debounced — nothing on outCh until the tick fires.
	select {
	case ev := <-outCh:
		t.Fatalf("Ctrl+R must defer the dispatch behind the debounce tick, got %#v", ev)
	default:
	}

	updated, _ = m.Update(permissionDebounceTickMsg{gen: m.permissionDebounceGen})
	_ = updated

	select {
	case got := <-outCh:
		require.NotNil(t, got.update)
		require.NotNil(t, got.update.PermissionMode)
		assert.Equal(t, client.NeoPermissionModeReadOnly, *got.update.PermissionMode)
		assert.Nil(t, got.update.ApprovalMode,
			"permission-mode toggle must not include approvalMode")
	default:
		t.Fatal("the debounce tick at the current gen must dispatch")
	}
}

func TestModel_Update_CtrlR_RapidPressesCollapseToOneDispatch(t *testing.T) {
	t.Parallel()

	// Mirror of TestModel_Update_CtrlA_RapidPressesCollapseToOneDispatch for
	// the permission axis. Two rapid Ctrl+R presses must net out to the
	// starting state (default → read-only → default), so the only dispatch
	// that fires carries permission=default.
	outCh := make(chan outboundEvent, 2)
	m := NewModel(ModelConfig{
		OutCh:                 outCh,
		MessageSent:           true,
		TaskCreated:           true,
		InitialPermissionMode: client.NeoPermissionModeDefault,
	})

	for range 2 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
		m = updated.(Model)
	}
	require.Equal(t, client.NeoPermissionModeDefault, m.permissionMode)
	require.Equal(t, 2, m.permissionDebounceGen)

	// Stale tick: gen mismatch, no dispatch.
	updated, _ := m.Update(permissionDebounceTickMsg{gen: 1})
	m = updated.(Model)
	select {
	case ev := <-outCh:
		t.Fatalf("stale-gen tick must not dispatch, got %#v", ev)
	default:
	}

	// Current tick: one dispatch with the netted-out final value.
	updated, _ = m.Update(permissionDebounceTickMsg{gen: 2})
	_ = updated
	select {
	case got := <-outCh:
		require.NotNil(t, got.update)
		require.NotNil(t, got.update.PermissionMode)
		assert.Equal(t, client.NeoPermissionModeDefault, *got.update.PermissionMode)
	default:
		t.Fatal("the current-gen tick must dispatch the final value")
	}
}

func TestModel_View_ModeChips(t *testing.T) {
	t.Parallel()

	// Default modes (manual approval, default permission) collapse to no chips
	// so the status bar stays uncluttered. Each non-default value gets its own
	// chip; multiple chips concatenate with a space separator. We assert on
	// modeChips() directly rather than View() because the hint line itself
	// mentions "read-only" / "approval" in its keybindings legend.
	t.Run("DefaultsRenderNoChips", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{
			InitialApprovalMode:   client.NeoApprovalModeManual,
			InitialPermissionMode: client.NeoPermissionModeDefault,
		})
		assert.Empty(t, m.modeChips(), "manual+default must render no chips")
	})

	t.Run("BalancedRendersChip", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{InitialApprovalMode: client.NeoApprovalModeBalanced})
		assert.Contains(t, m.modeChips(), "balanced")
	})

	t.Run("AutoRendersChip", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{InitialApprovalMode: client.NeoApprovalModeAuto})
		assert.Contains(t, m.modeChips(), "auto-approve")
	})

	t.Run("ReadOnlyRendersChip", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{InitialPermissionMode: client.NeoPermissionModeReadOnly})
		assert.Contains(t, m.modeChips(), "read-only")
	})

	t.Run("MultipleChipsCoexist", func(t *testing.T) {
		t.Parallel()
		m := NewModel(ModelConfig{
			InitialApprovalMode:   client.NeoApprovalModeAuto,
			InitialPermissionMode: client.NeoPermissionModeReadOnly,
		})
		m.planMode = true
		chips := m.modeChips()
		assert.Contains(t, chips, "plan mode")
		assert.Contains(t, chips, "auto-approve")
		assert.Contains(t, chips, "read-only")
	})
}

func TestModel_Update_KeyEnter_FirstMessageCarriesAllModes(t *testing.T) {
	t.Parallel()

	// The first user message must snapshot all three axes (planMode,
	// approvalMode, permissionMode) into the outboundEvent envelope. The
	// dispatcher uses these to seed CreateNeoTask; if the snapshot is wrong
	// the cloud task is created with the wrong policy.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{
		OutCh:                 outCh,
		InitialApprovalMode:   client.NeoApprovalModeBalanced,
		InitialPermissionMode: client.NeoPermissionModeReadOnly,
	})
	m.planMode = true
	m.textInput.SetValue("kick off")

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = updated

	select {
	case got := <-outCh:
		assert.True(t, got.planMode)
		assert.Equal(t, client.NeoApprovalModeBalanced, got.approvalMode)
		assert.Equal(t, client.NeoPermissionModeReadOnly, got.permissionMode)
	default:
		t.Fatal("Enter must post the input to outCh")
	}
}

// newQuestionPendingModel returns a Model that has just received a question
// (ask-user) request — analogous to newApprovalPendingModel but routes through
// the isAskUserToolName branch so pendingIsQuestion gets set.
func newQuestionPendingModel(t *testing.T) Model {
	t.Helper()
	m := NewModel(ModelConfig{EventCh: make(chan UIEvent, 4), Busy: true})
	updated, _ := m.Update(UIApprovalRequest{
		ApprovalID: "appr_q",
		Message:    "What region?",
		ToolName:   "ux__ask_user",
	})
	return updated.(Model)
}

func TestModel_Update_UIApprovalResolved_AutoApprovedCommitsBlockAndClears(t *testing.T) {
	t.Parallel()

	// The cloud auto-resolved a pending approval. The TUI must commit an
	// "Auto-approved" feedback block to scrollback and reset the prompt — the
	// agent has already moved on, so the question prompt is no longer relevant.
	m := newApprovalPendingModel(t, make(chan outboundEvent, 1))
	require.True(t, m.pendingApproval, "precondition: approval is pending")

	updated, cmd := m.Update(UIApprovalResolved{ApprovalID: "appr_1", Approved: true})
	m = updated.(Model)

	assert.False(t, m.pendingApproval, "matching resolved must clear pendingApproval")
	assert.Empty(t, m.pendingApprovalID, "pendingApprovalID must reset")
	assert.False(t, m.pendingIsQuestion)
	assert.Empty(t, m.approvalPromptText, "approval prompt header must clear when approval resolves")
	assert.Equal(t, "Send a message...", m.textInput.Placeholder)

	idx := m.findBlockKind(blockApprovalAuto)
	require.GreaterOrEqual(t, idx, 0, "a blockApprovalAuto must be appended")
	assert.True(t, m.blocks[idx].approved, "approved flag must round-trip from msg.Approved")
	assert.False(t, m.blocks[idx].autoIsQuestion, "approval (not question) path must set autoIsQuestion=false")

	// The commit also queues a tea.Println so the block lands in terminal
	// scrollback. Verify the rendered string carries the "Auto-approved" verb.
	printed := collectPrintln(cmd)
	require.NotEmpty(t, printed, "auto-resolved block must be emitted to scrollback")
	assert.Contains(t, printed[len(printed)-1], "Auto-approved")
}

func TestModel_Update_UIApprovalResolved_AutoApprovedQuestionRendersAnsweredVerb(t *testing.T) {
	t.Parallel()

	// Question (ask-user) path: pendingIsQuestion=true. The auto-resolved block
	// must carry that flag through so the renderer says "Auto-answered" instead
	// of "Auto-approved" — auto-approving an ask-user is conceptually answering
	// with the default, not approving.
	m := newQuestionPendingModel(t)
	require.True(t, m.pendingApproval)
	require.True(t, m.pendingIsQuestion)

	updated, cmd := m.Update(UIApprovalResolved{ApprovalID: "appr_q", Approved: true})
	m = updated.(Model)

	assert.False(t, m.pendingApproval)
	idx := m.findBlockKind(blockApprovalAuto)
	require.GreaterOrEqual(t, idx, 0)
	assert.True(t, m.blocks[idx].autoIsQuestion, "question path must carry autoIsQuestion=true")

	printed := collectPrintln(cmd)
	require.NotEmpty(t, printed)
	assert.Contains(t, printed[len(printed)-1], "Auto-answered")
}

func TestModel_Update_UIApprovalResolved_AutoDeniedRendersDeniedVerb(t *testing.T) {
	t.Parallel()

	// Defensive: ApprovalMode=auto never denies today, but the wire format
	// supports it (ok=false) and a future cloud policy could exercise this
	// path. The TUI must render "Auto-denied" so the user sees what happened.
	m := newApprovalPendingModel(t, make(chan outboundEvent, 1))

	updated, cmd := m.Update(UIApprovalResolved{ApprovalID: "appr_1", Approved: false})
	m = updated.(Model)

	idx := m.findBlockKind(blockApprovalAuto)
	require.GreaterOrEqual(t, idx, 0)
	assert.False(t, m.blocks[idx].approved)

	printed := collectPrintln(cmd)
	require.NotEmpty(t, printed)
	assert.Contains(t, printed[len(printed)-1], "Auto-denied")
}

func TestModel_Update_UIApprovalResolved_EchoOfManualIsNoop(t *testing.T) {
	t.Parallel()

	// Manual confirmation already clears pendingApproval locally (the Enter
	// handler does that before sending upstream). When the cloud later echoes
	// our own user_confirmation back on the stream, the TUI must NOT render an
	// auto-approved block — pendingApproval is already false, so the handler
	// silently no-ops.
	m := NewModel(ModelConfig{EventCh: make(chan UIEvent, 4)})
	require.False(t, m.pendingApproval, "precondition: no pending approval")

	updated, _ := m.Update(UIApprovalResolved{ApprovalID: "appr_1", Approved: true})
	m = updated.(Model)

	assert.False(t, m.pendingApproval)
	assert.Equal(t, -1, m.findBlockKind(blockApprovalAuto),
		"echo of our own confirmation must not commit a blockApprovalAuto")
}

func TestModel_Update_UIApprovalResolved_MismatchedIDIsNoop(t *testing.T) {
	t.Parallel()

	// A resolved event arriving with a different approval ID than the one the
	// TUI is currently waiting on must be ignored — it's either stale or for a
	// different request. The current pending state must survive.
	m := newApprovalPendingModel(t, make(chan outboundEvent, 1))
	require.True(t, m.pendingApproval)
	require.Equal(t, "appr_1", m.pendingApprovalID)

	updated, _ := m.Update(UIApprovalResolved{ApprovalID: "appr_other", Approved: true})
	m = updated.(Model)

	assert.True(t, m.pendingApproval, "mismatched ID must not clear pending state")
	assert.Equal(t, "appr_1", m.pendingApprovalID)
	assert.Equal(t, -1, m.findBlockKind(blockApprovalAuto),
		"mismatched ID must not commit a blockApprovalAuto")
}

func TestRenderApprovalAuto_VerbVariants(t *testing.T) {
	t.Parallel()

	// Direct render-helper checks for the three string variants. lipgloss may
	// embed ANSI in the rendered output, so we assert on Contains.
	m := NewModel(ModelConfig{})

	cases := []struct {
		name           string
		approved       bool
		autoIsQuestion bool
		want           string
	}{
		{name: "auto-approved", approved: true, autoIsQuestion: false, want: "Auto-approved"},
		{name: "auto-answered", approved: true, autoIsQuestion: true, want: "Auto-answered"},
		{name: "auto-denied-approval", approved: false, autoIsQuestion: false, want: "Auto-denied"},
		{name: "auto-denied-question", approved: false, autoIsQuestion: true, want: "Auto-denied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := &block{kind: blockApprovalAuto, approved: tc.approved, autoIsQuestion: tc.autoIsQuestion}
			m.renderApprovalAuto(b)
			assert.Contains(t, b.rendered, tc.want)
		})
	}
}
