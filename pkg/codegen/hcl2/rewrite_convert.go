package hcl2

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v2/codegen"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

func sameSchemaTypes(xt, yt model.Type) bool {
	xs, _ := GetSchemaForType(xt)
	ys, _ := GetSchemaForType(yt)

	if xs == ys {
		return true
	}

	xu, ok := xs.(*schema.UnionType)
	if !ok {
		return false
	}
	yu, ok := ys.(*schema.UnionType)
	if !ok {
		return false
	}

	types := codegen.Set{}
	for _, t := range xu.ElementTypes {
		types.Add(t)
	}
	for _, t := range yu.ElementTypes {
		if !types.Has(t) {
			return false
		}
	}
	return true
}

func RewriteConversions(x model.Expression, to model.Type) model.Expression {
	switch x := x.(type) {
	case *model.AnonymousFunctionExpression:
		x.Body = RewriteConversions(x.Body, to)
	case *model.BinaryOpExpression:
		x.LeftOperand = RewriteConversions(x.LeftOperand, model.InputType(x.LeftOperandType()))
		x.RightOperand = RewriteConversions(x.RightOperand, model.InputType(x.RightOperandType()))
	case *model.ConditionalExpression:
		x.Condition = RewriteConversions(x.Condition, model.InputType(model.BoolType))
		x.TrueResult = RewriteConversions(x.TrueResult, to)
		x.FalseResult = RewriteConversions(x.FalseResult, to)

		diags := x.Typecheck(false)
		contract.Assert(len(diags) == 0)
	case *model.ForExpression:
		traverserType := model.NumberType
		if x.Key != nil {
			traverserType = model.StringType
			x.Key = RewriteConversions(x.Key, model.InputType(model.StringType))
		}
		if x.Condition != nil {
			x.Condition = RewriteConversions(x.Condition, model.InputType(model.BoolType))
		}

		valueType, diags := to.Traverse(model.MakeTraverser(traverserType))
		contract.Ignore(diags)

		x.Value = RewriteConversions(x.Value, valueType.(model.Type))

		diags = x.Typecheck(false)
		contract.Assert(len(diags) == 0)
	case *model.FunctionCallExpression:
		args := x.Args
		for _, param := range x.Signature.Parameters {
			if len(args) == 0 {
				break
			}
			args[0] = RewriteConversions(args[0], model.InputType(param.Type))
			args = args[1:]
		}
		if x.Signature.VarargsParameter != nil {
			for i := range args {
				args[i] = RewriteConversions(args[i], model.InputType(x.Signature.VarargsParameter.Type))
			}
		}
	case *model.IndexExpression:
		x.Key = RewriteConversions(x.Key, x.KeyType())
	case *model.ObjectConsExpression:
		for i := range x.Items {
			item := &x.Items[i]

			var traverser hcl.Traverser
			if lit, ok := item.Key.(*model.LiteralValueExpression); ok {
				traverser = hcl.TraverseIndex{Key: lit.Value}
			} else {
				traverser = model.MakeTraverser(model.StringType)
			}
			valueType, diags := to.Traverse(traverser)
			contract.Ignore(diags)

			item.Key = RewriteConversions(item.Key, model.InputType(model.StringType))
			item.Value = RewriteConversions(item.Value, model.InputType(valueType.(model.Type)))
		}
	case *model.TupleConsExpression:
		for i := range x.Expressions {
			valueType, diags := to.Traverse(hcl.TraverseIndex{Key: cty.NumberIntVal(int64(i))})
			contract.Ignore(diags)

			x.Expressions[i] = RewriteConversions(x.Expressions[i], valueType.(model.Type))
		}
	case *model.UnaryOpExpression:
		x.Operand = RewriteConversions(x.Operand, x.OperandType())
	}

	// If the expression's type is directly assignable to the destination type, no conversion is necessary.
	if to.AssignableFrom(x.Type()) && sameSchemaTypes(to, x.Type()) {
		return x
	}
	return NewConvertCall(x, to)
}
