package python

import (
	"github.com/hashicorp/hcl/v2"
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
	then model.Expression) (model.Expression, bool) {

	if len(args) != 1 {
		return nil, false
	}

	switch then := then.(type) {
	case *model.IndexExpression:
		then.Collection = args[0]
	case *model.RelativeTraversalExpression:
		then.Source = args[0]
	case *model.ScopeTraversalExpression:
		arg, ok := args[0].(*model.ScopeTraversalExpression)
		if !ok {
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
		if v, ok := g.parseProxyApply(args, then.Body); ok {
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
