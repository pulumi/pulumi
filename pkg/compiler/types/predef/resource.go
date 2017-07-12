// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package predef contains a set of tokens and/or symbols that are so-called "predefined"; they map to real abstractions
// defined elsewhere, like the Lumi standard library, but don't actually define them.
package predef

import (
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/compiler/types"
)

// IsResourceType returns true if the given type symbol represents the standard resource class.
func IsResourceType(t symbols.Type) bool {
	return types.HasBaseName(t, LumiStdlibResourceClass)
}

// IsLatentResourceProperty returns true if the given type symbol t represents the standard resource class and the
// property type symbol pt represents a latent property of that resource type.  A latent property is one whose value may
// not be known during certain phases like planning; the interpreter attempts to proceed despite this.
func IsLatentResourceProperty(t symbols.Type, pt symbols.Type) bool {
	if IsResourceType(t) {
		if _, isfunc := pt.(*symbols.FunctionType); !isfunc {
			return true
		}
	}
	return false
}
