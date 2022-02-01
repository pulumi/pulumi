package gen

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

type optionalTemp struct {
	Name  string
	Value model.Expression
}

func (ot *optionalTemp) Type() model.Type {
	return ot.Value.Type()
}

func (ot *optionalTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return ot.Type().Traverse(traverser)
}

func (ot *optionalTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type optionalSpiller struct{}

func (os *optionalSpiller) postVisitor(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	if ok, x, destType := os.recognizeInvokeArgThatNeedsOptionalConversion(expr); ok {
		if schemaType, ok := pcl.GetSchemaForType(destType); ok {
			if schemaType, ok := schemaType.(*schema.ObjectType); ok {
				// map of item name to optional type wrapper fn
				optionalPrimitives := make(map[string]schema.Type)
				for _, v := range schemaType.Properties {
					if !v.IsRequired() {
						ty := codegen.UnwrapType(v.Type)
						switch ty {
						case schema.NumberType, schema.BoolType, schema.IntType, schema.StringType:
							optionalPrimitives[v.Name] = ty
						}
					}
				}
				for i, item := range x.Items {
					// keys for schematized objects should be simple strings
					if key, ok := item.Key.(*model.LiteralValueExpression); ok {
						if model.StringType.AssignableFrom(key.Type()) {
							strKey := key.Value.AsString()
							if schemaType, isOptional := optionalPrimitives[strKey]; isOptional {
								functionName := os.getOptionalConversion(schemaType)
								expectedModelType := os.getExpectedModelType(schemaType)

								x.Items[i].Value = &model.FunctionCallExpression{
									Name: functionName,
									Signature: model.StaticFunctionSignature{
										Parameters: []model.Parameter{{
											Name: "val",
											Type: expectedModelType,
										}},
										ReturnType: model.NewOptionalType(expectedModelType),
									},
									Args: []model.Expression{item.Value},
								}
							}
						}
					}
				}
			}
		}
	}
	return expr, nil
}

func (*optionalSpiller) getOptionalConversion(ty schema.Type) string {
	switch ty {
	case schema.NumberType:
		return "goOptionalFloat64"
	case schema.BoolType:
		return "goOptionalBool"
	case schema.IntType:
		return "goOptionalInt"
	case schema.StringType:
		return "goOptionalString"
	default:
		return ""
	}
}

func (*optionalSpiller) getExpectedModelType(ty schema.Type) model.Type {
	switch ty {
	case schema.NumberType:
		return model.NumberType
	case schema.BoolType:
		return model.BoolType
	case schema.IntType:
		return model.IntType
	case schema.StringType:
		return model.StringType
	default:
		return nil
	}
}

func (g *generator) rewriteOptionals(
	x model.Expression,
	spiller *optionalSpiller,
) (model.Expression, []*optionalTemp, hcl.Diagnostics) {
	x, diags := model.VisitExpression(x, nil, spiller.postVisitor)
	return x, nil, diags
}

func (os *optionalSpiller) recognizeInvokeArgThatNeedsOptionalConversion(
	x model.Expression,
) (bool, *model.ObjectConsExpression, model.Type) {

	switch x := x.(type) {
	case *model.FunctionCallExpression:
		if x.Name != "invoke" {
			return false, nil, nil
		}

		// ignore output-versioned invokes as they do not need converting
		if isOutputInvoke, _, _ := pcl.RecognizeOutputVersionedInvoke(x); !isOutputInvoke {
			return false, nil, nil
		}

		destType := x.Args[1].Type()
		expr := x.Args[1]

		if c := os.recognizeNestedConvert(expr); c != nil {
			expr = c.Args[0]
			destType = c.Signature.ReturnType
		}

		switch expr := expr.(type) {
		case *model.ObjectConsExpression:
			return true, expr, destType
		}

		return false, nil, nil
	default:
		return false, nil, nil
	}
}

// Recognize `__convert(_convert(... x@_convert(v, ty)))` returning `x`.
func (os *optionalSpiller) recognizeNestedConvert(x model.Expression) *model.FunctionCallExpression {
	var r *model.FunctionCallExpression
	for {
		c := os.recognizeIntrinsicConvert(x)
		if c == nil {
			return r
		}
		r = c
		x = c.Args[0]
	}
}

func (*optionalSpiller) recognizeIntrinsicConvert(x model.Expression) *model.FunctionCallExpression {
	switch x := x.(type) {
	case *model.FunctionCallExpression:
		if x.Name == pcl.IntrinsicConvert {
			return x
		}
	}
	return nil
}
