// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type localStackProvider struct {
	decrypter config.ValueDecrypter
}

func (p localStackProvider) GetTarget(name tokens.QName) (*deploy.Target, error) {
	contract.Require(name != "", "name")

	config, err := getConfiguration(name)
	if err != nil {
		return nil, err
	}

	decryptedConfig := make(map[tokens.ModuleMember]string)

	for k, v := range config {
		decrypted, err := v.Value(p.decrypter)
		if err != nil {
			return nil, errors.Wrap(err, "could not decrypt configuration value")
		}
		decryptedConfig[k] = decrypted
	}

	return &deploy.Target{Name: name, Config: decryptedConfig}, nil
}

func (p localStackProvider) GetSnapshot(name tokens.QName) (*deploy.Snapshot, error) {
	contract.Require(name != "", "name")
	_, _, snapshot, _, err := getStack(name)
	return snapshot, err
}

type localStackMutation struct {
	name tokens.QName
}

func (p localStackProvider) BeginMutation(name tokens.QName) (engine.SnapshotMutation, error) {
	return localStackMutation{name: name}, nil
}

func (m localStackMutation) End(snapshot *deploy.Snapshot) error {
	contract.Assert(m.name == snapshot.Namespace)

	name, config, _, _, err := getStack(snapshot.Namespace)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return saveStack(name, config, snapshot)
}

func getStack(name tokens.QName) (tokens.QName,
	map[tokens.ModuleMember]config.Value, *deploy.Snapshot, string, error) {
	workspace, err := newWorkspace()
	if err != nil {
		return "", nil, nil, "", err
	}

	contract.Require(name != "", "name")
	file := workspace.StackPath(name)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		return "", nil, nil, file, errors.Errorf("resource deserialization failed; illegal markup extension: '%v'", ext)
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, nil, file, err
		}
		return "", nil, nil, file, err
	}

	// Unmarshal the contents into a checkpoint structure.
	var checkpoint stack.Checkpoint
	if err = m.Unmarshal(b, &checkpoint); err != nil {
		return "", nil, nil, file, err
	}

	_, config, snapshot, err := stack.DeserializeCheckpoint(&checkpoint)
	if err != nil {
		return "", nil, nil, file, err
	}

	// Ensure the snapshot passes verification before returning it, to catch bugs early.
	if verifyerr := snapshot.VerifyIntegrity(); verifyerr != nil {
		return "", nil, nil, file,
			errors.Wrapf(verifyerr, "snapshot integrity failure; refusing to use it")
	}

	return name, config, snapshot, file, nil
}

func saveStack(name tokens.QName,
	config map[tokens.ModuleMember]config.Value, snap *deploy.Snapshot) error {
	workspace, err := newWorkspace()
	if err != nil {
		return err
	}
	file := workspace.StackPath(name)

	// Make a serializable stack and then use the encoder to encode it.
	m, ext := encoding.Detect(file)
	if m == nil {
		return errors.Errorf("resource serialization failed; illegal markup extension: '%v'", ext)
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	dep := stack.SerializeCheckpoint(name, config, snap)
	b, err := m.Marshal(dep)
	if err != nil {
		return errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// Back up the existing file if it already exists.
	bck := backupTarget(file)

	// Ensure the directory exists.
	if err = os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// And now write out the new snapshot file, overwriting that location.
	if err = ioutil.WriteFile(file, b, 0600); err != nil {
		return errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// And if we are retaining historical checkpoint information, write it out again
	if isTruthy(os.Getenv("PULUMI_RETAIN_CHECKPOINTS")) {
		if err = ioutil.WriteFile(fmt.Sprintf("%v.%v", file, time.Now().UnixNano()), b, 0600); err != nil {
			return errors.Wrap(err, "An IO error occurred during the current operation")
		}
	}

	// Finally, *after* writing the checkpoint, check the integrity.  This is done afterwards so that we write
	// out the checkpoint file since it may contain resource state updates.  But we will warn the user that the
	// file is already written and might be bad.
	if verifyerr := snap.VerifyIntegrity(); verifyerr != nil {
		return errors.Wrapf(verifyerr,
			"snapshot integrity failure; it was already written to %s, but is invalid (a backup is available at %s)",
			file, bck)
	}

	return nil
}

func isTruthy(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}

func removeStack(name tokens.QName) error {
	contract.Require(name != "", "name")

	workspace, err := newWorkspace()
	if err != nil {
		return err
	}

	// Just make a backup of the file and don't write out anything new.
	file := workspace.StackPath(name)
	backupTarget(file)
	return nil
}

// backupTarget makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupTarget(file string) string {
	contract.Require(file != "", "file")
	bck := file + ".bak"
	err := os.Rename(file, bck)
	contract.IgnoreError(err) // ignore errors.
	// IDEA: consider multiple backups (.bak.bak.bak...etc).
	return bck
}
