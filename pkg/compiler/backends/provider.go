// Copyright 2016 Marapongo, Inc. All rights reserved.

package backends

import (
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds/aws"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/util"
)

func New(arch Arch, d diag.Sink) core.Backend {
	var be core.Backend

	switch arch.Cloud {
	case clouds.AWSArch:
		be = aws.New(d)
	case clouds.NoArch:
		util.FailM("Expected a valid cloud architecture for backends.New")
	default:
		util.FailMF("Cloud architecture '%v' not yet supported", clouds.Names[arch.Cloud])
	}

	switch arch.Scheduler {
	case schedulers.NoArch:
		// Nothing to do.
	default:
		util.FailMF("Scheduler architecture '%v' not yet supported", schedulers.Names[arch.Scheduler])
	}

	return be
}
