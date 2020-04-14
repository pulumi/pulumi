package nodejs

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

func isOutputType(t model.Type) bool {
	switch t := t.(type) {
	case *model.OutputType:
		return true
	case *model.UnionType:
		for _, t := range t.ElementTypes {
			if _, isOutput := t.(*model.OutputType); isOutput {
				return true
			}
		}
	}
	return false
}

// canLiftScopeTraversalExpression returns true if this variable access expression can be lifted. Any variable access
// expression that does not contain references to potentially-undefined values (e.g. optional fields of a resource) can
// be lifted.
func (g *generator) canLiftScopeTraversalExpression(v *model.ScopeTraversalExpression) bool {
	for _, p := range v.Parts {
		t := model.GetTraversableType(p)
		if model.IsOptionalType(t) || !isOutputType(t) {
			return false
		}
	}
	return true
}

// parseProxyApply attempts to match the given parsed apply against the pattern (call __applyArg 0). If the call
// matches, it returns the ScopeTraversalExpression that corresponds to argument zero, which can then be generated as a
// proxied apply call.
func (g *generator) parseProxyApply(args []*model.ScopeTraversalExpression,
	then *model.AnonymousFunctionExpression) (*model.ScopeTraversalExpression, bool) {

	if len(args) != 1 {
		return nil, false
	}

	thenTraversal, ok := then.Body.(*model.ScopeTraversalExpression)
	if !ok || thenTraversal.Parts[0] != then.Parameters[0] {
		return nil, false
	}
	if !g.canLiftScopeTraversalExpression(thenTraversal) {
		return nil, false
	}

	traversal := hcl.TraversalJoin(args[0].Traversal, thenTraversal.Traversal[1:])
	expr, diags := g.program.BindExpression(&hclsyntax.ScopeTraversalExpr{
		Traversal: traversal,
		SrcRange:  traversal.SourceRange(),
	})
	contract.Assert(len(diags) == 0)
	return expr.(*model.ScopeTraversalExpression), true
}

func referencesCallbackParameter(expr model.Expression, parameters codegen.Set) bool {
	has := false
	visitor := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		if expr, isScopeTraversal := expr.(*model.ScopeTraversalExpression); isScopeTraversal {
			if parameters.Has(expr.Parts[0]) {
				has = true
			}
		}
		return expr, nil
	}

	_, diags := model.VisitExpression(expr, model.IdentityVisitor, visitor)
	contract.Assert(len(diags) == 0)
	return has
}

// parseInterpolate attempts to match the given parsed apply against the pattern (output /* mix of expressions and
// calls to __applyArg).
//
// A legal expression for the match is any expression that does not contain any calls to __applyArg: an expression that
// does contain such calls requires an apply.
//
// If the call matches, parseInterpolate returns an appropriate call to the __interpolate intrinsic with a mix of
// expressions and variable accesses that correspond to the __applyArg calls.
func (g *generator) parseInterpolate(args []*model.ScopeTraversalExpression,
	then *model.AnonymousFunctionExpression) (model.Expression, bool) {

	template, ok := then.Body.(*model.TemplateExpression)
	if !ok {
		return nil, false
	}

	parameters, indices := codegen.Set{}, map[*model.Variable]int{}
	for i, p := range then.Parameters {
		parameters.Add(p)
		indices[p] = i
	}

	exprs := make([]model.Expression, len(template.Parts))
	for i, expr := range template.Parts {
		traversal, isTraversal := expr.(*model.ScopeTraversalExpression)
		switch {
		case isTraversal && parameters.Has(traversal.Parts[0]):
			if !g.canLiftScopeTraversalExpression(traversal) {
				return nil, false
			}
			arg := args[indices[traversal.Parts[0].(*model.Variable)]]
			traversal := hcl.TraversalJoin(arg.Traversal, traversal.Traversal[1:])
			expr, diags := g.program.BindExpression(&hclsyntax.ScopeTraversalExpr{
				Traversal: traversal,
				SrcRange:  traversal.SourceRange(),
			})
			contract.Assert(len(diags) == 0)
			exprs[i] = expr
		case !referencesCallbackParameter(expr, parameters):
			exprs[i] = expr
		default:
			return nil, false
		}
	}

	return newInterpolateCall(exprs), true
}

// lowerProxyApplies lowers certain calls to the apply intrinsic into proxied property accesses and/or calls to the
// pulumi.interpolate function. Concretely, this boils down to rewriting the following shapes
// - (call __apply (resource variable access) (call __applyArg 0))
// - (call __apply (resource variable access 0) ... (resource variable access n)
//       (output /* some mix of expressions and calls to __applyArg))
// into (respectively)
// - (resource variable access)
// - (call __interpolate /* mix of literals and variable accesses that correspond to the __applyArg calls)
//
// The generated code requires that the target version of `@pulumi/pulumi` supports output proxies.
func (g *generator) lowerProxyApplies(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	rewriter := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		// Ignore the node if it is not a call to the apply intrinsic.
		apply, ok := expr.(*model.FunctionCallExpression)
		if !ok || apply.Name != hcl2.IntrinsicApply {
			return expr, nil
		}

		// Parse the apply call.
		args, then := hcl2.ParseApplyCall(apply)

		// Attempt to match (call __apply (rvar) (call __applyArg 0))
		if v, ok := g.parseProxyApply(args, then); ok {
			return v, nil
		}

		// Attempt to match (call __apply (rvar 0) ... (rvar n) (output /* mix of literals and calls to __applyArg)
		if v, ok := g.parseInterpolate(args, then); ok {
			return v, nil
		}

		return expr, nil
	}
	return model.VisitExpression(expr, model.IdentityVisitor, rewriter)
}
