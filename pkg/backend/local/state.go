// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/pack"
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

// update is an implementation of engine.Update backed by local state.
type update struct {
	root   string
	pkg    *pack.Package
	target *deploy.Target
}

func (u *update) GetRoot() string {
	return u.root
}

func (u *update) GetPackage() *pack.Package {
	return u.pkg
}

func (u *update) GetTarget() *deploy.Target {
	return u.target
}

type localStackMutation struct {
	name tokens.QName
}

func (u *update) BeginMutation() (engine.SnapshotMutation, error) {
	return &localStackMutation{name: u.target.Name}, nil
}

func (m *localStackMutation) End(snapshot *deploy.Snapshot) error {
	stack := snapshot.Stack
	contract.Assert(m.name == stack)

	config, _, _, err := getStack(stack)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return saveStack(stack, config, snapshot)
}

func (b *localBackend) newUpdate(stackName tokens.QName, pkg *pack.Package, root string) (*update, error) {
	contract.Require(stackName != "", "stackName")

	// Construct the deployment target.
	target, err := b.getTarget(stackName)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &update{
		root:   root,
		pkg:    pkg,
		target: target,
	}, nil
}

func (b *localBackend) getTarget(stackName tokens.QName) (*deploy.Target, error) {
	cfg, err := state.Configuration(b.d, stackName)
	if err != nil {
		return nil, err
	}
	decrypter, err := defaultCrypter(stackName, cfg)
	if err != nil {
		return nil, err
	}
	_, snapshot, _, err := getStack(stackName)
	if err != nil {
		return nil, err
	}
	return &deploy.Target{
		Name:      stackName,
		Config:    cfg,
		Decrypter: decrypter,
		Snapshot:  snapshot,
	}, nil
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

	glog.V(7).Infof("Saved stack %s checkpoint to: %s (backup=%s)", name, file, bck)

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

	historyDir := w.HistoryDirectory(name)
	return os.RemoveAll(historyDir)
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

// getHistory returns locally stored update history. The first element of the result will be
// the most recent update record.
func getHistory(name tokens.QName) ([]backend.UpdateInfo, error) {
	w, err := workspace.New()
	if err != nil {
		return nil, err
	}
	contract.Require(name != "", "name")

	dir := w.HistoryDirectory(name)
	allFiles, err := ioutil.ReadDir(dir)
	if err != nil {
		// History doesn't exist until a stack has been updated.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var updates []backend.UpdateInfo
	for _, file := range allFiles {
		filepath := path.Join(dir, file.Name())

		// Open all of the history files, ignoring the checkpoints.
		if !strings.HasSuffix(filepath, ".history.json") {
			continue
		}

		var update backend.UpdateInfo
		b, err := ioutil.ReadFile(filepath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading history file %s", filepath)
		}
		err = json.Unmarshal(b, &update)
		if err != nil {
			return nil, errors.Wrapf(err, "reading history file %s", filepath)
		}

		updates = append(updates, update)
	}

	return updates, nil
}

// addToHistory saves the UpdateInfo and makes a copy of the current Checkpoint file.
func addToHistory(name tokens.QName, update backend.UpdateInfo) error {
	contract.Require(name != "", "name")

	w, err := workspace.New()
	if err != nil {
		return err
	}

	dir := w.HistoryDirectory(name)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// Prefix for the update and checkpoint files.
	pathPrefix := path.Join(dir, fmt.Sprintf("%s-%d", name, update.StartTime))

	// Save the history file.
	b, err := json.MarshalIndent(&update, "", "    ")
	if err != nil {
		return err
	}

	historyFile := fmt.Sprintf("%s.history.json", pathPrefix)
	if err = ioutil.WriteFile(historyFile, b, os.ModePerm); err != nil {
		return err
	}

	// Make a copy of the checkpoint file. (Assuming it aleady exists.)
	b, err = ioutil.ReadFile(w.StackPath(name))
	if err != nil {
		return err
	}

	checkpointFile := fmt.Sprintf("%s.checkpoint.json", pathPrefix)
	return ioutil.WriteFile(checkpointFile, b, os.ModePerm)
}
