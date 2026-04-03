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

package docsrender

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	body := "Some **bold** text."
	result, err := RenderMarkdown("Hello", body)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Hello")
}

func TestRenderMarkdownNoTitle(t *testing.T) {
	t.Parallel()

	body := "# Already has title\n\nSome text."
	result, err := RenderMarkdown("", body)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestFilterCodeBlocksByLanguageMultiLang(t *testing.T) {
	t.Parallel()

	source := []byte(`Some text.

` + "```typescript" + `
console.log('hello');
` + "```" + `

` + "```python" + `
print('hello')
` + "```" + `

` + "```go" + `
fmt.Println("hello")
` + "```" + `

More text.
`)
	tree := ParseMarkdown(source)
	result := string(FilterCodeBlocksByLanguage(source, tree, "python"))

	assert.Contains(t, result, "print('hello')")
	assert.NotContains(t, result, "console.log")
	assert.NotContains(t, result, "fmt.Println")
	assert.Contains(t, result, "Some text.")
	assert.Contains(t, result, "More text.")
}

func TestFilterCodeBlocksPreservesShell(t *testing.T) {
	t.Parallel()

	source := []byte(
		"```python\nprint('hi')\n```\n\n" +
			"```sh\ncurl example.com\n```\n\n" +
			"```typescript\nconsole.log('hi');\n```\n")
	tree := ParseMarkdown(source)
	result := string(FilterCodeBlocksByLanguage(source, tree, "python"))

	assert.Contains(t, result, "print('hi')")
	assert.Contains(t, result, "curl example.com")
	assert.NotContains(t, result, "console.log")
}

func TestFilterCodeBlocksPreservesIsolated(t *testing.T) {
	t.Parallel()

	source := []byte(`Some text.

` + "```yaml" + `
key: value
` + "```" + `

More text.
`)
	tree := ParseMarkdown(source)
	result := string(FilterCodeBlocksByLanguage(source, tree, "python"))

	assert.Contains(t, result, "key: value")
}

func TestExtractLinks(t *testing.T) {
	t.Parallel()

	source := []byte(`Check [Stacks](/docs/iac/concepts/stacks) and
[AWS Provider](/registry/packages/aws) for details.
Also see [Stacks](/docs/iac/concepts/stacks) again.
And [Google](https://google.com).
`)
	tree := ParseMarkdown(source)
	links := ExtractLinks(source, tree)

	assert.Equal(t, []Link{
		{URL: "/docs/iac/concepts/stacks", Title: "Stacks"},
		{URL: "/registry/packages/aws", Title: "AWS Provider"},
		{URL: "https://google.com", Title: "Google"},
	}, links)
}

func TestExtractInternalLinks(t *testing.T) {
	t.Parallel()

	md := `Check [Stacks](/docs/iac/concepts/stacks) and [Google](https://google.com).`
	links := ExtractInternalLinks(md)

	require.Len(t, links, 1)
	assert.Equal(t, "/docs/iac/concepts/stacks", links[0].URL)
}

func TestNumberLinks(t *testing.T) {
	t.Parallel()

	md := `See [Stacks](/docs/stacks) and [Projects](/docs/projects).`
	annotated, links := NumberLinks(md)

	require.Len(t, links, 2)
	assert.Contains(t, annotated, "🔗1")
	assert.Contains(t, annotated, "🔗2")
}

func TestWebURL(t *testing.T) {
	t.Parallel()

	t.Run("docs path", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "https://www.pulumi.com/docs/iac/concepts/stacks/",
			WebURL("https://www.pulumi.com", "iac/concepts/stacks"))
	})

	t.Run("registry path", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "https://www.pulumi.com/registry/packages/aws/",
			WebURL("https://www.pulumi.com", "registry/packages/aws"))
	})
}
