// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package predef contains a set of tokens and/or symbols that are so-called "predefined"; they map to real abstractions
// defined elsewhere, like the Lumi standard library, but don't actually define them.
package predef

import (
	"github.com/pulumi/lumi/pkg/tokens"
)

var (
	// LumiStdlib is the predefined Lumi standard library.
	LumiStdlib = tokens.NewPackageToken("lumi")
	// LumiStdlibMainModule is the predefined Lumi standard library's main module.
	LumiStdlibMainModule = tokens.NewModuleToken(LumiStdlib, tokens.DefaultModule)
	// LumiStdlibResourceModule is the predefined Lumi standard library's resource module.
	LumiStdlibResourceModule = tokens.NewModuleToken(LumiStdlib, "resource")
	// LumiStdlibResourceClass is the penultimate resource base class, from which all resources derive.
	LumiStdlibResourceClass = tokens.NewTypeToken(LumiStdlibResourceModule, "Resource")
)
