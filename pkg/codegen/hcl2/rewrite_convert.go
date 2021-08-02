package hcl2

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
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

// rewriteConversions implements the core of RewriteConversions. It returns the rewritten expression and true if the
// type of the expression may have changed.
func rewriteConversions(x model.Expression, to model.Type) (model.Expression, bool) {
	// If rewriting an operand changed its type and the type of the expression depends on the type of that operand, the
	// expression must be typechecked in order to update its type.
	var typecheck bool

	switch x := x.(type) {
	case *model.AnonymousFunctionExpression:
		x.Body, _ = rewriteConversions(x.Body, to)
	case *model.BinaryOpExpression:
		x.LeftOperand, _ = rewriteConversions(x.LeftOperand, model.InputType(x.LeftOperandType()))
		x.RightOperand, _ = rewriteConversions(x.RightOperand, model.InputType(x.RightOperandType()))
	case *model.ConditionalExpression:
		var trueChanged, falseChanged bool
		x.Condition, _ = rewriteConversions(x.Condition, model.InputType(model.BoolType))
		x.TrueResult, trueChanged = rewriteConversions(x.TrueResult, to)
		x.FalseResult, falseChanged = rewriteConversions(x.FalseResult, to)
		typecheck = trueChanged || falseChanged
	case *model.ForExpression:
		traverserType := model.NumberType
		if x.Key != nil {
			traverserType = model.StringType
			x.Key, _ = rewriteConversions(x.Key, model.InputType(model.StringType))
		}
		if x.Condition != nil {
			x.Condition, _ = rewriteConversions(x.Condition, model.InputType(model.BoolType))
		}

		valueType, diags := to.Traverse(model.MakeTraverser(traverserType))
		contract.Ignore(diags)

		x.Value, typecheck = rewriteConversions(x.Value, valueType.(model.Type))
	case *model.FunctionCallExpression:
		args := x.Args
		for _, param := range x.Signature.Parameters {
			if len(args) == 0 {
				break
			}
			args[0], _ = rewriteConversions(args[0], model.InputType(param.Type))
			args = args[1:]
		}
		if x.Signature.VarargsParameter != nil {
			for i := range args {
				args[i], _ = rewriteConversions(args[i], model.InputType(x.Signature.VarargsParameter.Type))
			}
		}
	case *model.IndexExpression:
		x.Key, _ = rewriteConversions(x.Key, x.KeyType())
	case *model.ObjectConsExpression:
		if v := resolveDiscriminatedUnions(x, to); v != nil {
			to = v
			typecheck = true
		}
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

			var valueChanged bool
			item.Key, _ = rewriteConversions(item.Key, model.InputType(model.StringType))
			item.Value, valueChanged = rewriteConversions(item.Value, valueType.(model.Type))
			typecheck = typecheck || valueChanged
		}
	case *model.TupleConsExpression:
		for i := range x.Expressions {
			valueType, diags := to.Traverse(hcl.TraverseIndex{Key: cty.NumberIntVal(int64(i))})
			contract.Ignore(diags)

			var exprChanged bool
			x.Expressions[i], exprChanged = rewriteConversions(x.Expressions[i], valueType.(model.Type))
			typecheck = typecheck || exprChanged
		}
	case *model.UnaryOpExpression:
		x.Operand, _ = rewriteConversions(x.Operand, model.InputType(x.OperandType()))
	}

	var typeChanged bool
	if typecheck {
		diags := x.Typecheck(false)
		contract.Assert(len(diags) == 0)
		typeChanged = true
	}

	// If we can convert a primitive value in place, do so.
	if value, ok := convertPrimitiveValues(x, to); ok {
		x, typeChanged = value, true
	}
	// If the expression's type is directly assignable to the destination type, no conversion is necessary.
	if to.AssignableFrom(x.Type()) && sameSchemaTypes(to, x.Type()) {
		return x, typeChanged
	}

	// Otherwise, wrap the expression in a call to __convert.
	return NewConvertCall(x, to), true
}

// resolveDiscriminatedUnions reduces discriminated unions of object types to the type that matches
// the shape of the given object cons expression. A given object expression would only match a single
// case of the union.
func resolveDiscriminatedUnions(obj *model.ObjectConsExpression, modelType model.Type) model.Type {
	modelUnion, ok := modelType.(*model.UnionType)
	if !ok {
		return nil
	}
	schType, ok := GetSchemaForType(modelUnion)
	if !ok {
		return nil
	}
	schType = codegen.UnwrapType(schType)
	union, ok := schType.(*schema.UnionType)
	if !ok || union.Discriminator == "" {
		return nil
	}

	objTypes := GetDiscriminatedUnionObjectMapping(modelUnion)
	for _, item := range obj.Items {
		name, ok := item.Key.(*model.LiteralValueExpression)
		if !ok || name.Value.AsString() != union.Discriminator {
			continue
		}

		lit, ok := item.Value.(*model.TemplateExpression)
		if !ok {
			continue
		}

		discriminatorValue := lit.Parts[0].(*model.LiteralValueExpression).Value.AsString()
		if ref, ok := union.Mapping[discriminatorValue]; ok {
			discriminatorValue = strings.TrimPrefix(ref, "#/types/")
		}
		if t, ok := objTypes[discriminatorValue]; ok {
			return t
		}
	}

	return nil
}

// RewriteConversions wraps automatic conversions indicated by the HCL2 spec and conversions to schema-annotated types
// in calls to the __convert intrinsic.
//
// Note that the result is a bit out of line with the HCL2 spec, as static conversions may happen earlier than they
// would at runtime. For example, consider the case of a tuple of strings that is being converted to a list of numbers:
//
//     [a, b, c]
//
// Calling RewriteConversions on this expression with a destination type of list(number) would result in this IR:
//
//     [__convert(a), __convert(b), __convert(c)]
//
// If any of these conversions fail, the evaluation of the tuple itself fails. The HCL2 evaluation semantics, however,
// would convert the tuple _after_ it has been evaluated. The IR that matches these semantics is
//
//     __convert([a, b, c])
//
// This transform uses the former representation so that it can appropriately insert calls to `__convert` in the face
// of schema-annotated types. There is a reasonable argument to be made that RewriteConversions should not be
// responsible for propagating schema annotations, and that this pass should be split in two: one pass would insert
// conversions that match HCL2 evaluation semantics, and another would insert calls to some separate intrinsic in order
// to propagate schema information.
func RewriteConversions(x model.Expression, to model.Type) model.Expression {
	x, _ = rewriteConversions(x, to)
	return x
}

// convertPrimitiveValues returns a new expression if the given expression can be converted to another primitive type
// (bool, int, number, string) that matches the target type.
func convertPrimitiveValues(from model.Expression, to model.Type) (model.Expression, bool) {
	var expression model.Expression
	switch {
	case to.AssignableFrom(from.Type()) || to.AssignableFrom(model.DynamicType):
		return nil, false
	case to.AssignableFrom(model.BoolType):
		if stringLiteral, ok := extractStringValue(from); ok {
			if value, err := convert.Convert(cty.StringVal(stringLiteral), cty.Bool); err == nil {
				expression = &model.LiteralValueExpression{Value: value}
			}
		}
	case to.AssignableFrom(model.IntType), to.AssignableFrom(model.NumberType):
		if stringLiteral, ok := extractStringValue(from); ok {
			if value, err := convert.Convert(cty.StringVal(stringLiteral), cty.Number); err == nil {
				expression = &model.LiteralValueExpression{Value: value}
			}
		}
	case to.AssignableFrom(model.StringType):
		if stringValue, ok := convertLiteralToString(from); ok {
			expression = &model.TemplateExpression{
				Parts: []model.Expression{&model.LiteralValueExpression{
					Value: cty.StringVal(stringValue),
				}},
			}
		}
	}
	if expression == nil {
		return nil, false
	}

	diags := expression.Typecheck(false)
	contract.Assert(len(diags) == 0)

	expression.SetLeadingTrivia(from.GetLeadingTrivia())
	expression.SetTrailingTrivia(from.GetTrailingTrivia())
	return expression, true
}

// extractStringValue returns a string if the given expression is a template expression containing a single string
// literal value.
func extractStringValue(arg model.Expression) (string, bool) {
	template, ok := arg.(*model.TemplateExpression)
	if !ok || len(template.Parts) != 1 {
		return "", false
	}
	lit, ok := template.Parts[0].(*model.LiteralValueExpression)
	if !ok || model.StringType.ConversionFrom(lit.Type()) == model.NoConversion {
		return "", false
	}
	return lit.Value.AsString(), true
}

// convertLiteralToString converts a literal of type Bool, Int, or Number to its string representation. It also handles
// the unary negate operation in front of a literal number.
func convertLiteralToString(from model.Expression) (string, bool) {
	switch expr := from.(type) {
	case *model.UnaryOpExpression:
		if expr.Operation == hclsyntax.OpNegate {
			if operandValue, ok := convertLiteralToString(expr.Operand); ok {
				return "-" + operandValue, true
			}
		}
	case *model.LiteralValueExpression:
		if stringValue, err := convert.Convert(expr.Value, cty.String); err == nil {
			return stringValue.AsString(), true
		}
	}
	return "", false
}
