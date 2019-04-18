// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filestate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gocloud.dev/blob"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/secrets"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const DisableCheckpointBackupsEnvVar = "PULUMI_DISABLE_CHECKPOINT_BACKUPS"

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking bool

// update is an implementation of engine.Update backed by local state.
type update struct {
	root    string
	proj    *workspace.Project
	target  *deploy.Target
	backend *localBackend
}

func (u *update) GetRoot() string {
	return u.root
}

func (u *update) GetProject() *workspace.Project {
	return u.proj
}

func (u *update) GetTarget() *deploy.Target {
	return u.target
}

func (b *localBackend) newUpdate(stackName tokens.QName, proj *workspace.Project, root string) (*update, error) {
	contract.Require(stackName != "", "stackName")

	// Construct the deployment target.
	target, err := b.getTarget(stackName)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &update{
		root:    root,
		proj:    proj,
		target:  target,
		backend: b,
	}, nil
}

func (b *localBackend) getTarget(stackName tokens.QName) (*deploy.Target, error) {
	stackConfigFile := b.stackConfigFile
	if stackConfigFile == "" {
		f, err := workspace.DetectProjectStackPath(stackName)
		if err != nil {
			return nil, err
		}
		stackConfigFile = f
	}

	stk, err := workspace.LoadProjectStack(stackConfigFile)
	if err != nil {
		return nil, err
	}
	decrypter, err := defaultCrypter(stackName, stk.Config, stackConfigFile)
	if err != nil {
		return nil, err
	}
	snapshot, _, err := b.getStack(stackName)
	if err != nil {
		return nil, err
	}
	return &deploy.Target{
		Name:      stackName,
		Config:    stk.Config,
		Decrypter: decrypter,
		Snapshot:  snapshot,
	}, nil
}

func (b *localBackend) getStack(name tokens.QName) (*deploy.Snapshot, string, error) {
	if name == "" {
		return nil, "", errors.New("invalid empty stack name")
	}

	file := b.stackPath(name)

	chk, err := b.getCheckpoint(name)
	if err != nil {
		return nil, file, errors.Wrap(err, "failed to load checkpoint")
	}

	// Materialize an actual snapshot object.
	snapshot, err := stack.DeserializeCheckpoint(chk)
	if err != nil {
		return nil, "", err
	}

	// Ensure the snapshot passes verification before returning it, to catch bugs early.
	if !DisableIntegrityChecking {
		if verifyerr := snapshot.VerifyIntegrity(); verifyerr != nil {
			return nil, file,
				errors.Wrapf(verifyerr, "%s: snapshot integrity failure; refusing to use it", file)
		}
	}

	return snapshot, file, nil
}

// GetCheckpoint loads a checkpoint file for the given stack in this project, from the current project workspace.
func (b *localBackend) getCheckpoint(stackName tokens.QName) (*apitype.CheckpointV3, error) {
	chkpath := b.stackPath(stackName)
	bytes, err := b.bucket.ReadAll(context.TODO(), chkpath)
	if err != nil {
		return nil, err
	}

	return stack.UnmarshalVersionedCheckpointToLatestCheckpoint(bytes)
}

func (b *localBackend) saveStack(name tokens.QName, snap *deploy.Snapshot, sm secrets.Manager) (string, error) {
	// Make a serializable stack and then use the encoder to encode it.
	file := b.stackPath(name)
	m, ext := encoding.Detect(file)
	if m == nil {
		return "", errors.Errorf("resource serialization failed; illegal markup extension: '%v'", ext)
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	chk, err := stack.SerializeCheckpoint(name, snap, sm)
	if err != nil {
		return "", errors.Wrap(err, "serializaing checkpoint")
	}
	byts, err := m.Marshal(chk)
	if err != nil {
		return "", errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// Back up the existing file if it already exists.
	bck := backupTarget(b.bucket, file)

	// And now write out the new snapshot file, overwriting that location.
	if err = b.bucket.WriteAll(context.TODO(), file, byts, nil); err != nil {
		return "", errors.Wrap(err, "An IO error occurred during the current operation")
	}

	logging.V(7).Infof("Saved stack %s checkpoint to: %s (backup=%s)", name, file, bck)

	// And if we are retaining historical checkpoint information, write it out again
	if cmdutil.IsTruthy(os.Getenv("PULUMI_RETAIN_CHECKPOINTS")) {
		if err = b.bucket.WriteAll(context.TODO(), fmt.Sprintf("%v.%v", file, time.Now().UnixNano()), byts, nil); err != nil {
			return "", errors.Wrap(err, "An IO error occurred during the current operation")
		}
	}

	if !DisableIntegrityChecking {
		// Finally, *after* writing the checkpoint, check the integrity.  This is done afterwards so that we write
		// out the checkpoint file since it may contain resource state updates.  But we will warn the user that the
		// file is already written and might be bad.
		if verifyerr := snap.VerifyIntegrity(); verifyerr != nil {
			return "", errors.Wrapf(verifyerr,
				"%s: snapshot integrity failure; it was already written, but is invalid (backup available at %s)",
				file, bck)
		}
	}

	return file, nil
}

// removeStack removes information about a stack from the current workspace.
func (b *localBackend) removeStack(name tokens.QName) error {
	contract.Require(name != "", "name")

	// Just make a backup of the file and don't write out anything new.
	file := b.stackPath(name)
	backupTarget(b.bucket, file)

	historyDir := b.historyDirectory(name)
	return removeAllByPrefix(b.bucket, historyDir)
}

// backupTarget makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupTarget(bucket *blob.Bucket, file string) string {
	contract.Require(file != "", "file")
	bck := file + ".bak"
	err := renameObject(bucket, file, bck)
	contract.IgnoreError(err) // ignore errors.
	// IDEA: consider multiple backups (.bak.bak.bak...etc).
	return bck
}

// backupStack copies the current Checkpoint file to ~/.pulumi/backups.
func (b *localBackend) backupStack(name tokens.QName) error {
	contract.Require(name != "", "name")

	// Exit early if backups are disabled.
	if cmdutil.IsTruthy(os.Getenv(DisableCheckpointBackupsEnvVar)) {
		return nil
	}

	// Read the current checkpoint file. (Assuming it aleady exists.)
	stackPath := b.stackPath(name)
	byts, err := b.bucket.ReadAll(context.TODO(), stackPath)
	if err != nil {
		return err
	}

	// Get the backup directory.
	backupDir := b.backupDirectory(name)

	// Write out the new backup checkpoint file.
	stackFile := filepath.Base(stackPath)
	ext := filepath.Ext(stackFile)
	base := strings.TrimSuffix(stackFile, ext)
	backupFile := fmt.Sprintf("%s.%v%s", base, time.Now().UnixNano(), ext)
	return b.bucket.WriteAll(context.TODO(), filepath.Join(backupDir, backupFile), byts, nil)
}

func (b *localBackend) stackPath(stack tokens.QName) string {
	path := filepath.Join(b.StateDir(), workspace.StackDir)
	if stack != "" {
		path = filepath.Join(path, fsutil.QnamePath(stack)+".json")
	}

	return path
}

func (b *localBackend) historyDirectory(stack tokens.QName) string {
	contract.Require(stack != "", "stack")
	return filepath.Join(b.StateDir(), workspace.HistoryDir, fsutil.QnamePath(stack))
}

func (b *localBackend) backupDirectory(stack tokens.QName) string {
	contract.Require(stack != "", "stack")
	return filepath.Join(b.StateDir(), workspace.BackupDir, fsutil.QnamePath(stack))
}

// getHistory returns locally stored update history. The first element of the result will be
// the most recent update record.
func (b *localBackend) getHistory(name tokens.QName) ([]backend.UpdateInfo, error) {
	contract.Require(name != "", "name")

	dir := b.historyDirectory(name)
	allFiles, err := listBucket(b.bucket, dir)
	if err != nil {
		// History doesn't exist until a stack has been updated.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var updates []backend.UpdateInfo

	// listBucket returns the array sorted by file name, but because of how we name files, older updates come before
	// newer ones. Loop backwards so we added the newest updates to the array we will return first.
	for i := len(allFiles) - 1; i >= 0; i-- {
		file := allFiles[i]
		filepath := file.Key

		// Open all of the history files, ignoring the checkpoints.
		if !strings.HasSuffix(filepath, ".history.json") {
			continue
		}

		var update backend.UpdateInfo
		b, err := b.bucket.ReadAll(context.TODO(), filepath)
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
func (b *localBackend) addToHistory(name tokens.QName, update backend.UpdateInfo) error {
	contract.Require(name != "", "name")

	dir := b.historyDirectory(name)

	// Prefix for the update and checkpoint files.
	pathPrefix := path.Join(dir, fmt.Sprintf("%s-%d", name, time.Now().UnixNano()))

	// Save the history file.
	byts, err := json.MarshalIndent(&update, "", "    ")
	if err != nil {
		return err
	}

	historyFile := fmt.Sprintf("%s.history.json", pathPrefix)
	if err = b.bucket.WriteAll(context.TODO(), historyFile, byts, nil); err != nil {
		return err
	}

	// Make a copy of the checkpoint file. (Assuming it already exists.)
	checkpointFile := fmt.Sprintf("%s.checkpoint.json", pathPrefix)
	return b.bucket.Copy(context.TODO(), checkpointFile, b.stackPath(name), nil)
}
