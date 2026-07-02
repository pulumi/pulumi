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
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/pgavlin/goldmark/ast"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func ptr[T any](v T) *T { return &v }

// helpMarkdownStyle returns the glamour style for description Markdown: default text color, bold
// and underline only. When styled is false the same layout renders without escape sequences, so
// nothing leaks into pipes or files.
func helpMarkdownStyle(styled bool) ansi.StyleConfig {
	style := ansi.StyleConfig{
		Document:   ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{BlockSuffix: "\n"}},
		BlockQuote: ansi.StyleBlock{Indent: ptr(uint(1)), IndentToken: ptr("│ ")},
		Paragraph:  ansi.StyleBlock{},
		List:       ansi.StyleList{LevelIndent: 2},
		// glamour only applies the URL's block prefix and suffix when the link has separate text,
		// so bare URLs render without the parentheses.
		Link:        ansi.StylePrimitive{BlockPrefix: "(", BlockSuffix: ")"},
		Item:        ansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: ansi.StylePrimitive{BlockPrefix: ". "},
	}
	if !styled {
		style.Code = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{BlockPrefix: "`", BlockSuffix: "`"}}
		return style
	}
	style.Heading = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Bold: ptr(true)}}
	style.Strong = ansi.StylePrimitive{Bold: ptr(true)}
	style.Emph = ansi.StylePrimitive{Italic: ptr(true)}
	style.Link.Underline = ptr(true)
	style.LinkText = ansi.StylePrimitive{Bold: ptr(true)}
	style.Code = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Bold: ptr(true), Italic: ptr(true)}}
	return style
}

// droppedHelpSections are the lower-cased titles of heading sections that only document SDK
// authoring or importing, and so are removed from `do` help.
var droppedHelpSections = map[string]bool{
	"example usage": true,
	"example":       true,
	"import":        true,
}

// gluedHeadingRegexp mops up example/import headings that no Markdown parser can recognize because
// the schema glued them to the end of the preceding paragraph rather than starting a new line.
var gluedHeadingRegexp = regexp.MustCompile(`(?i)#{2,4}\s*(?:Example Usage|Import|Example)\b[^\n]*`)

// renderDescription prepares a schema resource/function description for display in `do` help.
func renderDescription(comment string) string {
	md := descriptionMarkdown(comment)
	if md == "" {
		return ""
	}
	return tidyRendered(renderMarkdown(md, cmdutil.InteractiveTerminal()))
}

// tidyRendered drops the trailing whitespace glamour pads each line with, removes the common left
// margin, and trims surrounding blank lines.
func tidyRendered(s string) string {
	lines := strings.Split(s, "\n")

	margin := -1
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" {
			continue
		}
		indent := len(trimmed) - len(strings.TrimLeft(trimmed, " "))
		if margin < 0 || indent < margin {
			margin = indent
		}
	}
	if margin < 0 {
		margin = 0
	}

	for i, line := range lines {
		line = strings.TrimRight(line, " \t")
		if len(line) >= margin {
			line = line[margin:]
		}
		lines[i] = line
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

// descriptionMarkdown returns the cleaned Markdown of a schema description, before any terminal
// styling.
func descriptionMarkdown(comment string) string {
	if comment == "" {
		return ""
	}

	source := []byte(comment)
	node := schema.ParseDocs(source)
	stripExamples(source, node)
	resolveDocRefs(node)
	md := schema.RenderDocsToString(source, node)

	md = gluedHeadingRegexp.ReplaceAllString(md, "")
	md = cleanComment(md)
	return strings.TrimSpace(md)
}

// stripExamples removes the parts of the documentation that only make sense when authoring a
// resource in an SDK: code blocks, example shortcodes, and the dropped heading sections.
func stripExamples(source []byte, node ast.Node) {
	dropToLevel := 0 // >0 while removing the body of a dropped heading section
	var next ast.Node
	for c := node.FirstChild(); c != nil; c = next {
		next = c.NextSibling()

		if dropToLevel > 0 {
			if h, ok := c.(*ast.Heading); ok && h.Level <= dropToLevel {
				dropToLevel = 0 // a same-or-higher heading ends the section; re-evaluate it below
			} else {
				node.RemoveChild(node, c)
				continue
			}
		}

		switch c := c.(type) {
		case *ast.Heading:
			title := strings.ToLower(strings.TrimSpace(string(c.Text(source))))
			if droppedHelpSections[title] {
				dropToLevel = c.Level
				node.RemoveChild(node, c)
				continue
			}
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			node.RemoveChild(node, c)
			continue
		case *schema.Shortcode:
			switch string(c.Name) {
			case schema.ExampleShortcode, schema.ExamplesShortcode:
				node.RemoveChild(node, c)
				continue
			}
		}

		stripExamples(source, c)
	}
}

// resolveDocRefs replaces `{{% ref %}}` shortcodes with the referenced entity's name, rendered as
// inline code.
func resolveDocRefs(node ast.Node) {
	var next ast.Node
	for c := node.FirstChild(); c != nil; c = next {
		next = c.NextSibling()

		if ref, ok := c.(*schema.Ref); ok {
			node.ReplaceChild(node, c, ast.NewString([]byte("`"+docRefName(ref.Destination)+"`")))
			continue
		}
		resolveDocRefs(c)
	}
}

// docRefName returns a readable name for a `{{% ref %}}` destination, e.g.
// "#/resources/pkg:mod:Bucket/properties/versioning" -> "versioning".
func docRefName(destination string) string {
	if _, fragment, ok := strings.Cut(destination, "#"); ok {
		destination = fragment
	}
	segments := strings.Split(strings.Trim(destination, "/"), "/")
	name := segments[len(segments)-1]
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// renderMarkdown renders Markdown for the terminal, falling back to the input unchanged on any
// renderer error.
func renderMarkdown(md string, styled bool) string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(helpMarkdownStyle(styled)),
		// 0 disables hard wrapping, leaving line breaks to the terminal so links never break mid-word.
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return md
	}
	rendered, err := renderer.Render(md)
	if err != nil {
		return md
	}
	return rendered
}
