// Copyright 2026, Pulumi Corporation.
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
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// A function declared with multiArgumentInputs must be invoked positionally, passing one argument
// per input rather than wrapping them in a single object argument:
//
//	invoke("pkg:index:fn", arg1, arg2)            // required
//	invoke("pkg:index:fn", { p1 = arg1, p2 = arg2 }) // rejected by the binder
//
// The optional invokeOptions object is passed as a final positional argument, after one argument
// per input. To supply invokeOptions while omitting an optional input, pass null for that input:
//
//	invoke("pkg:index:fn", arg1, null, { provider = p })
//
// Positional invokes are bound against a per-input signature (see positionalInvokeSignature) and
// then normalized to the object form by rewritePositionalInvokes, so that the rest of PCL and every
// program generator only ever observe the object form.

// invokeUsesObjectArgument reports whether an invoke's second argument is a single object assignable
// to the function's input object (in either its plain or its inputy shape), i.e. the object-argument
// form rather than the positional form.
func (b *binder) invokeUsesObjectArgument(fn *schema.Function, args []model.Expression) bool {
	if fn.Inputs == nil || len(args) < 2 {
		return false
	}
	plain := b.schemaTypeToType(fn.Inputs)
	inputy := b.schemaTypeToType(fn.Inputs.InputShape)
	argType := args[1].Type()
	return plain.ConversionFrom(argType) != model.NoConversion ||
		inputy.ConversionFrom(argType) != model.NoConversion
}

// positionalInvokeSignature builds the signature for a positional multi-argument invoke:
// (token, <input 1>, <input 2>, ..., invokeOptions?). The parameters follow the order declared by
// the function's multiArgumentInputs. Optional inputs accept null, allowing a caller to skip them
// while still supplying the trailing invokeOptions argument.
func (b *binder) positionalInvokeSignature(fn *schema.Function) model.StaticFunctionSignature {
	params := make([]model.Parameter, 0, len(fn.Inputs.Properties)+2)
	params = append(params, model.Parameter{Name: "token", Type: model.StringType})
	for _, prop := range fn.Inputs.Properties {
		t := b.schemaTypeToType(prop.Type)
		if !prop.IsRequired() && !model.IsOptionalType(t) {
			t = model.NewOptionalType(t)
		}
		params = append(params, model.Parameter{Name: prop.Name, Type: t})
	}
	params = append(params, model.Parameter{
		Name: "invokeOptions",
		Type: model.NewOptionalType(invokeOptionsType()),
	})

	var returnType model.Type
	if fn.ReturnType == nil {
		returnType = model.NewObjectType(map[string]model.Type{})
	} else {
		returnType = b.schemaTypeToType(fn.ReturnType)
	}

	return model.StaticFunctionSignature{
		Parameters:          params,
		ReturnType:          model.NewPromiseType(returnType),
		MultiArgumentInputs: fn.MultiArgumentInputs,
	}
}

// invokeFunction re-resolves the schema function referenced by an invoke call's arguments. It
// mirrors the lookup performed during signature binding (sharing invokeTokenArgument and
// loadPackageSchema) and relies on the package cache, so it is cheap to call again after binding.
func (b *binder) invokeFunction(args []model.Expression) (*schema.Function, bool) {
	token, _, _, ok := invokeTokenArgument(args)
	if !ok {
		return nil, false
	}
	pkg, _, _, diags := DecomposeToken(token, hcl.Range{})
	if diags.HasErrors() {
		return nil, false
	}
	pkgSchema, err := b.loadPackageSchema(pkg)
	if err != nil {
		return nil, false
	}
	fn, _, found, err := pkgSchema.LookupFunction(token)
	if err != nil || !found {
		return nil, false
	}
	return fn, true
}

// rewritePositionalInvokes normalizes every positional multi-argument invoke in the program into the
// object-argument form, so that program generators only need to handle the object form.
func (b *binder) rewritePositionalInvokes() hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, n := range b.nodes {
		diags = diags.Extend(n.VisitExpressions(nil, b.rewritePositionalInvoke))
	}
	return diags
}

// rewritePositionalInvoke rewrites a single positional multi-argument invoke call into the object
// form: invoke(token, v1, v2) becomes invoke(token, { p1 = v1, p2 = v2 }), preserving any trailing
// invokeOptions argument. Non-invoke expressions and invokes already in object form are returned
// unchanged.
func (b *binder) rewritePositionalInvoke(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	call, ok := expr.(*model.FunctionCallExpression)
	if !ok || call.Name != Invoke || !call.Signature.MultiArgumentInputs {
		return expr, nil
	}
	fn, ok := b.invokeFunction(call.Args)
	if !ok || fn.Inputs == nil {
		return expr, nil
	}
	// A successfully-bound multi-argument invoke is always positional; guard against re-processing a
	// call that is already in object form.
	if b.invokeUsesObjectArgument(fn, call.Args) {
		return expr, nil
	}

	// The positional signature is (token, input₁ … inputₙ, invokeOptions?), and the binder rejects
	// any further arguments. So the arguments after the token are one value per input (with optional
	// inputs possibly null), optionally followed by a single invokeOptions object.
	numInputs := len(fn.Inputs.Properties)
	inputArgs := call.Args[1:]
	var optionsArg model.Expression
	if len(inputArgs) > numInputs {
		contract.Assertf(len(inputArgs) == numInputs+1,
			"positional invoke of %q bound with %d arguments for %d inputs", fn.Token, len(inputArgs), numInputs)
		optionsArg, inputArgs = inputArgs[numInputs], inputArgs[:numInputs]
	}

	// Build the object argument, keyed by the input property names in declaration order.
	items := make([]model.ObjectConsItem, len(inputArgs))
	for i, value := range inputArgs {
		items[i] = model.ObjectConsItem{
			Key:   &model.LiteralValueExpression{Value: cty.StringVal(fn.Inputs.Properties[i].Name)},
			Value: value,
		}
	}
	args := &model.ObjectConsExpression{
		Tokens: syntax.NewObjectConsTokens(len(items)),
		Items:  items,
	}
	// Typecheck computes and caches the object's type from its items. The constructed object has
	// string-literal keys and already-bound values, so it cannot produce diagnostics.
	typecheckDiags := args.Typecheck(false)
	contract.Assertf(len(typecheckDiags) == 0,
		"constructing the invoke argument object for %q produced diagnostics: %v", fn.Token, typecheckDiags)
	annotateObjectProperties(args.Type(), fn.Inputs)

	newArgs := []model.Expression{call.Args[0], args}
	if optionsArg != nil {
		newArgs = append(newArgs, optionsArg)
	}

	sig, err := b.signatureForArgs(fn, args.Type())
	if err != nil {
		// Binding already succeeded against the positional signature; leave the call as-is.
		return expr, nil
	}
	sig.MultiArgumentInputs = fn.MultiArgumentInputs

	return &model.FunctionCallExpression{
		Syntax:      call.Syntax,
		Tokens:      syntax.NewFunctionCallTokens(call.Name, len(newArgs)),
		Name:        call.Name,
		Signature:   sig,
		Args:        newArgs,
		ExpandFinal: call.ExpandFinal,
	}, nil
}
