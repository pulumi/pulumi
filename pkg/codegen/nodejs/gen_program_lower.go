package nodejs

import (
	"github.com/hashicorp/hcl/v2"
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

func isPromiseType(t model.Type) bool {
	switch t := t.(type) {
	case *model.PromiseType:
		return true
	case *model.UnionType:
		isPromise := false
		for _, t := range t.ElementTypes {
			switch t.(type) {
			case *model.OutputType:
				return false
			case *model.PromiseType:
				isPromise = true
			}
		}
		return isPromise
	}
	return false
}

// canLiftTraversal returns true if this traversal can be lifted. Any traversal that does not traverse
// possibly-undefined values can be lifted.
func (g *generator) canLiftTraversal(parts []model.Traversable) bool {
	for _, p := range parts {
		t := model.GetTraversableType(p)
		if model.IsOptionalType(t) || isPromiseType(t) {
			return false
		}
	}
	return true
}

// parseProxyApply attempts to match the given parsed apply against the pattern (call __applyArg 0). If the call
// matches, it returns the ScopeTraversalExpression that corresponds to argument zero, which can then be generated as a
// proxied apply call.
func (g *generator) parseProxyApply(args []model.Expression,
	then model.Expression) (model.Expression, bool) {

	if len(args) != 1 {
		return nil, false
	}

	switch then := then.(type) {
	case *model.IndexExpression:
		if model.IsOptionalType(args[0].Type()) || isPromiseType(args[0].Type()) {
			return nil, false
		}
		then.Collection = args[0]
	case *model.RelativeTraversalExpression:
		if model.IsOptionalType(args[0].Type()) || isPromiseType(args[0].Type()) {
			return nil, false
		}
		then.Source = args[0]
	case *model.ScopeTraversalExpression:
		arg, ok := args[0].(*model.ScopeTraversalExpression)
		if !ok || isPromiseType(arg.Type()) {
			return nil, false
		}

		if !g.canLiftTraversal(then.Parts) {
			return nil, false
		}
		parts := make([]model.Traversable, len(arg.Parts)+len(then.Parts)-1)
		copy(parts, arg.Parts)
		copy(parts[len(arg.Parts):], then.Parts[1:])

		then.RootName = arg.RootName
		then.Traversal = hcl.TraversalJoin(arg.Traversal, then.Traversal[1:])
		then.Parts = parts
	default:
		return nil, false
	}

	diags := then.Typecheck(false)
	contract.Assert(len(diags) == 0)
	return then, true
}

func callbackParameterReferences(expr model.Expression, parameters codegen.Set) []*model.Variable {
	var refs []*model.Variable
	visitor := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		if expr, isScopeTraversal := expr.(*model.ScopeTraversalExpression); isScopeTraversal {
			if parameters.Has(expr.Parts[0]) {
				refs = append(refs, expr.Parts[0].(*model.Variable))
			}
		}
		return expr, nil
	}

	_, diags := model.VisitExpression(expr, model.IdentityVisitor, visitor)
	contract.Assert(len(diags) == 0)
	return refs
}

// parseInterpolate attempts to match the given parsed apply against the pattern (output /* mix of expressions and
// calls to __applyArg).
//
// A legal expression for the match is any expression that does not contain any calls to __applyArg: an expression that
// does contain such calls requires an apply.
//
// If the call matches, parseInterpolate returns an appropriate call to the __interpolate intrinsic with a mix of
// expressions and variable accesses that correspond to the __applyArg calls.
func (g *generator) parseInterpolate(args []model.Expression,
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
		parameterRefs := callbackParameterReferences(expr, parameters)
		if len(parameterRefs) == 0 {
			exprs[i] = expr
			continue
		}

		proxyArgs := make([]model.Expression, len(parameterRefs))
		for i, p := range parameterRefs {
			argIndex, ok := indices[p]
			contract.Assert(ok)

			proxyArgs[i] = args[argIndex]
		}

		expr, ok := g.parseProxyApply(proxyArgs, expr)
		if !ok {
			return nil, false
		}
		exprs[i] = expr
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
		if v, ok := g.parseProxyApply(args, then.Body); ok {
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
