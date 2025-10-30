package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// FunctionSignature represents a possibly-type-polymorphic function signature.
type FunctionSignature = model.FunctionSignature

// Parameter represents a single function parameter.
type Parameter = model.Parameter

// StaticFunctionSignature records the parameters and return type of a function.
type StaticFunctionSignature = model.StaticFunctionSignature

// GenericFunctionSignature represents a type-polymorphic function signature. The underlying function will be
// invoked by GenericFunctionSignature.GetSignature to compute the static signature of the function.
type GenericFunctionSignature = model.GenericFunctionSignature

// Function represents a function definition.
type Function = model.Function

// NewFunction creates a new function with the given signature.
func NewFunction(signature FunctionSignature) *Function {
	return model.NewFunction(signature)
}

