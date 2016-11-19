// Copyright 2016 Marapongo, Inc. All rights reserved.

package backends

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds/aws"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
)

func New(arch Arch, d diag.Sink) core.Backend {
	var be core.Backend

	switch arch.Cloud {
	case clouds.AWSArch:
		be = aws.New(d)
	case clouds.NoArch:
		glog.Fatalf("Expected a valid cloud architecture for backends.New")
	default:
		glog.Fatalf("Cloud architecture '%v' not yet supported", clouds.Names[arch.Cloud])
	}

	switch arch.Scheduler {
	case schedulers.NoArch:
		// Nothing to do.
	default:
		glog.Fatalf("Scheduler architecture '%v' not yet supported", schedulers.Names[arch.Scheduler])
	}

	return be
}
