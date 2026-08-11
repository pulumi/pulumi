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
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
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
func (f fakeTemplate) Publisher() string   { return "" }
func (f fakeTemplate) Download(ctx context.Context) (cmdTemplates.ProjectTemplate, error) {
	return cmdTemplates.ProjectTemplate{}, nil
}

type fakeRegistryTemplate struct {
	fakeTemplate
	publisher string
}

func (f fakeRegistryTemplate) Publisher() string { return f.publisher }

// scriptedSelect answers each prompt in order, asserting the option offered is present. An error
// entry is returned from the prompt as-is.
func scriptedSelect(t *testing.T, answers ...any) (selectFunc, *[]([]string)) {
	t.Helper()
	var offered [][]string
	i := 0
	return func(message string, options []string, opts display.Options) (int, error) {
		offered = append(offered, options)
		require.Less(t, i, len(answers), "unexpected extra prompt: %q with %v", message, options)
		answer := answers[i]
		i++
		if err, ok := answer.(error); ok {
			return 0, err
		}
		idx := slices.Index(options, answer.(string))
		require.GreaterOrEqual(t, idx, 0, "scripted answer %q not offered in %v", answer, options)
		return idx, nil
	}, &offered
}

// noFetch fails the test if the full template fetch is triggered; provider/language selections
// must resolve from the project templates alone, never waiting on the templates API.
func noFetch(t *testing.T) fetchTemplatesFunc {
	return func() ([]cmdTemplates.Template, error) {
		t.Error("the full template fetch must not run for this path")
		return nil, nil
	}
}

func fetchOf(templates ...cmdTemplates.Template) fetchTemplatesFunc {
	return func() ([]cmdTemplates.Template, error) { return templates, nil }
}

// projectOnly assembles the inputs for a flow whose rows all come from the project templates.
func projectOnly(project []cmdTemplates.Template, fetchAll fetchTemplatesFunc) guidedTemplates {
	return guidedTemplates{project: project, fetchAll: fetchAll}
}

// runGuided drives one run of the guided prompts to completion.
func runGuided(
	t guidedTemplates, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	return newGuided(t, opts, sel).choose()
}

func TestGuidedResolvesFeaturedProvider(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "aws-python"},
		fakeTemplate{name: "gcp-go"},
	}
	sel, _ := scriptedSelect(t, "AWS", "TypeScript")

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name())
}

func TestGuidedShowsNoRegistryRowsWithoutRegistryTemplates(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, offered := scriptedSelect(t, "AWS", "TypeScript")

	_, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	require.NoError(t, err)
	require.Len(t, *offered, 2, "expected exactly cloud + language prompts")
	assert.Equal(t, []string{"AWS", optionBrowseAll}, (*offered)[0])
}

func TestGuidedUnfeaturedProviderIsOnlyReachableThroughBrowseAll(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-go"},
		fakeTemplate{name: "linode-go"},
	}
	sel, offered := scriptedSelect(t, optionBrowseAll, "linode-go    ")

	got, err := runGuided(projectOnly(templates, fetchOf(templates...)), display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "linode-go", got.Name())

	require.Len(t, *offered, 2)
	assert.Equal(t, []string{"AWS", optionBrowseAll}, (*offered)[0], "an unfeatured provider earns no cloud row")
}

func TestGuidedNoneResolvesToBareTemplate(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "typescript"},
	}
	sel, offered := scriptedSelect(t, "Basic Pulumi Program", "TypeScript")

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
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
	sel, offered := scriptedSelect(t, "Basic Pulumi Program", "TypeScript")

	_, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, []string{"AWS", "Azure", "Google Cloud", "Basic Pulumi Program", optionBrowseAll}, (*offered)[0],
		"None sits below the clouds so the curated providers stay adjacent")
}

func TestGuidedBrowseAllListsEveryTemplateInline(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "broken", err: errors.New("boom")},
	}
	sel, offered := scriptedSelect(t, optionBrowseAll, "aws-typescript    ")

	got, err := runGuided(projectOnly(templates, fetchOf(templates...)), display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name())

	require.Len(t, *offered, 2)
	require.Len(t, (*offered)[1], 2, "browse all must list every template, including broken ones")
	assert.Contains(t, (*offered)[1][1], BrokenTemplateDescription, "broken templates sort last and are marked")
}

func TestGuidedInterruptInBrowseAllAborts(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, _ := scriptedSelect(t, optionBrowseAll, terminal.InterruptErr)

	got, err := runGuided(projectOnly(templates, fetchOf(templates...)), display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, terminal.InterruptErr, "Ctrl-C must abort the flow, not navigate")
}

func TestGuidedNoneJavaIsSplitByBuildSystem(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "java"},
		fakeTemplate{name: "java-gradle"},
	}
	sel, offered := scriptedSelect(t, "Basic Pulumi Program", "Java (Gradle)")

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
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

	_, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
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

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name())
	assert.NotContains(t, (*offered)[1], "Go", "broken templates must not be offered")
}

func TestGuidedFallsBackWhenEverythingIsBroken(t *testing.T) {
	t.Parallel()

	sel := func(string, []string, display.Options) (int, error) {
		t.Error("no prompt may be shown when every template is broken")
		return 0, nil
	}

	got, err := runGuided(guidedTemplates{
		project:  []cmdTemplates.Template{fakeTemplate{name: "aws-typescript", err: errors.New("boom")}},
		database: []cmdTemplates.Template{fakeRegistryTemplate{fakeTemplate{name: "vpc", err: errors.New("boom")}, "acme"}},
		fetchAll: noFetch(t),
	}, display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errFallBackToFlatList, "the flat chooser is the only surface that marks broken templates")
}

func TestGuidedInterruptAtLanguageStepAborts(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{
		fakeTemplate{name: "aws-typescript"},
		fakeTemplate{name: "gcp-go"},
	}
	sel, _ := scriptedSelect(t, "AWS", terminal.InterruptErr)

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, terminal.InterruptErr, "Ctrl-C must abort the flow, not navigate")
}

func TestGuidedInterruptAtFirstStepPropagates(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel := func(string, []string, display.Options) (int, error) {
		return 0, terminal.InterruptErr
	}

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, terminal.InterruptErr)
}

func TestGuidedOrgRowOpensOrgTemplatesAndSkipsLanguage(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	registry := fakeRegistryTemplate{fakeTemplate{name: "vpc-baseline", desc: "A baseline VPC"}, "acme"}
	sel, offered := scriptedSelect(t, "acme templates", "vpc-baseline    A baseline VPC")

	got, err := runGuided(guidedTemplates{
		project:  local,
		vcsOrgs:  []string{"acme"},
		fetchAll: fetchOf(local[0], registry),
	}, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc-baseline", got.Name())

	require.Len(t, *offered, 2, "org path should be cloud prompt + template list, no language prompt")
	assert.Equal(t, []string{"AWS", "acme templates", optionBrowseAll}, (*offered)[0],
		"org rows sit between the providers and Browse all")
}

func TestGuidedRegistryOnlyOrgRowNeedsNoUpstreamFetch(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	registry := []cmdTemplates.Template{
		fakeRegistryTemplate{fakeTemplate{name: "vpc-baseline", desc: "A baseline VPC"}, "acme"},
	}
	sel, offered := scriptedSelect(t, "acme templates", "vpc-baseline    A baseline VPC")

	got, err := runGuided(guidedTemplates{
		project:  local,
		database: registry,
		fetchAll: noFetch(t),
	}, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc-baseline", got.Name())

	assert.Equal(t, []string{"AWS", "acme templates", optionBrowseAll}, (*offered)[0],
		"an org that published to the registry earns a row without the service reporting a collection")
}

func TestGuidedOrgRowsMergeRegistryPublishersAndVcsSources(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	registry := []cmdTemplates.Template{
		fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "globex"},
		fakeRegistryTemplate{fakeTemplate{name: "eks"}, "acme"},
		fakeRegistryTemplate{fakeTemplate{name: "broken", err: errors.New("boom")}, "initech"},
		fakeRegistryTemplate{fakeTemplate{name: "legacy"}, ""},
	}
	sel, offered := scriptedSelect(t, terminal.InterruptErr)

	_, err := runGuided(guidedTemplates{
		project:  local,
		database: registry,
		vcsOrgs:  []string{"acme", "umbrella"},
		fetchAll: noFetch(t),
	}, display.Options{}, sel)
	assert.ErrorIs(t, err, terminal.InterruptErr)

	assert.Equal(t, []string{
		"AWS", "acme templates", "globex templates", "umbrella templates", optionBrowseAll,
	}, (*offered)[0], "each org appears once, sorted; a publisher-less or broken template earns no row")
}

func TestGuidedOrgRowListsOnlyThatOrg(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	all := []cmdTemplates.Template{
		local[0],
		fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "globex"},
		fakeRegistryTemplate{fakeTemplate{name: "eks"}, "acme"},
		fakeRegistryTemplate{fakeTemplate{name: "gke"}, "acme"},
		fakeRegistryTemplate{fakeTemplate{name: "legacy"}, ""},
	}
	sel, offered := scriptedSelect(t, "acme templates", "eks    ")

	got, err := runGuided(guidedTemplates{
		project:  local,
		vcsOrgs:  []string{"acme", "globex"},
		fetchAll: fetchOf(all...),
	}, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "eks", got.Name())

	assert.Equal(t, []string{
		"AWS", "acme templates", "globex templates", optionBrowseAll,
	}, (*offered)[0], "one row per org, sorted")
	assert.Equal(t, []string{"eks    ", "gke    "}, (*offered)[1], "org row lists only that org's templates")
}

func TestGuidedOrgRowsOnlyWithEmptyCatalog(t *testing.T) {
	t.Parallel()

	registry := fakeRegistryTemplate{fakeTemplate{name: "vpc", desc: "A VPC"}, "acme"}
	sel, offered := scriptedSelect(t, "acme templates", "vpc    A VPC")

	got, err := runGuided(guidedTemplates{
		vcsOrgs:  []string{"acme"},
		fetchAll: fetchOf(registry),
	}, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc", got.Name())

	assert.Equal(t, []string{"acme templates", optionBrowseAll}, (*offered)[0],
		"an empty catalog still goes through the cloud prompt so Browse all stays reachable")
}

func TestGuidedEmptyOrgRowNotesAndReturnsToCloudPrompt(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, _ := scriptedSelect(t, "acme templates", "AWS", "TypeScript")

	var notice bytes.Buffer
	got, err := runGuided(guidedTemplates{
		project:  local,
		vcsOrgs:  []string{"acme"},
		fetchAll: fetchOf(local...),
	}, display.Options{Stdout: &notice}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name(),
		"an org row with no templates must return to the cloud prompt")
	assert.Contains(t, notice.String(), `No templates found for organization "acme"`)
}

func TestGuidedUnavailableTemplatesReturnToCloudPrompt(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	failed := func() ([]cmdTemplates.Template, error) { return nil, errors.New("service unreachable") }

	for _, tt := range []struct {
		name   string
		row    string
		notice string
	}{
		{"org row", "acme templates", `Could not load templates for organization "acme": service unreachable`},
		{"browse all", optionBrowseAll, "Could not load the full template list: service unreachable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sel, _ := scriptedSelect(t, tt.row, "AWS", "TypeScript")
			var notice bytes.Buffer
			got, err := runGuided(guidedTemplates{
				project:  local,
				vcsOrgs:  []string{"acme"},
				fetchAll: failed,
			}, display.Options{Stdout: &notice}, sel)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, "aws-typescript", got.Name(),
				"a row that cannot load templates must return to the cloud prompt, not abandon the flow")
			assert.Contains(t, notice.String(), tt.notice)
		})
	}
}

func TestGuidedEmptyBrowseAllNotesAndReturnsToCloudPrompt(t *testing.T) {
	t.Parallel()

	// An org whose configured VCS collection turns out to hold nothing, and no local templates:
	// Browse all has an empty list to offer, which survey reports as an unhelpful internal error.
	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, _ := scriptedSelect(t, optionBrowseAll, "AWS", "TypeScript")

	var notice bytes.Buffer
	got, err := runGuided(guidedTemplates{
		project:  local,
		vcsOrgs:  []string{"acme"},
		fetchAll: fetchOf(),
	}, display.Options{Stdout: &notice}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "aws-typescript", got.Name())
	assert.Contains(t, notice.String(), "No templates found.")
}

func TestGuidedLazyFetchRunsOncePerFlow(t *testing.T) {
	t.Parallel()

	local := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	registry := fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}
	fetches := 0
	fetch := func() ([]cmdTemplates.Template, error) {
		// The org has nothing on the first fetch, sending the flow back to the cloud prompt.
		fetches++
		if fetches == 1 {
			return local, nil
		}
		return []cmdTemplates.Template{local[0], registry}, nil
	}
	sel, _ := scriptedSelect(t, "acme templates", "acme templates", "vpc    ")

	got, err := runGuided(guidedTemplates{
		project:  local,
		vcsOrgs:  []string{"acme"},
		fetchAll: fetch,
	}, display.Options{Stdout: io.Discard}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc", got.Name())
	assert.Equal(t, 2, fetches, "choose calls the fetcher per selection; memoization is the caller's")
}

func TestChooseTemplateFromListLabelsIncludeDescription(t *testing.T) {
	t.Parallel()

	first := fakeRegistryTemplate{fakeTemplate{name: "vpc", desc: "A VPC by acme"}, "acme"}
	second := fakeRegistryTemplate{fakeTemplate{name: "vpc", desc: "A VPC by globex"}, "globex"}
	sel, offered := scriptedSelect(t, "vpc    A VPC by globex")

	got, err := chooseTemplateFromList(
		[]cmdTemplates.Template{first, second}, display.Options{}, sel)
	require.NoError(t, err)
	assert.Equal(t, second, got, "descriptions must disambiguate same-named templates")

	require.Len(t, *offered, 1)
	assert.Contains(t, (*offered)[0], "vpc    A VPC by acme")
}

func TestChooseTemplateFromListDisambiguatesDuplicateNames(t *testing.T) {
	t.Parallel()

	first := fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}
	second := fakeRegistryTemplate{fakeTemplate{name: "vpc"}, "acme"}
	var offered []string
	sel := func(message string, options []string, opts display.Options) (int, error) {
		offered = options
		return 1, nil
	}

	got, err := chooseTemplateFromList(
		[]cmdTemplates.Template{first, second}, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc", got.Name())

	require.Len(t, offered, 2)
	assert.NotEqual(t, offered[0], offered[1], "identical labels must be suffixed so the rows stay distinct")
	assert.Contains(t, offered[1], "(2)")
}

func TestGuidedFallsBackWhenNothingIsCurated(t *testing.T) {
	t.Parallel()

	// A name the catalog can't decompose yields an empty catalog, so guided defers to the flat list.
	templates := []cmdTemplates.Template{fakeTemplate{name: "unparseable"}}
	sel := func(string, []string, display.Options) (int, error) {
		t.Error("no prompt may be shown when the catalog is empty")
		return 0, nil
	}

	got, err := runGuided(projectOnly(templates, noFetch(t)), display.Options{}, sel)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, errFallBackToFlatList)
}
