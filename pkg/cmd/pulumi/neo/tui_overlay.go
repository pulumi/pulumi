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
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

// overlayHint is the persistent footer line of the overlay so users always
// know how to dismiss it and scroll. It sits below the viewport rather than
// above it so the eye lands on the tool detail content first.
const overlayHint = "Tool details · ctrl+o or esc to close · ↑↓ pgup/pgdn to scroll · home/end for top/bottom"

var (
	// overlayHintStyle is the dim cue at the bottom of the alt-screen. Faint
	// (not bold) keeps it out of the way of the actual content.
	overlayHintStyle = lipgloss.NewStyle().Faint(true)
	// overlayTitleStyle styles the tool function name in cyan bold so each
	// section's title pops out at a glance.
	overlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	overlayMetaStyle  = lipgloss.NewStyle().Faint(true)
	// overlayArgsHead is bold blue for the Arguments subhead. Distinct from
	// the Result subhead so the eye can find the boundary at a glance even
	// without indentation cues.
	overlayArgsHead = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	// overlayResultHead is bold green for an ok result, bold red when the
	// call returned an error — same color discipline as the inline tool
	// markers (toolOKMarker / toolErrMarker) so the two surfaces stay
	// visually consistent.
	overlayResultHeadOK    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	overlayResultHeadError = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	overlayErrorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	overlayEmptyStyle      = lipgloss.NewStyle().Faint(true).Italic(true)
	overlayDividerStyle    = lipgloss.NewStyle().Faint(true)
	// Section title markers mirror the inline transcript: green ⏺ for ok,
	// red for error, dim cyan for in-flight. Same glyph as toolOKMarker so
	// users get the same visual cue whether they're reading scrollback or
	// the overlay.
	overlayMarkerOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("⏺")
	overlayMarkerError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("⏺")
	overlayMarkerPending = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Faint(true).Render("⏺")
)

// overlayModel is the alt-screen viewer for tool-call args/results. The
// viewport handles scrolling natively; the wrapper takes care of rebuilding
// content from the Model's tool history and rendering the persistent header.
type overlayModel struct {
	vp     viewport.Model
	width  int
	height int
}

// newOverlayModel constructs an overlay sized to the current terminal. The
// viewport reserves one row for the persistent header.
func newOverlayModel(width, height int) overlayModel {
	vp := viewport.New(width, max(height-1, 1))
	return overlayModel{vp: vp, width: width, height: height}
}

// SetSize re-sizes the overlay (and its viewport) in response to a window
// resize. Caller should call Refresh afterwards so wrapped content reflows.
func (o *overlayModel) SetSize(width, height int) {
	o.width = width
	o.height = height
	o.vp.Width = width
	o.vp.Height = max(height-1, 1)
}

// Update forwards messages (key/mouse) to the viewport so it can scroll.
func (o *overlayModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	o.vp, cmd = o.vp.Update(msg)
	return cmd
}

// Refresh rebuilds the viewport content from history. The viewport is
// scrolled to the bottom so the most recent tool call is on screen when the
// overlay opens.
func (o *overlayModel) Refresh(history []toolCallRecord) {
	o.vp.SetContent(renderOverlayBody(history, o.width))
	o.vp.GotoBottom()
}

// View renders the viewport with the persistent hint pinned to the bottom of
// the alt-screen. The hint is below the content so the eye lands on the tool
// detail first and only drops to the hint when looking for the close key.
func (o *overlayModel) View() string {
	return o.vp.View() + "\n" + overlayHintStyle.Render(overlayHint)
}

// renderOverlayBody composes one section per history record, joined by a
// faint horizontal divider so the boundary between calls is unambiguous.
// Long string values inside the pretty-printed JSON are word-wrapped to the
// viewport width so the viewport never has to horizontally scroll.
func renderOverlayBody(history []toolCallRecord, width int) string {
	if len(history) == 0 {
		return overlayEmptyStyle.Render(
			"No tool calls yet. Once the agent invokes a tool its arguments and result will appear here.")
	}
	sections := make([]string, 0, len(history))
	for i := range history {
		sections = append(sections, renderToolSection(&history[i], width))
	}
	return strings.Join(sections, "\n\n"+sectionDivider(width)+"\n\n")
}

// sectionDivider renders the faint horizontal rule used between tool-call
// sections. We use a box-drawing character so the line reads as a divider
// even on terminals that render Faint as a subtle grey.
func sectionDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return overlayDividerStyle.Render(strings.Repeat("─", width))
}

// renderToolSection formats one tool call: title line + faint metadata line +
// Arguments and Result subsections. Pending calls render an "(in flight)"
// placeholder instead of a result block.
func renderToolSection(rec *toolCallRecord, width int) string {
	var b strings.Builder

	funcName, argSummary := toolLabelParts(rec.Name, rec.Args)
	b.WriteString(toolMarker(rec))
	b.WriteString(" ")
	b.WriteString(overlayTitleStyle.Render(funcName))
	if argSummary != "" {
		b.WriteString(overlayMetaStyle.Render(" (\"" + argSummary + "\")"))
	}
	b.WriteString("\n")

	meta := "started " + rec.StartedAt.Format("15:04:05")
	switch {
	case rec.Pending:
		meta += " · running"
	default:
		meta += " · " + rec.CompletedAt.Sub(rec.StartedAt).Round(1e6).String()
		if rec.IsError {
			meta += " · " + overlayErrorStyle.Render("error")
		} else {
			meta += " · ok"
		}
	}
	b.WriteString(overlayMetaStyle.Render(meta))
	b.WriteString("\n\n")

	b.WriteString(overlayArgsHead.Render("Arguments"))
	b.WriteString("\n")
	b.WriteString(indent(wrapForOverlay(formatJSON(rec.Args), width), "  "))
	b.WriteString("\n\n")

	b.WriteString(resultHead(rec).Render("Result"))
	b.WriteString("\n")
	if rec.Pending {
		b.WriteString(indent(overlayEmptyStyle.Render("(in flight)"), "  "))
	} else {
		b.WriteString(indent(wrapForOverlay(formatJSON(rec.Result), width), "  "))
	}

	return b.String()
}

// toolMarker picks the ⏺ glyph color for the section title based on call
// state: green for an ok completion, red for an error, dim cyan while the
// call is still in flight.
func toolMarker(rec *toolCallRecord) string {
	switch {
	case rec.Pending:
		return overlayMarkerPending
	case rec.IsError:
		return overlayMarkerError
	default:
		return overlayMarkerOK
	}
}

// resultHead picks the color of the "Result" subhead: green on ok, red on
// error. Keeps the visual rhythm of the section consistent with the marker
// at the top.
func resultHead(rec *toolCallRecord) lipgloss.Style {
	if rec.IsError {
		return overlayResultHeadError
	}
	return overlayResultHeadOK
}

// formatJSON renders raw as a YAML-ish key/value block. Unlike pretty-printed
// JSON, the output drops curly braces, square brackets, and key/value
// quoting, and multi-line strings are expanded onto their own indented
// lines rather than escaped with "\n". This is purely a display format —
// the result is not meant to round-trip back to JSON.
//
// Empty input renders "(empty)" so consumers always see an explicit signal.
// Invalid JSON falls back to the raw bytes so the user still gets *something*.
func formatJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(empty)"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	var b strings.Builder
	formatValue(&b, v, 0)
	return strings.TrimRight(b.String(), "\n")
}

// formatValue dispatches on the dynamic type produced by json.Unmarshal.
// Top-level scalars (string, number, bool, null) print verbatim; collections
// recurse through formatMap / formatSlice.
func formatValue(b *strings.Builder, v any, depth int) {
	switch x := v.(type) {
	case map[string]any:
		formatMap(b, x, depth)
	case []any:
		formatSlice(b, x, depth)
	case string:
		// At the top level (or as a continuation body) a multi-line string
		// is printed verbatim so file contents and shell output keep their
		// real line breaks. The caller is responsible for any leading indent.
		b.WriteString(x)
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case float64:
		b.WriteString(formatNumber(x))
	case nil:
		b.WriteString("null")
	default:
		// json.Unmarshal into `any` only produces the cases above, but be
		// defensive: print the Go default formatting.
		fmt.Fprintf(b, "%v", x)
	}
}

// formatMap emits sorted key/value pairs. A scalar value sits on the same
// line as its key; a complex (map/array) or multi-line string value starts
// on the next line, indented two spaces deeper than the key.
func formatMap(b *strings.Builder, m map[string]any, depth int) {
	if len(m) == 0 {
		b.WriteString("(empty)")
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pad := strings.Repeat(" ", depth)
	for i, k := range keys {
		if i > 0 {
			b.WriteString("\n")
		}
		v := m[k]
		if isInlineScalar(v) {
			b.WriteString(pad)
			b.WriteString(k)
			b.WriteString(": ")
			formatValue(b, v, depth+2)
			continue
		}
		// Block form: "key:" on its own line, value indented two further.
		b.WriteString(pad)
		b.WriteString(k)
		b.WriteString(":\n")
		writeBlock(b, v, depth+2)
	}
}

// formatSlice emits one element per line, prefixed with "- ". Scalar
// elements sit on the same line as the bullet; complex elements have their
// rendered body indented two columns past the bullet.
func formatSlice(b *strings.Builder, s []any, depth int) {
	if len(s) == 0 {
		b.WriteString("(empty)")
		return
	}
	pad := strings.Repeat(" ", depth)
	for i, v := range s {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(pad)
		b.WriteString("- ")
		if isInlineScalar(v) {
			formatValue(b, v, depth+2)
			continue
		}
		// Render the child, then re-indent its continuation lines so they
		// align past the "- " marker. The first line of the child output
		// has no leading pad (we just wrote the bullet); subsequent lines
		// need the bullet's two-space offset added.
		var child strings.Builder
		formatValue(&child, v, 0)
		rendered := child.String()
		first, rest, hasRest := strings.Cut(rendered, "\n")
		b.WriteString(strings.TrimLeft(first, " "))
		if hasRest {
			b.WriteString("\n")
			b.WriteString(indent(rest, pad+"  "))
		}
	}
}

// writeBlock renders v as the body of a "key:" line, indented `depth`
// spaces on every line so it lines up under the key. Strings are written
// verbatim with their newlines preserved; maps and arrays defer to
// formatValue, which threads `depth` down to formatMap / formatSlice so
// their per-line padding lines up too.
func writeBlock(b *strings.Builder, v any, depth int) {
	if s, ok := v.(string); ok {
		b.WriteString(indent(s, strings.Repeat(" ", depth)))
		return
	}
	formatValue(b, v, depth)
}

// isInlineScalar reports whether v can be rendered on the same line as its
// owning key or bullet. Multi-line strings, maps, and arrays cannot — they
// need their own indented block.
func isInlineScalar(v any) bool {
	switch x := v.(type) {
	case map[string]any, []any:
		return false
	case string:
		return !strings.Contains(x, "\n")
	default:
		return true
	}
}

// formatNumber prints a JSON number (always float64) without trailing zeros
// or exponent form. Integer values print as e.g. "0", "42", "-7"; fractional
// values use the shortest decimal form via strconv.FormatFloat 'g' precision -1.
func formatNumber(x float64) string {
	if x == float64(int64(x)) {
		return strconv.FormatInt(int64(x), 10)
	}
	return strconv.FormatFloat(x, 'g', -1, 64)
}

// wrapForOverlay word-wraps content to width-2 so the 2-space indent in
// renderToolSection still leaves us inside the terminal. Falls back to the
// unwrapped string when the width is too small to be useful.
func wrapForOverlay(s string, width int) string {
	if width <= 4 {
		return s
	}
	return wordwrap.String(s, width-2)
}

// indent prefixes every line of s with prefix.
func indent(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
