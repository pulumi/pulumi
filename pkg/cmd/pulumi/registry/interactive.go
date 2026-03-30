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

package registry

import (
	"fmt"
	"io"
	"sort"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	surveyui "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// registryItem implements list.Item for the bubbles list component.
type registryItem struct {
	title      string // full formatted line for display
	filterText string // text used for fuzzy filtering (e.g., just the name)
	value      int    // index into the original slice
}

func (i registryItem) FilterValue() string { return i.title }
func (i registryItem) Title() string       { return i.title }
func (i registryItem) Description() string { return "" }

// registryDelegate renders list items with a compact single-line style.
type registryDelegate struct{}

func (d registryDelegate) Height() int                             { return 1 }
func (d registryDelegate) Spacing() int                            { return 0 }
func (d registryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d registryDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(registryItem)
	if !ok {
		return
	}

	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	line := cursor + i.title
	if index == m.Index() {
		line = selectedStyle.Render(line)
	} else {
		line = normalStyle.Render(line)
	}

	fmt.Fprint(w, line)
}

var (
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14")) // bright cyan, matching SpecPrompt
	normalStyle = lipgloss.NewStyle()
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13")) // bright magenta, matching SpecHeadline
	countStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

// interactiveListModel is the bubbletea model for browsing registry items.
type interactiveListModel struct {
	list       list.Model
	header     string // column header line shown above the items
	totalCount int    // total number of items
	showCount  bool   // whether to show the item count at bottom
	choice     int    // index of selected item, -1 if none
	quitting   bool
}

func (m interactiveListModel) Init() tea.Cmd {
	return nil
}

func (m interactiveListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			// During filtering, intercept enter to accept filter AND select in one step.
			if key.Matches(msg, key.NewBinding(key.WithKeys("enter"))) {
				// Let the list accept the filter first.
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(msg)
				// Now select the current item.
				if item, ok := m.list.SelectedItem().(registryItem); ok {
					m.choice = item.value
				}
				m.quitting = true
				return m, tea.Sequence(cmd, tea.Quit)
			}
			break
		}
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if item, ok := m.list.SelectedItem().(registryItem); ok {
				m.choice = item.value
			}
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"))):
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		extra := 2 // header + potential filter line
		if m.showCount {
			extra++ // count line at bottom
		}
		m.list.SetHeight(msg.Height - extra)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m interactiveListModel) View() string {
	view := m.list.View()

	// Remove blank lines that the list component emits for empty title/status areas.
	lines := strings.Split(view, "\n")
	var cleaned []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	view = strings.Join(cleaned, "\n")

	if m.header != "" {
		headerLine := headerStyle.Render("  " + m.header)
		// If a filter is applied (but input hidden), show it below the header.
		if m.list.FilterState() == list.FilterApplied {
			filterText := m.list.FilterValue()
			if filterText != "" {
				headerLine += "\n" + countStyle.Render(fmt.Sprintf("  Filter: %s (esc to clear)", filterText))
			}
		}
		view = headerLine + "\n" + view
	}
	if m.showCount {
		view += "\n" + countStyle.Render(fmt.Sprintf("  %d items", m.totalCount))
	}
	return view
}

// runInteractiveList shows an interactive filterable list and returns the index
// of the selected item, or -1 if the user quit without selecting.
func runInteractiveList(title string, header string, items []registryItem) (int, error) {
	return runInteractiveListOpts(title, header, items, true)
}

// runInteractiveListInline renders a short inline menu using survey (no alt-screen),
// so prior output stays visible above the list. Use for small menus only.
func runInteractiveListInline(title string, _ string, items []registryItem) (int, error) {
	options := make([]string, len(items))
	for i, item := range items {
		options[i] = item.title
	}
	var selected string
	prompt := &survey.Select{
		Message:  title,
		Options:  options,
		PageSize: len(options),
	}
	if err := survey.AskOne(prompt, &selected, surveyui.SurveyIcons(cmdutil.GetGlobalColorization())); err != nil {
		return -1, nil //nolint:nilerr // user cancelled
	}
	for i, opt := range options {
		if opt == selected {
			return items[i].value, nil
		}
	}
	return -1, nil
}

// runInteractiveListWithBanner shows a pre-rendered banner (e.g., package overview)
// above a short menu in alt-screen. The banner is part of the view, not separate output.
func runInteractiveListWithBanner(banner string, items []registryItem) (int, error) {
	options := make([]string, len(items))
	for i, item := range items {
		options[i] = item.title
	}

	m := bannerMenuModel{
		banner:  banner,
		options: options,
		items:   items,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return -1, err
	}
	final := result.(bannerMenuModel)
	return final.choice, nil
}

// bannerMenuModel shows a rendered banner with a simple menu below it.
type bannerMenuModel struct {
	banner  string
	options []string
	items   []registryItem
	cursor  int
	choice  int
}

func (m bannerMenuModel) Init() tea.Cmd { return nil }

func (m bannerMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.choice = m.items[m.cursor].value
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.choice = -1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m bannerMenuModel) View() string {
	var b strings.Builder
	b.WriteString(m.banner)
	b.WriteString("\n")

	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
			b.WriteString(selectedStyle.Render(cursor+opt) + "\n")
		} else {
			b.WriteString(normalStyle.Render(cursor+opt) + "\n")
		}
	}

	b.WriteString("\n" + countStyle.Render("  ↑/↓ navigate • enter select • q quit"))
	return b.String()
}

// nameBoostingFilter is a custom filter that matches against the full title
// but boosts items where the filterText (name) matches more strongly.
func nameBoostingFilter(items []registryItem) list.FilterFunc {
	return func(term string, targets []string) []list.Rank {
		// Get default fuzzy matches against full title.
		ranks := fuzzy.Find(term, targets)
		sort.Stable(ranks)

		result := make([]list.Rank, len(ranks))
		for i, r := range ranks {
			result[i] = list.Rank{
				Index:          r.Index,
				MatchedIndexes: r.MatchedIndexes,
			}
		}

		// Boost: if the term matches the filterText (name) as a prefix or exact match,
		// push those items to the top.
		termLower := strings.ToLower(term)
		sort.SliceStable(result, func(i, j int) bool {
			iName := strings.ToLower(items[result[i].Index].filterText)
			jName := strings.ToLower(items[result[j].Index].filterText)

			iExact := iName == termLower
			jExact := jName == termLower
			if iExact != jExact {
				return iExact
			}

			iPrefix := strings.HasPrefix(iName, termLower)
			jPrefix := strings.HasPrefix(jName, termLower)
			if iPrefix != jPrefix {
				return iPrefix
			}

			iContains := strings.Contains(iName, termLower)
			jContains := strings.Contains(jName, termLower)
			if iContains != jContains {
				return iContains
			}

			return false // keep original fuzzy order
		})

		return result
	}
}

// showInViewport renders markdown content in a scrollable alt-screen viewport.
// Press q/esc to return.
func showInViewport(markdownContent string) {
	// Use a wide wrap (120 chars) that wraps prose but gives tables room.
	// Tables that exceed this still get truncated rather than wrapped into unreadable multi-line cells.
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(120),
		glamour.WithStylesFromJSONBytes([]byte(`{"document":{"margin":0}}`)),
	)
	var content string
	if err == nil {
		content, err = renderer.Render(markdownContent)
		if err != nil {
			content = markdownContent
		}
	} else {
		content = markdownContent
	}

	m := viewportModel{content: content}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, _ = p.Run()
}

type viewportModel struct {
	viewport viewport.Model
	content  string
	ready    bool
}

func (m viewportModel) Init() tea.Cmd { return nil }

func (m viewportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-1)
			m.viewport.SetContent(m.content)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 1
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m viewportModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	return m.viewport.View() + "\n" + countStyle.Render("  ↑/↓ scroll • q back")
}

// showTabbedCode shows code examples with language tabs in alt-screen.
// Left/right or tab to switch languages, q to go back.
func showTabbedCode(title string, languages []string, codeByLang map[string]string) {
	m := tabbedCodeModel{
		title:     title,
		languages: languages,
		code:      codeByLang,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, _ = p.Run()
}

type tabbedCodeModel struct {
	title     string
	languages []string
	code      map[string]string
	active    int
	viewport  viewport.Model
	ready     bool
}

func (m tabbedCodeModel) Init() tea.Cmd { return nil }

func (m tabbedCodeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.active = (m.active + 1) % len(m.languages)
			m.updateContent()
		case "shift+tab", "left", "h":
			m.active = (m.active - 1 + len(m.languages)) % len(m.languages)
			m.updateContent()
		}
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-3) // room for title + tabs + help
			m.updateContent()
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 3
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *tabbedCodeModel) updateContent() {
	lang := m.languages[m.active]
	code := m.code[lang]

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
		glamour.WithStylesFromJSONBytes([]byte(`{"document":{"margin":0}}`)),
	)
	var content string
	if err == nil {
		md := fmt.Sprintf("```%s\n%s\n```\n", lang, code)
		content, err = renderer.Render(md)
		if err != nil {
			content = code
		}
	} else {
		content = code
	}
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

func (m tabbedCodeModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Title.
	titleLine := lipgloss.NewStyle().Bold(true).Render(m.title)

	// Tab bar.
	var tabs strings.Builder
	for i, lang := range m.languages {
		if i == m.active {
			tab := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("14")).
				Padding(0, 1).
				Render(lang)
			tabs.WriteString(tab)
		} else {
			tab := lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				Padding(0, 1).
				Render(lang)
			tabs.WriteString(tab)
		}
		tabs.WriteString(" ")
	}

	help := countStyle.Render("  ←/→ switch language • ↑/↓ scroll • q back")

	return titleLine + "\n" + tabs.String() + "\n" + m.viewport.View() + "\n" + help
}

func runInteractiveListOpts(title string, header string, items []registryItem, altScreen bool) (int, error) {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	l := list.New(listItems, registryDelegate{}, 80, 20)
	l.Title = ""
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)
	l.Filter = nameBoostingFilter(items)
	l.Styles.Title = lipgloss.NewStyle()

	m := interactiveListModel{list: l, header: header, totalCount: len(items), showCount: true, choice: -1}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return -1, err
	}

	final := result.(interactiveListModel)
	return final.choice, nil
}
