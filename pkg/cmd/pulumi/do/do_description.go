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

// helpWrapWidth is the column at which description text is wrapped.
const helpWrapWidth = 80

// linkStartMarker and linkEndMarker are emitted by the style around a link's text and URL so
// tidyRendered can keep the whole link on one line when wrapping; wordJoiner temporarily replaces
// the spaces between them.
const (
	linkStartMarker = "\x02"
	linkEndMarker   = "\x03"
	wordJoiner      = "\x01"
)

var linkRegexp = regexp.MustCompile(linkStartMarker + "[^" + linkEndMarker + "]*" + linkEndMarker)

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
		// so bare URLs render without the parentheses (and without the wrap markers).
		Link:        ansi.StylePrimitive{BlockPrefix: "(", BlockSuffix: ")" + linkEndMarker},
		LinkText:    ansi.StylePrimitive{BlockPrefix: linkStartMarker},
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
	style.LinkText.Bold = ptr(true)
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
	return renderMarkdown(md, cmdutil.InteractiveTerminal())
}

// tidyRendered wraps glamour's output to helpWrapWidth without splitting links, drops the trailing
// whitespace glamour pads each line with, and trims surrounding blank lines.
func tidyRendered(s string) string {
	// Join each link into a single unbreakable word for wrapLine; the joiners become spaces again
	// after wrapping.
	s = linkRegexp.ReplaceAllStringFunc(s, func(link string) string {
		return strings.ReplaceAll(link, " ", wordJoiner)
	})
	s = strings.NewReplacer(linkStartMarker, "", linkEndMarker, "").Replace(s)

	lines := strings.Split(s, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, wrapLine(strings.TrimRight(line, " \t"), helpWrapWidth)...)
	}
	s = strings.Trim(strings.Join(wrapped, "\n"), "\n")
	return strings.ReplaceAll(s, wordJoiner, " ")
}

// wrapLine wraps a line at width columns, breaking only at spaces. Continuation lines of a
// blockquote keep the `│ ` rail.
func wrapLine(line string, width int) []string {
	if visibleWidth(line) <= width {
		return []string{line}
	}

	prefix := ""
	if strings.HasPrefix(stripEscapes(line), "│ ") {
		prefix = "│ "
	}

	var out []string
	var current string
	currentWidth := 0
	for _, word := range strings.Split(line, " ") {
		w := visibleWidth(word)
		switch {
		case current == "":
			current, currentWidth = word, w
		case currentWidth+1+w <= width:
			current += " " + word
			currentWidth += 1 + w
		default:
			out = append(out, current)
			current = prefix + word
			currentWidth = visibleWidth(prefix) + w
		}
	}
	return append(out, current)
}

var ansiEscapeRegexp = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripEscapes(s string) string {
	return ansiEscapeRegexp.ReplaceAllString(s, "")
}

func visibleWidth(s string) int {
	return len([]rune(stripEscapes(s)))
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
		// Disable glamour's own wrapping, which breaks between a link's text and its URL;
		// tidyRendered wraps instead, keeping links whole.
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return md
	}
	rendered, err := renderer.Render(md)
	if err != nil {
		return md
	}
	return tidyRendered(rendered)
}
