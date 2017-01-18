// Copyright 2016 Marapongo, Inc. All rights reserved.

package options

import (
	"github.com/marapongo/mu/pkg/compiler/backends"
	"github.com/marapongo/mu/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Pwd     string                 // the working directory for the compilation.
	Diag    diag.Sink              // a sink to use for all diagnostics.
	Arch    backends.Arch          // a target cloud architecture.
	Cluster string                 // a named cluster with predefined settings to target.
	Args    map[string]interface{} // optional blueprint arguments passed at the CLI.
}

// Default returns the default set of compiler options.
func Default(pwd string) *Options {
	return &Options{
		Pwd:  pwd,
		Diag: diag.DefaultSink(pwd),
	}
}
