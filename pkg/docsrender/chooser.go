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

// ResolveChoosers resolves chooser blocks in the markdown AST. The selections
// map chooser type (e.g., "language") to the desired value (e.g., "python").
// If no selection exists for a chooser type, all options are shown.
// The source must be the original bytes used to parse tree.
func ResolveChoosers(source []byte, tree ast.Node, selections map[string]string) []byte {
	resolveChooserBlocks(source, tree, selections)

	md := &markdown.Renderer{}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(md, 100)))
	var buf bytes.Buffer
	err := r.Render(&buf, source, tree)
	contract.AssertNoErrorf(err, "rendering after chooser resolution")
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// resolveChooserBlocks walks the AST looking for sequences of HTMLBlock nodes
// that form chooser/option/close patterns, then resolves them in place.
func resolveChooserBlocks(source []byte, tree ast.Node, selections map[string]string) {
	// Collect top-level children into a slice for safe iteration during mutation.
	var children []ast.Node
	for c := tree.FirstChild(); c != nil; c = c.NextSibling() {
		children = append(children, c)
	}

	i := 0
	for i < len(children) {
		node := children[i]
		text := htmlBlockText(source, node)
		kind, value, isClose, ok := parseChooserComment(text)
		if !ok || isClose || kind != "chooser" {
			i++
			continue
		}

		chooserType := value
		selected := selections[chooserType]

		// Scan forward to find options and the closing tag.
		type option struct {
			value string
			nodes []ast.Node
		}
		var options []option
		var currentOption *option
		j := i + 1
		closed := false

		for j < len(children) {
			t := htmlBlockText(source, children[j])
			k, v, ic, ok2 := parseChooserComment(t)
			if ok2 && k == "option" && !ic {
				if currentOption != nil {
					options = append(options, *currentOption)
				}
				currentOption = &option{value: v}
				j++
				continue
			}
			if ok2 && k == "chooser" && ic {
				if currentOption != nil {
					options = append(options, *currentOption)
				}
				closed = true
				j++
				break
			}
			if currentOption != nil {
				currentOption.nodes = append(currentOption.nodes, children[j])
			}
			j++
		}

		if !closed {
			// Unclosed chooser — leave as-is.
			i = j
			continue
		}

		// Remove the chooser open tag, all content, and close tag.
		for k := i; k < j; k++ {
			tree.RemoveChild(tree, children[k])
		}

		// Find the insertion point (the node after the removed range, or nil for end).
		var insertBefore ast.Node
		if j < len(children) {
			insertBefore = children[j]
		}

		if selected != "" {
			// Insert only the selected option's content.
			for _, opt := range options {
				if strings.EqualFold(opt.value, selected) {
					for _, n := range opt.nodes {
						tree.InsertBefore(tree, insertBefore, n)
					}
					break
				}
			}
		} else {
			// No selection: show all options with labels.
			for _, opt := range options {
				for _, n := range opt.nodes {
					tree.InsertBefore(tree, insertBefore, n)
				}
			}
		}

		i = j
	}
}

// htmlBlockText extracts the text content from an HTMLBlock node.
func htmlBlockText(source []byte, node ast.Node) string {
	hb, ok := node.(*ast.HTMLBlock)
	if !ok {
		return ""
	}
	var buf bytes.Buffer
	for i := 0; i < hb.Lines().Len(); i++ {
		seg := hb.Lines().At(i)
		buf.Write(seg.Value(source))
	}
	return strings.TrimSpace(buf.String())
}

// parseChooserComment parses an HTML comment that may be a chooser directive.
// Returns (kind, value, isClose, ok).
//
// Examples:
//
//	"<!-- chooser: language -->" → ("chooser", "language", false, true)
//	"<!-- option: typescript -->" → ("option", "typescript", false, true)
//	"<!-- /chooser -->"          → ("chooser", "", true, true)
//	"<!-- /option -->"           → ("option", "", true, true)
//	"<!-- something else -->"    → ("", "", false, false)
func parseChooserComment(text string) (kind, value string, isClose, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "<!--") || !strings.HasSuffix(text, "-->") {
		return "", "", false, false
	}
	inner := strings.TrimSpace(text[4 : len(text)-3])

	// Check for close tags: /chooser or /option.
	if strings.HasPrefix(inner, "/") {
		tag := strings.TrimSpace(inner[1:])
		switch tag {
		case "chooser":
			return "chooser", "", true, true
		case "option":
			return "option", "", true, true
		default:
			return "", "", false, false
		}
	}

	// Check for open tags: "chooser: TYPE" or "option: VALUE".
	parts := strings.SplitN(inner, ":", 2)
	if len(parts) != 2 {
		return "", "", false, false
	}
	tag := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	switch tag {
	case "chooser", "option":
		return tag, val, false, true
	default:
		return "", "", false, false
	}
}
