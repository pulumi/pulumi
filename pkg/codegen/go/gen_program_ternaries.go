// Copyright 2020, Pulumi Corporation.
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
)

type ternaryTemp struct {
	Name  string
	Value *model.ConditionalExpression
}

func (tt *ternaryTemp) Type() model.Type {
	return tt.Value.Type()
}

func (tt *ternaryTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return tt.Type().Traverse(traverser)
}

func (tt *ternaryTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type tempSpiller struct {
	temps   []*ternaryTemp
	count   int
	fnDepth int
}

func (ta *tempSpiller) preVisit(x model.Expression) (model.Expression, hcl.Diagnostics) {
	if _, ok := x.(*model.AnonymousFunctionExpression); ok {
		ta.fnDepth++
	}
	return x, nil
}

func (ta *tempSpiller) spillExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	if _, ok := x.(*model.AnonymousFunctionExpression); ok {
		ta.fnDepth--
		return x, nil
	}

	// Don't spill ternaries inside anonymous functions; they will be handled
	// when the function body is lowered in genAnonymousFunctionExpression.
	if ta.fnDepth > 0 {
		return x, nil
	}

	var temp *ternaryTemp
	switch x := x.(type) {
	case *model.ConditionalExpression:
		temp = &ternaryTemp{
			Name:  fmt.Sprintf("tmp%d", ta.count),
			Value: x,
		}
		ta.temps = append(ta.temps, temp)
		ta.count++
	default:
		return x, nil
	}
	return &model.ScopeTraversalExpression{
		RootName:  temp.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: ""}},
		Parts:     []model.Traversable{temp},
	}, nil
}

func (g *generator) rewriteTernaries(
	x model.Expression,
	spiller *tempSpiller,
) (model.Expression, []*ternaryTemp, hcl.Diagnostics) {
	spiller.temps = nil
	x, diags := model.VisitExpression(x, spiller.preVisit, spiller.spillExpression)

	return x, spiller.temps, diags
}
