// Copyright 2024, Pulumi Corporation.
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

package newcmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
)

func TestSanitizeTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"https://user:pass@example.com/path?param=value", "https://example.com/path"},
		{"https://user:pass@example.com", "https://example.com"},
		{"https://example.com/path?param=value", "https://example.com/path"},
		{"ssh://user@hostname/project/repo", "ssh://hostname/project/repo"},
		{"typescript", "typescript"},
		{"aws-typescript", "aws-typescript"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := sanitizeTemplate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChooseTemplateNonInteractiveReturnsNil(t *testing.T) {
	t.Parallel()

	got, err := ChooseTemplate(
		[]cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}},
		display.Options{IsInteractive: false},
	)
	require.NoError(t, err)
	assert.Nil(t, got)
}

type fakeSource struct {
	project     []cmdTemplates.Template
	database    []cmdTemplates.Template
	vcsOrgs     []string
	all         []cmdTemplates.Template
	projectErr  error
	databaseErr error
	allErr      error
}

func (f fakeSource) ProjectTemplates() ([]cmdTemplates.Template, error) {
	return f.project, f.projectErr
}

func (f fakeSource) DatabaseTemplates() ([]cmdTemplates.Template, error) {
	return f.database, f.databaseErr
}

func (f fakeSource) VcsTemplateSourceOrgs() []string             { return f.vcsOrgs }
func (f fakeSource) Templates() ([]cmdTemplates.Template, error) { return f.all, f.allErr }

// sourceOf mirrors a local checkout with nothing published to the registry: project carries
// everything.
func sourceOf(templates ...cmdTemplates.Template) fakeSource {
	return fakeSource{project: templates, all: templates}
}

// unsplitSource is a source created to resolve a named template, whose cloud listing ran unsplit.
// Reaching for one of its guided fetches fails the test.
type unsplitSource struct{ all []cmdTemplates.Template }

const unsplitGuidedFetchMsg = "a source resolving a named template must not be asked for the guided fetches"

func (s unsplitSource) Templates() ([]cmdTemplates.Template, error) { return s.all, nil }

func (s unsplitSource) ProjectTemplates() ([]cmdTemplates.Template, error) {
	panic(unsplitGuidedFetchMsg)
}

func (s unsplitSource) DatabaseTemplates() ([]cmdTemplates.Template, error) {
	panic(unsplitGuidedFetchMsg)
}

func (s unsplitSource) VcsTemplateSourceOrgs() []string { panic(unsplitGuidedFetchMsg) }

func TestUseGuidedFlowOnlyWhenNothingIsNamedAndWeCanPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args newArgs
		want bool
	}{
		{"nothing named", newArgs{interactive: true}, true},
		{"template named", newArgs{interactive: true, templateNameOrURL: "aws-typescript"}, false},
		{
			"url named",
			newArgs{interactive: true, templateNameOrURL: "https://github.com/pulumi/examples"},
			false,
		},
		{"auto-accept", newArgs{interactive: true, yes: true}, false},
		{"non-interactive", newArgs{yes: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.args.useGuidedFlow())
		})
	}
}

// guidedArgs names no template, so template selection takes the guided flow.
var guidedArgs = newArgs{interactive: true}

func TestResolveTemplateFallsBackToFlatWhenNothingIsCurated(t *testing.T) {
	t.Parallel()

	// A name the catalog can't decompose yields no providers, so guided must defer to the flat list.
	sel, offered := scriptedSelect(t, "second-name    ")
	var notice bytes.Buffer
	got, err := resolveTemplate(
		sourceOf(fakeTemplate{name: "unparseable"}, fakeTemplate{name: "second-name"}),
		guidedArgs, display.Options{Stdout: &notice}, sel,
	)
	require.NoError(t, err)
	assert.Equal(t, "second-name", got.Name())

	require.Len(t, *offered, 1, "the flat list is the only prompt shown")
	assert.Contains(t, notice.String(), "Falling back to the full template list.")
}

func TestResolveTemplateFallbackHandlesTinyTemplateSets(t *testing.T) {
	t.Parallel()

	sel := func(string, []string, display.Options) (int, error) {
		t.Error("no prompt may be shown for zero or one template")
		return 0, nil
	}

	_, err := resolveTemplate(fakeSource{}, guidedArgs, display.Options{}, sel)
	assert.ErrorContains(t, err, "no templates")

	got, err := resolveTemplate(
		sourceOf(fakeTemplate{name: "unparseable"}), guidedArgs, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, "unparseable", got.Name(), "a single template is chosen without prompting")
}

func TestResolveTemplateBrowseAllListsInline(t *testing.T) {
	t.Parallel()

	sel, _ := scriptedSelect(t, optionBrowseAll, "aws-typescript    ")

	got, err := resolveTemplate(
		sourceOf(fakeTemplate{name: "aws-typescript"}), guidedArgs, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, "aws-typescript", got.Name())
}

func TestResolveTemplateMapsInterruptToNoTemplateSelected(t *testing.T) {
	t.Parallel()

	sel := func(string, []string, display.Options) (int, error) {
		return 0, terminal.InterruptErr
	}

	got, err := resolveTemplate(
		sourceOf(fakeTemplate{name: "aws-typescript"}), guidedArgs, display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorContains(t, err, "no template selected")
}

func TestResolveTemplateReturnsGuidedTemplate(t *testing.T) {
	t.Parallel()

	sel, _ := scriptedSelect(t, "AWS", "TypeScript")

	got, err := resolveTemplate(
		sourceOf(fakeTemplate{name: "aws-typescript"}), guidedArgs, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, "aws-typescript", got.Name())
}

func TestResolveTemplatePropagatesErrors(t *testing.T) {
	t.Parallel()

	sel := func(string, []string, display.Options) (int, error) {
		return 0, errors.New("boom")
	}

	_, err := resolveTemplate(
		sourceOf(fakeTemplate{name: "aws-typescript"}), guidedArgs, display.Options{}, sel)
	assert.ErrorContains(t, err, "boom")
}

func TestResolveTemplateWithoutGuidedFlowUsesTheFullSetOnly(t *testing.T) {
	t.Parallel()

	src := unsplitSource{all: []cmdTemplates.Template{
		fakeTemplate{name: "aws-go"},
		fakeTemplate{name: "gcp-go"},
	}}

	sel, offered := scriptedSelect(t, "gcp-go    ")
	got, err := resolveTemplate(
		src, newArgs{interactive: true, templateNameOrURL: "gcp"}, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, "gcp-go", got.Name())
	require.Len(t, *offered, 1, "a named template disambiguates against the flat list")

	noPrompt := func(string, []string, display.Options) (int, error) {
		t.Error("--yes must never prompt")
		return 0, nil
	}
	got, err = resolveTemplate(src, newArgs{yes: true}, display.Options{}, noPrompt)
	require.NoError(t, err)
	assert.Nil(t, got, "--yes takes no template rather than guessing among several")
}

func TestSortedForDisplaySortsAndMarksBroken(t *testing.T) {
	t.Parallel()

	sorted := sortedForDisplay([]cmdTemplates.Template{
		fakeTemplate{name: "zeta"},
		fakeTemplate{name: "broken", err: errors.New("boom")},
		fakeTemplate{name: "alpha"},
	})
	require.Len(t, sorted, 3)
	assert.Equal(t, "alpha", sorted[0].Name())
	assert.Equal(t, "zeta", sorted[1].Name())
	assert.Equal(t, "broken", sorted[2].Name(), "broken templates sort to the end")
	assert.Contains(t, templateLabeler(sorted)(sorted[2]), BrokenTemplateDescription)
}
