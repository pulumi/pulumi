// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func SourceOrderLess(a, b hcl.Range) bool {
	return a.Filename < b.Filename || a.Start.Byte < b.Start.Byte
}

func SourceOrderBody(body *hclsyntax.Body) []hclsyntax.Node {
	items := make([]hclsyntax.Node, 0, len(body.Attributes)+len(body.Blocks))
	for _, attr := range body.Attributes {
		items = append(items, attr)
	}
	for _, block := range body.Blocks {
		items = append(items, block)
	}
	sort.Slice(items, func(i, j int) bool {
		return SourceOrderLess(items[i].Range(), items[j].Range())
	})
	return items
}

func SourceOrderBlocks(blocks []*hclsyntax.Block) []*hclsyntax.Block {
	sort.Slice(blocks, func(i, j int) bool {
		return SourceOrderLess(blocks[i].Range(), blocks[j].Range())
	})
	return blocks
}

func SourceOrderAttributes(attrMap map[string]*hclsyntax.Attribute) []*hclsyntax.Attribute {
	attrs := make([]*hclsyntax.Attribute, 0, len(attrMap))
	for _, attr := range attrMap {
		attrs = append(attrs, attr)
	}
	sort.Slice(attrs, func(i, j int) bool {
		return SourceOrderLess(attrs[i].Range(), attrs[j].Range())
	})
	return attrs
}

func SourceOrderDefinitions(defs []Definition) []Definition {
	sort.Slice(defs, func(i, j int) bool {
		return SourceOrderLess(defs[i].SyntaxNode().Range(), defs[j].SyntaxNode().Range())
	})
	return defs
}
