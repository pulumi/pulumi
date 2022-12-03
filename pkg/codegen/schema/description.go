// Copyright 2016-2022, Pulumi Corporation.
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

package schema

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/pgavlin/goldmark/ast"
)

// Description and node types

type Description []DescriptionNode

type PclConverter func(body string) (map[string]DescriptionCodeElement, error)

func (d Description) NarrowToLanguage(lang string) Description {
	// TODO: Consider normalizing legacy earlier.
	if legacy, s, _ := d.checkLegacy(); legacy {
		source := []byte(s)
		parsed := ParseDocs(source)
		LegacyFilterExamples(source, parsed, lang)
		return MakeMarkdownDescription(RenderDocsToString(source, parsed))
	}
	desc := make(Description, 0, len(d))
	for _, d := range d {
		if code, ok := d.(DescriptionCodeNode); ok {
			if code.Code == nil || code.Code[lang].Body == "" {
				continue
			}
			code.Code = map[string]DescriptionCodeElement{
				lang: code.Code[lang],
			}
			desc = append(desc)
		} else {
			desc = append(desc, d)
		}

	}
	return desc
}

func (d Description) RenderToMarkdown(convert PclConverter) (string, error) {
	if legacy, d, _ := d.checkLegacy(); legacy {
		return d, nil
	}
	var rendered string

	renderCode := func(node DescriptionCodeNode) {
		if node.Code == nil {
			return
		}
		rendered += node.Trivia.Leading.asMarkdown()
		rendered += "{{% " + ExamplesShortcode + " %}}\n"
		for lang, block := range node.Code {
			rendered += "\n" + block.Trivia.Leading.asMarkdown()
			rendered += "{{% " + ExampleShortcode + " %}}\n"

			rendered += "```" + lang
			rendered += block.Body
			rendered += "```\n"

			rendered += "{{% /" + ExampleShortcode + " %}}"
			rendered += block.Trivia.Trailing.asMarkdown() + "\n"
		}
		rendered += "{{% /" + ExamplesShortcode + " %}}"
		rendered += node.Trivia.Trailing.asMarkdown()
	}

	var errs multierror.Error
	for _, node := range d {
		switch node := node.(type) {
		case DescriptionMarkdownNode:
			rendered += node.Text
		case DescriptionCodeNode:
			renderCode(node)
		case DescriptionPclNode:
			if convert == nil {
				continue
			}
			code, err := convert(node.Body)
			if err != nil {
				errs.Errors = append(errs.Errors, err)
			}

			// We'll try to render what was returned, even if there was an error.
			renderCode(DescriptionCodeNode{
				descriptionNode: node.descriptionNode,
				Trivia:          node.Trivia,
				Code:            code,
			})
		}
	}
	return rendered, nil
}

// A pest effort attempt at legacy behavor during the transition from `string` to
// `Description`.
func (d Description) pretendLegacy() string {
	if legacy, s, _ := d.checkLegacy(); legacy {
		return s
	}
	s, _ := d.RenderToMarkdown(nil)
	return s
}

func LegacyFilterExamples(source []byte, node ast.Node, lang string) {
	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		LegacyFilterExamples(source, c, lang)

		next = c.NextSibling()
		switch c := c.(type) {
		case *ast.FencedCodeBlock:
			sourceLang := string(c.Language(source))
			if sourceLang != lang && sourceLang != "sh" {
				node.RemoveChild(node, c)
			}
		case *Shortcode:
			switch string(c.Name) {
			case ExampleShortcode:
				hasCode := false
				for gc := c.FirstChild(); gc != nil; gc = gc.NextSibling() {
					if gc.Kind() == ast.KindFencedCodeBlock {
						hasCode = true
						break
					}
				}
				if hasCode {
					var grandchild, nextGrandchild ast.Node
					for grandchild = c.FirstChild(); grandchild != nil; grandchild = nextGrandchild {
						nextGrandchild = grandchild.NextSibling()
						node.InsertBefore(node, c, grandchild)
					}
				}
				node.RemoveChild(node, c)
			case ExamplesShortcode:
				if first := c.FirstChild(); first != nil {
					first.SetBlankPreviousLines(c.HasBlankPreviousLines())
				}

				var grandchild, nextGrandchild ast.Node
				for grandchild = c.FirstChild(); grandchild != nil; grandchild = nextGrandchild {
					nextGrandchild = grandchild.NextSibling()
					node.InsertBefore(node, c, grandchild)
				}
				node.RemoveChild(node, c)
			}
		}
	}
}

type DescriptionCodeNode struct {
	descriptionNode
	Trivia DescriptionTrivia
	Code   map[string]DescriptionCodeElement
}

type DescriptionCodeElement struct {
	Trivia DescriptionTrivia
	Body   string
}

type DescriptionTrivia struct {
	Leading  DescriptionTriviaField
	Trailing DescriptionTriviaField
}

type DescriptionMarkdownNode struct {
	descriptionNode
	Text string
}

type DescriptionPclNode struct {
	descriptionNode
	Trivia DescriptionTrivia
	Body   string
}

type DescriptionYamlNode struct {
	descriptionNode
	Trivia DescriptionTrivia
	Body   string
}

type DescriptionNode interface {
	isDescriptionNode()
	legacyText() *string
}

type DescriptionTriviaField interface {
	DescriptionNode
	asMarkdown() string
	isDescriptionTriviaField()
}

// Helper functions for creating simple descriptions

func MakeMarkdownDescription(s string) Description {
	return Description{
		DescriptionMarkdownNode{Text: s},
	}
}

// Implementation details for marshaling and unmarshaling

func (d Description) marshal() (DescriptionSpec, error) {
	if isLegacy, legacyText, otherNodes := d.checkLegacy(); isLegacy {
		if len(otherNodes) != 0 {
			return DescriptionSpec{}, fmt.Errorf("cannot mutate a legacy description")
		}
		return DescriptionSpec{Legacy: legacyText}, nil
	}
	structured := make([]interface{}, len(d))
	for i, node := range d {
		structured[i] = node
	}
	return DescriptionSpec{Structured: structured}, nil
}

func (d Description) checkLegacy() (bool, string, []DescriptionNode) {
	var text *string
	var otherNodes []DescriptionNode

	for i, node := range d {
		if node.legacyText() != text {
			if text == nil {
				// We have hit a legacy node for the first time.
				text = node.legacyText()
				// All previous nodes don't match, so mark them as other.
				otherNodes = d[:i]
			} else {
				otherNodes = append(otherNodes, node)
			}
		}
	}

	if text != nil {
		return true, *text, otherNodes
	}
	return false, "", nil
}

type descriptionNode struct {
	// A pointer to the text that the enclosing Description was derived from.
	// legacyText is nil if the node is not from a legacy value.
	// legacyText is compared with pointer equality.
	legacytxt *string
}

func (d descriptionNode) isDescriptionNode()  {}
func (d descriptionNode) legacyText() *string { return d.legacytxt }

func (d DescriptionMarkdownNode) isDescriptionTriviaField() {}
func (d DescriptionMarkdownNode) asMarkdown() string {
	return d.Text
}
