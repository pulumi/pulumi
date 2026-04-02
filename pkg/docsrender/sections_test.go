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

func TestExtractHeadings(t *testing.T) {
	t.Parallel()

	source := []byte(`# Title

Some intro text.

## Getting Started

Start here.

## Configuration

### Basic Config

Set things up.

### Advanced Config

More options.

## Conclusion

Done.
`)
	tree := ParseMarkdown(source)
	headings := ExtractHeadings(source, tree)

	assert.Equal(t, []Heading{
		{Slug: "title", Title: "Title", Level: 1},
		{Slug: "getting-started", Title: "Getting Started", Level: 2},
		{Slug: "configuration", Title: "Configuration", Level: 2},
		{Slug: "basic-config", Title: "Basic Config", Level: 3},
		{Slug: "advanced-config", Title: "Advanced Config", Level: 3},
		{Slug: "conclusion", Title: "Conclusion", Level: 2},
	}, headings)
}

func TestExtractHeadingsWithInlineFormatting(t *testing.T) {
	t.Parallel()

	source := []byte("## The `foo` method\n\n## **Bold** heading\n")
	tree := ParseMarkdown(source)
	headings := ExtractHeadings(source, tree)

	assert.Equal(t, []Heading{
		{Slug: "the-foo-method", Title: "The foo method", Level: 2},
		{Slug: "bold-heading", Title: "Bold heading", Level: 2},
	}, headings)
}

func TestExtractHeadingsEmpty(t *testing.T) {
	t.Parallel()

	source := []byte("Just a paragraph with no headings.\n")
	tree := ParseMarkdown(source)
	headings := ExtractHeadings(source, tree)

	assert.Empty(t, headings)
}

func TestExtractSectionMiddle(t *testing.T) {
	t.Parallel()

	source := []byte(`## First

Content of first.

## Second

Content of second.

## Third

Content of third.
`)
	tree := ParseMarkdown(source)
	section := ExtractSection(source, tree, "second")

	assert.Equal(t, "## Second\n\nContent of second.", string(section))
}

func TestExtractSectionLast(t *testing.T) {
	t.Parallel()

	source := []byte(`## First

Content.

## Last

Final content here.
`)
	tree := ParseMarkdown(source)
	section := ExtractSection(source, tree, "last")

	assert.Equal(t, "## Last\n\nFinal content here.", string(section))
}

func TestExtractSectionWithNestedSubheadings(t *testing.T) {
	t.Parallel()

	source := []byte(`## Overview

Intro.

### Sub One

Sub content.

### Sub Two

More sub content.

## Next Section

Other stuff.
`)
	tree := ParseMarkdown(source)
	section := ExtractSection(source, tree, "overview")

	assert.Contains(t, string(section), "## Overview")
	assert.Contains(t, string(section), "### Sub One")
	assert.Contains(t, string(section), "### Sub Two")
	assert.NotContains(t, string(section), "## Next Section")
}

func TestExtractSectionNotFound(t *testing.T) {
	t.Parallel()

	source := []byte("## Existing\n\nContent.\n")
	tree := ParseMarkdown(source)
	section := ExtractSection(source, tree, "nonexistent")

	assert.Nil(t, section)
}

func TestExtractSectionCaseInsensitive(t *testing.T) {
	t.Parallel()

	source := []byte("## Getting Started\n\nContent.\n")
	tree := ParseMarkdown(source)
	section := ExtractSection(source, tree, "GETTING-STARTED")

	require.NotNil(t, section)
	assert.Contains(t, string(section), "Getting Started")
}

func TestExtractIntro(t *testing.T) {
	t.Parallel()

	source := []byte(`Some introductory text.

More intro.

## First Section

Content.
`)
	tree := ParseMarkdown(source)
	intro := ExtractIntro(source, tree)

	assert.Equal(t, "Some introductory text.\n\nMore intro.", string(intro))
}

func TestExtractIntroNoIntro(t *testing.T) {
	t.Parallel()

	source := []byte("## Starts with heading\n\nContent.\n")
	tree := ParseMarkdown(source)
	intro := ExtractIntro(source, tree)

	assert.Nil(t, intro)
}
