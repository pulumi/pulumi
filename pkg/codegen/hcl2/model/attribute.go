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

package model

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
)

// Attribute represents an HCL2 attribute.
type Attribute struct {
	// The syntax node for the attribute, if any.
	Syntax *hclsyntax.Attribute
	// The tokens for the attribute.
	Tokens syntax.AttributeTokens

	// The attribute's name.
	Name string
	// The attribute's value.
	Value Expression
}

// SyntaxNode returns the syntax node of the attribute, and will either return an *hclsyntax.Attribute or syntax.None.
func (a *Attribute) SyntaxNode() hclsyntax.Node {
	return syntaxOrNone(a.Syntax)
}

func (*Attribute) isBodyItem() {}

// BindAttribute binds an HCL2 attribute using the given scope and token map.
func BindAttribute(attribute *hclsyntax.Attribute, scope *Scope, tokens syntax.TokenMap) (*Attribute, hcl.Diagnostics) {
	value, diagnostics := BindExpression(attribute.Expr, scope, tokens)
	attributeTokens, _ := tokens.ForNode(attribute).(syntax.AttributeTokens)
	return &Attribute{
		Syntax: attribute,
		Tokens: attributeTokens,
		Name:   attribute.Name,
		Value:  value,
	}, diagnostics
}
