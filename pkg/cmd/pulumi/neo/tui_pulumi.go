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
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

// Pulumi block styling. Colors follow the CLI's preview output: green for
// create, yellow for update and replace, red for delete, blue for read/refresh.
// Diagnostics reuse the existing warning/error styles for consistency with the
// rest of the TUI's event feed.
var (
	pulumiBorderOpen = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8")).
				Padding(0, 1)
	pulumiBorderDone = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("6")).
				Padding(0, 1)
	pulumiBorderErr = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("1")).
			Padding(0, 1)
	pulumiHeaderStyle  = lipgloss.NewStyle().Bold(true)
	pulumiSubHeader    = lipgloss.NewStyle().Faint(true)
	pulumiMetaStyle    = lipgloss.NewStyle().Faint(true)
	pulumiOpCreate     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	pulumiOpUpdate     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	pulumiOpDelete     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	pulumiOpReplace    = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	pulumiOpRead       = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	pulumiOpRefresh    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	pulumiOpUnknown    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	pulumiDiagWarning  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	pulumiDiagError    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	pulumiStatusDim    = lipgloss.NewStyle().Faint(true)
	pulumiStatusDone   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	pulumiStatusFailed = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// addResource records a resource event on the block, deduping by URN. When
// a URN is seen again, the op stays from the first event (the ResourcePre
// that established what the engine is doing) and only the status is upgraded
// in place — later events like ResourceOutputs may carry only a status change.
func (s *pulumiBlockState) addResource(op display.StepOp, urn, typ, status string) {
	if existing, ok := s.resourceByURN[urn]; ok {
		if status != "" {
			s.resources[existing].status = status
		}
		return
	}
	s.resourceByURN[urn] = len(s.resources)
	s.resources = append(s.resources, pulumiResourceRow{op: op, urn: urn, typ: typ, status: status})
}

// findOpenPulumiBlock returns the index of the most recently added, still-open
// blockPulumiOp for the given tool name. An "open" block is one that hasn't
// received UIPulumiEnd yet (state.done == false). Returns -1 if none.
func (m *Model) findOpenPulumiBlock(toolName string) int {
	for i := len(m.blocks) - 1; i >= 0; i-- {
		b := m.blocks[i]
		if b.kind != blockPulumiOp || b.pulumi == nil {
			continue
		}
		if b.pulumi.toolName == toolName && !b.pulumi.done {
			return i
		}
	}
	return -1
}

// renderPulumiBlock produces the rendered string for a blockPulumiOp from its
// accumulated state. Called from renderBlock on every update and on resize.
func (m *Model) renderPulumiBlock(st *pulumiBlockState) string {
	if st == nil {
		return ""
	}

	title := "PulumiPreview"
	if !st.isPreview {
		title = "PulumiUp"
	}
	if st.stackName != "" {
		title = fmt.Sprintf("%s · %s", title, st.stackName)
	}
	header := pulumiHeaderStyle.Render(title)
	var status string
	switch {
	case !st.done:
		status = pulumiMetaStyle.Render(" · running")
	case st.err != "":
		status = pulumiStatusFailed.Render(" · failed")
	default:
		status = pulumiStatusDone.Render(" · done")
	}
	if st.elapsed != "" {
		status += pulumiMetaStyle.Render(" · " + st.elapsed)
	}

	var body strings.Builder
	body.WriteString(header + status + "\n")

	if len(st.resources) > 0 {
		resourcesHeader := "Planned changes"
		if !st.isPreview {
			resourcesHeader = "Changes"
		}
		body.WriteString("\n")
		body.WriteString(pulumiSubHeader.Render(resourcesHeader) + "\n")
		// Sort resources: group by op so the block reads top-down as "creates,
		// updates, replaces, deletes, …". Within each op, keep insertion order
		// so streaming doesn't shuffle earlier lines.
		rows := append([]pulumiResourceRow(nil), st.resources...)
		sort.SliceStable(rows, func(i, j int) bool {
			return tools.OpSortRank(rows[i].op) < tools.OpSortRank(rows[j].op)
		})
		for _, r := range rows {
			body.WriteString(renderPulumiResourceLine(r) + "\n")
		}
	}

	if len(st.diags) > 0 {
		body.WriteString("\n")
		body.WriteString(pulumiSubHeader.Render("Diagnostics") + "\n")
		for _, d := range st.diags {
			body.WriteString(renderPulumiDiagLine(d) + "\n")
		}
	}

	if len(st.counts) > 0 || st.done {
		body.WriteString("\n")
		body.WriteString(pulumiMetaStyle.Render(renderPulumiCounts(st.counts)) + "\n")
	}
	if st.err != "" {
		body.WriteString("\n")
		body.WriteString(pulumiStatusFailed.Render("error: "+st.err) + "\n")
	}

	// Choose the border color based on terminal state. Pad the box to fit
	// comfortably in the viewport — subtract 4 for the border+padding glyphs.
	style := pulumiBorderOpen
	switch {
	case st.err != "":
		style = pulumiBorderErr
	case st.done:
		style = pulumiBorderDone
	}
	width := max(m.width-4, 20)
	return style.Width(width).Render(strings.TrimRight(body.String(), "\n"))
}

// renderPulumiResourceLine renders one resource row with an op-colored symbol,
// a short name, the fully-qualified type (dimmed), and a status suffix when
// relevant (e.g. "running", "done", "failed" for live up operations).
func renderPulumiResourceLine(r pulumiResourceRow) string {
	sym, colored := pulumiOpGlyph(r.op)
	// Extract the short name from the URN (the last `::`-delimited segment).
	name := r.urn
	if i := strings.LastIndex(name, "::"); i >= 0 {
		name = name[i+2:]
	}
	line := fmt.Sprintf("  %s %s", colored.Render(sym), name)
	if r.typ != "" {
		line += " " + pulumiStatusDim.Render("("+r.typ+")")
	}
	switch r.status {
	case "running":
		line += " " + pulumiMetaStyle.Render("…")
	case "done":
		line += " " + pulumiStatusDone.Render("✓")
	case "failed":
		line += " " + pulumiStatusFailed.Render("✗")
	}
	return line
}

// renderPulumiDiagLine renders one diagnostic, trimming excessive whitespace so
// multi-line messages render as one line in the block. If the URN is present,
// it's appended as a dim suffix so errors can be associated with resources
// without visually dominating the row.
func renderPulumiDiagLine(d pulumiDiagRow) string {
	glyph := "⚠"
	style := pulumiDiagWarning
	switch strings.ToLower(d.severity) {
	case "error", "fatal":
		glyph = "✗"
		style = pulumiDiagError
	case "info", "debug":
		glyph = "ℹ"
		style = pulumiStatusDim
	}
	msg := strings.TrimSpace(strings.ReplaceAll(d.message, "\n", " "))
	line := fmt.Sprintf("  %s %s", style.Render(glyph), msg)
	if d.urn != "" {
		short := d.urn
		if i := strings.LastIndex(short, "::"); i >= 0 {
			short = short[i+2:]
		}
		line += " " + pulumiStatusDim.Render("("+short+")")
	}
	return line
}

// renderPulumiCounts prints a compact "3 create · 1 update" counts line with
// deterministic ordering. Empty / all-same maps render as "no changes". The
// underlying ordering and filtering live in tools.FormatChangeCounts so the
// agent-facing UpdateSummary text and the live TUI footer stay in sync.
func renderPulumiCounts(counts display.ResourceChanges) string {
	out := tools.FormatChangeCounts(counts, " · ")
	if out == "" {
		return "no changes"
	}
	return out
}

// pulumiOpGlyph picks the symbol and color for each StepOp. Mirrors the Pulumi
// CLI's diff display: + creates, - deletes, ~ updates, +- replaces. Uses the
// typed StepOp constants from pkg/resource/deploy so adding a new op causes a
// compiler-visible miss in the switch (it falls through to the "·" default).
func pulumiOpGlyph(op display.StepOp) (string, lipgloss.Style) {
	switch op {
	case deploy.OpCreate, deploy.OpCreateReplacement:
		return "+", pulumiOpCreate
	case deploy.OpUpdate:
		return "~", pulumiOpUpdate
	case deploy.OpDelete, deploy.OpDeleteReplaced:
		return "-", pulumiOpDelete
	case deploy.OpReplace:
		return "+-", pulumiOpReplace
	case deploy.OpRead, deploy.OpReadReplacement:
		return "→", pulumiOpRead
	case deploy.OpRefresh:
		return "↻", pulumiOpRefresh
	case deploy.OpImport, deploy.OpImportReplacement:
		return "⇒", pulumiOpCreate
	default:
		return "·", pulumiOpUnknown
	}
}
