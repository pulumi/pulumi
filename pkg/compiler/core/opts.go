// Copyright 2016 Marapongo, Inc. All rights reserved.

package core

import (
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Args are a set of command line arguments supplied to a blueprint.
type Args map[tokens.Name]interface{}

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag diag.Sink // a sink to use for all diagnostics.
	Args Args      // optional blueprint arguments passed at the CLI.
}

// DefaultOptions returns the default set of compiler options.
func DefaultOptions() *Options {
	return &Options{}
}
