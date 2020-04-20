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

package hcl2

import (
	"os"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v2/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

type binder struct {
	options []model.BindOption
	host    plugin.Host

	packageSchemas map[string]*packageSchema

	tokens syntax.TokenMap
	nodes  []Node
	root   *model.Scope
}

// BindProgram performs semantic analysis on the given set of HCL2 files that represent a single program. The given
// host, if any, is used for loading any resource plugins necessary to extract schema information.
func BindProgram(files []*syntax.File, host plugin.Host, opts ...model.BindOption) (*Program, hcl.Diagnostics, error) {
	if host == nil {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, err
		}
		ctx, err := plugin.NewContext(nil, nil, nil, nil, cwd, nil, nil)
		if err != nil {
			return nil, nil, err
		}
		host = ctx.Host

		defer contract.IgnoreClose(ctx)
	}

	b := &binder{
		options:        opts,
		host:           host,
		tokens:         syntax.NewTokenMapForFiles(files),
		packageSchemas: map[string]*packageSchema{},
		root:           model.NewRootScope(syntax.None),
	}

	// Define null.
	b.root.Define("null", &model.Constant{
		Name:          "null",
		ConstantValue: cty.NullVal(cty.DynamicPseudoType),
	})
	// Define builtin functions.
	for name, fn := range pulumiBuiltins {
		b.root.DefineFunction(name, fn)
	}
	// Define the invoke function.
	b.root.DefineFunction("invoke", model.NewFunction(model.GenericFunctionSignature(b.bindInvokeSignature)))

	var diagnostics hcl.Diagnostics

	// Sort files in source order, then declare all top-level nodes in each.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	for _, f := range files {
		fileDiags, err := b.declareNodes(f)
		if err != nil {
			return nil, nil, err
		}
		diagnostics = append(diagnostics, fileDiags...)
	}

	// Now bind the nodes.
	for _, n := range b.nodes {
		diagnostics = append(diagnostics, b.bindNode(n)...)
	}

	return &Program{
		Nodes:  b.nodes,
		files:  files,
		binder: b,
	}, diagnostics, nil
}

// declareNodes declares all of the top-level nodes in the given file. This invludes config, resources, outputs, and
// locals.
func (b *binder) declareNodes(file *syntax.File) (hcl.Diagnostics, error) {
	var diagnostics hcl.Diagnostics

	// Declare body items in source order.
	for _, item := range model.SourceOrderBody(file.Body) {
		switch item := item.(type) {
		case *hclsyntax.Attribute:
			attrDiags := b.declareNode(item.Name, &LocalVariable{
				Syntax:       item,
				VariableName: item.Name,
			})
			diagnostics = append(diagnostics, attrDiags...)
		case *hclsyntax.Block:
			switch item.Type {
			case "config":
				name, typ := "<unnamed>", model.Type(model.DynamicType)
				switch len(item.Labels) {
				case 1:
					name = item.Labels[0]
				case 2:
					name = item.Labels[0]

					typeExpr, diags := model.BindExpressionText(item.Labels[1], model.TypeScope, item.LabelRanges[1].Start)
					diagnostics = append(diagnostics, diags...)
					typ = typeExpr.Type()
				default:
					diagnostics = append(diagnostics, labelsErrorf(item, "config variables must have exactly one or two labels"))
				}

				// TODO(pdg): check body for valid contents

				diags := b.declareNode(name, &ConfigVariable{
					typ:          typ,
					Syntax:       item,
					VariableName: name,
				})
				diagnostics = append(diagnostics, diags...)
			case "resource":
				if len(item.Labels) != 2 {
					diagnostics = append(diagnostics, labelsErrorf(item, "resource variables must have exactly two labels"))
				}

				tokens, _ := b.tokens.ForNode(item).(*syntax.BlockTokens)
				resource := &Resource{
					Syntax: item,
					Tokens: tokens,
				}
				declareDiags := b.declareNode(item.Labels[0], resource)
				diagnostics = append(diagnostics, declareDiags...)

				if err := b.loadReferencedPackageSchemas(resource); err != nil {
					return nil, err
				}
			case "output":
				name, typ := "<unnamed>", model.Type(model.DynamicType)
				switch len(item.Labels) {
				case 1:
					name = item.Labels[0]
				case 2:
					name = item.Labels[0]

					typeExpr, diags := model.BindExpressionText(item.Labels[1], model.TypeScope, item.LabelRanges[1].Start)
					diagnostics = append(diagnostics, diags...)
					typ = typeExpr.Type()
				default:
					diagnostics = append(diagnostics, labelsErrorf(item, "config variables must have exactly one or two labels"))
				}

				// TODO(pdg): check body for valid contents

				diags := b.declareNode(name, &OutputVariable{
					typ:          typ,
					Syntax:       item,
					VariableName: name,
				})
				diagnostics = append(diagnostics, diags...)
			}
		}
	}

	return diagnostics, nil
}

// declareNode declares a single top-level node. If a node with the same name has already been declared, it returns an
// appropriate diagnostic.
func (b *binder) declareNode(name string, n Node) hcl.Diagnostics {
	if !b.root.Define(name, n) {
		existing, _ := b.root.BindReference(name)
		return hcl.Diagnostics{errorf(existing.SyntaxNode().Range(), "%q already declared", name)}
	}
	b.nodes = append(b.nodes, n)
	return nil
}

func (b *binder) bindExpression(node hclsyntax.Node) (model.Expression, hcl.Diagnostics) {
	return model.BindExpression(node, b.root, b.tokens, b.options...)
}
