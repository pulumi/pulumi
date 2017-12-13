// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking bool

type localStackProvider struct {
	d         diag.Sink
	decrypter config.Decrypter
}

func newLocalStackProvider(d diag.Sink, decrypter config.Decrypter) *localStackProvider {
	return &localStackProvider{d: d, decrypter: decrypter}
}

func (p *localStackProvider) GetTarget(name tokens.QName) (*deploy.Target, error) {
	contract.Require(name != "", "name")

	config, err := state.Configuration(p.d, name)
	if err != nil {
		return nil, err
	}

	return &deploy.Target{Name: name, Config: config, Decrypter: p.decrypter}, nil
}

func (p *localStackProvider) GetSnapshot(name tokens.QName) (*deploy.Snapshot, error) {
	contract.Require(name != "", "name")
	_, snapshot, _, err := getStack(name)
	return snapshot, err
}

type localStackMutation struct {
	name tokens.QName
}

func (p *localStackProvider) BeginMutation(name tokens.QName) (engine.SnapshotMutation, error) {
	return localStackMutation{name: name}, nil
}

func (m localStackMutation) End(snapshot *deploy.Snapshot) error {
	stack := snapshot.Stack
	contract.Assert(m.name == stack)

	config, _, _, err := getStack(stack)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return saveStack(stack, config, snapshot)
}

func getStack(name tokens.QName) (config.Map, *deploy.Snapshot, string, error) {
	// Find a path to the stack file.
	w, err := workspace.New()
	if err != nil {
		return nil, nil, "", err
	}

	contract.Require(name != "", "name")
	file := w.StackPath(name)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		return nil, nil, file,
			errors.Errorf("resource deserialization failed; illegal markup extension: '%v'", ext)
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, nil, file, err
	}

	// Unmarshal the contents into a checkpoint structure.
	var chk stack.Checkpoint
	if err = m.Unmarshal(b, &chk); err != nil {
		return nil, nil, file, err
	}

	// Materialize an actual snapshot object.
	snapshot, err := stack.DeserializeCheckpoint(&chk)
	if err != nil {
		return nil, nil, "", err
	}

	// Ensure the snapshot passes verification before returning it, to catch bugs early.
	if !DisableIntegrityChecking {
		if verifyerr := snapshot.VerifyIntegrity(); verifyerr != nil {
			return nil, nil, file,
				errors.Wrapf(verifyerr, "%s: snapshot integrity failure; refusing to use it", file)
		}
	}

	return chk.Config, snapshot, file, nil
}

func saveStack(name tokens.QName, config map[tokens.ModuleMember]config.Value, snap *deploy.Snapshot) error {
	w, err := workspace.New()
	if err != nil {
		return err
	}

	// Make a serializable stack and then use the encoder to encode it.
	file := w.StackPath(name)
	m, ext := encoding.Detect(file)
	if m == nil {
		return errors.Errorf("resource serialization failed; illegal markup extension: '%v'", ext)
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	chk := stack.SerializeCheckpoint(name, config, snap)
	b, err := m.Marshal(chk)
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
	if cmdutil.IsTruthy(os.Getenv("PULUMI_RETAIN_CHECKPOINTS")) {
		if err = ioutil.WriteFile(fmt.Sprintf("%v.%v", file, time.Now().UnixNano()), b, 0600); err != nil {
			return errors.Wrap(err, "An IO error occurred during the current operation")
		}
	}

	if !DisableIntegrityChecking {
		// Finally, *after* writing the checkpoint, check the integrity.  This is done afterwards so that we write
		// out the checkpoint file since it may contain resource state updates.  But we will warn the user that the
		// file is already written and might be bad.
		if verifyerr := snap.VerifyIntegrity(); verifyerr != nil {
			return errors.Wrapf(verifyerr,
				"%s: snapshot integrity failure; it was already written, but is invalid (backup available at %s)",
				file, bck)
		}
	}

	return nil
}

// removeStack removes information about a stack from the current workspace.
func removeStack(name tokens.QName) error {
	contract.Require(name != "", "name")

	w, err := workspace.New()
	if err != nil {
		return err
	}

	// Just make a backup of the file and don't write out anything new.
	file := w.StackPath(name)
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
