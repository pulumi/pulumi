// Copyright 2016 Marapongo, Inc. All rights reserved.

// Package predef contains a set of tokens and/or symbols that are so-called "predefined"; they map to real abstractions
// defined elsewhere, like the Mu standard library, but don't actually define them.
package predef

import (
	"github.com/marapongo/mu/pkg/tokens"
)

var (
	// MuPackage is the predefined Mu standard library.
	MuPackage tokens.Package = tokens.NewPackageToken("mu")
	// MuDefaultModule is the predefined Mu standard library's main module.
	MuDefaultModule tokens.Module = tokens.NewModuleToken(MuPackage, tokens.DefaultModule)
	// MuResourceClass is the penultimate resource base class, from which all resources derive.
	MuResourceClass tokens.Type = tokens.NewTypeToken(MuDefaultModule, "Resource")
)
