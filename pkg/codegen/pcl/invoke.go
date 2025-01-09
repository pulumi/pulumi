// Copyright 2016-2024, Pulumi Corporation.
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
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/zclconf/go-cty/cty"
)

// Invoke is the name of the PCL `invoke` intrinsic, which can be used to invoke provider functions.
const Invoke = "invoke"

func getInvokeToken(call *hclsyntax.FunctionCallExpr) (string, hcl.Range, bool) {
	if call.Name != Invoke || len(call.Args) < 1 {
		return "", hcl.Range{}, false
	}
	template, ok := call.Args[0].(*hclsyntax.TemplateExpr)
	if !ok || len(template.Parts) != 1 {
		return "", hcl.Range{}, false
	}
	literal, ok := template.Parts[0].(*hclsyntax.LiteralValueExpr)
	if !ok {
		return "", hcl.Range{}, false
	}
	if literal.Val.Type() != cty.String {
		return "", hcl.Range{}, false
	}
	return literal.Val.AsString(), call.Args[0].Range(), true
}

// annotateObjectProperties annotates the properties of an object expression with the
// types of the corresponding properties in the schema. This is used to provide type
// information for invoke calls that didn't have type annotations.
//
// This function will recursively annotate the properties of objects that are nested
// within the object expression type.
func annotateObjectProperties(modelType model.Type, schemaType schema.Type) {
	if optionalType, ok := schemaType.(*schema.OptionalType); ok && optionalType != nil {
		schemaType = optionalType.ElementType
	}

	switch arg := modelType.(type) {
	case *model.ObjectType:
		if schemaObjectType, ok := schemaType.(*schema.ObjectType); ok && schemaObjectType != nil {
			schemaProperties := make(map[string]schema.Type)
			for _, schemaProperty := range schemaObjectType.Properties {
				schemaProperties[schemaProperty.Name] = schemaProperty.Type
			}

			// top-level annotation for the type itself
			arg.Annotate(schemaType)
			// now for each property, annotate it with the associated type from the schema
			for propertyName, propertyType := range arg.Properties {
				if associatedType, ok := schemaProperties[propertyName]; ok {
					annotateObjectProperties(propertyType, associatedType)
				}
			}
		}
	case *model.ListType:
		underlyingArrayType := arg.ElementType
		if schemaArrayType, ok := schemaType.(*schema.ArrayType); ok && schemaArrayType != nil {
			underlyingSchemaArrayType := schemaArrayType.ElementType
			annotateObjectProperties(underlyingArrayType, underlyingSchemaArrayType)
		}

	case *model.TupleType:
		if schemaArrayType, ok := schemaType.(*schema.ArrayType); ok && schemaArrayType != nil {
			underlyingSchemaArrayType := schemaArrayType.ElementType
			elementTypes := arg.ElementTypes
			for _, elemType := range elementTypes {
				annotateObjectProperties(elemType, underlyingSchemaArrayType)
			}
		}
	case *model.UnionType:
		// sometimes optional schema types are represented as unions: None | T
		// in this case, we want to collapse the union and annotate the underlying type T
		if len(arg.ElementTypes) == 2 && arg.ElementTypes[0] == model.NoneType {
			annotateObjectProperties(arg.ElementTypes[1], schemaType)
		} else if len(arg.ElementTypes) == 2 && arg.ElementTypes[1] == model.NoneType {
			annotateObjectProperties(arg.ElementTypes[0], schemaType)
		} else { //nolint:staticcheck // TODO https://github.com/pulumi/pulumi/issues/10993
			// We need to handle the case where the schema type is a union type.
		}
	}
}

func (b *binder) bindInvokeSignature(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
	if len(args) < 1 {
		return b.zeroSignature(), nil
	}

	template, ok := args[0].(*model.TemplateExpression)
	if !ok || len(template.Parts) != 1 {
		return b.zeroSignature(), hcl.Diagnostics{tokenMustBeStringLiteral(args[0])}
	}
	lit, ok := template.Parts[0].(*model.LiteralValueExpression)
	if !ok || model.StringType.ConversionFrom(lit.Type()) == model.NoConversion {
		return b.zeroSignature(), hcl.Diagnostics{tokenMustBeStringLiteral(args[0])}
	}

	token, tokenRange := lit.Value.AsString(), args[0].SyntaxNode().Range()
	pkg, _, _, diagnostics := DecomposeToken(token, tokenRange)
	if diagnostics.HasErrors() {
		return b.zeroSignature(), diagnostics
	}

	pkgInfo := PackageInfo{
		name: pkg,
	}
	pkgSchema, ok := b.options.packageCache.entries[pkgInfo]
	if !ok {
		if b.options.skipInvokeTypecheck {
			return b.zeroSignature(), nil
		}
		return b.zeroSignature(), hcl.Diagnostics{unknownPackage(pkg, tokenRange)}
	}

	fn, tk, ok, err := pkgSchema.LookupFunction(token)
	if err != nil {
		if b.options.skipInvokeTypecheck {
			return b.zeroSignature(), nil
		}

		return b.zeroSignature(), hcl.Diagnostics{functionLoadError(token, err, tokenRange)}
	} else if !ok {
		if b.options.skipInvokeTypecheck {
			return b.zeroSignature(), nil
		}

		return b.zeroSignature(), hcl.Diagnostics{unknownFunction(token, tokenRange)}
	}

	lit.Value = cty.StringVal(tk)

	if len(args) < 2 {
		return b.zeroSignature(), hcl.Diagnostics{errorf(tokenRange, "missing second arg")}
	}
	sig, err := b.signatureForArgs(fn, args[1])
	if err != nil {
		diag := hcl.Diagnostics{errorf(tokenRange, "Invoke binding error: %v", err)}
		return b.zeroSignature(), diag
	}

	// annotate the input args on the expression with the input type of the function
	if argsObject, isObjectExpression := args[1].(*model.ObjectConsExpression); isObjectExpression {
		if fn.Inputs != nil {
			annotateObjectProperties(argsObject.Type(), fn.Inputs)
		}
	}

	sig.MultiArgumentInputs = fn.MultiArgumentInputs
	return sig, nil
}

func invokeOptionsType() model.Type {
	return model.NewObjectType(map[string]model.Type{
		// using dynamic (any) types for expressions expecting a Resource type because
		// we don't have a way to represent a Resource type in the PCL.
		"provider":          model.NewOptionalType(model.DynamicType),
		"parent":            model.NewOptionalType(model.DynamicType),
		"version":           model.NewOptionalType(model.StringType),
		"pluginDownloadUrl": model.NewOptionalType(model.StringType),
		"dependsOn":         model.NewOptionalType(model.NewListType(model.DynamicType)),
	})
}

func (b *binder) makeSignature(argsType, returnType model.Type) model.StaticFunctionSignature {
	return model.StaticFunctionSignature{
		Parameters: []model.Parameter{
			{
				Name: "token",
				Type: model.StringType,
			},
			{
				Name: "args",
				Type: argsType,
			},
			{
				Name: "invokeOptions",
				Type: model.NewOptionalType(invokeOptionsType()),
			},
		},
		ReturnType: returnType,
	}
}

func (b *binder) zeroSignature() model.StaticFunctionSignature {
	return b.makeSignature(model.NewOptionalType(model.DynamicType), model.DynamicType)
}

func (b *binder) signatureForArgs(fn *schema.Function, args model.Expression) (model.StaticFunctionSignature, error) {
	if args != nil && b.useOutputVersion(fn, args) {
		return b.outputVersionSignature(fn)
	}
	return b.regularSignature(fn), nil
}

// Heuristic to decide when to use `fnOutput` form of a function. Will
// conservatively prefer `false` unless bind option choose to prefer otherwise.
// It decides to return `true` if doing so avoids the need to introduce an `apply` form to
// accommodate `Output` args (`Promise` args do not count).
func (b *binder) useOutputVersion(fn *schema.Function, args model.Expression) bool {
	if fn.ReturnType == nil {
		// No code emitted for an `fnOutput` form, impossible.
		return false
	}

	if b.options.preferOutputVersionedInvokes {
		return true
	}

	if fn.Inputs == nil || len(fn.Inputs.Properties) == 0 {
		// use the output version when there are actual args to use
		return false
	}

	outputFormParamType := b.schemaTypeToType(fn.Inputs.InputShape)
	regularFormParamType := b.schemaTypeToType(fn.Inputs)
	argsType := args.Type()

	if regularFormParamType.ConversionFrom(argsType) == model.NoConversion &&
		outputFormParamType.ConversionFrom(argsType) == model.SafeConversion &&
		model.ContainsOutputs(argsType) {
		return true
	}

	return false
}

func (b *binder) regularSignature(fn *schema.Function) model.StaticFunctionSignature {
	var argsType model.Type
	if fn.Inputs == nil {
		argsType = model.NewOptionalType(model.NewObjectType(map[string]model.Type{}))
	} else {
		argsType = b.schemaTypeToType(fn.Inputs)
	}

	var returnType model.Type
	if fn.ReturnType == nil {
		returnType = model.NewObjectType(map[string]model.Type{})
	} else {
		returnType = b.schemaTypeToType(fn.ReturnType)
	}

	return b.makeSignature(argsType, model.NewPromiseType(returnType))
}

func (b *binder) outputVersionSignature(fn *schema.Function) (model.StaticFunctionSignature, error) {
	if !fn.NeedsOutputVersion() {
		return model.StaticFunctionSignature{}, fmt.Errorf("Function %s does not have an Output version", fn.Token)
	}

	// Given `fn.NeedsOutputVersion()==true` `fn.ReturnType != nil`.
	var argsType model.Type
	if fn.Inputs != nil {
		argsType = b.schemaTypeToType(fn.Inputs.InputShape)
	} else {
		argsType = model.NewObjectType(map[string]model.Type{})
	}
	returnType := b.schemaTypeToType(fn.ReturnType)
	return b.makeSignature(argsType, model.NewOutputType(returnType)), nil
}

// Detects invoke calls that use an output version of a function.
func IsOutputVersionInvokeCall(call *model.FunctionCallExpression) bool {
	if call.Name == Invoke {
		// Currently binder.bindInvokeSignature will assign
		// either DynamicType, a Promise<T>, or an Output<T>
		// for the return type of an invoke. Output<T> implies
		// that an output version has been picked.
		_, returnsOutput := call.Signature.ReturnType.(*model.OutputType)
		return returnsOutput
	}
	return false
}

// Pattern matches to recognize `__convert(objCons(..))` pattern that
// is used to annotate object constructors with appropriate nominal
// types. If the expression matches, returns true followed by the
// constructor expression and the appropriate type.
func RecognizeTypedObjectCons(theExpr model.Expression) (bool, *model.ObjectConsExpression, model.Type) {
	expr, isFunc := theExpr.(*model.FunctionCallExpression)
	if !isFunc {
		return false, nil, nil
	}

	if expr.Name != IntrinsicConvert {
		return false, nil, nil
	}

	if len(expr.Args) != 1 {
		return false, nil, nil
	}

	objCons, isObjCons := expr.Args[0].(*model.ObjectConsExpression)
	if !isObjCons {
		return false, nil, nil
	}

	return true, objCons, expr.Type()
}

// Pattern matches to recognize an encoded call to an output-versioned
// invoke, such as `invoke(token, __convert(objCons(..)))`. If
// matching, returns the `args` expression and its schema-bound type.
func RecognizeOutputVersionedInvoke(
	expr *model.FunctionCallExpression,
) (bool, *model.ObjectConsExpression, model.Type) {
	if !IsOutputVersionInvokeCall(expr) {
		return false, nil, nil
	}

	if len(expr.Args) < 2 {
		return false, nil, nil
	}

	return RecognizeTypedObjectCons(expr.Args[1])
}
