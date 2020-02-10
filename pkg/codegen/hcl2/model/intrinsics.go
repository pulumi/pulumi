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
	"github.com/pulumi/pulumi/pkg/util/contract"
)

const (
	// IntrinsicApply is the name of the apply intrinsic.
	IntrinsicApply = "__apply"
)

// NewApplyCall returns a new expression that represents a call to IntrinsicApply.
func NewApplyCall(args []*ScopeTraversalExpression, then *AnonymousFunctionExpression) *FunctionCallExpression {
	signature := FunctionSignature{
		Parameters: make([]Parameter, len(args)+1),
	}

	returnsOutput := false
	exprs := make([]Expression, len(args)+1)
	for i, a := range args {
		exprs[i] = a
		if _, isOutput := a.Type().(*OutputType); isOutput {
			returnsOutput = true
		}
		signature.Parameters[i] = Parameter{
			Name: then.Signature.Parameters[i].Name,
			Type: a.Type(),
		}
	}
	exprs[len(exprs)-1] = then
	signature.Parameters[len(signature.Parameters)-1] = Parameter{
		Name: "then",
		Type: then.Type(),
	}

	if returnsOutput {
		signature.ReturnType = NewOutputType(then.Signature.ReturnType)
	} else {
		signature.ReturnType = NewPromiseType(then.Signature.ReturnType)
	}

	return &FunctionCallExpression{
		Name:      IntrinsicApply,
		Signature: signature,
		Args:      exprs,
	}
}

// ParseApplyCall extracts the apply arguments and the continuation from a call to the apply intrinsic.
func ParseApplyCall(c *FunctionCallExpression) (applyArgs []*ScopeTraversalExpression,
	then *AnonymousFunctionExpression) {

	contract.Assert(c.Name == IntrinsicApply)

	args := make([]*ScopeTraversalExpression, len(c.Args)-1)
	for i, a := range c.Args[:len(args)] {
		args[i] = a.(*ScopeTraversalExpression)
	}

	return args, c.Args[len(c.Args)-1].(*AnonymousFunctionExpression)
}
