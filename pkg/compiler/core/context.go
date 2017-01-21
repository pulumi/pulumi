// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
)

// Context is a bag of state common throughout all compiler passes.
type Context struct {
	Path    string             // the root directory.
	Diag    diag.Sink          // the diagnostics sink to use.
	Pkgs    symbols.PackageMap // all imported/bound packages.
	Currpkg *symbols.Package   // the current package being compiled.
}

// NewContext creates a new context with the given state.
func NewContext(path string, d diag.Sink) *Context {
	return &Context{
		Path: path,
		Diag: d,
		Pkgs: make(symbols.PackageMap),
	}
}
