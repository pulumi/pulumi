// Copyright 2016 Marapongo, Inc. All rights reserved.

package awsecs

import (
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/compiler/backends/schedulers"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

// New returns a fresh instance of an AWS ECS Scheduler implementation.  This requires an AWS backend, since ECS only
// works in an AWS environment.  The code-gen outputs are idiomatic ECS, using Tasks and so on.
//
// For more details, see https://github.com/marapongo/mu/blob/master/docs/targets.md#aws-ec2-container-service-ecs
func New(d diag.Sink, be clouds.Cloud) schedulers.Scheduler {
	arch := be.Arch()
	if arch != clouds.AWS {
		d.Errorf(errors.ErrorIllegalCloudSchedulerCombination, clouds.Names[arch], "awsecs")
	}
	return &awsECSScheduler{d: d, be: be}
}

type awsECSScheduler struct {
	schedulers.Scheduler
	d  diag.Sink
	be clouds.Cloud // an AWS cloud provider.
}

func (s *awsECSScheduler) Arch() schedulers.Arch {
	return schedulers.AWSECS
}

func (s *awsECSScheduler) Diag() diag.Sink {
	return s.d
}

func (s *awsECSScheduler) CodeGen(comp core.Compiland) {
	// For now, simply rely on the underlying AWS code-generator to do its thing.
	s.be.CodeGen(comp)
}
