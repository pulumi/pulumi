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

// formatJSON returns a pretty-printed representation of raw. If raw can't be
// parsed as JSON it falls back to the raw bytes as a string so the user still
// sees something. An empty payload renders as "(empty)" so consumers don't
// have to special-case nil.
func formatJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(empty)"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(out)
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
