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
	"os"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
)

type binder struct {
	host plugin.Host

	packageSchemas map[string]*packageSchema

	tokens syntax.TokenMap
	stack  []hclsyntax.Node
	root   *Scope
}

// BindProgram performs semantic analysis on the given set of HCL2 files that represent a single program. The given
// host, if any, is used for loading any resource plugins necessary to extract schema information.
func BindProgram(files []*syntax.File, host plugin.Host) (*Program, hcl.Diagnostics, error) {
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
	}

	b := &binder{
		host:           host,
		tokens:         syntax.NewTokenMapForFiles(files),
		packageSchemas: map[string]*packageSchema{},
		root:           NewRootScope(syntax.None),
	}

	var diagnostics hcl.Diagnostics

	// Sort files in source order, then declare all top-level nodes in each.
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	for _, f := range files {
		diagnostics = append(diagnostics, b.declareNodes(f)...)
	}

	// Sort nodes in source order so downstream operations are deterministic.
	var nodes []Node
	for _, n := range b.root.defs {
		nodes = append(nodes, n.(Node))
	}
	SourceOrderNodes(nodes)

	// Load referenced package schemas.
	for _, n := range nodes {
		if err := b.loadReferencedPackageSchemas(n); err != nil {
			return nil, nil, err
		}
	}

	// Now bind the nodes.
	for _, n := range nodes {
		diagnostics = append(diagnostics, b.bindNode(n)...)
	}

	return &Program{
		Nodes:  nodes,
		files:  files,
		binder: b,
	}, diagnostics, nil
}

// declareNodes declares all of the top-level nodes in the given file. This invludes config, resources, outputs, and
// locals.
func (b *binder) declareNodes(file *syntax.File) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	// Declare blocks (config, resources, outputs), then attributes (locals)
	for _, block := range SourceOrderBlocks(file.Body.Blocks) {
		switch block.Type {
		case "config":
			if len(block.Labels) != 0 {
				diagnostics = append(diagnostics, labelsErrorf(block, "config blocks do not support labels"))
			}

			for _, attr := range SourceOrderAttributes(block.Body.Attributes) {
				diagnostics = append(diagnostics, errorf(attr.Range(), "unsupported attribute %q in config block", attr.Name))
			}

			for _, variable := range SourceOrderBlocks(block.Body.Blocks) {
				if len(variable.Labels) > 1 {
					diagnostics = append(diagnostics, labelsErrorf(block, "config variables must have no more than one label"))
				}

				diagnostics = append(diagnostics, b.declareNode(variable.Type, &ConfigVariable{
					Syntax: variable,
				})...)
			}
		case "resource":
			if len(block.Labels) != 2 {
				diagnostics = append(diagnostics, labelsErrorf(block, "resource variables must have exactly two labels"))
			}

			var tokens syntax.BlockTokens
			if ts, ok := b.tokens.ForNode(block); ok {
				tokens, _ = ts.(syntax.BlockTokens)
			}

			diagnostics = append(diagnostics, b.declareNode(block.Labels[0], &Resource{
				Syntax: block,
				Tokens: tokens,
			})...)
		case "outputs":
			if len(block.Labels) != 0 {
				diagnostics = append(diagnostics, labelsErrorf(block, "outputs blocks do not support labels"))
			}

			for _, attr := range SourceOrderAttributes(block.Body.Attributes) {
				diagnostics = append(diagnostics, errorf(attr.Range(), "unsupported attribute %q in outputs block", attr.Name))
			}

			for _, variable := range SourceOrderBlocks(block.Body.Blocks) {
				if len(variable.Labels) > 1 {
					diagnostics = append(diagnostics, labelsErrorf(block, "output variables must have no more than one label"))
				}

				diagnostics = append(diagnostics, b.declareNode(variable.Type, &OutputVariable{
					Syntax: variable,
				})...)
			}
		}
	}

	for _, attr := range SourceOrderAttributes(file.Body.Attributes) {
		diagnostics = append(diagnostics, b.declareNode(attr.Name, &LocalVariable{
			Syntax: attr,
		})...)
	}

	return diagnostics
}

// declareNode declares a single top-level node. If a node with the same name has already been declared, it returns an
// appropriate diagnostic.
func (b *binder) declareNode(name string, n Node) hcl.Diagnostics {
	if !b.root.Define(name, n) {
		existing, _ := b.root.BindReference(name)
		return hcl.Diagnostics{errorf(existing.SyntaxNode().Range(), "%q already declared", name)}
	}
	return nil
}

func (b *binder) bindExpression(node hclsyntax.Node) (Expression, hcl.Diagnostics) {
	return BindExpression(node, b.root, b.tokens)
}
