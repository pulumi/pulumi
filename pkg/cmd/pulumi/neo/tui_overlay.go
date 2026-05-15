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
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

const overlayHint = "Tool details · ctrl+o or esc to close · ↑↓ pgup/pgdn to scroll · home/end for top/bottom"

var (
	overlayHintStyle       = lipgloss.NewStyle().Faint(true)
	overlayTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	overlayMetaStyle       = lipgloss.NewStyle().Faint(true)
	overlayArgsHead        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	overlayResultHeadOK    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	overlayResultHeadError = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	overlayErrorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	overlayEmptyStyle      = lipgloss.NewStyle().Faint(true).Italic(true)
	overlayDividerStyle    = lipgloss.NewStyle().Faint(true)
	overlayMarkerOK        = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("⏺")
	overlayMarkerError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("⏺")
	overlayMarkerPending   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Faint(true).Render("⏺")
)

type overlayModel struct {
	vp     viewport.Model
	width  int
	height int
}

func newOverlayModel(width, height int) overlayModel {
	return overlayModel{
		vp:     viewport.New(width, max(height-1, 1)),
		width:  width,
		height: height,
	}
}

func (o *overlayModel) SetSize(width, height int) {
	o.width = width
	o.height = height
	o.vp.Width = width
	o.vp.Height = max(height-1, 1)
}

func (o *overlayModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	o.vp, cmd = o.vp.Update(msg)
	return cmd
}

// Refresh rebuilds the viewport content and snaps to the bottom so the most
// recent call is on screen when the overlay opens.
func (o *overlayModel) Refresh(history []toolCallRecord) {
	o.vp.SetContent(renderOverlayBody(history, o.width))
	o.vp.GotoBottom()
}

// View pins the hint below the viewport rather than above it so the eye
// lands on the tool detail first.
func (o *overlayModel) View() string {
	return o.vp.View() + "\n" + overlayHintStyle.Render(overlayHint)
}

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

func sectionDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return overlayDividerStyle.Render(strings.Repeat("─", width))
}

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

func resultHead(rec *toolCallRecord) lipgloss.Style {
	if rec.IsError {
		return overlayResultHeadError
	}
	return overlayResultHeadOK
}

// formatJSON renders raw as a YAML-ish key/value block — no braces, brackets,
// or key/value quoting, and multi-line strings are expanded onto their own
// indented lines. Display-only; the output is not meant to round-trip back
// to JSON.
func formatJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "(empty)"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "(could not parse as JSON: " + err.Error() + ")\n\n" + string(raw)
	}
	var b strings.Builder
	formatValue(&b, v, 0)
	return strings.TrimRight(b.String(), "\n")
}

func formatValue(b *strings.Builder, v any, depth int) {
	switch x := v.(type) {
	case map[string]any:
		formatMap(b, x, depth)
	case []any:
		formatSlice(b, x, depth)
	case string:
		b.WriteString(x)
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case float64:
		b.WriteString(formatNumber(x))
	case nil:
		b.WriteString("null")
	}
}

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
		b.WriteString(pad)
		b.WriteString(k)
		if isInlineScalar(v) {
			b.WriteString(": ")
			formatValue(b, v, depth+2)
			continue
		}
		b.WriteString(":\n")
		writeBlock(b, v, depth+2)
	}
}

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
		// Re-indent continuation lines so they align past the "- " marker:
		// the first line has none (the bullet itself takes that slot), the
		// rest get the bullet's two-space offset.
		var child strings.Builder
		formatValue(&child, v, 0)
		first, rest, hasRest := strings.Cut(child.String(), "\n")
		b.WriteString(strings.TrimLeft(first, " "))
		if hasRest {
			b.WriteString("\n")
			b.WriteString(indent(rest, pad+"  "))
		}
	}
}

func writeBlock(b *strings.Builder, v any, depth int) {
	if s, ok := v.(string); ok {
		b.WriteString(indent(s, strings.Repeat(" ", depth)))
		return
	}
	formatValue(b, v, depth)
}

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

// formatNumber sidesteps strconv's default float formatting for integer-valued
// floats so exit_code:0 doesn't print as "0e+00".
func formatNumber(x float64) string {
	if x == float64(int64(x)) {
		return strconv.FormatInt(int64(x), 10)
	}
	return strconv.FormatFloat(x, 'g', -1, 64)
}

func wrapForOverlay(s string, width int) string {
	if width <= 4 {
		return s
	}
	return wordwrap.String(s, width-2)
}

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
