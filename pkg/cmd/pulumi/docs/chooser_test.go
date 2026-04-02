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

package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChoosers(t *testing.T) {
	t.Parallel()

	t.Run("single chooser with two options", func(t *testing.T) {
		t.Parallel()
		md := "Before.\n\n<!-- chooser: language -->\n" +
			"<!-- option: typescript -->\nconsole.log('hi');\n<!-- /option -->\n" +
			"<!-- option: python -->\nprint('hi')\n<!-- /option -->\n" +
			"<!-- /chooser -->\n\nAfter."
		blocks := ParseChoosers(md)
		require.Len(t, blocks, 3)

		// First block: plain text
		pt, ok := blocks[0].(PlainText)
		require.True(t, ok)
		assert.Contains(t, pt.Text, "Before.")

		// Second block: chooser
		ch, ok := blocks[1].(Chooser)
		require.True(t, ok)
		assert.Equal(t, "language", ch.Type)
		require.Len(t, ch.Options, 2)
		assert.Equal(t, "typescript", ch.Options[0].Value)
		assert.Contains(t, ch.Options[0].Content, "console.log")
		assert.Equal(t, "python", ch.Options[1].Value)
		assert.Contains(t, ch.Options[1].Content, "print")

		// Third block: plain text
		pt, ok = blocks[2].(PlainText)
		require.True(t, ok)
		assert.Contains(t, pt.Text, "After.")
	})

	t.Run("plain text only", func(t *testing.T) {
		t.Parallel()
		md := "Just plain text with no choosers."
		blocks := ParseChoosers(md)
		require.Len(t, blocks, 1)
		pt, ok := blocks[0].(PlainText)
		require.True(t, ok)
		assert.Equal(t, md, pt.Text)
	})

	t.Run("unmatched chooser open tag", func(t *testing.T) {
		t.Parallel()
		md := "Before.\n<!-- chooser: language -->\nNo closing tag."
		blocks := ParseChoosers(md)
		// Should treat the open tag as plain text
		assert.NotEmpty(t, blocks)
	})

	t.Run("multiple choosers", func(t *testing.T) {
		t.Parallel()
		md := "<!-- chooser: language -->\n<!-- option: go -->\n" +
			"Go code\n<!-- /option -->\n<!-- /chooser -->\n\nMiddle text.\n\n" +
			"<!-- chooser: os -->\n<!-- option: linux -->\n" +
			"Linux\n<!-- /option -->\n<!-- /chooser -->"
		blocks := ParseChoosers(md)
		chooserCount := 0
		for _, b := range blocks {
			if _, ok := b.(Chooser); ok {
				chooserCount++
			}
		}
		assert.Equal(t, 2, chooserCount)
	})
}

func TestResolveChoosers(t *testing.T) {
	t.Parallel()

	langChooser := Chooser{
		Type: "language",
		Options: []Option{
			{Value: "typescript", Content: "TS code"},
			{Value: "python", Content: "PY code"},
		},
	}

	t.Run("flag takes priority", func(t *testing.T) {
		t.Parallel()
		blocks := []ContentBlock{langChooser}
		prefs := &Preferences{Language: "python"}
		result := ResolveChoosers(blocks, prefs, "typescript", "", false)
		assert.Contains(t, result, "TS code")
		assert.NotContains(t, result, "PY code")
	})

	t.Run("preference fallback", func(t *testing.T) {
		t.Parallel()
		blocks := []ContentBlock{langChooser}
		prefs := &Preferences{Language: "python"}
		result := ResolveChoosers(blocks, prefs, "", "", false)
		assert.Contains(t, result, "PY code")
		assert.NotContains(t, result, "TS code")
	})

	t.Run("no selection shows all", func(t *testing.T) {
		t.Parallel()
		blocks := []ContentBlock{langChooser}
		prefs := &Preferences{}
		result := ResolveChoosers(blocks, prefs, "", "", false)
		assert.Contains(t, result, "TS code")
		assert.Contains(t, result, "PY code")
	})

	t.Run("session selection reused", func(t *testing.T) {
		t.Parallel()
		blocks := []ContentBlock{
			langChooser,
			PlainText{Text: "\n"},
			langChooser,
		}
		prefs := &Preferences{}
		// First chooser resolved via flag, second should reuse via session
		result := ResolveChoosers(blocks, prefs, "python", "", false)
		// Both should show python, not the fallback "show all"
		assert.NotContains(t, result, "TS code")
	})

	t.Run("OS chooser with flag", func(t *testing.T) {
		t.Parallel()
		osChooser := Chooser{
			Type: "os",
			Options: []Option{
				{Value: "linux", Content: "apt-get install"},
				{Value: "macos", Content: "brew install"},
			},
		}
		blocks := []ContentBlock{osChooser}
		prefs := &Preferences{}
		result := ResolveChoosers(blocks, prefs, "", "macos", false)
		assert.Contains(t, result, "brew install")
		assert.NotContains(t, result, "apt-get")
	})

	t.Run("inline chooser output", func(t *testing.T) {
		t.Parallel()
		inlineChooser := Chooser{
			Type:   "language",
			Prefix: "Install with ",
			Suffix: " today.",
			Options: []Option{
				{Value: "typescript", Content: "npm"},
				{Value: "python", Content: "pip"},
			},
		}
		blocks := []ContentBlock{inlineChooser}
		prefs := &Preferences{}
		result := ResolveChoosers(blocks, prefs, "python", "", false)
		assert.Equal(t, "Install with pip today.", result)
	})
}

func TestStripLeftoverTags(t *testing.T) {
	t.Parallel()

	t.Run("removes full-line tags", func(t *testing.T) {
		t.Parallel()
		input := "Before.\n<!-- chooser: language -->\nContent.\n<!-- /chooser -->\nAfter."
		result := stripLeftoverTags(input)
		assert.NotContains(t, result, "<!-- chooser")
		assert.NotContains(t, result, "<!-- /chooser")
		assert.Contains(t, result, "Before.")
		assert.Contains(t, result, "Content.")
		assert.Contains(t, result, "After.")
	})

	t.Run("preserves non-chooser comments", func(t *testing.T) {
		t.Parallel()
		input := "Before.\n<!-- This is a regular comment -->\nAfter."
		result := stripLeftoverTags(input)
		assert.Contains(t, result, "<!-- This is a regular comment -->")
	})
}

func TestPreferencesGetSet(t *testing.T) {
	t.Parallel()

	t.Run("language", func(t *testing.T) {
		t.Parallel()
		p := &Preferences{}
		p.Set("language", "go")
		assert.Equal(t, "go", p.Get("language"))
	})

	t.Run("os", func(t *testing.T) {
		t.Parallel()
		p := &Preferences{}
		p.Set("os", "linux")
		assert.Equal(t, "linux", p.Get("os"))
	})

	t.Run("cloud", func(t *testing.T) {
		t.Parallel()
		p := &Preferences{}
		p.Set("cloud", "aws")
		assert.Equal(t, "aws", p.Get("cloud"))
	})

	t.Run("unknown type returns empty", func(t *testing.T) {
		t.Parallel()
		p := &Preferences{Language: "go"}
		assert.Equal(t, "", p.Get("unknown"))
	})
}

func TestFilterCodeBlocksByLanguage(t *testing.T) {
	t.Parallel()

	multiLangExample := `## Example Usage

` + "```typescript" + `
console.log('hello');
` + "```" + `



` + "```python" + `
print('hello')
` + "```" + `



` + "```go" + `
fmt.Println("hello")
` + "```" + `

## Next Section
`

	t.Run("filters to python", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "python")
		assert.Contains(t, result, "```python")
		assert.Contains(t, result, "print('hello')")
		assert.NotContains(t, result, "```typescript")
		assert.NotContains(t, result, "```go")
		assert.Contains(t, result, "## Next Section")
	})

	t.Run("filters to go", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "go")
		assert.Contains(t, result, "```go")
		assert.NotContains(t, result, "```typescript")
		assert.NotContains(t, result, "```python")
	})

	t.Run("no filter when language is empty", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "")
		assert.Equal(t, multiLangExample, result)
	})

	t.Run("keeps all when language not found", func(t *testing.T) {
		t.Parallel()
		result := FilterCodeBlocksByLanguage(multiLangExample, "rust")
		assert.Contains(t, result, "```typescript")
		assert.Contains(t, result, "```python")
		assert.Contains(t, result, "```go")
	})

	t.Run("preserves isolated code blocks", func(t *testing.T) {
		t.Parallel()
		content := "Some text\n\n```bash\necho hello\n```\n\nMore text\n\n```python\nx = 1\n```\n"
		result := FilterCodeBlocksByLanguage(content, "python")
		assert.Contains(t, result, "```bash")
		assert.Contains(t, result, "```python")
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", FilterCodeBlocksByLanguage("", "python"))
	})

	t.Run("no language tag preserved", func(t *testing.T) {
		t.Parallel()
		content := "```\nplain code\n```\n"
		result := FilterCodeBlocksByLanguage(content, "python")
		assert.Equal(t, content, result)
	})

	t.Run("single code block not filtered", func(t *testing.T) {
		t.Parallel()
		content := "Text\n\n```python\nx = 1\n```\n\nMore text\n"
		result := FilterCodeBlocksByLanguage(content, "go")
		assert.Equal(t, content, result)
	})

	t.Run("unclosed fence does not panic", func(t *testing.T) {
		t.Parallel()
		content := "```python\nx = 1\n"
		result := FilterCodeBlocksByLanguage(content, "python")
		assert.Equal(t, content, result)
	})
}
