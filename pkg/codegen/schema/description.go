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
)

// Description and node types

type Description []DescriptionNode

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

type DescriptionPlainNode struct {
	descriptionNode
	Text string
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
func (d descriptionNode) legacyText() *string { return d.legacyText() }

func (d DescriptionMarkdownNode) isDescriptionTriviaField() {}
func (d DescriptionPlainNode) isDescriptionTriviaField()    {}
