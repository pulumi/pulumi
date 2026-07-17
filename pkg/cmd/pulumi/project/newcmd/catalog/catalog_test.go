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

func TestFeaturedOrder(t *testing.T) {
	t.Parallel()

	featured := Featured()
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

	for _, p := range Others() {
		assert.NotEqual(t, "none", p.ID, "None must not appear in the Other expansion")
	}
}

func TestResolveNoneUsesBareTemplateNames(t *testing.T) {
	t.Parallel()

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
			name, ok := Resolve("none", tt.language)
			require.True(t, ok)
			assert.Equal(t, tt.want, name)
		})
	}
}

func TestJavaDisplayNameIsSplitOnlyUnderNone(t *testing.T) {
	t.Parallel()

	displayNames := func(id string) []string {
		p, ok := Get(id)
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

	none, ok := Get("none")
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

	others := Others()
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

	aws, ok := Get("aws")
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
			_, ok := Resolve(tt.provider, tt.language)
			assert.Equal(t, tt.want, ok)
		})
	}
}

func TestResolveBuildsTemplateName(t *testing.T) {
	t.Parallel()

	name, ok := Resolve("aws", "typescript")
	require.True(t, ok)
	assert.Equal(t, "aws-typescript", name)

	name, ok = Resolve("rediscloud", "python")
	require.True(t, ok)
	assert.Equal(t, "rediscloud-python", name)
}

func TestResolveUnknownProvider(t *testing.T) {
	t.Parallel()

	_, ok := Resolve("nope", "typescript")
	assert.False(t, ok)
}

func TestNoNonTemplateDirectoriesLeakIn(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"tests", "scripts", "generator", "metadata"} {
		_, ok := Get(id)
		assert.False(t, ok, "%q is not a template provider", id)
	}
}
