// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"fmt"
	"os"

	goerr "github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/compiler/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/environment"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func (eng *Engine) initEnvCmd(name string, pkgarg string) (*envCmdInfo, error) {
	return eng.initEnvCmdName(tokens.QName(name), pkgarg)
}

func (eng *Engine) initEnvCmdName(name tokens.QName, pkgarg string) (*envCmdInfo, error) {
	// If the name is blank, use the default.
	if name == "" {
		name = eng.getCurrentEnv()
	}
	if name == "" {
		return nil, goerr.Errorf("missing environment name (and no default found)")
	}

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
func (eng *Engine) createEnv(name tokens.QName) {
	env := &deploy.Target{Name: name}
	if err := eng.Environment.SaveEnvironment(env, nil); err == nil {
		fmt.Fprintf(eng.Stdout, "Environment '%v' initialized; see `pulumi deploy` to deploy into it\n", name)
		eng.setCurrentEnv(name, false)
	}
}

// newWorkspace creates a new workspace using the current working directory.
func (eng *Engine) newWorkspace() (workspace.W, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return workspace.New(pwd, eng.Diag())
}

// getCurrentEnv reads the current environment.
func (eng *Engine) getCurrentEnv() tokens.QName {
	var name tokens.QName
	w, err := eng.newWorkspace()
	if err == nil {
		name = w.Settings().Env
	}
	if err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
	}
	return name
}

// setCurrentEnv changes the current environment to the given environment name, issuing an error if it doesn't exist.
func (eng *Engine) setCurrentEnv(name tokens.QName, verify bool) {
	if verify {
		if _, _, _, err := eng.Environment.GetEnvironment(name); err != nil {
			return // no environment by this name exists, bail out.
		}
	}

	// Switch the current workspace to that environment.
	w, err := eng.newWorkspace()
	if err == nil {
		w.Settings().Env = name
		err = w.Save()
	}
	if err != nil {
		eng.Diag().Errorf(errors.ErrorIO, err)
	}
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
