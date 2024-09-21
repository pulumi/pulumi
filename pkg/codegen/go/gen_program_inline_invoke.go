// Copyright 2023-2024, Pulumi Corporation.
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

package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
)

type inlineInvokeTemp struct {
	Name  string
	Value *model.FunctionCallExpression
}

func (temp *inlineInvokeTemp) Type() model.Type {
	return temp.Value.Type()
}

func (temp *inlineInvokeTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return temp.Type().Traverse(traverser)
}

func (temp *inlineInvokeTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type inlineInvokeSpiller struct {
	temps []*inlineInvokeTemp
	count int
}

func (spiller *inlineInvokeSpiller) spillExpression(
	expr model.Expression,
	g *generator,
) (model.Expression, hcl.Diagnostics) {
	switch expr := expr.(type) {
	case *model.FunctionCallExpression:
		isOutputInvoke, _, _ := pcl.RecognizeOutputVersionedInvoke(expr)
		if expr.Name == "invoke" && !isOutputInvoke {
			// non-output-versioned invokes are the only ones that need to be converted
			// because their return type Tuple(InvokeResult, error)
			_, _, fn, _ := g.functionName(expr.Args[0])
			tempName := fmt.Sprintf("invoke%s%d", fn, spiller.count)
			if spiller.count == 0 {
				tempName = "invoke" + fn
			}
			temp := &inlineInvokeTemp{
				Name:  tempName,
				Value: expr,
			}
			spiller.temps = append(spiller.temps, temp)

			spiller.count++
			reference := model.VariableReference(&model.Variable{
				Name: temp.Name,
			})

			return reference, nil
		}

		return expr, nil
	default:
		return expr, nil
	}
}

func (g *generator) rewriteInlineInvokes(x model.Expression) (model.Expression, []*inlineInvokeTemp) {
	spiller := g.inlineInvokeSpiller
	spiller.temps = nil
	spill := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		return spiller.spillExpression(expr, g)
	}
	expr, _ := model.VisitExpression(x, spill, spill)
	return expr, spiller.temps
}
