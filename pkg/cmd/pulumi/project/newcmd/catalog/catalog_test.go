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

package catalog

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTemplateNames mirrors a representative slice of the real pulumi/templates names so the derived
// catalog exercises featured providers, the None build-system split, and a few "other" providers.
var testTemplateNames = []string{
	"aws-typescript", "aws-python", "aws-bun", "aws-csharp", "aws-fsharp",
	"aws-go", "aws-java", "aws-scala", "aws-visualbasic", "aws-yaml",
	"azure-typescript", "azure-python", "azure-csharp", "azure-fsharp", "azure-go", "azure-java", "azure-yaml",
	"gcp-typescript", "gcp-python", "gcp-csharp", "gcp-fsharp", "gcp-go", "gcp-java", "gcp-visualbasic", "gcp-yaml",
	"typescript", "python", "go", "csharp", "fsharp", "java", "java-gradle", "javascript", "bun", "visualbasic", "yaml",
	"alicloud-typescript", "azuredevops-python", "linode-go", "rediscloud-python", "rediscloud-go",
	"aws-hcl", "hcl",
}

func testCatalog() *Catalog { return New(testTemplateNames) }

func languageNames(t *testing.T, cat *Catalog, providerID string) []string {
	t.Helper()
	require.Contains(t, cat.templateNames, providerID)
	p := cat.provider(providerID)
	names := make([]string, len(p.Languages))
	for i, l := range p.Languages {
		names[i] = l.DisplayName
	}
	return names
}

func TestFeaturedOrder(t *testing.T) {
	t.Parallel()

	featured := testCatalog().Featured()
	require.Len(t, featured, 3)
	assert.Equal(t, "aws", featured[0].ID)
	assert.Equal(t, "azure", featured[1].ID)
	assert.Equal(t, "gcp", featured[2].ID)
}

func TestNoneIsItsOwnPseudoProvider(t *testing.T) {
	t.Parallel()

	none, ok := testCatalog().None()
	require.True(t, ok)
	assert.Equal(t, "none", none.ID)
	assert.Equal(t, "None", none.DisplayName)

	_, ok = New([]string{"aws-typescript"}).None()
	assert.False(t, ok, "None must be absent when there are no bare templates")
}

func TestNoneIsNotInOthers(t *testing.T) {
	t.Parallel()

	for _, p := range testCatalog().Others() {
		assert.NotEqual(t, "none", p.ID, "None must not appear in the Other expansion")
	}
}

func TestResolveNoneUsesBareTemplateNames(t *testing.T) {
	t.Parallel()

	cat := testCatalog()
	tests := []struct{ language, want string }{
		{"typescript", "typescript"},
		{"python", "python"},
		{"javascript", "javascript"},
		{"java", "java"},
		{"java-gradle", "java-gradle"},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			t.Parallel()
			name, ok := cat.Resolve("none", tt.language)
			require.True(t, ok)
			assert.Equal(t, tt.want, name)
		})
	}
}

func TestJavaDisplayNameIsSplitOnlyUnderNone(t *testing.T) {
	t.Parallel()

	cat := testCatalog()

	none := languageNames(t, cat, "none")
	assert.Contains(t, none, "Java (Maven)")
	assert.Contains(t, none, "Java (Gradle)")
	assert.NotContains(t, none, "Java")

	aws := languageNames(t, cat, "aws")
	assert.Contains(t, aws, "Java")
	assert.NotContains(t, aws, "Java (Maven)")
}

func TestNoneLanguageOrder(t *testing.T) {
	t.Parallel()

	displayNames := languageNames(t, testCatalog(), "none")
	assert.Equal(t, []string{
		"TypeScript", "Python", "Go", "C#", "YAML", "Java (Maven)",
		"Java (Gradle)", "JavaScript", "Bun", "F#", "Visual Basic", "HCL",
	}, displayNames)
}

func TestOthersAreAlphabeticalAndNotFeatured(t *testing.T) {
	t.Parallel()

	others := testCatalog().Others()
	require.NotEmpty(t, others)

	featuredIDs := make([]string, len(featuredProviders))
	for i, p := range featuredProviders {
		featuredIDs[i] = p.id
	}
	names := make([]string, len(others))
	for i, p := range others {
		assert.NotContains(t, featuredIDs, p.ID, "%s should not be featured", p.ID)
		names[i] = p.DisplayName
	}
	assert.True(t, sort.StringsAreSorted(names),
		"Others() not sorted by DisplayName; keep the otherProviders declaration alphabetical: %v", names)
}

func TestLanguageOrderByUsage(t *testing.T) {
	t.Parallel()

	displayNames := languageNames(t, testCatalog(), "aws")
	assert.Equal(t, []string{
		"TypeScript", "Python", "Go", "C#", "YAML", "Java", "Bun", "F#", "Scala", "Visual Basic", "HCL",
	}, displayNames)
}

func TestLanguagesAreFilteredPerProvider(t *testing.T) {
	t.Parallel()

	cat := testCatalog()
	tests := []struct {
		provider string
		language string
		want     bool
	}{
		{"aws", "scala", true},
		{"azure", "scala", false},
		{"azure", "bun", false},
		{"azure", "visualbasic", false},
		{"gcp", "visualbasic", true},
		{"azuredevops", "python", true},
		{"azuredevops", "typescript", false},
		{"rediscloud", "go", true},
		{"rediscloud", "yaml", false},
	}
	for _, tt := range tests {
		t.Run(tt.provider+"-"+tt.language, func(t *testing.T) {
			t.Parallel()
			_, ok := cat.Resolve(tt.provider, tt.language)
			assert.Equal(t, tt.want, ok)
		})
	}
}

func TestResolveBuildsTemplateName(t *testing.T) {
	t.Parallel()

	cat := testCatalog()
	name, ok := cat.Resolve("aws", "typescript")
	require.True(t, ok)
	assert.Equal(t, "aws-typescript", name)

	name, ok = cat.Resolve("rediscloud", "python")
	require.True(t, ok)
	assert.Equal(t, "rediscloud-python", name)
}

func TestResolveUnknownProvider(t *testing.T) {
	t.Parallel()

	_, ok := testCatalog().Resolve("nope", "typescript")
	assert.False(t, ok)
}

func TestSplitTemplateName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		provider, lang string
		ok             bool
	}{
		{"aws-typescript", "aws", "typescript", true},
		{"aws-hcl", "aws", "hcl", true},
		{"hcl", "none", "hcl", true},
		{"rediscloud-go", "rediscloud", "go", true},
		{"typescript", "none", "typescript", true},
		{"java", "none", "java", true},
		{"java-gradle", "none", "java-gradle", true},
		{"vpc-baseline", "", "", false},
		{"scripts", "", "", false},
		{"", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider, lang, ok := splitTemplateName(tt.name)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.provider, provider)
			assert.Equal(t, tt.lang, lang)
		})
	}
}

func TestEmptyCatalog(t *testing.T) {
	t.Parallel()

	assert.True(t, New(nil).Empty())
	assert.True(t, New([]string{"vpc-baseline", "scripts"}).Empty(), "unparseable names yield no providers")
	assert.False(t, testCatalog().Empty())
}

func TestUncuratedProviderStaysOutOfCatalog(t *testing.T) {
	t.Parallel()

	cat := New([]string{"newcloud-go"})
	assert.True(t, cat.Empty(), "an uncurated provider must fall through to Browse all templates")
}

func TestCompoundTemplateNamesStayOutOfCatalog(t *testing.T) {
	t.Parallel()

	cat := New([]string{
		"aws-typescript", "kubernetes-go",
		"container-aws-typescript", "kubernetes-aws-go", "esc-connector-lambda-python", "vm-gcp-csharp",
	})

	for _, id := range []string{"container-aws", "kubernetes-aws", "esc-connector-lambda", "vm-gcp"} {
		assert.NotContains(t, cat.templateNames, id, "%s must not be minted as a provider", id)
	}
	assert.Contains(t, cat.templateNames, "aws")
	assert.Contains(t, cat.templateNames, "kubernetes", "plain kubernetes is curated and must stay")
}
