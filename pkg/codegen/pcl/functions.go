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

package pcl

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

func getEntriesSignature(
	args []model.Expression,
	options bindOptions,
) (model.StaticFunctionSignature, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	keyType, valueType := model.Type(model.DynamicType), model.Type(model.DynamicType)
	signature := model.StaticFunctionSignature{
		Parameters: []model.Parameter{{
			Name: "collection",
			Type: model.DynamicType,
		}},
	}

	if len(args) == 1 {
		strictCollectionTypechecking := !options.skipRangeTypecheck
		keyT, valueT, diags := model.GetCollectionTypes(
			model.ResolveOutputs(args[0].Type()),
			args[0].SyntaxNode().Range(),
			strictCollectionTypechecking)
		keyType, valueType, diagnostics = keyT, valueT, append(diagnostics, diags...)
	}

	elementType := model.NewObjectType(map[string]model.Type{
		"key":   keyType,
		"value": valueType,
	})
	signature.ReturnType = model.NewListType(elementType)
	return signature, diagnostics
}

func pulumiBuiltins(options bindOptions) map[string]*model.Function {
	return map[string]*model.Function{
		"element": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				var diagnostics hcl.Diagnostics

				listType, returnType := model.Type(model.DynamicType), model.Type(model.DynamicType)
				if len(args) > 0 {
					switch t := model.ResolveOutputs(args[0].Type()).(type) {
					case *model.ListType:
						listType, returnType = args[0].Type(), t.ElementType
					case *model.TupleType:
						_, elementType := model.UnifyTypes(t.ElementTypes...)
						listType, returnType = args[0].Type(), elementType
					default:
						if !options.skipRangeTypecheck {
							// we are in strict mode, so we should error if the type is not a list or tuple
							rng := args[0].SyntaxNode().Range()
							diagnostics = hcl.Diagnostics{&hcl.Diagnostic{
								Severity: hcl.DiagError,
								Summary:  "the first argument to 'element' must be a list or tuple",
								Subject:  &rng,
							}}
						}
					}
				}
				return model.StaticFunctionSignature{
					Parameters: []model.Parameter{
						{
							Name: "list",
							Type: listType,
						},
						{
							Name: "index",
							Type: model.NumberType,
						},
					},
					ReturnType: returnType,
				}, diagnostics
			})),
		"entries": model.NewFunction(model.GenericFunctionSignature(
			func(arguments []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				return getEntriesSignature(arguments, options)
			})),

		"fileArchive": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: ArchiveType,
		}),
		"remoteArchive": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "uri",
				Type: model.StringType,
			}},
			ReturnType: ArchiveType,
		}),
		"assetArchive": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "assets",
				Type: model.NewMapType(AssetOrArchiveType),
			}},
			ReturnType: ArchiveType,
		}),
		"fileAsset": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: AssetType,
		}),
		"stringAsset": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "value",
				Type: model.StringType,
			}},
			ReturnType: AssetType,
		}),
		"remoteAsset": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "uri",
				Type: model.StringType,
			}},
			ReturnType: AssetType,
		}),
		"join": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{
				{
					Name: "separator",
					Type: model.StringType,
				},
				{
					Name: "strings",
					Type: model.NewListType(model.StringType),
				},
			},
			ReturnType: model.StringType,
		}),
		"length": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				var diagnostics hcl.Diagnostics

				valueType := model.Type(model.DynamicType)
				if len(args) > 0 {
					valueType = args[0].Type()
					elementType := UnwrapOption(model.ResolveOutputs(valueType))
					switch valueType := elementType.(type) {
					case *model.ListType, *model.MapType, *model.ObjectType, *model.TupleType:
						// OK
					default:
						if model.StringType.ConversionFrom(valueType) == model.NoConversion {
							rng := args[0].SyntaxNode().Range()
							diagnostics = hcl.Diagnostics{&hcl.Diagnostic{
								Severity: hcl.DiagError,
								Summary:  "the first argument to 'length' must be a list, map, object, tuple, or string",
								Subject:  &rng,
							}}
						}
					}
				}
				return model.StaticFunctionSignature{
					Parameters: []model.Parameter{{
						Name: "value",
						Type: valueType,
					}},
					ReturnType: model.IntType,
				}, diagnostics
			})),
		"lookup": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				var diagnostics hcl.Diagnostics

				mapType, elementType := model.Type(model.DynamicType), model.Type(model.DynamicType)
				if len(args) > 0 {
					switch t := model.ResolveOutputs(args[0].Type()).(type) {
					case *model.MapType:
						mapType, elementType = args[0].Type(), t.ElementType
					case *model.ObjectType:
						var unifiedType model.Type
						for _, t := range t.Properties {
							_, unifiedType = model.UnifyTypes(unifiedType, t)
						}
						mapType, elementType = args[0].Type(), unifiedType
					default:
						rng := args[0].SyntaxNode().Range()
						diagnostics = hcl.Diagnostics{&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "the first argument to 'lookup' must be a map",
							Subject:  &rng,
						}}
					}
				}
				return model.StaticFunctionSignature{
					Parameters: []model.Parameter{
						{
							Name: "map",
							Type: mapType,
						},
						{
							Name: "key",
							Type: model.StringType,
						},
						{
							Name: "default",
							Type: model.NewOptionalType(elementType),
						},
					},
					ReturnType: elementType,
				}, diagnostics
			})),
		"mimeType": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"range": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{
				{
					Name: "fromOrTo",
					Type: model.NumberType,
				},
				{
					Name: "to",
					Type: model.NewOptionalType(model.NumberType),
				},
			},
			ReturnType: model.NewListType(model.IntType),
		}),
		"readDir": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: model.NewListType(model.StringType),
		}),
		"readFile": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"filebase64": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"filebase64sha256": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "path",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"secret": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				valueType := model.Type(model.DynamicType)
				if len(args) == 1 {
					valueType = args[0].Type()
				}

				return model.StaticFunctionSignature{
					Parameters: []model.Parameter{{
						Name: "value",
						Type: valueType,
					}},
					ReturnType: model.NewOutputType(valueType),
				}, nil
			})),
		"unsecret": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				valueType := model.Type(model.DynamicType)
				if len(args) == 1 {
					valueType = args[0].Type()
				}

				return model.StaticFunctionSignature{
					Parameters: []model.Parameter{{
						Name: "value",
						Type: valueType,
					}},
					ReturnType: model.NewOutputType(valueType),
				}, nil
			})),
		"sha1": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "input",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"split": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{
				{
					Name: "separator",
					Type: model.StringType,
				},
				{
					Name: "string",
					Type: model.StringType,
				},
			},
			ReturnType: model.NewListType(model.StringType),
		}),
		"toBase64": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "value",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"fromBase64": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "value",
				Type: model.StringType,
			}},
			ReturnType: model.StringType,
		}),
		"toJSON": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "value",
				Type: model.DynamicType,
			}},
			ReturnType: model.StringType,
		}),
		// Returns the name of the current stack
		"stack": model.NewFunction(model.StaticFunctionSignature{
			ReturnType: model.StringType,
		}),
		// Returns the name of the current project
		"project": model.NewFunction(model.StaticFunctionSignature{
			ReturnType: model.StringType,
		}),
		// Returns the name of the current organization
		"organization": model.NewFunction(model.StaticFunctionSignature{
			ReturnType: model.StringType,
		}),
		// Returns the directory from which pulumi was run
		"cwd": model.NewFunction(model.StaticFunctionSignature{
			ReturnType: model.StringType,
		}),
		"notImplemented": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "errorMessage",
				Type: model.StringType,
			}},
			ReturnType: model.DynamicType,
		}),
		// Returns either the single item in a list, none if the list is empty or errors.
		"singleOrNone": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				var diagnostics hcl.Diagnostics

				valueType := model.Type(model.DynamicType)
				if len(args) == 1 {
					valueType = args[0].Type()
				} else {
					diagnostics = hcl.Diagnostics{&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "'singleOrNone' only expects one argument",
					}}
				}

				elementType := model.Type(model.DynamicType)

				switch valueType := model.ResolveOutputs(valueType).(type) {
				case *model.ListType:
					elementType = valueType.ElementType
				case *model.TupleType:
					if len(valueType.ElementTypes) > 0 {
						elementType = valueType.ElementTypes[0]
					}
				default:
					if valueType != model.DynamicType {
						rng := args[0].SyntaxNode().Range()
						diagnostics = hcl.Diagnostics{&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "the first argument to 'singleOrNone' must be a list or tuple",
							Subject:  &rng,
						}}
					}
				}

				return model.StaticFunctionSignature{
					Parameters: []model.Parameter{{
						Name: "value",
						Type: valueType,
					}},
					ReturnType: model.NewOptionalType(elementType),
				}, diagnostics
			})),
		"getOutput": model.NewFunction(model.StaticFunctionSignature{
			Parameters: []model.Parameter{{
				Name: "stackReference",
				// TODO: should be StackReference resource type but I'm not sure how to define that
				Type: model.DynamicType,
			}, {
				Name: "outputName",
				Type: model.StringType,
			}},
			ReturnType: model.NewOutputType(model.DynamicType),
		}),
		"try": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				var diagnostics hcl.Diagnostics

				// `try` is a variadic PCL function, so we can bind it in all cases except that in which there are no arguments.
				// When we do bind it, we generate a type based on the types of the actual arguments provided. So a call of the
				// form `try(e1, e2, ... en)` will be typed as `(t1, t2, ..., tn) -> output(dynamic)`, where `ti` is the type of
				// `ei`.
				if len(args) == 0 {
					diagnostics = hcl.Diagnostics{&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "'try' expects at least one argument",
					}}
				}

				parameters := make([]model.Parameter, len(args))
				for i, arg := range args {
					parameters[i] = model.Parameter{
						Name: fmt.Sprintf("arg%d", i),
						Type: arg.Type(),
					}
				}

				// TODO[#18555] perhaps the return type should be an OutputType so
				// apply can be called on it (since it may be an output).
				sig := model.StaticFunctionSignature{
					Parameters: parameters,
					ReturnType: model.NewOutputType(model.DynamicType),
				}

				return sig, diagnostics
			},
		)),
		"can": model.NewFunction(model.GenericFunctionSignature(
			func(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
				var diagnostics hcl.Diagnostics

				// if the input is not a single argument, we should error
				if len(args) != 1 {
					diagnostics = append(diagnostics, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "'can' expects exactly one argument",
					})
				}

				var argType model.Type
				argType = model.DynamicType
				if len(args) == 1 {
					argType = args[0].Type()
				}

				// if the input type is dynamic or an output we need to return an output bool,
				// but otherwise we can return a plain bool
				var returnType model.Type
				returnType = model.BoolType
				if isOutput(argType) || argType == model.DynamicType {
					returnType = model.NewOutputType(returnType)
				}

				sig := model.StaticFunctionSignature{
					Parameters: []model.Parameter{
						{
							Name: "arg",
							Type: argType,
						},
					},
					ReturnType: returnType,
				}

				return sig, diagnostics
			},
		)),
		"rootDirectory": model.NewFunction(model.StaticFunctionSignature{
			ReturnType: model.StringType,
		}),
	}
}
