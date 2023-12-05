// Copyright 2016-2022, Pulumi Corporation.
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
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"

	"github.com/pulumi/pulumi/pkg/v3/engine"

	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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

func (b *localBackend) newQuery(
	ctx context.Context,
	op backend.QueryOperation,
) (engine.QueryInfo, error) {
	return &localQuery{root: op.Root, proj: op.Proj}, nil
}

func (b *localBackend) newUpdate(
	ctx context.Context,
	secretsProvider secrets.Provider,
	ref *localBackendReference,
	op backend.UpdateOperation,
) (*update, error) {
	contract.Requiref(ref != nil, "ref", "must not be nil")

	// Construct the deployment target.
	target, err := b.getTarget(ctx, secretsProvider, ref,
		op.StackConfiguration.Config, op.StackConfiguration.Decrypter)
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

func (b *localBackend) getTarget(
	ctx context.Context,
	secretsProvider secrets.Provider,
	ref *localBackendReference,
	cfg config.Map,
	dec config.Decrypter,
) (*deploy.Target, error) {
	contract.Requiref(ref != nil, "ref", "must not be nil")
	stack, err := b.GetStack(ctx, ref)
	if err != nil {
		return nil, err
	}
	snapshot, err := stack.Snapshot(ctx, secretsProvider)
	if err != nil {
		return nil, err
	}
	return &deploy.Target{
		Name:         ref.Name(),
		Organization: "organization", // filestate has no organizations really, but we just always say it's "organization"
		Config:       cfg,
		Decrypter:    dec,
		Snapshot:     snapshot,
	}, nil
}

var errCheckpointNotFound = errors.New("checkpoint does not exist")

// stackExists simply does a check that the checkpoint file we expect for this stack exists.
func (b *localBackend) stackExists(
	ctx context.Context,
	ref *localBackendReference,
) (string, error) {
	contract.Requiref(ref != nil, "ref", "must not be nil")

	chkpath := b.stackPath(ctx, ref)
	exists, err := b.bucket.Exists(ctx, chkpath)
	if err != nil {
		return chkpath, fmt.Errorf("failed to load checkpoint: %w", err)
	}
	if !exists {
		return chkpath, errCheckpointNotFound
	}

	return chkpath, nil
}

func (b *localBackend) getSnapshot(ctx context.Context,
	secretsProvider secrets.Provider, ref *localBackendReference,
) (*deploy.Snapshot, error) {
	contract.Requiref(ref != nil, "ref", "must not be nil")

	checkpoint, err := b.getCheckpoint(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// Materialize an actual snapshot object.
	snapshot, err := stack.DeserializeCheckpoint(ctx, secretsProvider, checkpoint)
	if err != nil {
		return nil, err
	}

	// Ensure the snapshot passes verification before returning it, to catch bugs early.
	if !backend.DisableIntegrityChecking {
		if err := snapshot.VerifyIntegrity(); err != nil {
			return nil, fmt.Errorf("snapshot integrity failure; refusing to use it: %w", err)
		}
	}

	return snapshot, nil
}

// GetCheckpoint loads a checkpoint file for the given stack in this project, from the current project workspace.
func (b *localBackend) getCheckpoint(ctx context.Context, ref *localBackendReference) (*apitype.CheckpointV3, error) {
	chkpath := b.stackPath(ctx, ref)
	bytes, err := b.bucket.ReadAll(ctx, chkpath)
	if err != nil {
		return nil, err
	}
	m := encoding.JSON
	if encoding.IsCompressed(bytes) {
		m = encoding.Gzip(m)
	}

	return stack.UnmarshalVersionedCheckpointToLatestCheckpoint(m, bytes)
}

func (b *localBackend) saveCheckpoint(
	ctx context.Context,
	ref *localBackendReference,
	checkpoint *apitype.VersionedCheckpoint,
) (backupFile string, file string, _ error) {
	// Make a serializable stack and then use the encoder to encode it.
	file = b.stackPath(ctx, ref)
	m, ext := encoding.Detect(strings.TrimSuffix(file, ".gz"))
	if m == nil {
		return "", "", fmt.Errorf("resource serialization failed; illegal markup extension: '%v'", ext)
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	if b.gzip {
		if filepath.Ext(file) != encoding.GZIPExt {
			file = file + ".gz"
		}
		m = encoding.Gzip(m)
	} else {
		file = strings.TrimSuffix(file, ".gz")
	}

	byts, err := m.Marshal(checkpoint)
	if err != nil {
		return "", "", fmt.Errorf("An IO error occurred while marshalling the checkpoint: %w", err)
	}

	// Back up the existing file if it already exists. Don't delete the original, the following WriteAll will
	// atomically replace it anyway and various other bits of the system depend on being able to find the
	// .json file to know the stack currently exists (see https://github.com/pulumi/pulumi/issues/9033 for
	// context).
	filePlain := strings.TrimSuffix(file, ".gz")
	fileGzip := filePlain + ".gz"
	// We need to make sure that an out of date state file doesn't exist so we
	// only keep the file of the type we are working with.
	bckGzip := backupTarget(ctx, b.bucket, fileGzip, b.gzip)
	bckPlain := backupTarget(ctx, b.bucket, filePlain, !b.gzip)
	if b.gzip {
		backupFile = bckGzip
	} else {
		backupFile = bckPlain
	}

	// And now write out the new snapshot file, overwriting that location.
	if err = b.bucket.WriteAll(ctx, file, byts, nil); err != nil {

		b.mutex.Lock()
		defer b.mutex.Unlock()

		// FIXME: Would be nice to make these configurable
		delay, _ := time.ParseDuration("1s")
		maxDelay, _ := time.ParseDuration("30s")
		backoff := 1.2

		// Retry the write 10 times in case of upstream bucket errors
		_, _, err = retry.Until(ctx, retry.Acceptor{
			Delay:    &delay,
			MaxDelay: &maxDelay,
			Backoff:  &backoff,
			Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
				// And now write out the new snapshot file, overwriting that location.
				err := b.bucket.WriteAll(ctx, file, byts, nil)
				if err != nil {
					logging.V(7).Infof("Error while writing snapshot to: %s (attempt=%d, error=%s)", file, try, err)
					if try > 10 {
						return false, nil, fmt.Errorf("An IO error occurred while writing the new snapshot file: %w", err)
					}
					return false, nil, nil
				}
				return true, nil, nil
			},
		})
		if err != nil {
			return backupFile, "", err
		}
	}

	logging.V(7).Infof("Saved stack %s checkpoint to: %s (backup=%s)", ref.FullyQualifiedName(), file, backupFile)

	// And if we are retaining historical checkpoint information, write it out again
	if b.Env.GetBool(env.SelfManagedRetainCheckpoints) {
		if err = b.bucket.WriteAll(ctx, fmt.Sprintf("%v.%v", file, time.Now().UnixNano()), byts, nil); err != nil {
			return backupFile, "", fmt.Errorf("An IO error occurred while writing the new snapshot file: %w", err)
		}
	}

	return backupFile, file, nil
}

func (b *localBackend) saveStack(
	ctx context.Context,
	ref *localBackendReference, snap *deploy.Snapshot,
	sm secrets.Manager,
) (string, error) {
	contract.Requiref(ref != nil, "ref", "ref was nil")
	chk, err := stack.SerializeCheckpoint(ref.FullyQualifiedName(), snap, sm, false /* showSecrets */)
	if err != nil {
		return "", fmt.Errorf("serializaing checkpoint: %w", err)
	}

	backup, file, err := b.saveCheckpoint(ctx, ref, chk)
	if err != nil {
		return "", err
	}

	if !backend.DisableIntegrityChecking {
		// Finally, *after* writing the checkpoint, check the integrity.  This is done afterwards so that we write
		// out the checkpoint file since it may contain resource state updates.  But we will warn the user that the
		// file is already written and might be bad.
		if verifyerr := snap.VerifyIntegrity(); verifyerr != nil {
			return "", fmt.Errorf(
				"%s: snapshot integrity failure; it was already written, but is invalid (backup available at %s): %w",
				file, backup, verifyerr)
		}
	}

	return file, nil
}

// removeStack removes information about a stack from the current workspace.
func (b *localBackend) removeStack(ctx context.Context, ref *localBackendReference) error {
	contract.Requiref(ref != nil, "ref", "must not be nil")

	// Just make a backup of the file and don't write out anything new.
	file := b.stackPath(ctx, ref)
	backupTarget(ctx, b.bucket, file, false)

	historyDir := ref.HistoryDir()
	return removeAllByPrefix(ctx, b.bucket, historyDir)
}

// backupTarget makes a backup of an existing file, in preparation for writing a new one.
func backupTarget(ctx context.Context, bucket Bucket, file string, keepOriginal bool) string {
	contract.Requiref(file != "", "file", "must not be empty")
	bck := file + ".bak"

	err := bucket.Copy(ctx, bck, file, nil)
	if err != nil {
		logging.V(5).Infof("error copying %s to %s: %s", file, bck, err)
	}

	if !keepOriginal {
		err = bucket.Delete(ctx, file)
		if err != nil {
			logging.V(5).Infof("error deleting source object after rename: %v (%v) skipping", file, err)
		}
	}

	// IDEA: consider multiple backups (.bak.bak.bak...etc).
	return bck
}

// backupStack copies the current Checkpoint file to ~/.pulumi/backups.
func (b *localBackend) backupStack(ctx context.Context, ref *localBackendReference) error {
	contract.Requiref(ref != nil, "ref", "must not be nil")

	// Exit early if backups are disabled.
	if b.Env.GetBool(env.SelfManagedDisableCheckpointBackups) {
		return nil
	}

	// Read the current checkpoint file. (Assuming it aleady exists.)
	stackPath := b.stackPath(ctx, ref)
	byts, err := b.bucket.ReadAll(ctx, stackPath)
	if err != nil {
		return err
	}

	// Get the backup directory.
	backupDir := ref.BackupDir()

	// Write out the new backup checkpoint file.
	stackFile := filepath.Base(stackPath)
	ext := filepath.Ext(stackFile)
	base := strings.TrimSuffix(stackFile, ext)
	if ext2 := filepath.Ext(base); ext2 != "" && ext == encoding.GZIPExt {
		// base: stack-name.json, ext: .gz
		// ->
		// base: stack-name, ext: .json.gz
		ext = ext2 + ext
		base = strings.TrimSuffix(base, ext2)
	}
	backupFile := fmt.Sprintf("%s.%v%s", base, time.Now().UnixNano(), ext)
	return b.bucket.WriteAll(ctx, filepath.Join(backupDir, backupFile), byts, nil)
}

func (b *localBackend) stackPath(ctx context.Context, ref *localBackendReference) string {
	if ref == nil {
		return StacksDir
	}

	// We can't use listBucket here for as we need to do a partial prefix match on filename, while the
	// "dir" option to listBucket is always suffixed with "/". Also means we don't need to save any
	// results in a slice.
	plainPath := filepath.ToSlash(ref.StackBasePath()) + ".json"
	gzipedPath := plainPath + ".gz"

	bucketIter := b.bucket.List(&blob.ListOptions{
		Delimiter: "/",
		Prefix:    plainPath,
	})

	var plainObj *blob.ListObject
	for {
		file, err := bucketIter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			// Error fetching the available ojects, assume .json
			return plainPath
		}

		// plainObj will always come out first since allObjs is sorted by Key
		if file.Key == plainPath {
			plainObj = file
		} else if file.Key == gzipedPath {
			// We have a plain .json file and it was modified after this gzipped one so use it.
			if plainObj != nil && plainObj.ModTime.After(file.ModTime) {
				return plainPath
			}
			// else use the gzipped object
			return gzipedPath
		}
	}
	// Couldn't find any objects, assume nongzipped path?
	return plainPath
}

// getHistory returns locally stored update history. The first element of the result will be
// the most recent update record.
func (b *localBackend) getHistory(
	ctx context.Context,
	stack *localBackendReference,
	pageSize int, page int,
) ([]backend.UpdateInfo, error) {
	contract.Requiref(stack != nil, "stack", "must not be nil")

	dir := stack.HistoryDir()
	// TODO: we could consider optimizing the list operation using `page` and `pageSize`.
	// Unfortunately, this is mildly invasive given the gocloud List API.
	allFiles, err := listBucket(ctx, b.bucket, dir)
	if err != nil {
		// History doesn't exist until a stack has been updated.
		if gcerrors.Code(err) == gcerrors.NotFound {
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
		if !strings.HasSuffix(filepath, ".history.json") &&
			!strings.HasSuffix(filepath, ".history.json.gz") {
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
		b, err := b.bucket.ReadAll(ctx, filepath)
		if err != nil {
			return nil, fmt.Errorf("reading history file %s: %w", filepath, err)
		}
		m := encoding.JSON
		if encoding.IsCompressed(b) {
			m = encoding.Gzip(m)
		}
		err = m.Unmarshal(b, &update)
		if err != nil {
			return nil, fmt.Errorf("reading history file %s: %w", filepath, err)
		}

		updates = append(updates, update)
	}

	return updates, nil
}

func (b *localBackend) renameHistory(ctx context.Context, oldName, newName *localBackendReference) error {
	contract.Requiref(oldName != nil, "oldName", "must not be nil")
	contract.Requiref(newName != nil, "newName", "must not be nil")

	oldHistory := oldName.HistoryDir()
	newHistory := newName.HistoryDir()

	allFiles, err := listBucket(ctx, b.bucket, oldHistory)
	if err != nil {
		// if there's nothing there, we don't really need to do a rename.
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil
		}
		return err
	}

	for _, file := range allFiles {
		fileName := objectName(file)
		oldBlob := path.Join(oldHistory, fileName)

		// The filename format is <stack-name>-<timestamp>.[checkpoint|history].json[.gz], we need to change
		// the stack name part but retain the other parts. If we find files that don't match this format
		// ignore them.
		dashIndex := strings.LastIndex(fileName, "-")
		if dashIndex == -1 || (fileName[:dashIndex] != oldName.name.String()) {
			// No dash or the string up to the dash isn't the old name
			continue
		}

		newFileName := newName.name.String() + fileName[dashIndex:]
		newBlob := path.Join(newHistory, newFileName)

		if err := b.bucket.Copy(ctx, newBlob, oldBlob, nil); err != nil {
			return fmt.Errorf("copying history file: %w", err)
		}
		if err := b.bucket.Delete(ctx, oldBlob); err != nil {
			return fmt.Errorf("deleting existing history file: %w", err)
		}
	}

	return nil
}

// addToHistory saves the UpdateInfo and makes a copy of the current Checkpoint file.
func (b *localBackend) addToHistory(ctx context.Context, ref *localBackendReference, update backend.UpdateInfo) error {
	contract.Requiref(ref != nil, "ref", "must not be nil")

	dir := ref.HistoryDir()

	// Prefix for the update and checkpoint files.
	pathPrefix := path.Join(dir, fmt.Sprintf("%s-%d", ref.name, time.Now().UnixNano()))

	m, ext := encoding.JSON, "json"
	if b.gzip {
		m = encoding.Gzip(m)
		ext += ".gz"
	}

	// Save the history file.
	byts, err := m.Marshal(&update)
	if err != nil {
		return err
	}

	historyFile := fmt.Sprintf("%s.history.%s", pathPrefix, ext)
	if err = b.bucket.WriteAll(ctx, historyFile, byts, nil); err != nil {
		return err
	}

	// Make a copy of the checkpoint file. (Assuming it already exists.)
	checkpointFile := fmt.Sprintf("%s.checkpoint.%s", pathPrefix, ext)
	return b.bucket.Copy(ctx, checkpointFile, b.stackPath(ctx, ref), nil)
}
