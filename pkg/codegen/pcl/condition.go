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

package pcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// Condition represents a conditional part of the program.
type Condition struct {
	node

	syntax *hclsyntax.Block

	// the name of block declaration
	name string

	TrueExpression  model.Expression
	FalseExpression model.Expression

	TrueBlock  Program
	FalseBlock Program
}

// SyntaxNode returns the syntax node associated with the component.
func (c *Condition) SyntaxNode() hclsyntax.Node {
	return c.syntax
}

func (c *Condition) Name() string {
	return c.name
}

func (c *Condition) Type() model.Type {
	return c.TrueExpression.Type()
}

func (c *Condition) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	panic("not implemented")
}

func (c *Condition) VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics {
	panic("not implemented")
}
