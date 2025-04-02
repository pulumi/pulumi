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
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// hasReferenceTo returns true if the source node has a reference to the target node.
// In other words, if the target node is a dependency of the source node.
func hasReferenceTo(source Node, target Node) bool {
	for _, dep := range source.getDependencies() {
		if dep.Name() == target.Name() {
			return true
		}
	}
	return false
}

// selfReferencingNode returns true if the given node references itself.
func selfReferencingNode(node Node) bool {
	return hasReferenceTo(node, node)
}

// isComponent returns true if the given node is a component.
func isComponent(node Node) bool {
	_, ok := node.(*Component)
	return ok
}

// mutuallyDependantComponents returns true if the given nodes are components that reference each other.
func mutuallyDependantComponents(a Node, b Node) bool {
	return isComponent(a) &&
		isComponent(b) &&
		hasReferenceTo(a, b) &&
		hasReferenceTo(b, a) &&
		a.Name() != b.Name()
}

// bindNode binds a single node in a program. The node's dependencies are bound prior to the node itself; it is an
// error for a node to depend--directly or indirectly--upon itself.
func (b *binder) bindNode(node Node) hcl.Diagnostics {
	if node.isBound() {
		return nil
	}
	if node.isBinding() {
		// We encountered the same node while binding its dependencies, so we have a circular reference.
		// However we need to make an exception for nodes of type Component when
		// - they are not self-referencing
		// - the circular reference is only between other nodes of type Component
		if !isComponent(node) || selfReferencingNode(node) {
			// TODO(pdg): print better trace
			rng := node.SyntaxNode().Range()
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "circular reference",
				Subject:  &rng,
			}}
		}
	}
	node.markBinding()

	var diagnostics hcl.Diagnostics

	deps := b.getDependencies(node)
	node.setDependencies(deps)

	// Bind any nodes this node depends on.
	for _, dep := range deps {
		if dep.isBinding() && mutuallyDependantComponents(node, dep) {
			// We encountered a dependant node that is already being bound
			// usually this is a circular reference, but we need to make an exception for nodes
			// that are components and reference each other (mutually dependant components)
			continue
		}
		diags := b.bindNode(dep)
		diagnostics = append(diagnostics, diags...)
	}

	switch node := node.(type) {
	case *ConfigVariable:
		diags := b.bindConfigVariable(node)
		diagnostics = append(diagnostics, diags...)
	case *LocalVariable:
		diags := b.bindLocalVariable(node)
		diagnostics = append(diagnostics, diags...)
	case *Resource:
		diags := b.bindResource(node)
		diagnostics = append(diagnostics, diags...)
	case *Component:
		diags := b.bindComponent(node)
		diagnostics = append(diagnostics, diags...)
	case *OutputVariable:
		diags := b.bindOutputVariable(node)
		diagnostics = append(diagnostics, diags...)
	case *Condition:
		diags := b.bindCondition(node)
		diagnostics = append(diagnostics, diags...)
	default:
		contract.Failf("unexpected node of type %T (%v)", node, node.SyntaxNode().Range())
	}

	node.markBound()
	return diagnostics
}

// getDependencies returns the dependencies for the given node.
func (b *binder) getDependencies(node Node) []Node {
	depSet := codegen.Set{}
	var deps []Node
	diags := hclsyntax.VisitAll(node.SyntaxNode(), func(node hclsyntax.Node) hcl.Diagnostics {
		var depName string
		switch node := node.(type) {
		case *hclsyntax.FunctionCallExpr:
			// TODO(pdg): function scope binds tighter than "normal" scope
			depName = node.Name
		case *hclsyntax.ScopeTraversalExpr:
			depName = node.Traversal.RootName()
		default:
			return nil
		}

		// Missing reference errors will be issued during expression binding.
		referent, _ := b.root.BindReference(depName)
		if node, ok := referent.(Node); ok && !depSet.Has(node) {
			depSet.Add(node)
			deps = append(deps, node)
		}
		return nil
	})
	contract.Assertf(len(diags) == 0, "unexpected diagnostics: %v", diags)
	return SourceOrderNodes(deps)
}

func expressionIsLiteralNull(expr model.Expression) bool {
	switch expr := expr.(type) {
	case *model.LiteralValueExpression:
		return expr.Value.IsNull()
	default:
		return false
	}
}

func (b *binder) bindConfigVariable(node *ConfigVariable) hcl.Diagnostics {
	block, diagnostics := model.BindBlock(node.syntax, model.StaticScope(b.root), b.tokens, b.options.modelOptions()...)
	if defaultValue, ok := block.Body.Attribute("default"); ok {
		node.DefaultValue = defaultValue.Value
		// when default is null and the type is not already optional
		// turn the config type T into option(T)
		if expressionIsLiteralNull(node.DefaultValue) && !model.IsOptionalType(node.typ) {
			node.typ = model.NewOptionalType(node.typ)
			node.Nullable = true
		}

		if model.InputType(node.typ).ConversionFrom(node.DefaultValue.Type()) == model.NoConversion {
			errorDiagnostic := model.ExprNotConvertible(model.InputType(node.typ), node.DefaultValue)
			diagnostics = append(diagnostics, errorDiagnostic)
		}
	}

	if attr, ok := block.Body.Attribute(LogicalNamePropertyKey); ok {
		logicalName, lDiags := getStringAttrValue(attr)
		if lDiags != nil {
			diagnostics = diagnostics.Append(lDiags)
		} else {
			node.logicalName = logicalName
		}
	}

	if descriptionAttr, ok := block.Body.Attribute("description"); ok {
		description, diags := getStringAttrValue(descriptionAttr)
		if diags != nil {
			diagnostics = diagnostics.Append(diags)
		} else {
			node.Description = description
		}
	}

	if nullableAttr, ok := block.Body.Attribute("nullable"); ok {
		nullable, diags := getBooleanAttributeValue(nullableAttr)
		if diags != nil {
			diagnostics = diagnostics.Append(diags)
		} else {
			node.Nullable = nullable
		}
	}

	node.Definition = block
	return diagnostics
}

func (b *binder) bindLocalVariable(node *LocalVariable) hcl.Diagnostics {
	attr, diagnostics := model.BindAttribute(node.syntax, b.root, b.tokens, b.options.modelOptions()...)
	node.Definition = attr
	return diagnostics
}

func (b *binder) bindOutputVariable(node *OutputVariable) hcl.Diagnostics {
	block, diagnostics := model.BindBlock(node.syntax, model.StaticScope(b.root), b.tokens, b.options.modelOptions()...)

	if logicalNameAttr, ok := block.Body.Attribute(LogicalNamePropertyKey); ok {
		logicalName, lDiags := getStringAttrValue(logicalNameAttr)
		if lDiags != nil {
			diagnostics = diagnostics.Append(lDiags)
		} else {
			node.logicalName = logicalName
		}
	}

	if value, ok := block.Body.Attribute("value"); ok {
		node.Value = value.Value
		if model.InputType(node.typ).ConversionFrom(node.Value.Type()) == model.NoConversion {
			diagnostics = append(diagnostics, model.ExprNotConvertible(model.InputType(node.typ), node.Value))
		}
	}
	node.Definition = block
	return diagnostics
}
