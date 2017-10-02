// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

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
