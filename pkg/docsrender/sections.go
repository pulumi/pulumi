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
	"bytes"
	"strings"

	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/renderer"
	"github.com/pgavlin/goldmark/renderer/markdown"
	"github.com/pgavlin/goldmark/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ExtractHeadings returns all headings found in the document in order.
func ExtractHeadings(source []byte, tree ast.Node) []Heading {
	var headings []Heading
	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := node.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		title := headingPlainText(source, h)
		headings = append(headings, Heading{
			Slug:  slugify(title),
			Title: title,
			Level: h.Level,
		})
		return ast.WalkSkipChildren, nil
	})
	return headings
}

// ExtractSection returns the markdown content for the section identified by slug.
// A section starts at its heading and extends to the next heading of equal or higher
// level (lower number), or to the end of the document. Returns nil if not found.
func ExtractSection(source []byte, tree ast.Node, slug string) []byte {
	slug = strings.ToLower(slug)

	var startNode ast.Node
	var startLevel int
	for c := tree.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok {
			continue
		}
		title := headingPlainText(source, h)
		if strings.EqualFold(slugify(title), slug) {
			startNode = c
			startLevel = h.Level
			break
		}
	}
	if startNode == nil {
		return nil
	}

	var nodes []ast.Node
	for c := startNode; c != nil; c = c.NextSibling() {
		if c != startNode {
			if h, ok := c.(*ast.Heading); ok && h.Level <= startLevel {
				break
			}
		}
		nodes = append(nodes, c)
	}

	return renderNodes(source, nodes)
}

// ExtractIntro returns all content before the first heading, or nil if the
// document starts with a heading.
func ExtractIntro(source []byte, tree ast.Node) []byte {
	var nodes []ast.Node
	for c := tree.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*ast.Heading); ok {
			break
		}
		nodes = append(nodes, c)
	}
	if len(nodes) == 0 {
		return nil
	}
	return renderNodes(source, nodes)
}

// GetHeadings parses the markdown and returns top-level (H2) headings.
func GetHeadings(md string) []Heading {
	source := []byte(md)
	tree := ParseMarkdown(source)
	all := ExtractHeadings(source, tree)
	var h2s []Heading
	for _, h := range all {
		if h.Level == 2 {
			h2s = append(h2s, h)
		}
	}
	return h2s
}

// GetSection parses the markdown and returns the section content for the given slug.
func GetSection(md, slug string) string {
	source := []byte(md)
	tree := ParseMarkdown(source)
	section := ExtractSection(source, tree, slug)
	if section == nil {
		return ""
	}
	return string(section)
}

// GetIntro returns the content before the first heading.
// If there is no text before the first heading, includes the first section.
func GetIntro(md string) string {
	source := []byte(md)
	tree := ParseMarkdown(source)
	intro := ExtractIntro(source, tree)
	if intro != nil {
		return string(intro)
	}

	// No text before first heading — include the first section.
	headings := ExtractHeadings(source, tree)
	if len(headings) == 0 {
		return md
	}
	section := ExtractSection(source, tree, headings[0].Slug)
	if section != nil {
		return string(section)
	}
	return md
}

// IntroContainsFirstHeading returns true if the document starts with a heading
// (no text before it), meaning the intro includes the first section.
func IntroContainsFirstHeading(md string) bool {
	source := []byte(md)
	tree := ParseMarkdown(source)
	return ExtractIntro(source, tree) == nil
}

// ExtractBundleTitle extracts the title from a "# Title" first line.
func ExtractBundleTitle(content string) string {
	if !strings.HasPrefix(content, "# ") {
		return ""
	}
	if idx := strings.Index(content, "\n"); idx >= 0 {
		return strings.TrimPrefix(content[:idx], "# ")
	}
	return strings.TrimPrefix(content, "# ")
}

// ExtractBundleDescription extracts the first sentence of the description.
func ExtractBundleDescription(content string) string {
	body := content
	if strings.HasPrefix(body, "# ") {
		if idx := strings.Index(body, "\n"); idx >= 0 {
			body = body[idx+1:]
		} else {
			return ""
		}
	}
	body = strings.TrimLeft(body, "\n")

	if strings.HasPrefix(body, "> **Deprecated:") {
		if idx := strings.Index(body, "\n"); idx >= 0 {
			body = strings.TrimLeft(body[idx+1:], "\n")
		}
	}

	if body == "" {
		return ""
	}

	firstLine := body
	if idx := strings.Index(body, "\n"); idx >= 0 {
		firstLine = body[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)

	if strings.HasPrefix(firstLine, "#") || strings.HasPrefix(firstLine, "<!--") {
		return ""
	}

	if idx := strings.Index(firstLine, ". "); idx >= 0 {
		firstLine = firstLine[:idx+1]
	}

	const maxLen = 80
	if len(firstLine) > maxLen {
		firstLine = firstLine[:maxLen-3] + "..."
	}

	return firstLine
}

// --- Internal helpers ---

func headingPlainText(source []byte, h *ast.Heading) string {
	var buf bytes.Buffer
	_ = ast.Walk(h, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Text:
			buf.Write(n.Segment.Value(source))
			if n.SoftLineBreak() {
				buf.WriteByte(' ')
			}
		case *ast.CodeSpan:
			for gc := n.FirstChild(); gc != nil; gc = gc.NextSibling() {
				if t, ok := gc.(*ast.Text); ok {
					buf.Write(t.Segment.Value(source))
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return buf.String()
}

func renderNodes(source []byte, nodes []ast.Node) []byte {
	doc := ast.NewDocument()
	for _, n := range nodes {
		if n.Parent() != nil {
			n.Parent().RemoveChild(n.Parent(), n)
		}
		doc.AppendChild(doc, n)
	}

	md := &markdown.Renderer{}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(md, 100)))
	var buf bytes.Buffer
	err := r.Render(&buf, source, doc)
	contract.AssertNoErrorf(err, "rendering nodes to markdown")
	return bytes.TrimRight(buf.Bytes(), "\n")
}
