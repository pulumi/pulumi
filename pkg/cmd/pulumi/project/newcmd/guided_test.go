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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type fakeTemplate struct {
	name string
	err  error
}

func (f fakeTemplate) Name() string        { return f.name }
func (f fakeTemplate) DisplayName() string { return f.name }
func (f fakeTemplate) Description() string { return "" }
func (f fakeTemplate) Error() error        { return f.err }
func (f fakeTemplate) FromRegistry() bool  { return false }
func (f fakeTemplate) Download(ctx context.Context) (workspace.Template, error) {
	return workspace.Template{}, nil
}

type fakeRegistryTemplate struct {
	fakeTemplate
	publisher string
}

func (f fakeRegistryTemplate) FromRegistry() bool { return true }

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

func TestGuidedSkipsSourceStepWithoutRegistryTemplates(t *testing.T) {
	t.Parallel()

	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}}
	sel, offered := scriptedSelect(t, "AWS", "TypeScript")

	_, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.Len(t, *offered, 2, "expected exactly provider + language prompts")
	assert.Contains(t, (*offered)[0], "AWS")
	assert.NotContains(t, (*offered)[0], sourcePulumiTemplates)
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
	assert.Equal(t, []string{"AWS", "Azure", "GCP", "None", optionOther}, (*offered)[0])
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

func TestGuidedRegistryTemplatesShowSourceStepAndSkipLanguage(t *testing.T) {
	t.Parallel()

	registry := fakeRegistryTemplate{fakeTemplate{name: "vpc-baseline"}, "acme"}
	templates := []cmdTemplates.Template{fakeTemplate{name: "aws-typescript"}, registry}
	sel, offered := scriptedSelect(t, sourceRegistryTemplates, "vpc-baseline")

	got, err := chooseGuided(templates, display.Options{}, sel)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "vpc-baseline", got.Name())

	require.Len(t, *offered, 2, "registry path should be source + template, no language prompt")
	assert.Contains(t, (*offered)[0], sourcePulumiTemplates)
	assert.Contains(t, (*offered)[0], sourceRegistryTemplates)
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
	require.NoError(t, err, "an empty catalog must not be a hard error")
	assert.Nil(t, got, "nil signals fallback to the flat chooser")
}
