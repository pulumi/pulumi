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

// overlayHeader is the persistent first line of the overlay so users always
// know how to dismiss it and scroll.
const overlayHeader = "Tool details · ctrl+o or esc to close · ↑↓ pgup/pgdn to scroll · home/end for top/bottom"

var (
	overlayHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	overlayMetaStyle   = lipgloss.NewStyle().Faint(true)
	overlaySubheadOK   = lipgloss.NewStyle().Bold(true)
	overlayErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	overlayEmptyStyle  = lipgloss.NewStyle().Faint(true).Italic(true)
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

// View renders the overlay as the persistent header on top of the viewport.
func (o *overlayModel) View() string {
	return overlayHeaderStyle.Render(overlayHeader) + "\n" + o.vp.View()
}

// renderOverlayBody composes one section per history record, separated by a
// blank line. Long string values inside the pretty-printed JSON are
// word-wrapped to the viewport width so the viewport never has to
// horizontally scroll.
func renderOverlayBody(history []toolCallRecord, width int) string {
	if len(history) == 0 {
		return overlayEmptyStyle.Render(
			"No tool calls yet. Once the agent invokes a tool its arguments and result will appear here.")
	}
	sections := make([]string, 0, len(history))
	for i := range history {
		sections = append(sections, renderToolSection(&history[i], width))
	}
	return strings.Join(sections, "\n\n")
}

// renderToolSection formats one tool call: title line + faint metadata line +
// Arguments and Result subsections. Pending calls render an "(in flight)"
// placeholder instead of a result block.
func renderToolSection(rec *toolCallRecord, width int) string {
	var b strings.Builder

	b.WriteString(styledToolLabel(rec.Name, rec.Args))
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

	b.WriteString(overlaySubheadOK.Render("Arguments"))
	b.WriteString("\n")
	b.WriteString(indent(wrapForOverlay(formatJSON(rec.Args), width), "  "))
	b.WriteString("\n\n")

	b.WriteString(overlaySubheadOK.Render("Result"))
	b.WriteString("\n")
	if rec.Pending {
		b.WriteString(indent(overlayEmptyStyle.Render("(in flight)"), "  "))
	} else {
		b.WriteString(indent(wrapForOverlay(formatJSON(rec.Result), width), "  "))
	}

	return b.String()
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
