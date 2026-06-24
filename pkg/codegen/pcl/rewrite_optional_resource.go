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

package pcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// IsOptionalResource reports whether the given traversable is a
// conditionally-created resource — a resource, read-resource, or component whose
// variable type is optional because it ranges over a boolean. Such a resource is
// `null`/`undefined` when the condition is false, so dereferencing one of its
// properties is only well-defined when the resource exists.
func IsOptionalResource(t model.Traversable) bool {
	switch t.(type) {
	case *Resource, *ReadResource, *Component:
		return model.IsOptionalType(model.GetTraversableType(t))
	default:
		return false
	}
}

// optionalResourceRoot reports whether expr is a property access (a traversal of
// at least one attribute) rooted at a conditionally-created resource. It returns
// a reference to the bare resource root so callers can guard the access against
// `null`.
func optionalResourceRoot(expr model.Expression) (*model.ScopeTraversalExpression, bool) {
	traversal, ok := expr.(*model.ScopeTraversalExpression)
	if !ok || len(traversal.Traversal) < 2 || len(traversal.Parts) == 0 {
		return nil, false
	}
	if !IsOptionalResource(traversal.Parts[0]) {
		return nil, false
	}
	rootRef := &model.ScopeTraversalExpression{
		RootName:  traversal.RootName,
		Traversal: traversal.Traversal[:1],
		Parts:     traversal.Parts[:1],
	}
	mustTypecheck(rootRef)
	return rootRef, true
}

// mustTypecheck typechecks an expression built during rewriting, where the
// operands are well-formed by construction, and asserts that it does so cleanly.
func mustTypecheck(expr model.Expression) {
	diags := expr.Typecheck(false)
	contract.Assertf(!diags.HasErrors(), "constructing optional-resource guard: %v", diags)
}

// RewriteOptionalResourceGuards guards dereferences of conditionally-created
// (boolean `range`) resources so the generated code is well-defined when the
// resource was not created. Any expression that reads a property off such a
// resource is wrapped as `root != null ? expr : null`, short-circuiting the
// access — and any apply over it — to `null` when the resource is absent.
//
// The guard is built from the resource references themselves rather than the
// original range condition so it narrows the optional type for type checkers.
// It must run before applies are rewritten so the apply is generated inside the
// guarded branch.
func RewriteOptionalResourceGuards(expr model.Expression) model.Expression {
	var roots []*model.ScopeTraversalExpression
	seen := map[string]bool{}
	collect := func(e model.Expression) (model.Expression, hcl.Diagnostics) {
		if rootRef, ok := optionalResourceRoot(e); ok && !seen[rootRef.RootName] {
			seen[rootRef.RootName] = true
			roots = append(roots, rootRef)
		}
		return e, nil
	}
	_, _ = model.VisitExpression(expr, model.IdentityVisitor, collect)
	if len(roots) == 0 {
		return expr
	}

	condition := model.Expression(notNull(roots[0]))
	for _, root := range roots[1:] {
		and := &model.BinaryOpExpression{
			LeftOperand:  condition,
			Operation:    hclsyntax.OpLogicalAnd,
			RightOperand: notNull(root),
		}
		mustTypecheck(and)
		condition = and
	}

	guarded := &model.ConditionalExpression{
		Condition:   condition,
		TrueResult:  expr,
		FalseResult: nullLiteral(),
	}
	mustTypecheck(guarded)
	return guarded
}

// notNull builds a `ref != null` comparison.
func notNull(ref model.Expression) *model.BinaryOpExpression {
	op := &model.BinaryOpExpression{
		LeftOperand:  ref,
		Operation:    hclsyntax.OpNotEqual,
		RightOperand: nullLiteral(),
	}
	mustTypecheck(op)
	return op
}

func nullLiteral() model.Expression {
	lit := &model.LiteralValueExpression{Value: cty.NullVal(cty.DynamicPseudoType)}
	mustTypecheck(lit)
	return lit
}
