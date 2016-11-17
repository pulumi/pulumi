// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/compiler/clouds"
	"github.com/marapongo/mu/pkg/compiler/schedulers"
	"github.com/marapongo/mu/pkg/diag"
)

// Options contains all of the settings a user can use to control the compiler's behavior.
type Options struct {
	Diag   diag.Sink // a sink to use for all diagnostics.
	Arch   Arch      // a target cloud architecture.
	Target string    // a named target to generate outputs against.
}

// DefaultOpts returns the default set of compiler options.
func DefaultOpts(pwd string) Options {
	return Options{
		Diag: diag.DefaultSink(pwd),
	}
}

// Arch is the target cloud "architecture" we are compiling against.
type Arch struct {
	Cloud     clouds.Arch
	Scheduler schedulers.Arch
}

func (a Arch) String() string {
	s := clouds.ArchNames[a.Cloud]
	if a.Scheduler != schedulers.NoArch {
		s += ":" + schedulers.ArchNames[a.Scheduler]
	}
	return s
}
