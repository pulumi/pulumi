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

const PulumiBlockName = "pulumi"

// PulumiBlock represents a program or component level configuration options
type PulumiBlock struct {
	node

	syntax *hclsyntax.Block

	Definition *model.Block

	// RequiredVersionRange is the version range of the engine that the program is compatible with.
	RequiredVersion model.Expression
}

// SyntaxNode returns the syntax node associated with the config variable.
func (p *PulumiBlock) SyntaxNode() hclsyntax.Node {
	return p.syntax
}

func (p *PulumiBlock) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return model.DynamicType, hcl.Diagnostics{cannotTraversePulumiBlock(traverser.SourceRange())}
}

func (p *PulumiBlock) VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics {
	return model.VisitExpressions(p.Definition, pre, post)
}

func (p *PulumiBlock) Name() string {
	return PulumiBlockName // There's only one pulumi block per scope
}

func (p *PulumiBlock) Type() model.Type {
	return model.DynamicType
}
