// Copyright 2016-2023, Pulumi Corporation.
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

package display

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	_brokenTemplateDescription = "(This template is currently broken)"
	_colorCyan                 = "#00AAAA"
	_colorWhite                = "#FFFFFF"
	_colorGreen                = "#00FF00"
)

// ChooseTemplate will prompt the user to choose amongst the available templates.
func ChooseTemplate(templates []workspace.Template, opts Options) (workspace.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !opts.IsInteractive {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	picker := newTemplatePicker(templates)
	final, err := tea.NewProgram(picker).Run()
	if err != nil {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}
	picker = final.(templatePicker)
	if picker.choice == nil {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	return picker.choice.Template(), nil
}

// templatePicker renders a list of templates to choose from.
type templatePicker struct {
	// choice is set to the selected item
	// when the user presses enter.
	//
	// Its value is nil if a template was not selected.
	choice *templatePickerItem

	list      list.Model
	templates []workspace.Template
}

var _ tea.Model = (*templatePicker)(nil)

func (t templatePicker) Init() tea.Cmd { return nil }

func (t templatePicker) View() string { return t.list.View() }

func (t templatePicker) Update(msg tea.Msg) (_ tea.Model, cmd tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.list.SetSize(msg.Width, msg.Height)
		t.list.Styles.TitleBar = t.list.Styles.TitleBar.Width(t.list.Width())

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return t, tea.Quit

		case tea.KeyEnter:
			if t.list.FilterState() == list.Filtering {
				// If we're filtering, let the list handle the Enter key.
				// This will just accept the filter value.
				break
			}

			t.choice = t.list.SelectedItem().(*templatePickerItem)
			return t, tea.Quit
		}
	}

	t.list, cmd = t.list.Update(msg)
	return t, cmd
}

// newTemplatePicker builds a templatePicker model
// that lets users pick from the given template.
func newTemplatePicker(templates []workspace.Template) templatePicker {
	items := make([]list.Item, len(templates))
	for i, t := range templates {
		items[i] = newTemplatePickerItem(t)
	}
	// Width and height are determined by the terminal size on startup.
	delegate := newTemplatePickerItemDelegate(templates)
	list := list.New(items, &delegate, 0 /* width */, 0 /* height */)

	list.Title = "Please choose a template"
	list.Styles.Title = list.Styles.Title.
		Foreground(lipgloss.Color(_colorCyan)).
		Background(lipgloss.NoColor{})

	// Show Enter in the help menu (accessible with ?).
	list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select template"),
			),
		}
	}

	return templatePicker{
		list:      list,
		templates: templates,
	}
}

// templatePickerItem is a single item in the templatePicker list.
type templatePickerItem struct{ t workspace.Template }

var _ list.DefaultItem = (*templatePickerItem)(nil)

// Template returns the actual template in the choice.
func (i *templatePickerItem) Template() workspace.Template { return i.t }

// FilterValue is the value that will be used to filter this item when the user types.
func (i *templatePickerItem) FilterValue() string { return i.t.Name }

// Title reports the title to display for this item in the list.
func (i *templatePickerItem) Title() string { return i.t.Name }

// Description reports the description for this item.
func (i *templatePickerItem) Description() string {
	return workspace.ValueOrDefaultProjectDescription("", i.t.ProjectDescription, i.t.Description)
}

func newTemplatePickerItem(t workspace.Template) *templatePickerItem {
	return &templatePickerItem{t: t}
}

// templatePickerItemDelegate renders *templatePickerItem values as items in the list.
type templatePickerItemDelegate struct {
	// Width of the column where the title is displayed.
	// This should be larger than the width of the longest title.
	titleWidth int

	// Style used for the ">" marker when an item is selected.
	arrowStyle lipgloss.Style
}

// Height reports the height each item will take up when rendered.
func (d *templatePickerItemDelegate) Height() int { return 1 }

// Spacing is the gap between cells.
func (d *templatePickerItemDelegate) Spacing() int { return 0 }

// Update receives messages all events on the list.
// We don't have any per-tick updates to perform so this is a no-op.
func (d *templatePickerItemDelegate) Update(_ tea.Msg, m *list.Model) tea.Cmd { return nil }

// Render writes a single item to the given io.Writer.
// and highlights the portions of the item name that match the filter value.
func (d *templatePickerItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	t, ok := item.(*templatePickerItem)
	contract.Assertf(ok, "expected *templatePickerItem, got %T", item)

	// Whether the item is selected.
	selectorCol := "  "
	if index == m.Index() {
		selectorCol = d.arrowStyle.Render("> ")
	}
	title := t.Title()
	padding := d.titleWidth - len(title)
	// Add an extra 4 columns of padding between the title and description.
	padding += 4

	if padding < 1 {
		// This is not possible because we set the column width
		// to more than the length of the longest title.
		// Guard against it nonetheless.
		padding = 1
	}
	fmt.Fprintf(w, "%s%s%s%s", selectorCol, title, strings.Repeat(" ", padding), t.Description())
}

func newTemplatePickerItemDelegate(templates []workspace.Template) templatePickerItemDelegate {
	delegate := templatePickerItemDelegate{
		arrowStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(_colorGreen)),
	}
	// Set the delegate.titleWidth to fit the largest title.
	for _, t := range templates {
		curWidth := len(t.Name)
		if curWidth > delegate.titleWidth {
			delegate.titleWidth = curWidth
		}
	}
	return delegate
}
