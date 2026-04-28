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
	"cmp"
	"fmt"
	"io"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma"
	"github.com/charmbracelet/x/ansi"
	mdstyles "github.com/pgavlin/markdown-kit/styles"
	mdview "github.com/pgavlin/markdown-kit/view"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/api"
)

// browseItem wraps an Operation so it fits bubbles/list.Item.
type browseItem struct{ op *api.Operation }

func (i browseItem) Title() string {
	return fmt.Sprintf("%-6s %s", i.op.Method, i.op.Path)
}

func (i browseItem) Description() string {
	summary := strings.TrimSpace(i.op.Summary)
	if summary == "" {
		summary = i.op.OperationID
	}
	return fmt.Sprintf("[%s]  %s", i.op.Tag, summary)
}

// FilterValue is what the list's built-in filter matches against.
func (i browseItem) FilterValue() string {
	return strings.Join([]string{
		i.op.Tag, i.op.Method, i.op.Path, i.op.OperationID, i.op.Summary,
	}, " ")
}

// categoryItem wraps an OpenAPI tag plus its member operations. It's the
// "folder" row in the categories view; selecting one drills down to an
// endpoints-mode list scoped to that tag.
type categoryItem struct {
	tag string
	ops []*api.Operation
	// filterValue pre-joins every underlying operation's FilterValue so the
	// list's built-in matcher surfaces a category when a filter matches any
	// endpoint inside it (e.g. typing "ListStacks" keeps the "Stacks" tag).
	filterValue string
}

func (c categoryItem) Title() string       { return c.tag }
func (c categoryItem) Description() string { return "" }
func (c categoryItem) FilterValue() string { return c.filterValue }

// singleLineDelegate renders each item on exactly one line and truncates
// (rather than wraps) items that exceed the list's current width, using
// ansi.Truncate so styling stays intact.
type singleLineDelegate struct {
	theme Theme
}

func (d *singleLineDelegate) Height() int                             { return 1 }
func (d *singleLineDelegate) Spacing() int                            { return 0 }
func (d *singleLineDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d *singleLineDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	selected := index == m.Index()
	prefix := "  "
	if selected {
		prefix = d.theme.Accent.Render("▸ ")
	}
	available := m.Width() - 2
	if available < 1 {
		available = 1
	}

	var row string
	switch it := item.(type) {
	case browseItem:
		// Method chip — colored per HTTP verb; Accent overrides when the row is
		// selected so the cursor still reads at a glance.
		methodStyle := d.theme.MethodStyle(it.op.Method)
		if selected {
			methodStyle = d.theme.Accent
		}
		method := methodStyle.Render(fmt.Sprintf("%-6s", it.op.Method))

		path := it.op.Path
		if selected {
			path = d.theme.Accent.Render(path)
		}

		var chips []string
		if it.op.IsPreview {
			chips = append(chips, d.theme.Accent.Render("[PREVIEW]"))
		}
		if it.op.IsDeprecated {
			chips = append(chips, d.theme.Warning.Render("[DEPRECATED]"))
		}

		row = method + " " + path
		if len(chips) > 0 {
			row += " " + strings.Join(chips, " ")
		}

	case categoryItem:
		tagStyle := lipgloss.NewStyle().Bold(true)
		if selected {
			tagStyle = d.theme.Accent
		}
		countStyle := d.theme.Dim
		if selected {
			countStyle = d.theme.Accent
		}
		row = tagStyle.Render(it.tag) + "  " + countStyle.Render(fmt.Sprintf("(%d)", len(it.ops)))

	default:
		return
	}

	fmt.Fprint(w, prefix+ansi.Truncate(row, available, "…"))
}

// browseFocus is which sub-pane receives keyboard input.
type browseFocus int

const (
	focusList browseFocus = iota
	focusDetails
)

// browseMode is which view the list pane is showing.
type browseMode int

const (
	// modeCategories shows one row per OpenAPI tag. Enter on a row drills into
	// that tag's endpoints. This is the default/startup view.
	modeCategories browseMode = iota
	// modeEndpoints shows every endpoint in the currently selected tag.
	// Esc or Backspace (when not filtering) returns to modeCategories.
	modeEndpoints
)

// browseModel is the Browse-tab model: a filterable list on the left and
// a scrollable markdown view on the right. Each pane is wrapped in a
// rounded-border lipgloss style; the split is user-resizable.
//
// The details pane is an `mdview.Model` from pgavlin/markdown-kit which gives
// us GFM rendering, `/`-search, vim-style selection, and heading navigation
// out of the box. Its KeyMap is toggled on/off with focus so its keys don't
// shadow the list's `/` filter when focus is on the list.
type browseModel struct {
	theme   Theme
	list    list.Model
	details mdview.Model

	// Mouse events route by cursor coordinate, independent of focus.
	focus browseFocus

	// mode + its supporting data: categoriesItems is the stable tag-level
	// list; endpointsByTag is a prebuilt per-tag slice of browseItems so
	// drilling in is an O(1) SetItems swap. selectedTag tracks which tag
	// we drilled into (for the list title + back navigation).
	mode            browseMode
	categoriesItems []list.Item
	endpointsByTag  map[string][]list.Item
	categoriesByTag map[string]*categoryItem
	selectedTag     string

	lastIndex     int
	width, height int
	innerRightW   int // last width fed to details; tracked to avoid redundant resizes
}

// listPct is the fixed horizontal share the list pane gets; the details
// pane gets the rest. A 50/50 split reads well for method+path lines
// alongside schema dumps without either pane feeling cramped.
const (
	listPct  = 0.5
	minPaneW = 32
)

func newBrowseModel(theme Theme, idx *api.Index) browseModel {
	categoriesItems, endpointsByTag, categoriesByTag := buildBrowseGrouping(idx)

	l := list.New(categoriesItems, &singleLineDelegate{theme: theme}, 0, 0)
	l.Title = "Categories"
	l.SetShowStatusBar(true)
	l.SetShowHelp(false) // the outer model owns the footer

	// Without a chroma theme the renderer emits plain text (no bold, no color),
	// which makes headings and inline code indistinguishable from prose. We
	// derive from markdown-kit's `Pulumi` style and drop the dark-gray
	// background on inline code spans — the background clashes with our pane
	// background and makes code spans harder to read, not easier.
	md := mdview.NewModel(
		mdview.WithWrap(true),
		mdview.WithTheme(browseChromaStyle),
	)
	// Details pane starts unfocused — the list owns the initial focus, so
	// markdown-kit's keys (which include `/` for search, overlapping with the
	// list's `/` filter) must be silent until the user tabs into details.
	md.KeyMap.SetEnabled(false)

	m := browseModel{
		theme:           theme,
		list:            l,
		details:         md,
		mode:            modeCategories,
		categoriesItems: categoriesItems,
		endpointsByTag:  endpointsByTag,
		categoriesByTag: categoriesByTag,
		lastIndex:       -1,
	}
	m.refreshDetails()
	return m
}

// buildBrowseGrouping buckets the flat operations list by tag and builds the
// three lookup structures the list pane needs:
//   - categoriesItems: one categoryItem per tag, sorted by tag name, with a
//     FilterValue that contains every member endpoint's FilterValue so the
//     built-in matcher naturally surfaces a category when the filter matches
//     any endpoint inside it.
//   - endpointsByTag: prebuilt []list.Item per tag so drilling in is a cheap
//     SetItems swap.
//   - categoriesByTag: map lookup used to render the details pane in
//     categories mode.
func buildBrowseGrouping(idx *api.Index) ([]list.Item, map[string][]list.Item, map[string]*categoryItem) {
	endpointsByTag := make(map[string][]list.Item)
	opsByTag := make(map[string][]*api.Operation)

	for _, op := range idx.Operations {
		tag := op.Tag
		if tag == "" {
			tag = "Miscellaneous"
		}
		endpointsByTag[tag] = append(endpointsByTag[tag], browseItem{op: op})
		opsByTag[tag] = append(opsByTag[tag], op)
	}

	tags := make([]string, 0, len(opsByTag))
	for t := range opsByTag {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	categoriesItems := make([]list.Item, 0, len(tags))
	categoriesByTag := make(map[string]*categoryItem, len(tags))
	for _, tag := range tags {
		ops := opsByTag[tag]
		// Concatenate every member op's FilterValue so a filter like "ListStacks"
		// matches the "Stacks" category even though the string doesn't appear in
		// the tag name itself. Tag is included first so category-name matches
		// still work.
		var fvBuilder strings.Builder
		fvBuilder.WriteString(tag)
		for _, op := range ops {
			fvBuilder.WriteByte(' ')
			fvBuilder.WriteString((browseItem{op: op}).FilterValue())
		}
		ci := &categoryItem{
			tag:         tag,
			ops:         ops,
			filterValue: fvBuilder.String(),
		}
		categoriesItems = append(categoriesItems, *ci)
		categoriesByTag[tag] = ci
	}
	return categoriesItems, endpointsByTag, categoriesByTag
}

// SelectedOp returns the op under the list's cursor, or nil when the cursor
// is on a category row or the list is empty.
func (m *browseModel) SelectedOp() *api.Operation {
	sel := m.list.SelectedItem()
	if bi, ok := sel.(browseItem); ok {
		return bi.op
	}
	return nil
}

// SelectedCategory returns the category under the cursor, or nil when the
// cursor is on an endpoint row or the list is empty.
func (m *browseModel) SelectedCategory() *categoryItem {
	sel := m.list.SelectedItem()
	if ci, ok := sel.(categoryItem); ok {
		return &ci
	}
	return nil
}

func (m *browseModel) refreshDetails() {
	switch m.mode {
	case modeEndpoints:
		op := m.SelectedOp()
		if op == nil {
			m.details.SetText("", "_No endpoint selected._")
			m.details.GotoTop()
			return
		}
		m.details.SetText(op.OperationID, api.RenderDescribeMarkdown(op))
		m.details.GotoTop()
	case modeCategories:
		cat := m.SelectedCategory()
		if cat == nil {
			m.details.SetText("", "_No category selected._")
			m.details.GotoTop()
			return
		}
		m.details.SetText(cat.tag, renderCategoryMarkdown(cat))
		m.details.GotoTop()
	}
}

// renderCategoryMarkdown produces a short overview for the details pane when
// a category row is highlighted: tag name, endpoint count, and a bullet list
// of `METHOD path` rows so users can decide whether to drill in.
func renderCategoryMarkdown(cat *categoryItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", cat.tag)
	fmt.Fprintf(&b, "_%d endpoint", len(cat.ops))
	if len(cat.ops) != 1 {
		b.WriteString("s")
	}
	b.WriteString("_\n\n")
	b.WriteString("## Endpoints\n\n")
	for _, op := range cat.ops {
		fmt.Fprintf(&b, "- `%s` %s", op.Method, op.Path)
		if op.IsPreview {
			b.WriteString(" _(preview)_")
		}
		if op.IsDeprecated {
			b.WriteString(" _(deprecated)_")
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n_Press `enter` to drill into this category._\n")
	return b.String()
}

// enterCategory drills the list pane into the endpoints of `tag`. The filter
// text (if any) carries over so a search like "ListStacks" narrows the
// drilled view to matching endpoints without the user having to re-type.
func (m *browseModel) enterCategory(tag string) {
	endpoints, ok := m.endpointsByTag[tag]
	if !ok {
		return
	}
	filter := m.list.FilterValue()
	state := m.list.FilterState()

	m.mode = modeEndpoints
	m.selectedTag = tag
	m.list.Title = tag
	m.list.SetItems(endpoints)

	// Preserve the filter. Applied (not Filtering) so the user isn't dropped
	// back into the filter input; they can refine with `/` if they want to.
	if filter != "" && state != list.Unfiltered {
		m.list.SetFilterText(filter)
		m.list.SetFilterState(list.FilterApplied)
	}
	m.lastIndex = -1
	m.refreshDetails()
}

// exitCategory pops the list pane back to the categories view, carrying the
// filter text across the same way `enterCategory` does.
func (m *browseModel) exitCategory() {
	filter := m.list.FilterValue()
	state := m.list.FilterState()

	m.mode = modeCategories
	m.list.Title = "Categories"
	m.list.SetItems(m.categoriesItems)

	if filter != "" && state != list.Unfiltered {
		m.list.SetFilterText(filter)
		m.list.SetFilterState(list.FilterApplied)
	}
	// Try to land the cursor on the tag we just left for visual continuity.
	if m.selectedTag != "" {
		for i, it := range m.list.VisibleItems() {
			if ci, ok := it.(categoryItem); ok && ci.tag == m.selectedTag {
				m.list.Select(i)
				break
			}
		}
	}
	m.selectedTag = ""
	m.lastIndex = -1
	m.refreshDetails()
}

// SetSize divides the width between the two panes according to listPct
// and shrinks the inner widgets by the pane's border+padding frame so
// the outer pane exactly matches the allocated space.
func (m *browseModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	outerLeft, outerRight := m.outerWidths()
	innerLeftW, innerH := m.theme.Inside(outerLeft, height, m.theme.PaneBorder)
	innerRightW, _ := m.theme.Inside(outerRight, height, m.theme.PaneBorder)

	m.list.SetSize(innerLeftW, innerH)
	m.details.SetSize(innerRightW, innerH)
	m.innerRightW = innerRightW
}

// outerWidths returns the two pane OUTER widths (including border +
// padding), clamped so neither pane shrinks below a readable minimum.
func (m browseModel) outerWidths() (leftW, rightW int) {
	leftW = clamp(int(float64(m.width)*listPct), minPaneW, m.width-minPaneW)
	rightW = m.width - leftW
	if rightW < minPaneW {
		rightW = minPaneW
		leftW = m.width - rightW
	}
	return leftW, rightW
}

// Init is required by tea.Model.
func (m browseModel) Init() tea.Cmd { return nil }

// Update forwards messages to the focused pane (or to both when the
// message isn't scroll/navigation) and syncs details when the selection
// changes. Mouse events bypass focus and route by cursor coordinate, so
// wheel scrolling works regardless of which pane owns keyboard focus.
func (m browseModel) Update(msg tea.Msg) (browseModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		return m.routeMouseWheel(msg), nil
	case tea.MouseMsg:
		return m.routeMouse(msg)
	case tea.KeyPressMsg:
		if key.Matches(msg, key.NewBinding(key.WithKeys("tab"))) {
			m.toggleFocus()
			return m, nil
		}
		return m.routeKey(msg)
	}

	// Fan out to both sub-models so their timers/blink cmds keep ticking.
	cmds := make([]tea.Cmd, 0, 2)
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	m.details, cmd = m.details.Update(msg)
	cmds = append(cmds, cmd)

	if idx := m.list.Index(); idx != m.lastIndex {
		m.lastIndex = idx
		m.refreshDetails()
	}
	return m, tea.Batch(cmds...)
}

// toggleFocus swaps which pane owns keyboard focus and toggles the markdown
// view's KeyMap so its bindings (`/`, `v`, `g`, ...) don't shadow the list's
// filter key when focus is on the list.
func (m *browseModel) toggleFocus() {
	if m.focus == focusList {
		m.focus = focusDetails
		m.details.KeyMap.SetEnabled(true)
	} else {
		m.focus = focusList
		m.details.KeyMap.SetEnabled(false)
	}
}

// routeKey delivers a key press to the focused pane, then syncs details
// if the list's cursor moved. Two-mode navigation is intercepted first:
//   - Esc in modeEndpoints pops back to the categories view (regardless of
//     which pane has focus, so Esc from a deep-dive into the details pane
//     also works).
//   - Enter on a category row (when the list has focus and isn't in filter-
//     typing mode) drills into that category.
func (m browseModel) routeKey(msg tea.KeyMsg) (browseModel, tea.Cmd) {
	// Esc-to-pop. We don't want to steal Esc while the user is typing into
	// the list's filter input (that Esc should cancel the filter instead),
	// and we don't want to steal it from the markdown view's active search
	// (markdown-kit binds `esc` to ClearSearch while a search is open). In
	// practice markdown-kit's ClearSearch fires before we see the Esc because
	// its KeyMap is engaged when focus is on the details pane — but once the
	// search is cleared, a second Esc reaches us and pops back out. That's
	// the UX we want.
	if key.Matches(msg, key.NewBinding(key.WithKeys("esc"))) &&
		m.mode == modeEndpoints &&
		m.list.FilterState() != list.Filtering {
		m.exitCategory()
		return m, nil
	}

	var cmd tea.Cmd
	if m.focus == focusDetails {
		m.details, cmd = m.details.Update(msg)
		return m, cmd
	}

	// Drill into a category on Enter, and support Backspace as an alternate
	// pop-back keystroke while focus is on the list pane. Both are gated on
	// the list NOT being in filter-typing mode so they don't clobber filter
	// input.
	if m.list.FilterState() != list.Filtering {
		if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) && m.mode == modeCategories {
			if cat := m.SelectedCategory(); cat != nil {
				m.enterCategory(cat.tag)
				return m, nil
			}
		}
		if key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))) && m.mode == modeEndpoints {
			m.exitCategory()
			return m, nil
		}
	}

	m.list, cmd = m.list.Update(msg)
	if idx := m.list.Index(); idx != m.lastIndex {
		m.lastIndex = idx
		m.refreshDetails()
	}
	return m, cmd
}

// routeMouse forwards non-wheel mouse events to the pane under the cursor.
// Wheel events are handled separately by routeMouseWheel because the
// markdown view doesn't subscribe to mouse events in its Update loop.
func (m browseModel) routeMouse(msg tea.MouseMsg) (browseModel, tea.Cmd) {
	outerLeft, _ := m.outerWidths()
	x := msg.Mouse().X
	if x < outerLeft {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	// Non-wheel mouse (click/drag) into the details pane — markdown-kit
	// doesn't currently handle these, so we swallow them rather than leak
	// them to both sub-models.
	return m, nil
}

// routeMouseWheel translates a wheel event into list navigation or markdown
// view scrolling based on the cursor's X coordinate.
func (m browseModel) routeMouseWheel(msg tea.MouseWheelMsg) browseModel {
	outerLeft, _ := m.outerWidths()
	if msg.Mouse().X < outerLeft {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		_ = cmd
		if idx := m.list.Index(); idx != m.lastIndex {
			m.lastIndex = idx
			m.refreshDetails()
		}
		return m
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.details.ScrollUp(3)
	case tea.MouseWheelDown:
		m.details.ScrollDown(3)
	case tea.MouseWheelLeft:
		m.details.ScrollLeft(3)
	case tea.MouseWheelRight:
		m.details.ScrollRight(3)
	}
	return m
}

// View composes the list and details panes side-by-side via the theme's
// Pane helper. Each pane clips its content to its inner area so overflow
// stays invisible behind the border instead of pushing the footer off
// screen.
func (m browseModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	outerLeft, outerRight := m.outerWidths()
	return joinPanes(
		m.theme.Pane(outerLeft, m.height, m.focus == focusList, m.list.View()),
		m.theme.Pane(outerRight, m.height, m.focus == focusDetails, m.details.View()),
	)
}

// clamp pins v into [lo, hi]. Works for any ordered type.
func clamp[T cmp.Ordered](v, lo, hi T) T {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// browseChromaStyle customizes markdown-kit's Pulumi palette for our use:
//   - CodeSpan loses the dark background (it clashed with pane bg) and
//     gets the pink accent so inline code still stands out.
//   - GenericStrong gains the pink accent so bold text — which we use for
//     section headings (via the transformer above) and for emphasis markers
//     like **required** — reads as a strong pink highlight.
var browseChromaStyle = func() *chroma.Style {
	s, err := mdstyles.Pulumi.Builder().
		Add(mdstyles.CodeSpan, "#d787af").
		Add(chroma.GenericStrong, "#d787af bold").
		Build()
	if err != nil {
		// Fall back to the stock Pulumi style rather than crash the TUI on a
		// style-build failure — worst case we see the old dark code band.
		return mdstyles.Pulumi
	}
	return s
}()
