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
)

func TestParseChooserComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		kind    string
		value   string
		isClose bool
		ok      bool
	}{
		{"<!-- chooser: language -->", "chooser", "language", false, true},
		{"<!-- option: typescript -->", "option", "typescript", false, true},
		{"<!-- option: python -->", "option", "python", false, true},
		{"<!-- /chooser -->", "chooser", "", true, true},
		{"<!-- /option -->", "option", "", true, true},
		{"<!-- chooser: os -->", "chooser", "os", false, true},
		{"<!-- something else -->", "", "", false, false},
		{"not a comment", "", "", false, false},
		{"", "", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			kind, value, isClose, ok := parseChooserComment(tt.input)
			assert.Equal(t, tt.kind, kind)
			assert.Equal(t, tt.value, value)
			assert.Equal(t, tt.isClose, isClose)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

func TestResolveChoosersWithSelection(t *testing.T) {
	t.Parallel()

	source := []byte(`Some text before.

<!-- chooser: language -->

<!-- option: typescript -->

TypeScript content here.

<!-- /option -->

<!-- option: python -->

Python content here.

<!-- /option -->

<!-- /chooser -->

Some text after.
`)
	tree := ParseMarkdown(source)
	result := string(ResolveChoosers(source, tree, map[string]string{"language": "python"}))

	assert.Contains(t, result, "Some text before.")
	assert.Contains(t, result, "Python content here.")
	assert.NotContains(t, result, "TypeScript content here.")
	assert.Contains(t, result, "Some text after.")
}

func TestResolveChoosersShowAll(t *testing.T) {
	t.Parallel()

	source := []byte(`Before.

<!-- chooser: language -->

<!-- option: typescript -->

TS code.

<!-- /option -->

<!-- option: python -->

PY code.

<!-- /option -->

<!-- /chooser -->

After.
`)
	tree := ParseMarkdown(source)
	result := string(ResolveChoosers(source, tree, map[string]string{}))

	assert.Contains(t, result, "Before.")
	assert.Contains(t, result, "TS code.")
	assert.Contains(t, result, "PY code.")
	assert.Contains(t, result, "After.")
}

func TestResolveChoosersEmptyChooser(t *testing.T) {
	t.Parallel()

	source := []byte(`Before.

<!-- chooser: language -->

<!-- /chooser -->

After.
`)
	tree := ParseMarkdown(source)
	result := string(ResolveChoosers(source, tree, map[string]string{"language": "python"}))

	assert.Contains(t, result, "Before.")
	assert.Contains(t, result, "After.")
}

func TestResolveChoosersUnknownSelection(t *testing.T) {
	t.Parallel()

	source := []byte(`<!-- chooser: language -->

<!-- option: typescript -->

TS only.

<!-- /option -->

<!-- /chooser -->
`)
	tree := ParseMarkdown(source)
	// Selection for a value not present in options — nothing is emitted for the chooser.
	result := string(ResolveChoosers(source, tree, map[string]string{"language": "go"}))

	assert.NotContains(t, result, "TS only.")
}

func TestResolveChoosersUnclosed(t *testing.T) {
	t.Parallel()

	source := []byte(`Before.

<!-- chooser: language -->

<!-- option: typescript -->

TS code.

After without close.
`)
	tree := ParseMarkdown(source)
	// Unclosed chooser is left as-is.
	result := string(ResolveChoosers(source, tree, map[string]string{"language": "typescript"}))

	assert.Contains(t, result, "Before.")
	assert.Contains(t, result, "TS code.")
}

func TestResolveChoosersCaseInsensitive(t *testing.T) {
	t.Parallel()

	source := []byte(`<!-- chooser: language -->

<!-- option: TypeScript -->

TS content.

<!-- /option -->

<!-- option: Python -->

PY content.

<!-- /option -->

<!-- /chooser -->
`)
	tree := ParseMarkdown(source)
	result := string(ResolveChoosers(source, tree, map[string]string{"language": "typescript"}))

	assert.Contains(t, result, "TS content.")
	assert.NotContains(t, result, "PY content.")
}

func TestResolveChoosersMultipleChoosers(t *testing.T) {
	t.Parallel()

	source := []byte(`<!-- chooser: language -->

<!-- option: typescript -->

TS.

<!-- /option -->

<!-- option: python -->

PY.

<!-- /option -->

<!-- /chooser -->

Middle text.

<!-- chooser: os -->

<!-- option: macos -->

Mac instructions.

<!-- /option -->

<!-- option: linux -->

Linux instructions.

<!-- /option -->

<!-- /chooser -->
`)
	tree := ParseMarkdown(source)
	result := string(ResolveChoosers(source, tree, map[string]string{
		"language": "python",
		"os":       "linux",
	}))

	assert.Contains(t, result, "PY.")
	assert.NotContains(t, result, "TS.")
	assert.Contains(t, result, "Middle text.")
	assert.Contains(t, result, "Linux instructions.")
	assert.NotContains(t, result, "Mac instructions.")
}
