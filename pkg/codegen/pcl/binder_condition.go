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
	"context"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

func (b *binder) bindCondition(ctx context.Context, node *Condition) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics
	body := node.syntax.Body

	if condAttr, ok := body.Attributes["condition"]; ok {
		expr, diags := model.BindExpression(condAttr.Expr, b.root, b.tokens, b.options.modelOptions()...)
		node.Condition = expr
		diagnostics = append(diagnostics, diags...)
	} else {
		diagnostics = append(diagnostics, errorf(node.syntax.OpenBraceRange,
			"condition block must have a 'condition' attribute"))
	}

	trueBlock := findSubBlock(body, "true")
	falseBlock := findSubBlock(body, "false")

	trueProgram, trueScope, diags := b.bindConditionBranch(ctx, node.syntax, trueBlock)
	diagnostics = append(diagnostics, diags...)
	node.TrueBlock = trueProgram

	if attr, ok := body.Attributes["trueValue"]; ok {
		expr, diags := model.BindExpression(attr.Expr, trueScope, b.tokens, b.options.modelOptions()...)
		node.TrueExpression = expr
		diagnostics = append(diagnostics, diags...)
	}

	falseProgram, falseScope, diags := b.bindConditionBranch(ctx, node.syntax, falseBlock)
	diagnostics = append(diagnostics, diags...)
	node.FalseBlock = falseProgram

	if attr, ok := body.Attributes["falseValue"]; ok {
		expr, diags := model.BindExpression(attr.Expr, falseScope, b.tokens, b.options.modelOptions()...)
		node.FalseExpression = expr
		diagnostics = append(diagnostics, diags...)
	}

	return diagnostics
}

func findSubBlock(body *hclsyntax.Body, name string) *hclsyntax.Block {
	for _, block := range body.Blocks {
		if block.Type == name {
			return block
		}
	}
	return nil
}

// bindConditionBranch declares and binds the nodes inside a condition's true/false sub-block against a child of the
// binder's root scope, then returns a Program containing those nodes and the child scope so the branch's *Value
// expression can be bound against it. The binder's root scope is temporarily swapped so existing declareNode/bindNode
// machinery targets the child scope; nested resources therefore shadow rather than clash with outer names.
func (b *binder) bindConditionBranch(
	ctx context.Context, syntaxNode hclsyntax.Node, block *hclsyntax.Block,
) (*Program, *model.Scope, hcl.Diagnostics) {
	branchScope := b.root.Push(syntaxNode)
	if block == nil {
		return &Program{binder: b}, branchScope, nil
	}

	var diagnostics hcl.Diagnostics
	oldRoot, oldNodes := b.root, b.nodes
	b.root = branchScope
	b.nodes = nil
	defer func() {
		b.root = oldRoot
		b.nodes = oldNodes
	}()

	var declared []Node

	// First pass: declare resources and reads so subsequent binding can see them.
	for _, item := range model.SourceOrderBody(block.Body) {
		blk, ok := item.(*hclsyntax.Block)
		if !ok {
			continue
		}
		switch blk.Type {
		case "resource", "read":
			if len(blk.Labels) != 2 {
				diagnostics = append(diagnostics, labelsErrorf(blk,
					"%s variables must have exactly two labels", blk.Type))
				continue
			}
			var n Node
			switch blk.Type {
			case "resource":
				n = &Resource{syntax: blk}
			case "read":
				n = &ReadResource{syntax: blk}
			}
			declared = append(declared, n)
			diagnostics = append(diagnostics, b.declareNode(blk.Labels[0], n)...)
			if err := b.loadReferencedPackageSchemas(ctx, n); err != nil {
				diagnostics = append(diagnostics, errorf(blk.Range(), "%s", err.Error()))
			}
		}
	}

	// Second pass: locals (attributes on the sub-block body).
	for _, item := range model.SourceOrderBody(block.Body) {
		attr, ok := item.(*hclsyntax.Attribute)
		if !ok {
			continue
		}
		v := &LocalVariable{syntax: attr}
		declared = append(declared, v)
		diagnostics = append(diagnostics, b.declareNode(attr.Name, v)...)
		if err := b.loadReferencedPackageSchemas(ctx, v); err != nil {
			diagnostics = append(diagnostics, errorf(attr.Range(), "%s", err.Error()))
		}
	}

	for _, n := range declared {
		diagnostics = append(diagnostics, b.bindNode(ctx, n)...)
	}

	return &Program{Nodes: declared, binder: b}, branchScope, diagnostics
}
