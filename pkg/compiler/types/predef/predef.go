// Copyright 2016 Pulumi, Inc. All rights reserved.

// Package predef contains a set of tokens and/or symbols that are so-called "predefined"; they map to real abstractions
// defined elsewhere, like the Coconut standard library, but don't actually define them.
package predef

import (
	"github.com/pulumi/coconut/pkg/tokens"
)

var (
	// CocoStdlib is the predefined Coconut standard library.
	CocoStdlib tokens.Package = tokens.NewPackageToken("coconut")
	// CocoStdlibMainModule is the predefined Coconut standard library's main module.
	CocoStdlibMainModule tokens.Module = tokens.NewModuleToken(CocoStdlib, tokens.DefaultModule)
	// CocoStdlibResouceModule is the predefined Coconut standard library's resource module.
	CocoStdlibResourceModule tokens.Module = tokens.NewModuleToken(CocoStdlib, "resource")
	// CocoStdlibResourceClass is the penultimate resource base class, from which all resources derive.
	CocoStdlibResourceClass tokens.Type = tokens.NewTypeToken(CocoStdlibResourceModule, "Resource")
)
