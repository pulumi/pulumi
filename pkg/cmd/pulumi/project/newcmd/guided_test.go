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

package newcmd

import (
	"context"
	"errors"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
)

type fakeTemplate struct {
	name string
	desc string
	err  error
}

func (f fakeTemplate) Name() string        { return f.name }
func (f fakeTemplate) DisplayName() string { return f.name }
func (f fakeTemplate) Description() string { return f.desc }
func (f fakeTemplate) Error() error        { return f.err }
func (f fakeTemplate) FromRegistry() bool  { return false }
func (f fakeTemplate) Publisher() string   { return "" }
func (f fakeTemplate) Download(ctx context.Context) (cmdTemplates.ProjectTemplate, error) {
	return cmdTemplates.ProjectTemplate{}, nil
}

type fakeRegistryTemplate struct {
	fakeTemplate
	publisher string
}

func (f fakeRegistryTemplate) FromRegistry() bool { return true }
func (f fakeRegistryTemplate) Publisher() string  { return f.publisher }

// scriptedSelect answers each prompt in order, asserting the option offered is present.
func scriptedSelect(t *testing.T, answers ...string) (selectFunc, *[]([]string)) {
	t.Helper()
	var offered [][]string
	i := 0
	return func(message string, options []string, opts display.Options) (string, error) {
		offered = append(offered, options)
		require.Less(t, i, len(answers), "unexpected extra prompt: %q with %v", message, options)
		answer := answers[i]
		i++
		require.Contains(t, options, answer, "scripted answer %q not offered in %v", answer, options)
		return answer, nil
	}, &offered
}

func TestFromRegistry(t *testing.T) {
	t.Parallel()

	assert.False(t, fakeTemplate{name: "aws-typescript"}.FromRegistry())
	assert.True(t, fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}.FromRegistry())
}

func TestGuidedResolvesFeaturedProvider(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "aws-python"},
		fakeTemplate{name: "gcp-go"},
	}
	sel, _ := scriptedSelect(t, "AWS", "TypeScript")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name())
}

func TestGuidedShowsNoRegistryRowsWithoutRegistryTemplates(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, offered := scriptedSelect(t, "AWS", "TypeScript")

	_, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.Len(t, *offered, 2, "expected exactly cloud + language prompts")
	assert.Equal(t, []string{"AWS", optionOther, optionBrowseAll}, (*offered)[0])
}

func TestGuidedOtherExpandsToSecondProviderList(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "linode-go"}}
	sel, offered := scriptedSelect(t, optionOther, "Linode", "Go")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "linode-go", got.Name())

	require.Len(t, *offered, 3)
	assert.Contains(t, (*offered)[0], optionOther)
	assert.Contains(t, (*offered)[1], "Linode")
	assert.NotContains(t, (*offered)[1], "AWS", "featured providers must not repeat under Other")
}

func TestGuidedNoneResolvesToBareTemplate(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "typescript"},
	}
	sel, offered := scriptedSelect(t, "None", "TypeScript")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "typescript", got.Name())

	require.Len(t, *offered, 2, "None must go straight to the language step")
}

func TestGuidedNonePositionInCloudPrompt(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "azure-typescript"},
		fakeTemplate{name: "gcp-typescript"},
		fakeTemplate{name: "typescript"},
	}
	sel, offered := scriptedSelect(t, "None", "TypeScript")

	_, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, []string{"AWS", "Azure", "GCP", "None", optionOther, optionBrowseAll}, (*offered)[0])
}

func TestGuidedBrowseAllFallsBackToFlatList(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, _ := scriptedSelect(t, optionBrowseAll)

	got, err := chooseGuided(templates, display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errFallBackToFlatList)
}

func TestGuidedNoneJavaIsSplitByBuildSystem(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "java"},
		fakeTemplate{name: "java-gradle"},
	}
	sel, offered := scriptedSelect(t, "None", "Java (Gradle)")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "java-gradle", got.Name())

	assert.Contains(t, (*offered)[1], "Java (Maven)")
	assert.NotContains(t, (*offered)[1], "Java (JBang)")
}

func TestGuidedLanguageListIsFilteredToProvider(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "azure-typescript"}}
	sel, offered := scriptedSelect(t, "Azure", "TypeScript")

	_, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.Len(t, *offered, 2)
	assert.NotContains(t, (*offered)[1], "Scala", "Azure has no scala template")
	assert.NotContains(t, (*offered)[1], "Bun", "Azure has no bun template")
}

func TestGuidedExcludesBrokenTemplates(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "aws-go", err: errors.New("boom")},
	}
	sel, offered := scriptedSelect(t, "AWS", "TypeScript")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name())
	assert.NotContains(t, (*offered)[1], "Go", "broken templates must not be offered")
}

func TestGuidedFallsBackWhenEverythingIsBroken(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript", err: errors.New("boom")},
		fakeRegistryTemplate{fakeTemplate{name: "vpc", err: errors.New("boom")}, "acme"},
	}
	sel := func(string, []string, display.Options) (string, error) {
		t.Error("no prompt may be shown when every template is broken")
		return "", nil
	}

	got, err := chooseGuided(templates, display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errFallBackToFlatList, "the flat chooser is the only surface that marks broken templates")
}

func TestGuidedInterruptGoesBackToPreviousStep(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "gcp-go"},
	}
	responses := []any{"AWS", terminal.InterruptErr, "GCP", "Go"}
	i := 0
	sel := func(message string, options []string, opts display.Options) (string, error) {
		require.Less(t, i, len(responses), "unexpected extra prompt: %q", message)
		r := responses[i]
		i++
		if err, ok := r.(error); ok {
			return "", err
		}
		require.Contains(t, options, r.(string))
		return r.(string), nil
	}

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "gcp-go", got.Name(), "interrupt at the language step must return to the provider step")
}

func TestGuidedInterruptAtFirstStepPropagates(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel := func(string, []string, display.Options) (string, error) {
		return "", terminal.InterruptErr
	}

	got, err := chooseGuided(templates, display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, terminal.InterruptErr)
}

func TestGuidedOrgRowOpensRegistryTemplatesAndSkipsLanguage(t *testing.T) {
	t.Parallel()

	registry := fakeRegistryTemplate{fakeTemplate{name: "vpc-baseline", desc: "A baseline VPC"}, "acme"}
	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}, registry}
	sel, offered := scriptedSelect(t, "acme templates (1)", "vpc-baseline    A baseline VPC")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc-baseline", got.Name())

	require.Len(t, *offered, 2, "org path should be cloud prompt + template list, no language prompt")
	assert.Equal(t, []string{"AWS", optionOther, "acme templates (1)", optionBrowseAll}, (*offered)[0],
		"org rows sit between Other and Browse all")
}

func TestGuidedGroupsRegistryRowsByPublisher(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "globex"},
		fakeRegistryTemplate{fakeTemplate{name: "eks"}, "acme"},
		fakeRegistryTemplate{fakeTemplate{name: "gke"}, "acme"},
		fakeRegistryTemplate{fakeTemplate{name: "legacy"}, ""},
	}
	sel, offered := scriptedSelect(t, "acme templates (2)", "eks    ")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "eks", got.Name())

	assert.Equal(t, []string{
		"AWS", optionOther, "Registry templates (1)", "acme templates (2)", "globex templates (1)", optionBrowseAll,
	}, (*offered)[0], "one row per publisher, sorted, unpublished group labeled generically")
	assert.Equal(t, []string{"eks    ", "gke    "}, (*offered)[1], "org row lists only that org's templates")
}

func TestGuidedInterruptInRegistryListGoesBackToCloudPrompt(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"},
	}
	responses := []any{"acme templates (1)", terminal.InterruptErr, "AWS", "TypeScript"}
	i := 0
	sel := func(message string, options []string, opts display.Options) (string, error) {
		require.Less(t, i, len(responses), "unexpected extra prompt: %q", message)
		r := responses[i]
		i++
		if err, ok := r.(error); ok {
			return "", err
		}
		require.Contains(t, options, r.(string))
		return r.(string), nil
	}

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name(), "interrupt in the registry list must return to the cloud prompt")
}

func TestChooseRegistryTemplateLabelsIncludeDescription(t *testing.T) {
	t.Parallel()

	first := fakeRegistryTemplate{fakeTemplate{name: "vpc", desc: "A VPC by acme"}, "acme"}
	second := fakeRegistryTemplate{fakeTemplate{name: "vpc", desc: "A VPC by globex"}, "globex"}
	sel, offered := scriptedSelect(t, "vpc    A VPC by globex")

	got, err := chooseRegistryTemplate(
		[]cmdTemplates.Template{first, second}, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, second, got, "descriptions must disambiguate same-named templates")

	require.Len(t, *offered, 1)
	assert.Contains(t, (*offered)[0], "vpc    A VPC by acme")
}

func TestChooseRegistryTemplateDisambiguatesDuplicateNames(t *testing.T) {
	t.Parallel()

	first := fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}
	second := fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}
	var offered []string
	sel := func(message string, options []string, opts display.Options) (string, error) {
		offered = options
		return options[1], nil
	}

	got, err := chooseRegistryTemplate(
		[]cmdTemplates.Template{first, second}, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc", got.Name())

	require.Len(t, offered, 2)
	assert.NotEqual(t, offered[0], offered[1], "identical labels must be suffixed so both are selectable")
	assert.Contains(t, offered[1], "(2)")
}

func TestChooseRegistryTemplateErrorsOnUnknownAnswer(t *testing.T) {
	t.Parallel()

	sel := func(string, []string, display.Options) (string, error) {
		return "not-a-template", nil
	}

	got, err := chooseRegistryTemplate(
		[]cmdTemplates.Template{fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}},
		display.Options{},
		sel,
	)
	assert.Nil(t, got)
	assert.ErrorContains(t, err, "no such option")
}

func TestGuidedFallsBackWhenNothingIsCurated(t *testing.T) {
	t.Parallel()

	// A name the catalog can't decompose yields an empty catalog, so guided defers to the flat list.
	templates := []cmdTemplates.Template{fakeTemplate{name: "unparseable"}}
	sel := func(string, []string, display.Options) (string, error) {
		t.Error("no prompt may be shown when the catalog is empty")
		return "", nil
	}

	got, err := chooseGuided(templates, display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errFallBackToFlatList)
}
