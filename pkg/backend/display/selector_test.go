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
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockTemplates() []workspace.Template {
	return []workspace.Template{
		{
			Name:        "moolumi-typescript",
			Description: "Deploys moolumi buckets to moolumi cloud in Typescript",
		},
		{
			Name:        "moolumi-python",
			Description: "Deploys moolumi buckets to moolumi cloud in Python",
		},
		{
			Name:        "moolumi-csharp",
			Description: "Deploys moolumi buckets to moolumi cloud in CSharp",
		},
		{
			Name:        "moolumi-go",
			Description: "Deploys moolumi buckets to moolumi cloud in Go",
		},
		{
			Name:        "moolumi-yaml",
			Description: "Deploys moolumi buckets to moolumi cloud in YAML",
		},
	}
}

// TestTemplatePickerItemDelegatePadding checks that all entries are equal length including padding.
func TestTemplatePickerItemDelegatePadding(t *testing.T) {
	t.Parallel()
	templates := mockTemplates()
	items := make([]list.Item, len(templates))
	for i, t := range templates {
		items[i] = newTemplatePickerItem(t)
	}

	delegate := newTemplatePickerItemDelegate(templates)
	l := list.New(items, &delegate, 0 /* width */, 0 /* height */)

	entries := make([]string, len(templates))

	for i, template := range templates {
		item := &templatePickerItem{t: template}

		var builder strings.Builder
		delegate.Render(&builder, l, 0, item)
		entries[i] = builder.String()
	}

	// Check that all entries are the same width.
	expectedLength := len(entries[0])
	assert.NotZero(t, expectedLength)
	for _, entry := range entries[1:] {
		assert.Len(t, entry, expectedLength)
	}
}

// TestTemplatePicker_search checks that filtering works properly.
func TestTemplatePicker_filter(t *testing.T) {
	t.Parallel()
	templates := mockTemplates()
	items := make([]list.Item, len(templates))
	for i, t := range templates {
		items[i] = newTemplatePickerItem(t)
	}

	picker := newTemplatePicker(templates)
	tm := teatest.NewTestModel(
		t,
		picker,
		teatest.WithInitialTermSize(300, 100),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("moolumi-typescript"))
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("/"),
	})

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Filter:"))
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	tm.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("moolumi-csharp"),
	})

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("moolumi-csharp")) &&
				!bytes.Contains(bts, []byte("moolumi-typescript"))
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	tm.Send(tea.KeyMsg{
		Type: tea.KeyEnter,
	})

	tm.Send(tea.KeyMsg{
		Type: tea.KeyEnter,
	})

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return true
		},
		teatest.WithCheckInterval(time.Millisecond*100),
		teatest.WithDuration(time.Second*3),
	)

	err := tm.Quit()
	assert.NoError(t, err)
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	final := tm.FinalModel(t)
	picker = final.(templatePicker)

	// Verify the return value of the program.
	assert.Equal(t, "moolumi-csharp", picker.choice.Template().Name)
}

// TestTemplatePicker_pick checks that the picker selects the right template based on arrow keys.
func TestTemplatePicker_pick(t *testing.T) {
	t.Parallel()

	var model tea.Model = newTemplatePicker([]workspace.Template{
		{Name: "foo", ProjectDescription: "my fancy template"},
		{Name: "bar", ProjectDescription: "another template to do things"},
		{Name: "baz", ProjectDescription: "a third template"},
	})
	model.Init()

	// Set the initial window size.
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Verify that all templates are shown in the initial view.
	view := model.View()
	assert.Contains(t, view, "Please choose a template")
	assert.Regexp(t, `foo\s+my fancy template`, view)
	assert.Regexp(t, `bar\s+another template to do things`, view)
	assert.Regexp(t, `baz\s+a third template`, view)

	// The first template is selected.
	assert.Regexp(t, `>.+foo`, view, "foo must be selected")

	// Select the second template.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	view = model.View()
	assert.Regexp(t, `>.+bar`, view, "bar must be selected")

	// Pick this template.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := model.(templatePicker)

	require.NotNil(t, final.choice, "a template must be selected")
	assert.Equal(t, "bar", final.choice.t.Name, "bar must be selected")
}

// TestTemplatePicker_cancel checks that the picker handles Control+C and has a nil choice.
func TestTemplatePicker_cancel(t *testing.T) {
	t.Parallel()

	var model tea.Model = newTemplatePicker([]workspace.Template{
		{Name: "foo", ProjectDescription: "my fancy template"},
		{Name: "bar", ProjectDescription: "another template to do things"},
		{Name: "baz", ProjectDescription: "a third template"},
	})
	model.Init()

	// Set the initial window size.
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Sanity check.
	view := model.View()
	assert.Contains(t, view, "Please choose a template")

	// The first template is selected.
	assert.Regexp(t, `>.+foo`, view, "foo must be selected")

	// Cancel the template picker.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	final := model.(templatePicker)

	assert.Nil(t, final.choice, "no template must be selected")
}
