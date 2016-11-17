// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/compiler/clouds"
	"github.com/marapongo/mu/pkg/compiler/schedulers"
	"github.com/marapongo/mu/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag   diag.Sink
	Target Target
}

// DefaultOpts returns the default set of compiler options.
func DefaultOpts(pwd string) Options {
	return Options{
		Diag: diag.DefaultSink(pwd),
	}
}

// Target is the target "architecture" we are compiling against.
type Target struct {
	Cloud     clouds.Target
	Scheduler schedulers.Target
}
