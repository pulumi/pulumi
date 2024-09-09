// Copyright 2020-2024, Pulumi Corporation.
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

type optionalSpiller struct {
	invocation         *model.FunctionCallExpression
	intrinsicConvertTo *model.Type
}

func (os *optionalSpiller) preVisitor(x model.Expression) (model.Expression, hcl.Diagnostics) {
	switch x := x.(type) {
	case *model.FunctionCallExpression:
		if x.Name == "invoke" {
			// recurse into invoke args
			isOutputInvoke, _, _ := pcl.RecognizeOutputVersionedInvoke(x)
			// ignore output-versioned invokes as they do not need converting
			if !isOutputInvoke {
				os.invocation = x
			}
			os.intrinsicConvertTo = nil
			return x, nil
		}
		if x.Name == pcl.IntrinsicConvert {
			if os.invocation != nil {
				os.intrinsicConvertTo = &x.Signature.ReturnType
			}
			return x, nil
		}
	case *model.ObjectConsExpression:
		if os.invocation == nil {
			return x, nil
		}
		destType := x.Type()
		if os.intrinsicConvertTo != nil {
			destType = *os.intrinsicConvertTo
		}
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
		// Clear before visiting children, require another __convert call to set again
		os.intrinsicConvertTo = nil
		return x, nil
	default:
		// Ditto
		os.intrinsicConvertTo = nil
		return x, nil
	}
	return x, nil
}

func (os *optionalSpiller) postVisitor(x model.Expression) (model.Expression, hcl.Diagnostics) {
	switch x := x.(type) {
	case *model.FunctionCallExpression:
		if x.Name == "invoke" {
			if x == os.invocation {
				// Clear invocation flag once we're done traversing children.
				os.invocation = nil
			}
		}
	}
	return x, nil
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
	// We want to recurse but we only want to use the previsitor, if post visitor is nil we don't
	// recurse.
	x, diags := model.VisitExpression(x, spiller.preVisitor, spiller.postVisitor)

	return x, nil, diags
}
