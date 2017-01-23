// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
)

// Package is a fully bound package symbol.
type Package struct {
	Node         *pack.Package
	Dependencies PackageMap
	Modules      ModuleMap
}

var _ Symbol = (*Package)(nil)

func (node *Package) symbol()             {}
func (node *Package) Name() tokens.Name   { return tokens.Name(node.Node.Name) }
func (node *Package) Token() tokens.Token { return tokens.Token(node.Node.Name) }
func (node *Package) Tree() diag.Diagable { return node.Node }
func (node *Package) String() string      { return string(node.Name()) }

// NewPackageSym returns a new Package symbol with the given node.
func NewPackageSym(node *pack.Package) *Package {
	return &Package{
		Node:         node,
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
