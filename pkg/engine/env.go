// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func (eng *Engine) planContextFromEnvironment(name tokens.QName, pkgarg string) (*planContext, error) {
	contract.Require(name != tokens.QName(""), "name")

	// Read in the deployment information, bailing if an IO error occurs.
	target, err := eng.Targets.GetTarget(name)
	if err != nil {
		return nil, errors.Wrap(err, "could not read target information")
	}

	snapshot, err := eng.Snapshots.GetSnapshot(name)
	if err != nil {
		return nil, errors.Wrap(err, "could not read snapshot information")
	}

	contract.Assert(target != nil)

	return &planContext{
		Target:     target,
		Snapshot:   snapshot,
		PackageArg: pkgarg,
	}, nil
}

type planContext struct {
	Target     *deploy.Target   // the target environment.
	Snapshot   *deploy.Snapshot // the environment's latest deployment snapshot
	PackageArg string           // an optional path to a package to pass to the compiler
}
