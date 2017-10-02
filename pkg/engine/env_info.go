// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/pkg/errors"
)

func (eng *Engine) GetCurrentEnvName() tokens.QName {
	return eng.getCurrentEnv()
}

func (eng *Engine) GetEnvironmentInfo(envName tokens.QName) (EnvironmentInfo, error) {
	curr := envName
	if curr == "" {
		curr = eng.getCurrentEnv()
	}
	if curr == "" {
		return EnvironmentInfo{}, errors.New("no current environment; either `pulumi env init` or `pulumi env select` one")
	}

	target, snapshot, checkpoint, err := eng.Environment.GetEnvironment(curr)
	if err != nil {
		return EnvironmentInfo{}, err
	}

	return EnvironmentInfo{Name: curr,
		Snapshot:   snapshot,
		Checkpoint: checkpoint,
		IsCurrent:  target.Name == curr,
	}, nil
}
