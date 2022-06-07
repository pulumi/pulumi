// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

// FunctionSignature represents a possibly-type-polymorphic function signature.
type FunctionSignature interface {
	// GetSignature returns the static signature for the function when invoked with the given arguments.
	GetSignature(arguments []Expression) (StaticFunctionSignature, hcl.Diagnostics)
}

// Parameter represents a single function parameter.
type Parameter struct {
	Name string // The name of the parameter.
	Type Type   // The type of the parameter.
}

// StaticFunctionSignature records the parameters and return type of a function.
type StaticFunctionSignature struct {
	// The function's fixed parameters.
	Parameters []Parameter
	// The function's variadic parameter, if any. Any arguments that follow a function's fixed arguments must be
	// assignable to this parameter.
	VarargsParameter *Parameter
	// The return type of the function.
	ReturnType Type
}

// GetSignature returns the static signature itself.
func (fs StaticFunctionSignature) GetSignature(arguments []Expression) (StaticFunctionSignature, hcl.Diagnostics) {
	return fs, nil
}

// GenericFunctionSignature represents a type-polymorphic function signature. The underlying function will be
// invoked by GenericFunctionSignature.GetSignature to compute the static signature of the function.
type GenericFunctionSignature func(arguments []Expression) (StaticFunctionSignature, hcl.Diagnostics)

// GetSignature returns the static function signature when it is invoked with the given arguments.
func (fs GenericFunctionSignature) GetSignature(arguments []Expression) (StaticFunctionSignature, hcl.Diagnostics) {
	return fs(arguments)
}

// Function represents a function definition.
type Function struct {
	signature FunctionSignature
}

// NewFunction creates a new function with the given signature.
func NewFunction(signature FunctionSignature) *Function {
	return &Function{signature: signature}
}

// SyntaxNode returns the syntax node for the function, which is always syntax.None.
func (f *Function) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// Traverse attempts to traverse the function definition. This will always fail: functions are not traversable.
func (f *Function) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return DynamicType, hcl.Diagnostics{cannotTraverseFunction(traverser.SourceRange())}
}

// GetSignature returns the static signature of the function when it is invoked with the given arguments.
func (f *Function) GetSignature(arguments []Expression) (StaticFunctionSignature, hcl.Diagnostics) {
	return f.signature.GetSignature(arguments)
}
