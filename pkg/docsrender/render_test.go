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

func TestRenderMarkdownRaw(t *testing.T) {
	t.Parallel()

	content := "# Hello\n\nSome **bold** text."
	result, err := RenderMarkdown(content, true)
	require.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestRenderMarkdownGlamour(t *testing.T) {
	t.Parallel()

	content := "# Hello\n\nSome **bold** text."
	result, err := RenderMarkdown(content, false)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Hello")
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

func TestFilterCodeBlocksNoLanguage(t *testing.T) {
	t.Parallel()

	source := []byte("```\nplain code\n```\n")
	tree := ParseMarkdown(source)
	result := string(FilterCodeBlocksByLanguage(source, tree, "python"))

	assert.Contains(t, result, "plain code")
}

func TestFilterCodeBlocksEmptyInput(t *testing.T) {
	t.Parallel()

	source := []byte("")
	tree := ParseMarkdown(source)
	result := FilterCodeBlocksByLanguage(source, tree, "python")

	assert.Empty(t, result)
}

func TestFilterCodeBlocksNoLang(t *testing.T) {
	t.Parallel()

	source := []byte("# Title\n\nSome text.\n")
	tree := ParseMarkdown(source)
	result := string(FilterCodeBlocksByLanguage(source, tree, ""))

	assert.Contains(t, result, "# Title")
	assert.Contains(t, result, "Some text.")
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

func TestExtractLinksNoLinks(t *testing.T) {
	t.Parallel()

	source := []byte("Just plain text with no links.\n")
	tree := ParseMarkdown(source)
	links := ExtractLinks(source, tree)

	assert.Empty(t, links)
}
