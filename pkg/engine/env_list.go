// Copyright 2017, Pulumi Corporation.  All rights reserved.

package engine

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/environment"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type EnvironmentInfo struct {
	Name       string
	Snapshot   *deploy.Snapshot
	Checkpoint *environment.Checkpoint
	IsCurrent  bool
}

func (eng *Engine) GetEnvironments() ([]EnvironmentInfo, error) {
	var envs []EnvironmentInfo

	// Read the environment directory.
	path := workspace.EnvPath("")
	files, err := ioutil.ReadDir(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Errorf("could not read environments: %v", err)
	}

	curr := eng.getCurrentEnv()
	for _, file := range files {
		// Ignore directories.
		if file.IsDir() {
			continue
		}

		// Skip files without valid extensions (e.g., *.bak files).
		envfn := file.Name()
		ext := filepath.Ext(envfn)
		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this environment's information.
		name := tokens.QName(envfn[:len(envfn)-len(ext)])
		target, snapshot, checkpoint, err := eng.Environment.GetEnvironment(name)
		if err != nil {
			continue // failure reading the environment information.
		}

		envs = append(envs, EnvironmentInfo{Name: target.Name.String(),
			Snapshot:   snapshot,
			Checkpoint: checkpoint,
			IsCurrent:  (curr == target.Name)})
	}

	return envs, nil
}
