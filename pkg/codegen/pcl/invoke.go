// Copyright 2016-2021, Pulumi Corporation.
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

	pkgSchema, ok := b.options.packageCache.entries[pkg]
	if !ok {
		return b.zeroSignature(), hcl.Diagnostics{unknownPackage(pkg, tokenRange)}
	}

	fn, ok := pkgSchema.functions[token]
	if !ok {
		canon := canonicalizeToken(token, pkgSchema.schema)
		if fn, ok = pkgSchema.functions[canon]; ok {
			token, lit.Value = canon, cty.StringVal(canon)
		}
	}
	if !ok {
		return b.zeroSignature(), hcl.Diagnostics{unknownFunction(token, tokenRange)}
	}

	sig, err := b.signatureForArgs(fn, args[1])
	if err != nil {
		diag := hcl.Diagnostics{errorf(tokenRange, "Invoke binding error: %v", err)}
		return b.zeroSignature(), diag
	}

	return sig, nil
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
				Name: "provider",
				Type: model.NewOptionalType(model.StringType),
			},
		},
		ReturnType: returnType,
	}
}

func (b *binder) zeroSignature() model.StaticFunctionSignature {
	return b.makeSignature(model.NewOptionalType(model.DynamicType), model.DynamicType)
}

func (b *binder) signatureForArgs(fn *schema.Function, args model.Expression) (model.StaticFunctionSignature, error) {
	if b.useOutputVersion(fn, args) {
		return b.outputVersionSignature(fn)
	}
	return b.regularSignature(fn), nil
}

// Heuristic to decide when to use `fnOutput` form of a function. Will
// conservatively prefer `false`. It only decides to return `true` if
// doing so avoids the need to introduce an `apply` form to
// accommodate `Output` args (`Promise` args do not count).
func (b *binder) useOutputVersion(fn *schema.Function, args model.Expression) bool {
	if !fn.NeedsOutputVersion() {
		// No code emitted for an `fnOutput` form, impossible.
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
	if fn.Outputs == nil {
		returnType = model.NewObjectType(map[string]model.Type{})
	} else {
		returnType = b.schemaTypeToType(fn.Outputs)
	}

	return b.makeSignature(argsType, model.NewPromiseType(returnType))
}

func (b *binder) outputVersionSignature(fn *schema.Function) (model.StaticFunctionSignature, error) {
	if !fn.NeedsOutputVersion() {
		return model.StaticFunctionSignature{}, fmt.Errorf("Function %s does not have an Output version", fn.Token)
	}

	// Given `fn.NeedsOutputVersion()==true`, can assume `fn.Inputs != nil`, `fn.Outputs != nil`.
	argsType := b.schemaTypeToType(fn.Inputs.InputShape)
	returnType := b.schemaTypeToType(fn.Outputs)
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
