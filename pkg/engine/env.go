// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"fmt"

	goerr "github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/environment"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func (eng *Engine) initEnvCmdName(name tokens.QName, pkgarg string) (*envCmdInfo, error) {
	contract.Require(name != tokens.QName(""), "name")

	// Read in the deployment information, bailing if an IO error occurs.
	target, snapshot, checkpoint, err := eng.Environment.GetEnvironment(name)
	if err != nil {
		return nil, goerr.Errorf("could not read environment information")
	}

	contract.Assert(target != nil)
	contract.Assert(checkpoint != nil)
	return &envCmdInfo{
		Target:     target,
		Checkpoint: checkpoint,
		Snapshot:   snapshot,
		PackageArg: pkgarg,
	}, nil
}

type envCmdInfo struct {
	Target     *deploy.Target          // the target environment.
	Checkpoint *environment.Checkpoint // the full serialized checkpoint from which this came.
	Snapshot   *deploy.Snapshot        // the environment's latest deployment snapshot
	PackageArg string                  // an optional path to a package to pass to the compiler
}

// createEnv just creates a new empty environment without deploying anything into it.
func (eng *Engine) createEnv(name tokens.QName) error {
	contract.Require(name != tokens.QName(""), "name")

	env := &deploy.Target{Name: name}
	return eng.Environment.SaveEnvironment(env, nil)
}

// removeTarget permanently deletes the environment's information from the local workstation.
func (eng *Engine) removeTarget(env *deploy.Target) error {
	if err := eng.Environment.RemoveEnvironment(env); err != nil {
		return err
	}
	msg := fmt.Sprintf("%sEnvironment '%s' has been removed!%s\n",
		colors.SpecAttention, env.Name, colors.Reset)
	fmt.Fprint(eng.Stdout, colors.ColorizeText(msg))
	return nil
}
