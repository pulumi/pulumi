package python

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

// parseProxyApply attempts to match the given parsed apply against the pattern (call __applyArg 0). If the call
// matches, it returns the ScopeTraversalExpression that corresponds to argument zero, which can then be generated as a
// proxied apply call.
func (g *generator) parseProxyApply(args []model.Expression,
	then *model.AnonymousFunctionExpression) (*model.ScopeTraversalExpression, bool) {

	if len(args) != 1 {
		return nil, false
	}

	arg, ok := args[0].(*model.ScopeTraversalExpression)
	if !ok {
		return nil, false
	}

	thenTraversal, ok := then.Body.(*model.ScopeTraversalExpression)
	if !ok || thenTraversal.Parts[0] != then.Parameters[0] {
		return nil, false
	}

	traversal := hcl.TraversalJoin(arg.Traversal, thenTraversal.Traversal[1:])
	expr, diags := g.program.BindExpression(&hclsyntax.ScopeTraversalExpr{
		Traversal: traversal,
		SrcRange:  traversal.SourceRange(),
	})
	contract.Assert(len(diags) == 0)
	return expr.(*model.ScopeTraversalExpression), true
}

// lowerProxyApplies lowers certain calls to the apply intrinsic into proxied property accesses. Concretely, this boils
// down to rewriting the following shape:
// - (call __apply (resource variable access) (call __applyArg 0))
// into
// - (resource variable access)
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

		return expr, nil
	}
	return model.VisitExpression(expr, model.IdentityVisitor, rewriter)
}

func (g *generator) getObjectSchema(typ model.Type) *schema.ObjectType {
	typ = model.ResolveOutputs(typ)

	if union, ok := typ.(*model.UnionType); ok {
		for _, t := range union.ElementTypes {
			if obj := g.getObjectSchema(t); obj != nil {
				return obj
			}
		}
		return nil
	}

	annotations := typ.GetAnnotations()
	if len(annotations) == 1 {
		return annotations[0].(*schema.ObjectType)
	}
	return nil
}

func (g *generator) lowerObjectKeys(expr model.Expression, camelCaseToSnakeCase map[string]string) {
	switch expr := expr.(type) {
	case *model.ObjectConsExpression:
		for _, item := range expr.Items {
			// Ignore non-literal keys
			if key, ok := item.Key.(*model.LiteralValueExpression); ok && key.Value.Type().Equals(cty.String) {
				if keyVal, ok := camelCaseToSnakeCase[key.Value.AsString()]; ok {
					key.Value = cty.StringVal(keyVal)
				}
			}

			g.lowerObjectKeys(item.Value, camelCaseToSnakeCase)
		}
	case *model.TupleConsExpression:
		for _, element := range expr.Expressions {
			g.lowerObjectKeys(element, camelCaseToSnakeCase)
		}
	}
}
