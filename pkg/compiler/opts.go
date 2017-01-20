// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag diag.Sink              // a sink to use for all diagnostics.
	Args map[string]interface{} // optional blueprint arguments passed at the CLI.
}

// DefaultOptions returns the default set of compiler options.
func DefaultOptions() *Options {
	return &Options{}
}
