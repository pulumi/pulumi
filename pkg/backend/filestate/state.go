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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"

	"github.com/pulumi/pulumi/pkg/v3/engine"

	"github.com/pkg/errors"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const DisableCheckpointBackupsEnvVar = "PULUMI_DISABLE_CHECKPOINT_BACKUPS"

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking bool

type localQuery struct {
	root string
	proj *workspace.Project
}

func (q *localQuery) GetRoot() string {
	return q.root
}

func (q *localQuery) GetProject() *workspace.Project {
	return q.proj
}

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

func (b *localBackend) newQuery(ctx context.Context,
	op backend.QueryOperation) (engine.QueryInfo, error) {

	return &localQuery{root: op.Root, proj: op.Proj}, nil
}

func (b *localBackend) newUpdate(stackName tokens.QName, op backend.UpdateOperation) (*update, error) {
	contract.Require(stackName != "", "stackName")

	// Construct the deployment target.
	target, err := b.getTarget(stackName, op.StackConfiguration.Config, op.StackConfiguration.Decrypter)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &update{
		root:    op.Root,
		proj:    op.Proj,
		target:  target,
		backend: b,
	}, nil
}

func (b *localBackend) getTarget(stackName tokens.QName, cfg config.Map, dec config.Decrypter) (*deploy.Target, error) {
	snapshot, _, err := b.getStack(stackName)
	if err != nil {
		return nil, err
	}
	return &deploy.Target{
		Name:      stackName,
		Config:    cfg,
		Decrypter: dec,
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
	chk, err := stack.SerializeCheckpoint(name, snap, sm, false /* showSecrets */)
	if err != nil {
		return "", errors.Wrap(err, "serializaing checkpoint")
	}
	byts, err := m.Marshal(chk)
	if err != nil {
		return "", errors.Wrap(err, "An IO error occurred while marshalling the checkpoint")
	}

	// Back up the existing file if it already exists.
	bck := backupTarget(b.bucket, file)

	// And now write out the new snapshot file, overwriting that location.
	if err = b.bucket.WriteAll(context.TODO(), file, byts, nil); err != nil {

		b.mutex.Lock()
		defer b.mutex.Unlock()

		// FIXME: Would be nice to make these configurable
		delay, _ := time.ParseDuration("1s")
		maxDelay, _ := time.ParseDuration("30s")
		backoff := 1.2

		// Retry the write 10 times in case of upstream bucket errors
		_, _, err = retry.Until(context.TODO(), retry.Acceptor{
			Delay:    &delay,
			MaxDelay: &maxDelay,
			Backoff:  &backoff,
			Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
				// And now write out the new snapshot file, overwriting that location.
				err := b.bucket.WriteAll(context.TODO(), file, byts, nil)
				if err != nil {
					logging.V(7).Infof("Error while writing snapshot to: %s (attempt=%d, error=%s)", file, try, err)
					if try > 10 {
						return false, nil, errors.Wrap(err, "An IO error occurred while writing the new snapshot file")
					}
					return false, nil, nil
				}
				return true, nil, nil
			},
		})
		if err != nil {
			return "", err
		}
	}

	logging.V(7).Infof("Saved stack %s checkpoint to: %s (backup=%s)", name, file, bck)

	// And if we are retaining historical checkpoint information, write it out again
	if cmdutil.IsTruthy(os.Getenv("PULUMI_RETAIN_CHECKPOINTS")) {
		if err = b.bucket.WriteAll(context.TODO(), fmt.Sprintf("%v.%v", file, time.Now().UnixNano()), byts, nil); err != nil {
			return "", errors.Wrap(err, "An IO error occurred while writing the new snapshot file")
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
func backupTarget(bucket Bucket, file string) string {
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
func (b *localBackend) getHistory(name tokens.QName, pageSize int, page int) ([]backend.UpdateInfo, error) {
	contract.Require(name != "", "name")

	dir := b.historyDirectory(name)
	// TODO: we could consider optimizing the list operation using `page` and `pageSize`.
	// Unfortunately, this is mildly invasive given the gocloud List API.
	allFiles, err := listBucket(b.bucket, dir)
	if err != nil {
		// History doesn't exist until a stack has been updated.
		if gcerrors.Code(errors.Cause(err)) == gcerrors.NotFound {
			return nil, nil
		}
		return nil, err
	}

	var historyEntries []*blob.ListObject

	// filter down to just history entries, reversing list to be in most recent order.
	// listBucket returns the array sorted by file name, but because of how we name files, older updates come before
	// newer ones.
	for i := len(allFiles) - 1; i >= 0; i-- {
		file := allFiles[i]
		filepath := file.Key

		// ignore checkpoints
		if !strings.HasSuffix(filepath, ".history.json") {
			continue
		}

		historyEntries = append(historyEntries, file)
	}

	start := 0
	end := len(historyEntries) - 1
	if pageSize > 0 {
		if page < 1 {
			page = 1
		}
		start = (page - 1) * pageSize
		end = start + pageSize - 1
		if end > len(historyEntries)-1 {
			end = len(historyEntries) - 1
		}
	}

	var updates []backend.UpdateInfo

	for i := start; i <= end; i++ {
		file := historyEntries[i]
		filepath := file.Key

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

func (b *localBackend) renameHistory(oldName tokens.QName, newName tokens.QName) error {
	contract.Require(oldName != "", "oldName")
	contract.Require(newName != "", "newName")

	oldHistory := b.historyDirectory(oldName)
	newHistory := b.historyDirectory(newName)

	allFiles, err := listBucket(b.bucket, oldHistory)
	if err != nil {
		// if there's nothing there, we don't really need to do a rename.
		if gcerrors.Code(errors.Cause(err)) == gcerrors.NotFound {
			return nil
		}
		return err
	}

	for _, file := range allFiles {
		fileName := objectName(file)
		oldBlob := path.Join(oldHistory, fileName)

		// The filename format is <stack-name>-<timestamp>.[checkpoint|history].json, we need to change
		// the stack name part but retain the other parts.
		newFileName := string(newName) + fileName[strings.LastIndex(fileName, "-"):]
		newBlob := path.Join(newHistory, newFileName)

		if err := b.bucket.Copy(context.TODO(), newBlob, oldBlob, nil); err != nil {
			return errors.Wrap(err, "copying history file")
		}
		if err := b.bucket.Delete(context.TODO(), oldBlob); err != nil {
			return errors.Wrap(err, "deleting existing history file")
		}
	}

	return nil
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
