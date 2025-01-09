// Copyright 2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Call is the name of the PCL `call` intrinsic, which can be used to invoke methods on resources.
//
// `call` has the following signature:
//
//	call(self, method, args)
//
// where `self` is the resource to invoke the method on, `method` is the name of the method to invoke, and `args` is an
// object containing the arguments to pass to the method.
const Call = "call"

// bindCallSignature accepts a list of arguments that have been passed to an invocation of the `call` intrinsic and
// attempts to construct a static function signature that can be used to check them, returning diagnostics if this is
// not possible.
func (b *binder) bindCallSignature(args []model.Expression) (model.StaticFunctionSignature, hcl.Diagnostics) {
	// Call must always be passed three arguments -- a receiver, a string indicating the name of the method to invoke, and
	// a set of arguments to pass to the method.
	if len(args) < 3 {
		var r hcl.Range
		if len(args) > 0 {
			r = args[0].SyntaxNode().Range()
		} else {
			r = hcl.Range{}
		}
		return b.zeroCallSignature(), hcl.Diagnostics{
			errorf(r, "call must be passed a receiver, method name, and arguments object"),
		}
	}

	// We've got the minimum number of arguments. We can start therefore by plucking out the receiver. Since any valid
	// receiver must be an object type (all resources are typed as objects whose properties are the resource's output
	// properties), we can query the type's annotations to find the linked resource node. From there we can retrieve the
	// schema and use this later on to resolve the method, etc.
	self := args[0]
	var selfRes *Resource
	if objectType, ok := self.Type().(*model.ObjectType); ok {
		if annotation, ok := model.GetObjectTypeAnnotation[*resourceAnnotation](objectType); ok {
			selfRes = annotation.node
		}
	}

	if selfRes == nil {
		return b.zeroCallSignature(), hcl.Diagnostics{
			errorf(args[0].SyntaxNode().Range(), "call's receiver must be a single resource"),
		}
	}

	// Next we'll grab the method name. This must be a string literal, and can't be e.g. a templated string.
	template, ok := args[1].(*model.TemplateExpression)
	if !ok || len(template.Parts) != 1 {
		return b.zeroCallSignature(), hcl.Diagnostics{
			errorf(args[1].SyntaxNode().Range(), "call's method name must be a string literal"),
		}
	}
	lit, ok := template.Parts[0].(*model.LiteralValueExpression)
	if !ok || model.StringType.ConversionFrom(lit.Type()) == model.NoConversion {
		return b.zeroCallSignature(), hcl.Diagnostics{
			errorf(args[1].SyntaxNode().Range(), "call's method name must be a string literal"),
		}
	}

	// Look up the method in the receiver's method list.
	methodName := lit.Value.AsString()
	var method *schema.Method
	for _, m := range selfRes.Schema.Methods {
		if m.Name == methodName {
			method = m
			break
		}
	}
	if method == nil {
		return b.zeroCallSignature(), hcl.Diagnostics{errorf(
			args[1].SyntaxNode().Range(),
			"resource type %q has no method %q", selfRes.Token, methodName,
		)}
	}

	// Construct a signature and return it.
	sigArgsType, err := b.callArgsType(method.Function)
	if err != nil {
		return b.zeroCallSignature(), hcl.Diagnostics{errorf(args[1].SyntaxNode().Range(), "%v", err.Error())}
	}
	sig := model.StaticFunctionSignature{
		Parameters: []model.Parameter{
			{
				Name: "self",
				Type: self.Type(),
			},
			{
				Name: "method",
				Type: model.StringType,
			},
			{
				Name: "args",
				Type: sigArgsType,
			},
		},
		ReturnType: b.schemaTypeToType(method.Function.ReturnType),
	}

	if argsObject, isObjectExpression := args[1].(*model.ObjectConsExpression); isObjectExpression {
		if method.Function.Inputs != nil {
			annotateObjectProperties(argsObject.Type(), method.Function.Inputs)
		}
	}

	sig.MultiArgumentInputs = method.Function.MultiArgumentInputs
	return sig, nil
}

// zeroCallSignature returns a "zero" signature for the `call` intrinsic that can be returned in the event constructing
// a strongly-typed signature fails. In this signature, the `self` and `args` inputs are dynamically typed, since we
// have presumably been unable to resolve more accurate types for them.
func (b *binder) zeroCallSignature() model.StaticFunctionSignature {
	return model.StaticFunctionSignature{
		Parameters: []model.Parameter{
			{
				Name: "self",
				Type: model.DynamicType,
			},
			{
				Name: "method",
				Type: model.StringType,
			},
			{
				Name: "args",
				Type: model.DynamicType,
			},
		},
		ReturnType: model.DynamicType,
	}
}

// callArgsType accepts a function schema and constructs a model.Type for its arguments. As part of this it both
// validates the presence of the special __self__ argument and removes it from the list of arguments that the `call`
// intrinsic should expect (since the intrinsic takes self as a separate positional argument).
func (b *binder) callArgsType(fn *schema.Function) (model.Type, error) {
	var self *schema.Property
	args := &schema.ObjectType{Properties: []*schema.Property{}}
	for _, input := range fn.Inputs.Properties {
		if input.Name == "__self__" {
			self = input
			continue
		}

		args.Properties = append(args.Properties, input)
	}

	if self == nil {
		return nil, fmt.Errorf("method %q has no __self__ input", fn.Token)
	}

	if len(args.Properties) == 0 {
		return model.NewOptionalType(model.NewObjectType(map[string]model.Type{})), nil
	}

	typ := b.schemaTypeToType(args)
	return typ, nil
}
