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

package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// palette is the TUI's color canon. Every styled surface pulls from this
// single source of truth — NO_COLOR is honoured automatically by lipgloss
// via termenv, so we don't need to handle it ourselves.
var palette = struct {
	Accent  color.Color // Pulumi pink
	Dim     color.Color
	Success color.Color
	Warning color.Color
	Error   color.Color
	Text    color.Color

	// HTTP method chip colors — mirror the docs site's endpoint-method-<verb> CSS.
	MethodGet    color.Color
	MethodPost   color.Color
	MethodPut    color.Color
	MethodPatch  color.Color
	MethodDelete color.Color
}{
	Accent:  lipgloss.Color("#E946C4"),
	Dim:     lipgloss.Color("241"),
	Success: lipgloss.Color("70"),
	Warning: lipgloss.Color("214"),
	Error:   lipgloss.Color("203"),
	Text:    lipgloss.Color("230"),

	MethodGet:    lipgloss.Color("39"),  // blue
	MethodPost:   lipgloss.Color("70"),  // green
	MethodPut:    lipgloss.Color("208"), // orange
	MethodPatch:  lipgloss.Color("171"), // magenta
	MethodDelete: lipgloss.Color("203"), // red
}

// Theme centralises every lipgloss style the TUI renders with. Styles are
// grouped by their on-screen role (chrome, tabs, panes, text spans).
type Theme struct {
	// App-level frame.
	App            lipgloss.Style
	TitleBar       lipgloss.Style
	Footer         lipgloss.Style
	ContentPadding lipgloss.Style // Inner padding for tab-body content.

	// Tab bar.
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	TabBar      lipgloss.Style

	// Bordered panes (body content). Focused swaps the border colour.
	PaneBorder        lipgloss.Style
	PaneBorderFocused lipgloss.Style

	// Text spans.
	Accent  lipgloss.Style
	Dim     lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style

	// Footer key hints.
	Key     lipgloss.Style
	KeyDesc lipgloss.Style

	// HTTP method chips keyed by uppercase verb.
	method map[string]lipgloss.Style
}

// newTheme returns the default theme populated from the palette. A future
// user-configurable theme would swap the palette and rebuild this struct.
func newTheme() Theme {
	pane := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(palette.Dim).
		Padding(0, 1)

	return Theme{
		App: lipgloss.NewStyle().Padding(0, 1),
		TitleBar: lipgloss.NewStyle().
			Bold(true).
			Foreground(palette.Accent).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(palette.Dim).
			Padding(0, 1),
		ContentPadding: lipgloss.NewStyle().Padding(1, 2),

		TabActive: lipgloss.NewStyle().
			Foreground(palette.Text).
			Background(palette.Accent).
			Padding(0, 2).
			Bold(true),
		TabInactive: lipgloss.NewStyle().
			Foreground(palette.Dim).
			Padding(0, 2),
		TabBar: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(palette.Dim),

		PaneBorder:        pane,
		PaneBorderFocused: pane.BorderForeground(palette.Accent),

		Accent:  lipgloss.NewStyle().Foreground(palette.Accent).Bold(true),
		Dim:     lipgloss.NewStyle().Foreground(palette.Dim),
		Error:   lipgloss.NewStyle().Foreground(palette.Error).Bold(true),
		Success: lipgloss.NewStyle().Foreground(palette.Success),
		Warning: lipgloss.NewStyle().Foreground(palette.Warning).Bold(true),
		Key:     lipgloss.NewStyle().Foreground(palette.Accent).Bold(true),
		KeyDesc: lipgloss.NewStyle().Foreground(palette.Dim),

		method: map[string]lipgloss.Style{
			"GET":    lipgloss.NewStyle().Foreground(palette.MethodGet).Bold(true),
			"POST":   lipgloss.NewStyle().Foreground(palette.MethodPost).Bold(true),
			"PUT":    lipgloss.NewStyle().Foreground(palette.MethodPut).Bold(true),
			"PATCH":  lipgloss.NewStyle().Foreground(palette.MethodPatch).Bold(true),
			"DELETE": lipgloss.NewStyle().Foreground(palette.MethodDelete).Bold(true),
		},
	}
}

// MethodStyle returns the color style for an HTTP verb chip. Unknown verbs
// fall back to Accent so any future method still renders visibly.
func (t Theme) MethodStyle(method string) lipgloss.Style {
	if s, ok := t.method[strings.ToUpper(method)]; ok {
		return s
	}
	return t.Accent
}

// Pane renders content inside an OUTER w×h bordered box, picking the
// focused variant of the border style when focused. Content is clipped to
// the exact inner area so the outer box is stable regardless of what the
// content tries to do.
//
// Frame sizes come from the style itself via
// lipgloss.Style.GetHorizontalFrameSize / GetVerticalFrameSize, so changes
// to the theme's border or padding flow through automatically.
func (t Theme) Pane(w, h int, focused bool, content string) string {
	style := t.PaneBorder
	if focused {
		style = t.PaneBorderFocused
	}
	innerW := w - style.GetHorizontalFrameSize()
	innerH := h - style.GetVerticalFrameSize()
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	return style.
		Width(w).
		Height(h).
		MaxHeight(h).
		Render(clipToLines(content, innerH, innerW))
}

// Inside returns the content width/height remaining inside the frame of
// the given style — i.e. outer w,h minus that style's border + padding.
// Use this to size sub-widgets (viewport, textarea, list) so they fit
// exactly inside a styled container.
func (t Theme) Inside(w, h int, style lipgloss.Style) (int, int) {
	innerW := w - style.GetHorizontalFrameSize()
	innerH := h - style.GetVerticalFrameSize()
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	return innerW, innerH
}

// clipToLines forces s to exactly h lines. Lines past h are dropped;
// fewer-than-h inputs are padded with blank lines so the pane height
// stays stable. Each line is also ANSI-truncated to w cols so no single
// line can burst the pane's horizontal border.
func clipToLines(s string, h, w int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "")
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// joinPanes renders a row of pre-styled panes side-by-side, top-aligned.
func joinPanes(panes ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, panes...)
}
