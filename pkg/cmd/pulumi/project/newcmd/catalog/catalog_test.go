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
}

func testCatalog() *Catalog { return New(testTemplateNames) }

func TestFeaturedOrder(t *testing.T) {
	t.Parallel()

	featured := testCatalog().Featured()
	require.Len(t, featured, 4)
	assert.Equal(t, "aws", featured[0].ID)
	assert.Equal(t, "azure", featured[1].ID)
	assert.Equal(t, "gcp", featured[2].ID)
	assert.Equal(t, "none", featured[3].ID)
	for _, p := range featured {
		assert.True(t, p.Featured, "%s should be featured", p.ID)
	}
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
	displayNames := func(id string) []string {
		p, ok := cat.Get(id)
		require.True(t, ok)
		names := make([]string, len(p.Languages))
		for i, l := range p.Languages {
			names[i] = l.DisplayName
		}
		return names
	}

	none := displayNames("none")
	assert.Contains(t, none, "Java (Maven)")
	assert.Contains(t, none, "Java (Gradle)")
	assert.NotContains(t, none, "Java")

	aws := displayNames("aws")
	assert.Contains(t, aws, "Java")
	assert.NotContains(t, aws, "Java (Maven)")
}

func TestNoneLanguageOrder(t *testing.T) {
	t.Parallel()

	none, ok := testCatalog().Get("none")
	require.True(t, ok)

	displayNames := make([]string, len(none.Languages))
	for i, l := range none.Languages {
		displayNames[i] = l.DisplayName
	}
	assert.Equal(t, []string{
		"TypeScript", "Python", "Go", "C#", "YAML", "Java (Maven)",
		"Java (Gradle)", "JavaScript", "Bun", "F#", "Visual Basic",
	}, displayNames)
}

func TestOthersAreAlphabeticalAndNotFeatured(t *testing.T) {
	t.Parallel()

	others := testCatalog().Others()
	require.NotEmpty(t, others)

	names := make([]string, len(others))
	for i, p := range others {
		assert.False(t, p.Featured, "%s should not be featured", p.ID)
		names[i] = p.DisplayName
	}
	assert.True(t, sort.StringsAreSorted(names), "Others() not sorted by DisplayName: %v", names)
}

func TestLanguageOrderByUsage(t *testing.T) {
	t.Parallel()

	aws, ok := testCatalog().Get("aws")
	require.True(t, ok)

	displayNames := make([]string, len(aws.Languages))
	for i, l := range aws.Languages {
		displayNames[i] = l.DisplayName
	}
	assert.Equal(t, []string{
		"TypeScript", "Python", "Go", "C#", "YAML", "Java", "Bun", "F#", "Scala", "Visual Basic",
	}, displayNames)

	ranks := make([]int, len(aws.Languages))
	for i, l := range aws.Languages {
		ranks[i] = languageOrder(l.ID)
	}
	assert.True(t, sort.IntsAreSorted(ranks), "languages not in non-decreasing usage rank: %v", ranks)
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

func TestUnknownProviderFallsBackToRawDisplayName(t *testing.T) {
	t.Parallel()

	cat := New([]string{"newcloud-go"})
	p, ok := cat.Get("newcloud")
	require.True(t, ok, "a provider not in the curated display map must still be reachable")
	assert.Equal(t, "newcloud", p.DisplayName)
}
