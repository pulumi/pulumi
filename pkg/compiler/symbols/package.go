// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package symbols

import (
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Package is a fully bound package symbol.
type Package struct {
	Node         *pack.Package
	Tok          tokens.Package
	Dependencies PackageMap
	Modules      ModuleMap
}

var _ Symbol = (*Package)(nil)

func (node *Package) Name() tokens.Name   { return tokens.Name(node.Node.Name) }
func (node *Package) Token() tokens.Token { return tokens.Token(node.Tok) }
func (node *Package) Special() bool       { return false }
func (node *Package) Tree() diag.Diagable { return node.Node }
func (node *Package) Default() *Module    { return node.Modules[tokens.DefaultModule] }
func (node *Package) String() string      { return string(node.Token()) }

// NewPackageSym returns a new Package symbol with the given node.
func NewPackageSym(node *pack.Package) *Package {
	return &Package{
		Node:         node,
		Tok:          tokens.Package(node.Name),
		Dependencies: make(PackageMap),
		Modules:      make(ModuleMap),
	}
}

// ResolvedPackage contains the bound package symbol plus full URL used to resolve it.
type ResolvedPackage struct {
	Pkg *Package
	URL pack.PackageURL
}

// PackageMap is a map from package token to a package plus its URL information.
type PackageMap map[tokens.PackageName]*ResolvedPackage

// ModuleMap is a map from module token to the associated symbols.
type ModuleMap map[tokens.ModuleName]*Module
