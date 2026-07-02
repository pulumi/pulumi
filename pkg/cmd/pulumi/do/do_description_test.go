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

package do

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescriptionMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("keeps intro prose and notes", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("Manages a Function.\n\n> **Note:** something important.")
		assert.Contains(t, got, "Manages a Function.")
		assert.Contains(t, got, "something important.")
	})

	t.Run("drops the new-form Example Usage section", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("Intro paragraph.\n\n" +
			"## Example Usage\n\n### Basic\n\n```typescript\nconst f = new aws.lambda.Function(\"f\");\n```\n\n" +
			"## Import\n\n```sh\npulumi import aws:lambda/function:Function f my-func\n```")
		assert.Contains(t, got, "Intro paragraph.")
		assert.NotContains(t, got, "Example Usage")
		assert.NotContains(t, got, "Import")
		assert.NotContains(t, got, "typescript")
		assert.NotContains(t, got, "pulumi import")
	})

	t.Run("drops the old-form examples shortcode block", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("Manages the contact.\n\n" +
			"{{% examples %}}\n## Example Usage\n{{% example %}}\n\n```python\nx = 1\n```\n{{% /example %}}\n{{% /examples %}}")
		assert.Contains(t, got, "Manages the contact.")
		assert.NotContains(t, got, "{{%")
		assert.NotContains(t, got, "Example Usage")
		assert.NotContains(t, got, "python")
	})

	t.Run("drops code examples that have no heading", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("Provides an endpoint resource.\n\n```typescript\nconst e = 1;\n```\n```go\ne := 1\n```")
		assert.Contains(t, got, "Provides an endpoint resource.")
		assert.NotContains(t, got, "typescript")
		assert.NotContains(t, got, "const e")
	})

	t.Run("drops a heading glued to the preceding paragraph", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("The password is stored in plain-text. ## Example Usage\n\n```typescript\nx\n```")
		assert.Contains(t, got, "plain-text.")
		assert.NotContains(t, got, "Example Usage")
	})

	t.Run("resolves language-choice spans to the canonical form", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("Conflicts with " +
			"<span pulumi-lang-nodejs=\"`imageUri`\" pulumi-lang-python=\"`image_uri`\">`imageUri`</span>.")
		assert.Contains(t, got, "imageUri")
		assert.NotContains(t, got, "<span")
		assert.NotContains(t, got, "pulumi-lang")
	})

	t.Run("ref shortcodes render as the referenced name", func(t *testing.T) {
		t.Parallel()
		got := descriptionMarkdown("See {{% ref #/resources/example:index:BucketPolicy %}} to attach a policy, " +
			"or the {{% ref #/resources/example:index:Bucket/properties/versioning %}} property.")
		assert.Equal(t, "See `BucketPolicy` to attach a policy, or the `versioning` property.", got)
	})

	t.Run("empty description yields empty output", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, descriptionMarkdown(""))
	})
}

func TestRenderDescriptionRunsCleanly(t *testing.T) {
	t.Parallel()

	// renderDescription pushes the cleaned Markdown through glamour; make sure that path does not
	// panic and still drops the example content.
	out := renderDescription("Intro.\n\n## Example Usage\n\n```go\nx := 1\n```")
	assert.Contains(t, out, "Intro.")
	assert.NotContains(t, out, "Example Usage")
}

func TestRenderMarkdownStyled(t *testing.T) {
	t.Parallel()

	const description = "Provides a bucket.\n\n" +
		"> **Note:** manage locking with the `aws.s3.BucketObjectLockConfiguration` resource, " +
		"or see [S3 on Outposts](https://example.com/outposts) for details. " +
		"This sentence pads the quoted paragraph well past a single line of eighty columns.\n\n" +
		"## Notes\n\nDone."

	out := renderMarkdown(descriptionMarkdown(description), true)

	// No colors — only bold (1), italic (3), underline (4), and resets.
	for _, seq := range ansiEscapeRegexp.FindAllString(out, -1) {
		assert.Contains(t, []string{"\x1b[1m", "\x1b[3m", "\x1b[4m", "\x1b[1;3m", "\x1b[0m"}, seq)
	}
	// Blockquotes carry the `│ ` rail, in the default text color.
	assert.Contains(t, stripANSI(out), "│ Note:")
	// Strong text and headings are bold.
	assert.Contains(t, out, "\x1b[1mNote:\x1b[0m")
	assert.Contains(t, out, "\x1b[1mNotes\x1b[0m")
	// Inline code (resource/property references) is bold italic, without backticks.
	assert.Contains(t, out, "\x1b[1;3maws.s3.BucketObjectLockConfiguration\x1b[0m")
	assert.NotContains(t, out, "`")
	// Link text is bold; the URL follows in parentheses, underlined but not bold.
	assert.Contains(t, out, "\x1b[1mS3 on Outposts\x1b[0m")
	assert.Contains(t, out, "(\x1b[4mhttps://example.com/outposts\x1b[0m)")

	// Text is wrapped at eighty columns, a link and its URL always share a line, and continuation
	// lines of a blockquote keep the rail.
	quoteLines := 0
	for _, line := range strings.Split(stripANSI(out), "\n") {
		assert.LessOrEqual(t, len([]rune(line)), 80)
		if strings.Contains(line, "S3 on Outposts") {
			assert.Contains(t, line, "S3 on Outposts (https://example.com/outposts)")
		}
		if strings.HasPrefix(line, "│ ") {
			quoteLines++
		}
	}
	assert.Greater(t, quoteLines, 1, "the wrapped blockquote should keep its rail on every line")
}

func TestRenderMarkdownPlain(t *testing.T) {
	t.Parallel()

	const description = "Provides a bucket.\n\n" +
		"> **Note:** manage locking with the `aws.s3.BucketObjectLockConfiguration` resource, " +
		"or see [S3 on Outposts](https://example.com/outposts) for details."

	out := renderMarkdown(descriptionMarkdown(description), false)

	// Piped or redirected output carries no escape sequences but keeps the same layout, including
	// the blockquote rail; inline code keeps its backticks so references still stand out.
	assert.Empty(t, ansiEscapeRegexp.FindAllString(out, -1))
	assert.Contains(t, out, "│ Note:")
	assert.Contains(t, out, "Provides a bucket.")
	assert.Contains(t, out, "`aws.s3.BucketObjectLockConfiguration`")
	assert.Contains(t, out, "(https://example.com/outposts)")
}
