// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag diag.Sink
}

// DefaultOpts returns the default set of compiler options.
func DefaultOpts(pwd string) Options {
	return Options{
		Diag: diag.DefaultSink(pwd),
	}
}
