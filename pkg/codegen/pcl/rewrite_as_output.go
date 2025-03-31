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
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// RewriteAsOutputs rewrites the given expression to replace any references to
// the given variables with calls to `toOutput`. This is used in, for example,
// a ForExpression inside a component to ensure all calls are correctly lifted
// with apply.
//
// In typescript, for instance, all inputs to a component resource are of type
// pulumi.Input<T> which can be either T, pulumi.Output<T> or Promise<T>. This
// rewriter will convert all the pulumi.Input's to pulumi.Output so codegen
// code is always valid.
func RewriteAsOutputs(x model.Expression, variablesToRewrite []string) (model.Expression, hcl.Diagnostics) {
	if x == nil {
		return x, nil
	}

	var diags hcl.Diagnostics
	switch x := x.(type) {
	case *model.AnonymousFunctionExpression:
		// If the function shadows a variable it is not an output so should not have it's type rewritten.
		vars := make([]string, 0, len(variablesToRewrite))
		for _, v := range variablesToRewrite {
			if !slices.ContainsFunc(x.Parameters, func(param *model.Variable) bool {
				return param.Name == v
			}) {
				vars = append(vars, v)
			}
		}

		newBody, bDiags := RewriteAsOutputs(x.Body, vars)
		x.Body = newBody
		diags = append(diags, bDiags...)
	case *model.BinaryOpExpression:
		lo, lDiags := RewriteAsOutputs(x.LeftOperand, variablesToRewrite)
		x.LeftOperand = lo
		diags = append(diags, lDiags...)
		ro, rDiags := RewriteAsOutputs(x.RightOperand, variablesToRewrite)
		x.RightOperand = ro
		diags = append(diags, rDiags...)
	case *model.ScopeTraversalExpression:
		if slices.Contains(variablesToRewrite, x.RootName) {
			return NewToOutputCall(x), nil
		}
	case *model.ConditionalExpression:
		cond, cDiags := RewriteAsOutputs(x.Condition, variablesToRewrite)
		diags = append(diags, cDiags...)
		x.Condition = cond
		tru, tDiags := RewriteAsOutputs(x.TrueResult, variablesToRewrite)
		diags = append(diags, tDiags...)
		x.TrueResult = tru
		fal, fDiags := RewriteAsOutputs(x.FalseResult, variablesToRewrite)
		diags = append(diags, fDiags...)
		x.FalseResult = fal
	case *model.ForExpression:
		col, cDiags := RewriteAsOutputs(x.Collection, variablesToRewrite)
		diags = append(diags, cDiags...)
		x.Collection = col
		cond, condDiags := RewriteAsOutputs(x.Condition, variablesToRewrite)
		diags = append(diags, condDiags...)
		x.Condition = cond
	case *model.FunctionCallExpression:
		args := x.Args
		x.Args = make([]model.Expression, 0, len(args))
		for _, arg := range args {
			arg, d := RewriteAsOutputs(arg, variablesToRewrite)
			diags = append(diags, d...)
			x.Args = append(x.Args, arg)
		}
	case *model.IndexExpression:
		coll, cDiags := RewriteAsOutputs(x.Collection, variablesToRewrite)
		diags = append(diags, cDiags...)
		x.Collection = coll
		key, kDiags := RewriteAsOutputs(x.Key, variablesToRewrite)
		diags = append(diags, kDiags...)
		x.Key = key
	case *model.ObjectConsExpression:
		for _, item := range x.Items {
			key, kDiags := RewriteAsOutputs(item.Key, variablesToRewrite)
			item.Key = key
			diags = append(diags, kDiags...)
			value, vDiags := RewriteAsOutputs(item.Value, variablesToRewrite)
			item.Value = value
			diags = append(diags, vDiags...)
		}
	case *model.TupleConsExpression:
		exprs := x.Expressions
		x.Expressions = make([]model.Expression, 0, len(exprs))
		for _, arg := range exprs {
			arg, d := RewriteAsOutputs(arg, variablesToRewrite)
			diags = append(diags, d...)
			x.Expressions = append(x.Expressions, arg)
		}
	case *model.UnaryOpExpression:
		op, d := RewriteAsOutputs(x.Operand, variablesToRewrite)
		diags = append(diags, d...)
		x.Operand = op
	}

	typecheckDiags := x.Typecheck(false)
	diags = append(diags, typecheckDiags...)

	return x, diags
}
