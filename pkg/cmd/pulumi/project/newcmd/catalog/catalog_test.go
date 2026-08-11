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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTemplateNames mirrors a representative slice of the real pulumi/templates names.
var testTemplateNames = []string{
	"aws-typescript", "aws-python", "aws-bun", "aws-csharp", "aws-fsharp",
	"aws-go", "aws-java", "aws-scala", "aws-visualbasic", "aws-yaml",
	"azure-typescript", "azure-python", "azure-csharp", "azure-fsharp", "azure-go", "azure-java", "azure-yaml",
	"gcp-typescript", "gcp-python", "gcp-csharp", "gcp-fsharp", "gcp-go", "gcp-java", "gcp-visualbasic", "gcp-yaml",
	"typescript", "python", "go", "csharp", "fsharp", "java", "java-gradle", "javascript", "bun", "visualbasic", "yaml",
	"alicloud-typescript", "azuredevops-python", "linode-go", "rediscloud-python", "rediscloud-go",
	"aws-hcl", "hcl",
}

func newFromNames(names []string) *Catalog[string] {
	return New(names, func(name string) string { return name })
}

func testCatalog() *Catalog[string] { return newFromNames(testTemplateNames) }

func providerByID(t *testing.T, cat *Catalog[string], providerID string) Provider {
	t.Helper()
	for _, p := range cat.Providers() {
		if p.ID == providerID {
			return p
		}
	}
	require.FailNow(t, "no provider "+providerID)
	return Provider{}
}

func languageNames(t *testing.T, cat *Catalog[string], providerID string) []string {
	t.Helper()
	p := providerByID(t, cat, providerID)
	names := make([]string, len(p.Languages))
	for i, l := range p.Languages {
		names[i] = l.DisplayName
	}
	return names
}

func providerIDs(cat *Catalog[string]) []string {
	found := cat.Providers()
	ids := make([]string, len(found))
	for i, p := range found {
		ids[i] = p.ID
	}
	return ids
}

func TestProviderOrder(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"aws", "azure", "gcp", "none"}, providerIDs(testCatalog()),
		"None sits below the clouds so the curated providers stay adjacent")
}

func TestNoneIsItsOwnPseudoProvider(t *testing.T) {
	t.Parallel()

	none := providerByID(t, testCatalog(), "none")
	assert.Equal(t, "Basic Pulumi Program", none.DisplayName)

	assert.Equal(t, []string{"aws"}, providerIDs(newFromNames([]string{"aws-typescript"})),
		"None must be absent when there are no bare templates")
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
		"TypeScript", "Python", "Go", "C#", "YAML", "HCL", "Java (Maven)",
		"Java (Gradle)", "JavaScript", "Bun", "F#", "Visual Basic",
	}, displayNames)
}

func TestUnfeaturedProvidersStayOutOfCatalog(t *testing.T) {
	t.Parallel()

	cat := testCatalog()
	for _, id := range []string{"alicloud", "azuredevops", "linode", "rediscloud"} {
		assert.NotContains(t, cat.templates, id, "%s must fall through to Browse all templates", id)
	}
}

func TestLanguageOrderByUsage(t *testing.T) {
	t.Parallel()

	displayNames := languageNames(t, testCatalog(), "aws")
	assert.Equal(t, []string{
		"TypeScript", "Python", "Go", "C#", "YAML", "HCL", "Java", "Bun", "F#", "Scala", "Visual Basic",
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
		{"gcp", "hcl", false},
		{"none", "scala", false},
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

	name, ok = cat.Resolve("azure", "java")
	require.True(t, ok)
	assert.Equal(t, "azure-java", name)
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

	assert.Empty(t, newFromNames(nil).Providers())
	assert.Empty(t, newFromNames([]string{"vpc-baseline", "scripts"}).Providers(),
		"unparseable names yield no providers")
	assert.NotEmpty(t, testCatalog().Providers())
}

func TestUncuratedProviderStaysOutOfCatalog(t *testing.T) {
	t.Parallel()

	assert.Empty(t, newFromNames([]string{"newcloud-go"}).Providers(),
		"an uncurated provider must fall through to Browse all templates")
}

func TestCompoundTemplateNamesStayOutOfCatalog(t *testing.T) {
	t.Parallel()

	cat := newFromNames([]string{
		"aws-typescript",
		"container-aws-typescript", "kubernetes-aws-go", "esc-connector-lambda-python", "vm-gcp-csharp",
	})

	for _, id := range []string{"container-aws", "kubernetes-aws", "esc-connector-lambda", "vm-gcp"} {
		assert.NotContains(t, cat.templates, id, "%s must not be minted as a provider", id)
	}
	assert.Contains(t, cat.templates, "aws")
}
