// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag        diag.Sink     // a sink to use for all diagnostics.
	SkipCodegen bool          // if true, no code-generation phases run.
	Arch        backends.Arch // a target cloud architecture.
	Target      string        // a named target to generate outputs against.
}

// DefaultOpts returns the default set of compiler options.
func DefaultOpts(pwd string) Options {
	return Options{
		Diag: diag.DefaultSink(pwd),
	}
}
