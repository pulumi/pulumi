// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/environment"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type localEnvProvider struct{}

func (p localEnvProvider) GetTarget(name tokens.QName) (*deploy.Target, error) {
	contract.Require(name != "", "name")

	target, _, err := getEnvironment(name)

	return target, err
}

func (p localEnvProvider) GetSnapshot(name tokens.QName) (*deploy.Snapshot, error) {
	contract.Require(name != "", "name")

	_, snapshot, err := getEnvironment(name)

	return snapshot, err
}

func (p localEnvProvider) SaveSnapshot(snapshot *deploy.Snapshot) error {
	target, _, err := getEnvironment(snapshot.Namespace)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if target == nil {
		target = &deploy.Target{Name: snapshot.Namespace}
	}

	return saveEnvironment(target, snapshot)
}

func getEnvironment(name tokens.QName) (*deploy.Target, *deploy.Snapshot, error) {
	contract.Require(name != "", "name")
	file := workspace.EnvPath(name)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		return nil, nil, errors.Errorf("resource deserialization failed; illegal markup extension: '%v'", ext)
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, err
		}
		return nil, nil, err
	}

	// Unmarshal the contents into a checkpoint structure.
	var checkpoint environment.Checkpoint
	if err = m.Unmarshal(b, &checkpoint); err != nil {
		return nil, nil, err
	}

	target, snapshot := environment.DeserializeCheckpoint(&checkpoint)
	contract.Assert(target != nil)
	return target, snapshot, nil
}

func saveEnvironment(env *deploy.Target, snap *deploy.Snapshot) error {
	contract.Require(env != nil, "env")
	file := workspace.EnvPath(env.Name)

	// Make a serializable environment and then use the encoder to encode it.
	m, ext := encoding.Detect(file)
	if m == nil {
		return errors.Errorf("resource serialization failed; illegal markup extension: '%v'", ext)
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	dep := environment.SerializeCheckpoint(env, snap)
	b, err := m.Marshal(dep)
	if err != nil {
		return errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// Back up the existing file if it already exists.
	backupTarget(file)

	// Ensure the directory exists.
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// And now write out the new snapshot file, overwriting that location.
	if err = ioutil.WriteFile(file, b, 0600); err != nil {
		return errors.Wrap(err, "An IO error occurred during the current operation")
	}

	return nil
}

func removeEnvironment(env *deploy.Target) error {
	contract.Require(env != nil, "env")
	// Just make a backup of the file and don't write out anything new.
	file := workspace.EnvPath(env.Name)
	backupTarget(file)
	return nil
}

// backupTarget makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupTarget(file string) {
	contract.Require(file != "", "file")
	err := os.Rename(file, file+".bak")
	contract.IgnoreError(err) // ignore errors.
	// IDEA: consider multiple backups (.bak.bak.bak...etc).
}
