// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Context is a bag of state common throughout all compiler passes.
type Context struct {
	Path string     // the root directory.
	Diag diag.Sink  // the diagnostics sink to use.
	Pkgs PackageMap // all imported/bound packages.
}

// PackageMap is a mapping of package token to fully bound package symbol.
type PackageMap map[tokens.Package]*symbols.Package

// NewContext creates a new context with the given state.
func NewContext(path string, d diag.Sink) *Context {
	return &Context{path, d, make(PackageMap)}
}
