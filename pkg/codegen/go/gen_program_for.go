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

package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
)

type forTemp struct {
	Name  string
	Value *model.ForExpression
}

func (ft *forTemp) Type() model.Type {
	return ft.Value.Type()
}

func (ft *forTemp) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return ft.Type().Traverse(traverser)
}

func (ft *forTemp) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

type forSpiller struct {
	temps []*forTemp
	count int
}

func (fs *forSpiller) spillExpression(x model.Expression) (model.Expression, hcl.Diagnostics) {
	f, ok := x.(*model.ForExpression)
	if !ok {
		return x, nil
	}
	// ponytail: only list comprehensions over list collections are lowered;
	// map collections and map results still fall through to genNYI. Extend
	// here when a converter example needs them.
	if f.Key != nil || f.Group {
		return x, nil
	}
	switch model.ResolveOutputs(f.Collection.Type()).(type) {
	case *model.ListType, *model.TupleType:
	default:
		return x, nil
	}
	temp := &forTemp{
		Name:  fmt.Sprintf("forResult%d", fs.count),
		Value: f,
	}
	fs.temps = append(fs.temps, temp)
	fs.count++
	return &model.ScopeTraversalExpression{
		RootName:  temp.Name,
		Traversal: hcl.Traversal{hcl.TraverseRoot{Name: ""}},
		Parts:     []model.Traversable{temp},
	}, nil
}

func (g *generator) rewriteForExpressions(
	x model.Expression,
	spiller *forSpiller,
) (model.Expression, []*forTemp, hcl.Diagnostics) {
	spiller.temps = nil
	x, diags := model.VisitExpression(x, nil, spiller.spillExpression)

	return x, spiller.temps, diags
}
