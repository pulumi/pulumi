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

func TestSlugify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "plain text", text: "Getting Started", want: "getting-started"},
		{name: "backticks stripped", text: "Using `pulumi up`", want: "using-pulumi-up"},
		{name: "asterisks stripped", text: "**Bold** heading", want: "bold-heading"},
		{name: "underscores stripped", text: "some_thing_here", want: "somethinghere"},
		{name: "multiple dashes collapsed", text: "one - two - three", want: "one-two-three"},
		{name: "empty string", text: "", want: ""},
		{name: "numbers preserved", text: "Step 1 of 3", want: "step-1-of-3"},
		{name: "leading trailing dashes trimmed", text: " Hello World ", want: "hello-world"},
		{name: "special chars removed", text: "What's new?", want: "whats-new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := slugify(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractHeadings(t *testing.T) {
	t.Parallel()

	t.Run("multiple levels", func(t *testing.T) {
		t.Parallel()
		// extractHeadings only matches lines starting with "## " (level 2+)
		// ### is "## " prefixed too, so it matches as level 3
		md := "# Title\n\n## Overview\n\nSome text.\n\n## Details\n\nMore text.\n\n## Next\n"
		headings := extractHeadings(md)
		require.Len(t, headings, 3)
		assert.Equal(t, 2, headings[0].level)
		assert.Equal(t, "Overview", headings[0].text)
		assert.Equal(t, "overview", headings[0].slug)
		assert.Equal(t, 2, headings[1].level)
		assert.Equal(t, "Details", headings[1].text)
		assert.Equal(t, 2, headings[2].level)
		assert.Equal(t, "Next", headings[2].text)
	})

	t.Run("only level 2 headings matched", func(t *testing.T) {
		t.Parallel()
		// extractHeadings requires "## " prefix — ### and #### don't start with "## "
		md := "## Parent\n\n### Child\n\n#### Grandchild\n"
		headings := extractHeadings(md)
		// Only "## Parent" matches the prefix check
		require.Len(t, headings, 1)
		assert.Equal(t, "Parent", headings[0].text)
	})

	t.Run("no headings", func(t *testing.T) {
		t.Parallel()
		md := "Just some plain text with no headings."
		headings := extractHeadings(md)
		assert.Empty(t, headings)
	})

	t.Run("single heading", func(t *testing.T) {
		t.Parallel()
		md := "## Only One\n\nContent here."
		headings := extractHeadings(md)
		require.Len(t, headings, 1)
		assert.Equal(t, "Only One", headings[0].text)
	})

	t.Run("ignores h1", func(t *testing.T) {
		t.Parallel()
		md := "# Title\n\nContent."
		headings := extractHeadings(md)
		assert.Empty(t, headings)
	})
}

func TestExtractSection(t *testing.T) {
	t.Parallel()

	md := "# Title\n\nIntro text.\n\n## First\n\nFirst content.\n\n" +
		"## Second\n\nSecond content.\n\n## Third\n\nThird content.\n"

	t.Run("middle section", func(t *testing.T) {
		t.Parallel()
		section := extractSection(md, "second")
		assert.Contains(t, section, "## Second")
		assert.Contains(t, section, "Second content.")
		assert.NotContains(t, section, "Third content.")
	})

	t.Run("last section", func(t *testing.T) {
		t.Parallel()
		section := extractSection(md, "third")
		assert.Contains(t, section, "## Third")
		assert.Contains(t, section, "Third content.")
	})

	t.Run("missing slug returns empty", func(t *testing.T) {
		t.Parallel()
		section := extractSection(md, "nonexistent")
		assert.Equal(t, "", section)
	})

	t.Run("nested subsections included", func(t *testing.T) {
		t.Parallel()
		nested := "## Parent\n\nParent text.\n\n### Child\n\nChild text.\n\n## Sibling\n\nSibling text.\n"
		section := extractSection(nested, "parent")
		assert.Contains(t, section, "### Child")
		assert.Contains(t, section, "Child text.")
		assert.NotContains(t, section, "Sibling text.")
	})
}

func TestExtractIntro(t *testing.T) {
	t.Parallel()

	t.Run("text before first heading", func(t *testing.T) {
		t.Parallel()
		md := "Some intro text.\n\n## Section\n\nSection text."
		intro := extractIntro(md)
		assert.Equal(t, "Some intro text.", intro)
	})

	t.Run("no text before first heading includes first section", func(t *testing.T) {
		t.Parallel()
		md := "## First\n\nFirst content.\n\n## Second\n\nSecond content."
		intro := extractIntro(md)
		assert.Contains(t, intro, "## First")
		assert.Contains(t, intro, "First content.")
		assert.NotContains(t, intro, "## Second")
	})

	t.Run("no headings returns all", func(t *testing.T) {
		t.Parallel()
		md := "Just plain text with no headings at all."
		intro := extractIntro(md)
		assert.Equal(t, md, intro)
	})

	t.Run("single section returns all", func(t *testing.T) {
		t.Parallel()
		md := "## Only Section\n\nContent here."
		intro := extractIntro(md)
		assert.Equal(t, md, intro)
	})
}

func TestIntroContainsFirstHeading(t *testing.T) {
	t.Parallel()

	t.Run("starts with heading", func(t *testing.T) {
		t.Parallel()
		assert.True(t, introContainsFirstHeading("## First\n\nContent."))
	})

	t.Run("starts with blank then heading", func(t *testing.T) {
		t.Parallel()
		assert.True(t, introContainsFirstHeading("\n\n## First\n\nContent."))
	})

	t.Run("starts with text", func(t *testing.T) {
		t.Parallel()
		assert.False(t, introContainsFirstHeading("Some text.\n\n## First"))
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		assert.False(t, introContainsFirstHeading(""))
	})
}

func TestExtractLinks(t *testing.T) {
	t.Parallel()

	t.Run("internal links found", func(t *testing.T) {
		t.Parallel()
		md := "See [Stacks](/docs/iac/concepts/stacks) and [AWS](/registry/packages/aws)."
		links := extractLinks(md)
		require.Len(t, links, 2)
		assert.Equal(t, "Stacks", links[0].text)
		assert.Equal(t, "/docs/iac/concepts/stacks", links[0].href)
		assert.Equal(t, "AWS", links[1].text)
		assert.Equal(t, "/registry/packages/aws", links[1].href)
	})

	t.Run("deduplication", func(t *testing.T) {
		t.Parallel()
		md := "See [Stacks](/docs/stacks) and [also stacks](/docs/stacks)."
		links := extractLinks(md)
		require.Len(t, links, 1)
	})

	t.Run("no internal links", func(t *testing.T) {
		t.Parallel()
		md := "See [Google](https://google.com) for more."
		links := extractLinks(md)
		assert.Empty(t, links)
	})

	t.Run("empty string", func(t *testing.T) {
		t.Parallel()
		links := extractLinks("")
		assert.Empty(t, links)
	})
}

func TestNumberLinks(t *testing.T) {
	t.Parallel()

	t.Run("links get numbered", func(t *testing.T) {
		t.Parallel()
		md := "See [Stacks](/docs/stacks) and [Projects](/docs/projects)."
		annotated, links := numberLinks(md)
		require.Len(t, links, 2)
		assert.Contains(t, annotated, "🔗1")
		assert.Contains(t, annotated, "🔗2")
	})

	t.Run("duplicate hrefs share number", func(t *testing.T) {
		t.Parallel()
		md := "See [Stacks](/docs/stacks) and [more stacks](/docs/stacks)."
		annotated, links := numberLinks(md)
		require.Len(t, links, 1)
		// Both occurrences should have 🔗1
		assert.Contains(t, annotated, "🔗1 [Stacks]")
		assert.Contains(t, annotated, "🔗1 [more stacks]")
	})

	t.Run("no links returns original", func(t *testing.T) {
		t.Parallel()
		md := "Plain text with no links."
		annotated, links := numberLinks(md)
		assert.Equal(t, md, annotated)
		assert.Nil(t, links)
	})
}

func TestStripExternalLinks(t *testing.T) {
	t.Parallel()

	md := "See [Stacks](/docs/stacks) and [Google](https://google.com)."
	result := stripExternalLinks(md)
	assert.Equal(t, "See [Stacks](/docs/stacks) and Google.", result)
}
