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

type Context struct {
	node

	syntax *hclsyntax.Block

	ID string

	// The definition of the default provider.
	Definition *model.Block

	Nodes []Node

	Providers []string
}

func (c *Context) SyntaxNode() hclsyntax.Node {
	return c.syntax
}

func (c *Context) Name() string {
	return c.ID
}

func (c *Context) Type() model.Type {
	return model.NoneType
}

func (c *Context) VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics {
	return model.VisitExpressions(c.Definition, pre, post)
}

func (c *Context) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return model.DynamicType.Traverse(traverser)
}

func (c *Context) declareNode(name string, n Node) hcl.Diagnostics {
	// TODO: check for nodes that are already declared
	c.Nodes = append(c.Nodes, n)
	return nil
}
