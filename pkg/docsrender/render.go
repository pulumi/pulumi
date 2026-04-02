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

	"github.com/charmbracelet/glamour"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/renderer"
	"github.com/pgavlin/goldmark/renderer/markdown"
	"github.com/pgavlin/goldmark/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// RenderMarkdown renders markdown content for terminal display.
// If raw is true, the content is returned as-is.
func RenderMarkdown(content string, raw bool) (string, error) {
	if raw {
		return content, nil
	}

	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(0))
	if err != nil {
		return content, nil
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content, nil
	}
	return rendered, nil
}

// FilterCodeBlocksByLanguage filters fenced code blocks to keep only those
// matching lang (or "sh"). Isolated code blocks not adjacent to other code
// blocks are always preserved. Adapts the pattern from pkg/codegen/docs.go.
func FilterCodeBlocksByLanguage(source []byte, tree ast.Node, lang string) []byte {
	if lang == "" {
		return renderTree(source, tree)
	}
	filterCodeBlocks(source, tree, lang)
	return renderTree(source, tree)
}

func filterCodeBlocks(source []byte, node ast.Node, lang string) {
	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		filterCodeBlocks(source, c, lang)
		next = c.NextSibling()

		cb, ok := c.(*ast.FencedCodeBlock)
		if !ok {
			continue
		}

		cbLang := string(cb.Language(source))
		if cbLang == "" || cbLang == lang || cbLang == "sh" || cbLang == "bash" || cbLang == "shell" {
			continue
		}

		// Only filter if this code block is part of a group (adjacent to other code blocks).
		if !hasAdjacentCodeBlock(c) {
			continue
		}

		node.RemoveChild(node, c)
	}
}

// hasAdjacentCodeBlock checks whether a node has a sibling code block nearby
// (within one node in either direction, skipping blank paragraphs).
func hasAdjacentCodeBlock(node ast.Node) bool {
	if prev := node.PreviousSibling(); prev != nil {
		if _, ok := prev.(*ast.FencedCodeBlock); ok {
			return true
		}
	}
	if next := node.NextSibling(); next != nil {
		if _, ok := next.(*ast.FencedCodeBlock); ok {
			return true
		}
	}
	return false
}

// ExtractLinks extracts all hyperlinks from the document, deduplicating by URL.
func ExtractLinks(source []byte, tree ast.Node) []Link {
	seen := map[string]bool{}
	var links []Link

	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}

		url := string(link.Destination)
		if seen[url] {
			return ast.WalkSkipChildren, nil
		}
		seen[url] = true

		title := linkPlainText(source, link)
		links = append(links, Link{URL: url, Title: title})
		return ast.WalkSkipChildren, nil
	})

	return links
}

func linkPlainText(source []byte, link *ast.Link) string {
	var buf bytes.Buffer
	for c := link.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		}
	}
	return buf.String()
}

func renderTree(source []byte, tree ast.Node) []byte {
	md := &markdown.Renderer{}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(md, 100)))
	var buf bytes.Buffer
	err := r.Render(&buf, source, tree)
	contract.AssertNoErrorf(err, "rendering markdown tree")
	return bytes.TrimRight(buf.Bytes(), "\n")
}
