// Copyright 2016-2025, Pulumi Corporation.
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
	"cmp"
	"slices"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// deduplicateSliceFunc provides a stable de-duplication on a slice of arbitrary elements.
//
// projection must return a order-able version of E, such that duplicate elements will be
// next to each-other.
func deduplicateSliceFunc[E any, S ~[]E, Projection cmp.Ordered](
	arr S, projection func(E) Projection, equality func(E, E) bool,
) S {
	type element struct {
		value      E
		idx        int
		projection Projection
	}
	elements := make([]element, 0, len(arr))
	for idx, v := range arr {
		elements = append(elements, element{v, idx, projection(v)})
	}

	slices.SortFunc(elements, func(a, b element) int {
		return cmp.Compare(a.projection, b.projection)
	})
	elements = slices.CompactFunc(elements, func(a, b element) bool {
		return equality(a.value, b.value)
	})
	slices.SortFunc(elements, func(a, b element) int {
		return cmp.Compare(a.idx, b.idx)
	})
	result := make(S, len(elements))
	for i, v := range elements {
		result[i] = v.value
	}
	return result
}

func syntaxOrNone(node hclsyntax.Node) hclsyntax.Node {
	if node == nil {
		return syntax.None
	}
	return node
}

// SourceOrderLess returns true if the first range precedes the second when ordered by source position. Positions are
// ordered first by filename, then by byte offset.
func SourceOrderLess(a, b hcl.Range) bool {
	return a.Filename < b.Filename || a.Start.Byte < b.Start.Byte
}

// SourceOrderBody sorts the contents of an HCL2 body in source order.
func SourceOrderBody(body *hclsyntax.Body) []hclsyntax.Node {
	items := slice.Prealloc[hclsyntax.Node](len(body.Attributes) + len(body.Blocks))
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

func VariableReference(v *Variable) *ScopeTraversalExpression {
	x := &ScopeTraversalExpression{
		RootName:  v.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: v.Name}},
		Parts:     []Traversable{v},
	}
	diags := x.Typecheck(false)
	contract.Assertf(len(diags) == 0, "error typechecking variable reference: %v", diags)
	return x
}

func ConstantReference(c *Constant) *ScopeTraversalExpression {
	x := &ScopeTraversalExpression{
		RootName:  c.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: c.Name}},
		Parts:     []Traversable{c},
	}
	diags := x.Typecheck(false)
	contract.Assertf(len(diags) == 0, "error typechecking constant reference: %v", diags)
	return x
}
