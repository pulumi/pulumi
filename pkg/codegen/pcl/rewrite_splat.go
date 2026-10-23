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
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// RewriteSplats rewrites each splat over a value that is not a list, set, or tuple into a splat over a list. HCL
// wraps such a value in a single-element list, or gives an empty list if the value is null, so `x[*].y` becomes
// `[for __value in [x] : __value if __value != null][*].y`. Run it after applies are rewritten, so that the source of
// each splat is not eventual.
func RewriteSplats(expr model.Expression) (model.Expression, hcl.Diagnostics) {
	rewrite := func(expr model.Expression) (model.Expression, hcl.Diagnostics) {
		splat, ok := expr.(*model.SplatExpression)
		if !ok || !splat.SourceIsValue() {
			return expr, nil
		}

		value := &model.Variable{Name: "__value", VariableType: splat.Source.Type()}
		reference := func() model.Expression {
			ref := &model.ScopeTraversalExpression{
				RootName:  value.Name,
				Traversal: hcl.Traversal{hcl.TraverseRoot{Name: value.Name}},
				Parts:     []model.Traversable{value},
			}
			mustTypecheck(ref)
			return ref
		}

		collection := &model.TupleConsExpression{Expressions: []model.Expression{splat.Source}}
		mustTypecheck(collection)
		condition := &model.BinaryOpExpression{
			LeftOperand:  reference(),
			Operation:    hclsyntax.OpNotEqual,
			RightOperand: nullLiteral(),
		}
		mustTypecheck(condition)
		source := &model.ForExpression{
			ValueVariable: value,
			Collection:    collection,
			Value:         reference(),
			Condition:     condition,
		}
		mustTypecheck(source)

		splat.Source = source
		return splat, splat.Typecheck(false)
	}
	return model.VisitExpression(expr, model.IdentityVisitor, rewrite)
}
