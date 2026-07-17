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
	"errors"
	"testing"

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

func TestTemplateChooserPicksGuidedOnlyWhenNothingIsNamed(t *testing.T) {
	t.Parallel()

	flat := func([]cmdTemplates.Template, display.Options) (cmdTemplates.Template, error) {
		return fakeTemplate{name: "flat"}, nil
	}
	guided := func([]cmdTemplates.Template, display.Options) (cmdTemplates.Template, error) {
		return fakeTemplate{name: "guided"}, nil
	}

	tests := []struct {
		name              string
		templateNameOrURL string
		expected          string
	}{
		{"nothing named", "", "guided"},
		{"template named", "aws-typescript", "flat"},
		{"url named", "https://github.com/pulumi/examples", "flat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := newArgs{
				chooseTemplate:       flat,
				chooseTemplateGuided: guided,
				templateNameOrURL:    tt.templateNameOrURL,
			}
			got, err := args.templateChooser()(nil, display.Options{})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got.Name())
		})
	}
}

func TestGuidedChooserFallsBackToFlatWhenNothingIsCurated(t *testing.T) {
	t.Parallel()

	flatCalled := false
	flat := func([]cmdTemplates.Template, display.Options) (cmdTemplates.Template, error) {
		flatCalled = true
		return fakeTemplate{name: "flat"}, nil
	}
	sel := func(string, []string, display.Options) (string, error) {
		t.Error("no prompt may be shown when the catalog is empty")
		return "", nil
	}

	// A name the catalog can't decompose yields no providers, so guided must defer to the flat list.
	got, err := guidedChooser(sel, flat)(
		[]cmdTemplates.Template{fakeTemplate{name: "unparseable"}},
		display.Options{IsInteractive: true},
	)
	require.NoError(t, err)
	assert.True(t, flatCalled)
	assert.Equal(t, "flat", got.Name())
}

func TestGuidedChooserReturnsGuidedTemplateWithoutFlat(t *testing.T) {
	t.Parallel()

	flat := func([]cmdTemplates.Template, display.Options) (cmdTemplates.Template, error) {
		t.Error("flat chooser must not run when guided resolves a template")
		return nil, nil
	}
	sel, _ := scriptedSelect(t, "AWS", "TypeScript")

	got, err := guidedChooser(sel, flat)(
		[]cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}},
		display.Options{IsInteractive: true},
	)
	require.NoError(t, err)
	assert.Equal(t, "aws-typescript", got.Name())
}

func TestGuidedChooserPropagatesErrors(t *testing.T) {
	t.Parallel()

	flat := func([]cmdTemplates.Template, display.Options) (cmdTemplates.Template, error) {
		t.Error("flat chooser must not run when guided errors")
		return nil, nil
	}
	sel := func(string, []string, display.Options) (string, error) {
		return "", errors.New("no template selected")
	}

	_, err := guidedChooser(sel, flat)(
		[]cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}},
		display.Options{IsInteractive: true},
	)
	assert.ErrorContains(t, err, "no template selected")
}

func TestGuidedChooserNonInteractiveReturnsNil(t *testing.T) {
	t.Parallel()

	sel := func(string, []string, display.Options) (string, error) {
		t.Error("no prompt may be shown when non-interactive")
		return "", nil
	}

	got, err := guidedChooser(sel, ChooseTemplate)(
		[]cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}},
		display.Options{IsInteractive: false},
	)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTemplatesToOptionArrayAndMapSortsAndMarksBroken(t *testing.T) {
	t.Parallel()

	options, byOption := templatesToOptionArrayAndMap([]cmdTemplates.Template{
		fakeTemplate{name: "zeta"},
		fakeTemplate{name: "broken", err: errors.New("boom")},
		fakeTemplate{name: "alpha"},
	})
	require.Len(t, options, 3)
	assert.Contains(t, options[0], "alpha")
	assert.Contains(t, options[1], "zeta")
	assert.Equal(t, "alpha", byOption[options[0]].Name())

	assert.Contains(t, options[2], "broken", "broken templates sort to the end")
	assert.Contains(t, options[2], BrokenTemplateDescription)
}
