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
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

func (b *binder) bindCondition(node *Condition) hcl.Diagnostics {
	// We need to subbind the true and false blocks to get the declared resources and variables. Then we need
	// to bind the true and false expressions using the scopes from the blocks.

	// Bind the true block.

	block, diagnostics := model.BindBlock(node.syntax, model.StaticScope(b.root), b.tokens, b.options.modelOptions()...)
	if block == nil {
		return diagnostics
	}

	if trueExpr, ok := block.Body.Attribute("trueValue"); ok {
		node.TrueExpression = trueExpr.Value
	}
	if falseExpr, ok := block.Body.Attribute("falseValue"); ok {
		node.FalseExpression = falseExpr.Value
	}

	return diagnostics
}
