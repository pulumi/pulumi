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
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type BodyItem interface {
	SyntaxNode() hclsyntax.Node

	isBodyItem()
}

type Block struct {
	Syntax *hclsyntax.Block
	Tokens syntax.BlockTokens

	Type   string
	Labels []string

	Items []BodyItem
}

func (b *Block) SyntaxNode() hclsyntax.Node {
	return b.Syntax
}

func (*Block) isBodyItem() {}

type ScopeSource interface {
	GetScopesForBlock(block *hclsyntax.Block) (ScopeSource, hcl.Diagnostics)
	GetScopeForAttribute(attribute *hclsyntax.Attribute) (*Scope, hcl.Diagnostics)
}

type staticScope struct {
	scope *Scope
}

func (s staticScope) GetScopesForBlock(block *hclsyntax.Block) (ScopeSource, hcl.Diagnostics) {
	return s, nil
}

func (s staticScope) GetScopeForAttribute(attribute *hclsyntax.Attribute) (*Scope, hcl.Diagnostics) {
	return s.scope, nil
}

func StaticScope(scope *Scope) ScopeSource {
	return staticScope{scope: scope}
}

// BindBlock binds an HCL2 block using the given scope source and token map.
func BindBlock(block *hclsyntax.Block, scopes ScopeSource, tokens syntax.TokenMap) (*Block, hcl.Diagnostics) {
	var diagnostics hcl.Diagnostics

	syntaxItems := SourceOrderBody(block.Body)
	items := make([]BodyItem, len(syntaxItems))
	for i, syntaxItem := range syntaxItems {
		var itemDiags hcl.Diagnostics
		switch syntaxItem := syntaxItem.(type) {
		case *hclsyntax.Attribute:
			scope, scopeDiags := scopes.GetScopeForAttribute(syntaxItem)
			diagnostics = append(diagnostics, scopeDiags...)

			items[i], itemDiags = BindAttribute(syntaxItem, scope, tokens)
		case *hclsyntax.Block:
			scopes, scopesDiags := scopes.GetScopesForBlock(syntaxItem)
			diagnostics = append(diagnostics, scopesDiags...)

			items[i], itemDiags = BindBlock(syntaxItem, scopes, tokens)
		default:
			contract.Failf("unexpected syntax item of type %T (%v)", syntaxItem, syntaxItem.Range())
		}
		diagnostics = append(diagnostics, itemDiags...)
	}

	toks, _ := tokens.ForNode(block)
	blockTokens, _ := toks.(syntax.BlockTokens)
	return &Block{
		Syntax: block,
		Tokens: blockTokens,
		Type:   block.Type,
		Labels: block.Labels,
		Items:  items,
	}, diagnostics
}
