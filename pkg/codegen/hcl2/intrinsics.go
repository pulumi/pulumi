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

package hcl2

import (
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

const (
	// IntrinsicApply is the name of the apply intrinsic.
	IntrinsicApply = "__apply"
)

func isOutput(t model.Type) bool {
	switch t := t.(type) {
	case *model.OutputType:
		return true
	case *model.UnionType:
		for _, t := range t.ElementTypes {
			if _, isOutput := t.(*model.OutputType); isOutput {
				return true
			}
		}
	}
	return false
}

// NewApplyCall returns a new expression that represents a call to IntrinsicApply.
func NewApplyCall(args []*model.ScopeTraversalExpression,
	then *model.AnonymousFunctionExpression) *model.FunctionCallExpression {

	signature := model.StaticFunctionSignature{
		Parameters: make([]model.Parameter, len(args)+1),
	}

	returnsOutput := false
	exprs := make([]model.Expression, len(args)+1)
	for i, a := range args {
		exprs[i] = a
		if isOutput := isOutput(a.Type()); isOutput {
			returnsOutput = true
		}
		signature.Parameters[i] = model.Parameter{
			Name: then.Signature.Parameters[i].Name,
			Type: a.Type(),
		}
	}
	exprs[len(exprs)-1] = then
	signature.Parameters[len(signature.Parameters)-1] = model.Parameter{
		Name: "then",
		Type: then.Type(),
	}

	if returnsOutput {
		signature.ReturnType = model.NewOutputType(then.Signature.ReturnType)
	} else {
		signature.ReturnType = model.NewPromiseType(then.Signature.ReturnType)
	}

	return &model.FunctionCallExpression{
		Name:      IntrinsicApply,
		Signature: signature,
		Args:      exprs,
	}
}

// ParseApplyCall extracts the apply arguments and the continuation from a call to the apply intrinsic.
func ParseApplyCall(c *model.FunctionCallExpression) (applyArgs []*model.ScopeTraversalExpression,
	then *model.AnonymousFunctionExpression) {

	contract.Assert(c.Name == IntrinsicApply)

	args := make([]*model.ScopeTraversalExpression, len(c.Args)-1)
	for i, a := range c.Args[:len(args)] {
		args[i] = a.(*model.ScopeTraversalExpression)
	}

	return args, c.Args[len(c.Args)-1].(*model.AnonymousFunctionExpression)
}
