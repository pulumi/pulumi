// Copyright 2016 Marapongo, Inc. All rights reserved.

package backends

import (
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds/aws"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers/awsecs"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/util"
)

func New(arch Arch, d diag.Sink) core.Backend {
	// Bind to the cloud provider.
	var cloud clouds.Cloud
	switch arch.Cloud {
	case clouds.AWS:
		// TODO(joe): come up with a way to get options from CLI/workspace/etc. to here.
		cloud = aws.New(d, aws.Options{})
	case clouds.None:
		util.FailM("Expected a non-None cloud architecture")
	default:
		util.FailMF("Cloud architecture '%v' not yet supported", clouds.Names[arch.Cloud])
	}
	util.Assert(cloud != nil)
	util.Assert(cloud.Arch() == arch.Cloud)

	// Set the backend to the cloud provider.
	var be core.Backend = cloud

	// Now bind to the scheduler, if any, wrapping the cloud and replacing the backend.
	var scheduler schedulers.Scheduler
	switch arch.Scheduler {
	case schedulers.None:
		// Nothing to do.
	case schedulers.AWSECS:
		scheduler = awsecs.New(d, cloud)
	default:
		util.FailMF("Scheduler architecture '%v' not yet supported", schedulers.Names[arch.Scheduler])
	}
	if scheduler != nil {
		util.Assert(scheduler.Arch() == arch.Scheduler)
		be = scheduler
	}

	return be
}
