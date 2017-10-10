// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/environment"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type EnvironmentInfo struct {
	Name       tokens.QName
	Snapshot   *deploy.Snapshot
	Checkpoint *environment.Checkpoint
}

func (eng *Engine) GetEnvironmentInfo(envName tokens.QName) (EnvironmentInfo, error) {
	contract.Require(envName != tokens.QName(""), "envName")

	_, snapshot, checkpoint, err := eng.Environment.GetEnvironment(envName)
	if err != nil {
		return EnvironmentInfo{}, err
	}

	return EnvironmentInfo{Name: envName,
		Snapshot:   snapshot,
		Checkpoint: checkpoint,
	}, nil
}
