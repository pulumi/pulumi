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
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// History bookkeeping
// -----------------------------------------------------------------------------

func TestAppendToolStart_BoundsHistory(t *testing.T) {
	t.Parallel()

	var hist []toolCallRecord
	for range maxToolHistory + 5 {
		hist = appendToolStart(hist, "shell__shell_execute", nil)
	}
	require.Len(t, hist, maxToolHistory,
		"history must be capped at maxToolHistory; oldest entries dropped off the front")
	for _, r := range hist {
		assert.True(t, r.Pending, "every appended start record should begin Pending")
	}
}

func TestCompleteToolCall_MatchesMostRecentPending(t *testing.T) {
	t.Parallel()

	hist := appendToolStart(nil, "fs__read", json.RawMessage(`{"path":"a.txt"}`))
	hist = appendToolStart(hist, "fs__read", json.RawMessage(`{"path":"b.txt"}`))

	completeToolCall(hist, "fs__read", json.RawMessage(`"content B"`), false)

	require.Len(t, hist, 2)
	// Most-recent matching pending entry (index 1) is completed first.
	assert.False(t, hist[1].Pending)
	assert.JSONEq(t, `"content B"`, string(hist[1].Result))
	// Earlier matching entry remains pending — it will be completed by a
	// subsequent UIToolCompleted.
	assert.True(t, hist[0].Pending)
}

func TestCompleteToolCall_NoMatchIsNoOp(t *testing.T) {
	t.Parallel()

	hist := appendToolStart(nil, "fs__read", nil)
	completeToolCall(hist, "fs__read", json.RawMessage(`"ok"`), false)
	require.False(t, hist[0].Pending)

	// A second completion for the same name finds no pending entry; it must
	// not stomp the already-completed record.
	completeToolCall(hist, "fs__read", json.RawMessage(`"different"`), true)
	assert.JSONEq(t, `"ok"`, string(hist[0].Result))
	assert.False(t, hist[0].IsError)
}

// -----------------------------------------------------------------------------
// JSON formatter
// -----------------------------------------------------------------------------

func TestFormatJSON(t *testing.T) {
	t.Parallel()

	t.Run("renders a flat map as key/value lines without JSON syntax", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`{"b":2,"a":1}`))
		assert.Equal(t, "a: 1\nb: 2", got)
	})

	t.Run("integer numbers render without decimals or exponent", func(t *testing.T) {
		t.Parallel()
		// Guards against "0e+00" for integer-valued float64s; see formatNumber.
		got := formatJSON(json.RawMessage(`{"exit_code":0,"size":1234567890}`))
		assert.Equal(t, "exit_code: 0\nsize: 1234567890", got)
	})

	t.Run("fractional numbers use the shortest decimal form", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`{"ratio":0.5}`))
		assert.Equal(t, "ratio: 0.5", got)
	})

	t.Run("multi-line string value expands onto indented lines", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`{"stdout":"a\nb\nc"}`))
		assert.Equal(t, "stdout:\n  a\n  b\n  c", got)
	})

	t.Run("nested map indents the child block", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`{"outer":{"inner":"val"}}`))
		assert.Equal(t, "outer:\n  inner: val", got)
	})

	t.Run("top-level string prints verbatim with newlines preserved", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`"line1\nline2\nline3"`))
		assert.Equal(t, "line1\nline2\nline3", got)
	})

	t.Run("top-level array renders each element with a bullet", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`[1,2,"three"]`))
		assert.Equal(t, "- 1\n- 2\n- three", got)
	})

	t.Run("array of maps continues each map under the bullet", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`[{"a":1,"b":2}]`))
		assert.Equal(t, "- a: 1\n  b: 2", got)
	})

	t.Run("empty collection renders an explicit placeholder", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "(empty)", formatJSON(json.RawMessage(`{}`)))
		assert.Equal(t, "(empty)", formatJSON(json.RawMessage(`[]`)))
	})

	t.Run("booleans and null stringify directly", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`{"ok":true,"bad":false,"none":null}`))
		assert.Equal(t, "bad: false\nnone: null\nok: true", got)
	})

	t.Run("falls back to raw bytes and surfaces the parse error", func(t *testing.T) {
		t.Parallel()
		got := formatJSON(json.RawMessage(`not-json{`))
		assert.Contains(t, got, "could not parse as JSON",
			"parse failure must surface the error so the user knows why the block looks raw")
		assert.Contains(t, got, "not-json{", "raw bytes still shown for inspection")
	})

	t.Run("empty input renders explicit placeholder", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "(empty)", formatJSON(nil))
		assert.Equal(t, "(empty)", formatJSON(json.RawMessage{}))
	})
}

// -----------------------------------------------------------------------------
// Overlay open / close / scroll
// -----------------------------------------------------------------------------

// ctrlKey builds a v2 bubbletea Ctrl+<r> key event.
func ctrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

func TestModel_Update_CtrlOOpensOverlay(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	updated, _ := m.Update(ctrlKey('o'))
	um := updated.(Model)

	assert.True(t, um.overlayActive, "ctrl+o must flip the overlay on")
	assert.True(t, um.View().AltScreen,
		"ctrl+o must mark the next View() as alt-screen so the overlay can take over the frame")
}

func TestModel_Update_CtrlOClosesOverlay(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	updated, _ := m.Update(ctrlKey('o'))
	um := updated.(Model)
	require.True(t, um.overlayActive)

	updated, _ = um.Update(ctrlKey('o'))
	um = updated.(Model)
	assert.False(t, um.overlayActive)
	assert.False(t, um.View().AltScreen, "closing the overlay must drop AltScreen on the next View()")
}

func TestModel_Update_EscClosesOverlay_NoCancel(t *testing.T) {
	t.Parallel()

	// Esc in the overlay must close it WITHOUT posting user_cancel — the
	// agent keeps running while the user reviews.
	outCh := make(chan outboundEvent, 1)
	m := NewModel(ModelConfig{OutCh: outCh, Busy: true})
	updated, _ := m.Update(ctrlKey('o'))
	um := updated.(Model)
	require.True(t, um.overlayActive)

	updated, _ = um.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	um = updated.(Model)
	assert.False(t, um.overlayActive, "esc in overlay must close it")
	assert.False(t, um.cancelling, "esc in overlay must NOT trigger agent cancellation")
	assert.False(t, um.View().AltScreen, "closing the overlay must drop AltScreen on the next View()")

	select {
	case ev := <-outCh:
		t.Fatalf("esc in overlay must not post any outbound event, got %T", ev.event)
	default:
	}
}

func TestModel_Update_CtrlCAndCtrlDCloseOverlay(t *testing.T) {
	t.Parallel()

	// A reflexive ctrl+c (or ctrl+d) while the overlay is open should dismiss
	// the overlay rather than arm the quit gate or cancel the agent. Once
	// back in the inline view a second ctrl+c can still quit normally.
	for _, r := range []rune{'c', 'd'} {
		outCh := make(chan outboundEvent, 1)
		m := NewModel(ModelConfig{OutCh: outCh, Busy: true})
		updated, _ := m.Update(ctrlKey('o'))
		um := updated.(Model)
		require.True(t, um.overlayActive)

		updated, _ = um.Update(ctrlKey(r))
		um = updated.(Model)
		assert.False(t, um.overlayActive, "ctrl+%c in overlay must close it", r)
		assert.False(t, um.ctrlCArmed, "ctrl+%c in overlay must not arm the quit gate", r)
		assert.False(t, um.cancelling, "ctrl+%c in overlay must not trigger cancellation", r)
		assert.False(t, um.View().AltScreen,
			"closing the overlay must drop AltScreen on the next View()")

		select {
		case ev := <-outCh:
			t.Fatalf("ctrl+%c in overlay must not post any outbound event, got %T", r, ev.event)
		default:
		}
	}
}

func TestModel_Update_OverlaySwallowsTyping(t *testing.T) {
	t.Parallel()

	// Printable keys must not reach the (hidden) input bar while the overlay
	// is open, otherwise typed text shows up after the overlay closes.
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(ctrlKey('o'))
	um := updated.(Model)
	require.True(t, um.overlayActive)

	for _, r := range "hello" {
		updated, _ = um.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		um = updated.(Model)
	}
	assert.Equal(t, "", um.textInput.Value(), "typing while overlay is open must not enter the input buffer")
}

func TestModel_Update_OverlayForwardsScrollKeys(t *testing.T) {
	t.Parallel()

	// Viewport offset isn't visible from outside, so the assertion is the
	// negative one: scroll keys must not trip the swallow-everything-else
	// default branch (which would close-or-no-op the overlay).
	m := NewModel(ModelConfig{})
	updated, _ := m.Update(ctrlKey('o'))
	um := updated.(Model)
	require.True(t, um.overlayActive)

	for _, code := range []rune{tea.KeyDown, tea.KeyUp, tea.KeyPgDown, tea.KeyPgUp, tea.KeyHome, tea.KeyEnd} {
		updated, _ = um.Update(tea.KeyPressMsg{Code: code})
		um = updated.(Model)
		assert.True(t, um.overlayActive, "scroll key %v must not close the overlay", code)
	}
}

// -----------------------------------------------------------------------------
// History updates from events
// -----------------------------------------------------------------------------

func TestModel_ToolEventsAppendHistory(t *testing.T) {
	t.Parallel()

	m := NewModel(ModelConfig{})
	args := json.RawMessage(`{"command":"echo hi"}`)
	result := json.RawMessage(`{"stdout":"hi\n","exit_code":0}`)

	updated, _ := m.Update(UIToolStarted{Name: "shell__shell_execute", Args: args})
	um := updated.(Model)
	require.Len(t, um.toolHistory, 1)
	assert.True(t, um.toolHistory[0].Pending)
	assert.Equal(t, "shell__shell_execute", um.toolHistory[0].Name)
	assert.JSONEq(t, string(args), string(um.toolHistory[0].Args))

	updated, _ = um.Update(UIToolCompleted{
		Name:    "shell__shell_execute",
		Args:    args,
		Result:  result,
		IsError: false,
	})
	um = updated.(Model)
	require.Len(t, um.toolHistory, 1)
	assert.False(t, um.toolHistory[0].Pending)
	assert.JSONEq(t, string(result), string(um.toolHistory[0].Result))
}

// -----------------------------------------------------------------------------
// Overlay body rendering
// -----------------------------------------------------------------------------

func TestRenderOverlayBody_EmptyState(t *testing.T) {
	t.Parallel()

	body := renderOverlayBody(nil, 80)
	assert.Contains(t, body, "No tool calls yet")
}

func TestRenderOverlayBody_IncludesArgsAndResult(t *testing.T) {
	t.Parallel()

	hist := appendToolStart(nil, "fs__read",
		json.RawMessage(`{"path":"/tmp/a.txt"}`))
	completeToolCall(hist, "fs__read",
		json.RawMessage(`{"content":"hello world"}`), false)

	body := renderOverlayBody(hist, 80)
	assert.Contains(t, body, "Arguments")
	assert.Contains(t, body, "/tmp/a.txt")
	assert.Contains(t, body, "Result")
	assert.Contains(t, body, "hello world")
	assert.NotContains(t, body, "(in flight)")
}

func TestRenderOverlayBody_DividerBetweenCalls(t *testing.T) {
	t.Parallel()

	hist := appendToolStart(nil, "fs__read", json.RawMessage(`{"path":"a.txt"}`))
	completeToolCall(hist, "fs__read", json.RawMessage(`"a"`), false)
	hist = appendToolStart(hist, "fs__read", json.RawMessage(`{"path":"b.txt"}`))
	completeToolCall(hist, "fs__read", json.RawMessage(`"b"`), false)

	body := renderOverlayBody(hist, 40)
	assert.Contains(t, body, strings.Repeat("─", 40))
	// Divider is strictly a between-sections affordance.
	single := renderOverlayBody(hist[:1], 40)
	assert.NotContains(t, single, strings.Repeat("─", 40))
}

func TestOverlayView_HintAtBottom(t *testing.T) {
	t.Parallel()

	o := newOverlayModel(80, 10)
	o.Refresh(nil)
	view := o.View()
	lines := strings.Split(view, "\n")
	last := lines[len(lines)-1]
	assert.Contains(t, last, "ctrl+o or esc to close",
		"hint must be on the last line; got last line %q", last)
}

func TestRenderOverlayBody_PendingShowsInFlight(t *testing.T) {
	t.Parallel()

	hist := appendToolStart(nil, "shell__shell_execute",
		json.RawMessage(`{"command":"sleep 100"}`))

	body := renderOverlayBody(hist, 80)
	assert.Contains(t, body, "(in flight)")
	assert.Contains(t, strings.ToLower(body), "running")
}
