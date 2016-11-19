// Copyright 2016 Marapongo, Inc. All rights reserved.

package backends

import (
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
)

// Arch is the target cloud "architecture" we are compiling against.
type Arch struct {
	Cloud     clouds.Arch
	Scheduler schedulers.Arch
}

func (a Arch) String() string {
	s := clouds.Names[a.Cloud]
	if a.Scheduler != schedulers.NoArch {
		s += ":" + schedulers.Names[a.Scheduler]
	}
	return s
}
