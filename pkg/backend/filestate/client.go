// Copyright 2016-2020, Pulumi Corporation.
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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	user "github.com/tweekmonster/luser"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // driver for azblob://
	_ "gocloud.dev/blob/fileblob"  // driver for file://
	"gocloud.dev/blob/gcsblob"     // driver for gs://
	_ "gocloud.dev/blob/s3blob"    // driver for s3://
	"gocloud.dev/gcerrors"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/edit"
	"github.com/pulumi/pulumi/pkg/v2/resource/stack"
	"github.com/pulumi/pulumi/pkg/v2/secrets"
	"github.com/pulumi/pulumi/pkg/v2/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/retry"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

const DisableCheckpointBackupsEnvVar = "PULUMI_DISABLE_CHECKPOINT_BACKUPS"

// DisableIntegrityChecking can be set to true to disable checkpoint state integrity verification.  This is not
// recommended, because it could mean proceeding even in the face of a corrupted checkpoint state file, but can
// be used as a last resort when a command absolutely must be run.
var DisableIntegrityChecking bool

func IsFileStateBackendURL(urlstr string) bool {
	u, err := url.Parse(urlstr)
	if err != nil {
		return false
	}

	return blob.DefaultURLMux().ValidBucketScheme(u.Scheme)
}

const FilePathPrefix = "file://"

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

var _ backend.Client = (*fileClient)(nil)

type fileClient struct {
	diag diag.Sink

	originalURL  string
	canonicalURL string
	stateDir     string

	bucket Bucket
	mutex  sync.Mutex
}

type fileUpdate struct {
	stackID   backend.StackIdentifier
	client    *fileClient
	info      backend.UpdateInfo
	permalink string
}

func NewClient(diag diag.Sink, url string) (backend.Client, error) {
	canonicalURL, bucket, err := getBucket(url)
	if err != nil {
		return nil, err
	}
	return &fileClient{
		diag:         diag,
		originalURL:  url,
		canonicalURL: canonicalURL,
		stateDir:     workspace.BookkeepingDir,
		bucket:       bucket,
	}, nil
}

// massageBlobPath takes the path the user provided and converts it to an appropriate form go-cloud
// can support.  Importantly, s3/azblob/gs paths should not be be touched. This will only affect
// file:// paths which have a few oddities around them that we want to ensure work properly.
func massageBlobPath(path string) (string, error) {
	if !strings.HasPrefix(path, FilePathPrefix) {
		// Not a file:// path.  Keep this untouched and pass directly to gocloud.
		return path, nil
	}

	// Strip off the "file://" portion so we can examine and determine what to do with the rest.
	path = strings.TrimPrefix(path, FilePathPrefix)

	// We need to specially handle ~.  The shell doesn't take care of this for us, and later
	// functions we run into can't handle this either.
	//
	// From https://stackoverflow.com/questions/17609732/expand-tilde-to-home-directory
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return "", errors.Wrap(err, "Could not determine current user to resolve `file://~` path.")
		}

		if path == "~" {
			path = usr.HomeDir
		} else {
			path = filepath.Join(usr.HomeDir, path[2:])
		}
	}

	// For file:// backend, ensure a relative path is resolved. fileblob only supports absolute paths.
	path, err := filepath.Abs(path)
	if err != nil {
		return "", errors.Wrap(err, "An IO error occurred while building the absolute path")
	}

	// Using example from https://godoc.org/gocloud.dev/blob/fileblob#example-package--OpenBucket
	// On Windows, convert "\" to "/" and add a leading "/". (See https://gocloud.dev/howto/blob/#local)
	path = filepath.ToSlash(path)
	if os.PathSeparator != '/' && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return FilePathPrefix + path, nil
}

func getBucket(originalURL string) (string, Bucket, error) {
	if !IsFileStateBackendURL(originalURL) {
		return "", nil, errors.Errorf("local URL %s has an illegal prefix; expected one of: %s",
			originalURL, strings.Join(blob.DefaultURLMux().BucketSchemes(), ", "))
	}

	u, err := massageBlobPath(originalURL)
	if err != nil {
		return "", nil, err
	}

	p, err := url.Parse(u)
	if err != nil {
		return "", nil, err
	}

	blobmux := blob.DefaultURLMux()

	// for gcp we want to support additional credentials
	// schemes on top of go-cloud's default credentials mux.
	if p.Scheme == gcsblob.Scheme {
		blobmux, err = GoogleCredentialsMux(context.TODO())
		if err != nil {
			return "", nil, err
		}
	}

	bucket, err := blobmux.OpenBucket(context.TODO(), u)
	if err != nil {
		return "", nil, errors.Wrapf(err, "unable to open bucket %s", u)
	}

	if !strings.HasPrefix(u, FilePathPrefix) {
		bucketSubDir := strings.TrimLeft(p.Path, "/")
		if bucketSubDir != "" {
			if !strings.HasSuffix(bucketSubDir, "/") {
				bucketSubDir += "/"
			}

			bucket = blob.PrefixedBucket(bucket, bucketSubDir)
		}
	}

	return u, &wrappedBucket{bucket: bucket}, nil
}

func Login(d diag.Sink, url string) (backend.Client, error) {
	client, err := NewClient(d, url)
	if err != nil {
		return nil, err
	}
	return client, workspace.StoreAccount(url, workspace.Account{}, true)
}

func (c *fileClient) Name() string {
	name, err := os.Hostname()
	contract.IgnoreError(err)
	if name == "" {
		name = "local"
	}
	return name
}

func (c *fileClient) URL() string {
	return c.originalURL
}

func (c *fileClient) User(ctx context.Context) (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return user.Username, nil
}

func (c *fileClient) DefaultSecretsManager() string {
	return passphrase.Type
}

func (c *fileClient) DoesProjectExist(ctx context.Context, owner, projectName string) (bool, error) {
	// Local backends don't really have multiple projects, so just return false here.
	return false, nil
}

func (c *fileClient) StackConsoleURL(stackID backend.StackIdentifier) (string, error) {
	// TODO: error here?
	return "", nil
}

func (c *fileClient) ListStacks(ctx context.Context, filter backend.ListStacksFilter) ([]apitype.StackSummary, error) {
	var stacks []apitype.StackSummary
	err := c.iterateLocalStacks(func(id backend.StackIdentifier, snapshot *deploy.Snapshot, path string) error {
		var lastUpdateTime *int64
		var resourceCount int
		if snapshot != nil {
			if t := snapshot.Manifest.Time; !t.IsZero() {
				unix := t.Unix()
				lastUpdateTime = &unix
			}
			resourceCount = len(snapshot.Resources)
		}

		stacks = append(stacks, apitype.StackSummary{
			OrgName:       id.Owner,
			ProjectName:   id.Project,
			StackName:     id.Stack,
			LastUpdate:    lastUpdateTime,
			ResourceCount: &resourceCount,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stacks, nil
}

func (c *fileClient) GetStack(ctx context.Context, stackID backend.StackIdentifier) (apitype.Stack, error) {
	_, _, err := c.getStack(stackID)
	if err != nil {
		return apitype.Stack{}, err
	}
	return apitype.Stack{
		OrgName:     stackID.Owner,
		ProjectName: stackID.Project,
		StackName:   tokens.QName(stackID.Stack),
	}, nil
}

func (c *fileClient) CreateStack(ctx context.Context, stackID backend.StackIdentifier,
	tags map[string]string) (apitype.Stack, error) {

	if _, _, err := c.getStack(stackID); err == nil {
		return apitype.Stack{}, &backend.StackAlreadyExistsError{StackName: stackID.Stack}
	}

	if err := c.writeStack(stackID, nil, nil); err != nil {
		return apitype.Stack{}, err
	}

	return apitype.Stack{
		OrgName:     stackID.Owner,
		ProjectName: stackID.Project,
		StackName:   tokens.QName(stackID.Stack),
	}, nil
}

func (c *fileClient) DeleteStack(ctx context.Context, stackID backend.StackIdentifier, force bool) (bool, error) {
	return false, c.deleteStack(stackID)
}

func (c *fileClient) RenameStack(ctx context.Context, currentID, newID backend.StackIdentifier) error {
	snapshot, _, err := c.getStack(currentID)
	if err != nil {
		return err
	}

	// Ensure the destination stack does not already exist.
	hasExisting, err := c.bucket.Exists(ctx, c.stackPath(newID))
	if err != nil {
		return err
	}
	if hasExisting {
		return fmt.Errorf("a stack named %v already exists", newID.Stack)
	}

	// If we have a snapshot, we need to rename the URNs inside it to use the new stack name.
	if snapshot != nil {
		if err = edit.RenameStack(snapshot, tokens.QName(newID.Stack), ""); err != nil {
			return err
		}
	}

	// Now save the snapshot with a new name (we pass nil to re-use the existing secrets manager from the snapshot).
	if err = c.writeStack(newID, snapshot, nil); err != nil {
		return err
	}

	// To remove the old stack, just make a backup of the file and don't write out anything new.
	file := c.stackPath(currentID)
	backupTarget(c.bucket, file)

	// And rename the histoy folder as well.
	return c.renameHistory(currentID, newID)
}

func (c *fileClient) UpdateStackTags(ctx context.Context, stack backend.StackIdentifier,
	tags map[string]string) error {

	return fmt.Errorf("stack tags are not supported in --local mode")
}

func (c *fileClient) GetStackHistory(ctx context.Context,
	stackID backend.StackIdentifier) ([]apitype.UpdateInfo, error) {

	var updates []apitype.UpdateInfo
	err := c.iterateHistory(stackID, func(_ backend.StackIdentifier, update backend.UpdateInfo) error {
		cfg := map[string]apitype.ConfigValue{}
		for key, value := range update.Config {
			s, err := value.Value(config.NopDecrypter)
			contract.AssertNoError(err)

			cfg[key.String()] = apitype.ConfigValue{
				String: s,
				Secret: value.Secure(),
				Object: value.Object(),
			}
		}

		resourceChanges := map[apitype.OpType]int{}
		for op, count := range update.ResourceChanges {
			resourceChanges[apitype.OpType(op)] = count
		}

		updates = append(updates, apitype.UpdateInfo{
			Kind:            update.Kind,
			StartTime:       update.StartTime,
			Message:         update.Message,
			Environment:     update.Environment,
			Config:          cfg,
			Result:          apitype.UpdateResult(update.Result),
			EndTime:         update.EndTime,
			ResourceChanges: resourceChanges,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *fileClient) GetLatestStackConfig(ctx context.Context, stackID backend.StackIdentifier) (config.Map, error) {
	var cfg config.Map
	done := fmt.Errorf("sentinel")
	err := c.iterateHistory(stackID, func(_ backend.StackIdentifier, update backend.UpdateInfo) error {
		cfg = update.Config
		return done
	})
	switch {
	case err == nil:
		return nil, backend.ErrNoPreviousDeployment
	case err != done:
		return nil, err
	default:
		return cfg, nil
	}
}

func (c *fileClient) ExportStackDeployment(ctx context.Context, stackID backend.StackIdentifier,
	version *int) (apitype.UntypedDeployment, error) {

	checkpoint, err := c.getCheckpoint(stackID)
	if err != nil {
		return apitype.UntypedDeployment{}, err
	}

	deployment, err := json.Marshal(checkpoint.Latest)
	if err != nil {
		return apitype.UntypedDeployment{}, err
	}

	return apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: deployment,
	}, nil
}

func (c *fileClient) ImportStackDeployment(ctx context.Context, stackID backend.StackIdentifier,
	deployment *apitype.UntypedDeployment) error {

	snapshot, err := stack.DeserializeUntypedDeployment(deployment, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	}
	return c.writeStack(stackID, snapshot, snapshot.SecretsManager)
}

func (c *fileClient) StartUpdate(ctx context.Context, kind apitype.UpdateKind, stackID backend.StackIdentifier,
	proj *workspace.Project, cfg config.Map, metadata apitype.UpdateMetadata, opts engine.UpdateOptions,
	tags map[string]string, dryRun bool) (backend.Update, error) {

	var link string
	if strings.HasPrefix(c.canonicalURL, FilePathPrefix) {
		u, _ := url.Parse(c.canonicalURL)
		u.Path = filepath.ToSlash(path.Join(u.Path, c.stackPath(stackID)))
		link = u.String()
	} else {
		l, err := c.bucket.SignedURL(ctx, c.stackPath(stackID), nil)
		if err != nil {
			// we log a warning here rather then returning an error to avoid exiting
			// pulumi with an error code.
			// printing a statefile perma link happens after all the providers have finished
			// deploying the infrastructure, failing the pulumi update because there was a
			// problem printing a statefile perma link can be missleading in automated CI environments.
			c.diag.Warningf(diag.Message("", "Could not get signed url for stack location: %v"), err)
		}
		link = l
	}

	return &fileUpdate{
		stackID: stackID,
		client:  c,
		info: backend.UpdateInfo{
			Kind:        kind,
			StartTime:   time.Now().Unix(),
			Message:     metadata.Message,
			Environment: metadata.Environment,
			Config:      cfg,
		},
		permalink: link,
	}, nil
}

func (c *fileClient) CancelCurrentUpdate(ctx context.Context, stackID backend.StackIdentifier) error {
	// Concurrency control is not yet supported by the local client.
	return nil
}

func (u *fileUpdate) ProgressURL() string {
	return ""
}

func (u *fileUpdate) PermalinkURL() string {
	return u.permalink
}

func (u *fileUpdate) RequiredPolicies() []apitype.RequiredPolicy {
	return nil
}

func (u *fileUpdate) RecordEvent(ctx context.Context, event apitype.EngineEvent) error {
	// If this is a summary event, record the resource changes.
	if event.SummaryEvent != nil {
		changes := engine.ResourceChanges{}
		for op, count := range event.SummaryEvent.ResourceChanges {
			changes[deploy.StepOp(op)] = count
		}
		u.info.ResourceChanges = changes
	}
	return nil
}

func (u *fileUpdate) PatchCheckpoint(ctx context.Context, deployment *apitype.DeploymentV3) error {
	b, err := json.Marshal(apitype.CheckpointV3{
		Stack:  tokens.QName(u.stackID.Stack),
		Latest: deployment,
	})
	if err != nil {
		return errors.Wrap(err, "marshalling checkpoint")
	}

	_, err = u.client.writeCheckpoint(u.stackID, &apitype.VersionedCheckpoint{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Checkpoint: json.RawMessage(b),
	})
	return err
}

func (u *fileUpdate) Complete(ctx context.Context, status apitype.UpdateStatus) error {
	u.info.EndTime = time.Now().Unix()
	if status == apitype.StatusSucceeded {
		u.info.Result = backend.SucceededResult
	} else {
		u.info.Result = backend.FailedResult
	}

	historyErr := u.client.addToHistory(u.stackID, u.info)
	backupErr := u.client.backupStack(u.stackID)
	if historyErr != nil {
		// We swallow backupErr as it is less important than the saveErr.
		return errors.Wrap(historyErr, "saving update info")
	}
	if backupErr != nil {
		return errors.Wrap(backupErr, "saving backup")
	}
	return nil
}

func (c *fileClient) stackName(stackID backend.StackIdentifier) tokens.QName {
	return tokens.QName(stackID.Stack)
}

func (c *fileClient) stackPath(stackID backend.StackIdentifier) string {
	stack := c.stackName(stackID)

	path := filepath.Join(c.stateDir, workspace.StackDir)
	if stack != "" {
		path = filepath.Join(path, fsutil.QnamePath(stack)+".json")
	}

	return path
}

func (c *fileClient) historyDirectory(id backend.StackIdentifier) string {
	contract.Require(c.stackName(id) != "", "id")
	return filepath.Join(c.stateDir, workspace.HistoryDir, fsutil.QnamePath(c.stackName(id)))
}

func (c *fileClient) backupDirectory(id backend.StackIdentifier) string {
	contract.Require(c.stackName(id) != "", "id")
	return filepath.Join(c.stateDir, workspace.BackupDir, fsutil.QnamePath(c.stackName(id)))
}

func (c *fileClient) getCheckpoint(stackID backend.StackIdentifier) (*apitype.CheckpointV3, error) {
	chkpath := c.stackPath(stackID)
	bytes, err := c.bucket.ReadAll(context.TODO(), chkpath)
	if err != nil {
		return nil, err
	}

	return stack.UnmarshalVersionedCheckpointToLatestCheckpoint(bytes)
}

func (c *fileClient) getStack(stackID backend.StackIdentifier) (*deploy.Snapshot, string, error) {
	if c.stackName(stackID) == "" {
		return nil, "", errors.New("invalid empty stack name")
	}

	file := c.stackPath(stackID)

	chk, err := c.getCheckpoint(stackID)
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

func (c *fileClient) writeStack(stackID backend.StackIdentifier, snap *deploy.Snapshot, sm secrets.Manager) error {
	// Make a serializable stack and then use the encoder to encode it.
	checkpoint, err := stack.SerializeCheckpoint(tokens.QName(stackID.Stack), snap, sm, false /* showSecrets */)
	if err != nil {
		return errors.Wrap(err, "serializaing checkpoint")
	}

	bck, err := c.writeCheckpoint(stackID, checkpoint)
	if err != nil {
		return err
	}

	if !DisableIntegrityChecking {
		// Finally, *after* writing the checkpoint, check the integrity.  This is done afterwards so that we write
		// out the checkpoint file since it may contain resource state updates.  But we will warn the user that the
		// file is already written and might be bad.
		if verifyerr := snap.VerifyIntegrity(); verifyerr != nil {
			file := c.stackPath(stackID)
			return errors.Wrapf(verifyerr,
				"%s: snapshot integrity failure; it was already written, but is invalid (backup available at %s)",
				file, bck)
		}
	}

	return nil
}

func (c *fileClient) writeCheckpoint(stackID backend.StackIdentifier,
	checkpoint *apitype.VersionedCheckpoint) (string, error) {

	// Make a serializable stack and then use the encoder to encode it.
	file := c.stackPath(stackID)
	m, ext := encoding.Detect(file)
	if m == nil {
		return "", errors.Errorf("resource serialization failed; illegal markup extension: '%v'", ext)
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	byts, err := m.Marshal(checkpoint)
	if err != nil {
		return "", errors.Wrap(err, "An IO error occurred while marshalling the checkpoint")
	}

	// Back up the existing file if it already exists.
	bck := backupTarget(c.bucket, file)

	// And now write out the new snapshot file, overwriting that location.
	if err = c.bucket.WriteAll(context.TODO(), file, byts, nil); err != nil {
		c.mutex.Lock()
		defer c.mutex.Unlock()

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
				err := c.bucket.WriteAll(context.TODO(), file, byts, nil)
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
			return bck, err
		}
	}

	logging.V(7).Infof("Saved stack %v checkpoint to: %s (backup=%s)", stackID, file, bck)

	// And if we are retaining historical checkpoint information, write it out again
	if cmdutil.IsTruthy(os.Getenv("PULUMI_RETAIN_CHECKPOINTS")) {
		if err = c.bucket.WriteAll(context.TODO(), fmt.Sprintf("%v.%v", file, time.Now().UnixNano()), byts, nil); err != nil {
			return bck, errors.Wrap(err, "An IO error occurred while writing the new snapshot file")
		}
	}

	return bck, nil
}

// removeStack removes information about a stack from the current workspace.
func (c *fileClient) deleteStack(id backend.StackIdentifier) error {
	contract.Require(c.stackName(id) != "", "id")

	// Just make a backup of the file and don't write out anything new.
	file := c.stackPath(id)
	backupTarget(c.bucket, file)

	historyDir := c.historyDirectory(id)
	return removeAllByPrefix(c.bucket, historyDir)
}

// backupStack copies the current Checkpoint file to ~/.pulumi/backups.
func (c *fileClient) backupStack(id backend.StackIdentifier) error {
	contract.Require(c.stackName(id) != "", "id")

	// Exit early if backups are disabled.
	if cmdutil.IsTruthy(os.Getenv(DisableCheckpointBackupsEnvVar)) {
		return nil
	}

	// Read the current checkpoint file. (Assuming it aleady exists.)
	stackPath := c.stackPath(id)
	byts, err := c.bucket.ReadAll(context.TODO(), stackPath)
	if err != nil {
		return err
	}

	// Get the backup directory.
	backupDir := c.backupDirectory(id)

	// Write out the new backup checkpoint file.
	stackFile := filepath.Base(stackPath)
	ext := filepath.Ext(stackFile)
	base := strings.TrimSuffix(stackFile, ext)
	backupFile := fmt.Sprintf("%s.%v%s", base, time.Now().UnixNano(), ext)
	return c.bucket.WriteAll(context.TODO(), filepath.Join(backupDir, backupFile), byts, nil)
}

type updateIteratorFunc func(id backend.StackIdentifier, update backend.UpdateInfo) error

// iterateHistory returns locally stored update history. The first element of the result will be
// the most recent update record.
func (c *fileClient) iterateHistory(id backend.StackIdentifier, foreach updateIteratorFunc) error {
	contract.Require(c.stackName(id) != "", "id")

	dir := c.historyDirectory(id)
	allFiles, err := listBucket(c.bucket, dir)
	if err != nil {
		// History doesn't exist until a stack has been updated.
		if gcerrors.Code(errors.Cause(err)) == gcerrors.NotFound {
			return nil
		}
		return err
	}

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
		b, err := c.bucket.ReadAll(context.TODO(), filepath)
		if err != nil {
			return errors.Wrapf(err, "reading history file %s", filepath)
		}
		if err = json.Unmarshal(b, &update); err != nil {
			return errors.Wrapf(err, "reading history file %s", filepath)
		}

		if err = foreach(id, update); err != nil {
			return err
		}
	}

	return nil
}

func (c *fileClient) renameHistory(oldID, newID backend.StackIdentifier) error {
	contract.Require(c.stackName(oldID) != "", "oldID")
	contract.Require(c.stackName(newID) != "", "newID")

	oldHistory := c.historyDirectory(oldID)
	newHistory := c.historyDirectory(newID)

	allFiles, err := listBucket(c.bucket, oldHistory)
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
		newFileName := string(c.stackName(newID)) + fileName[strings.LastIndex(fileName, "-"):]
		newBlob := path.Join(newHistory, newFileName)

		if err := c.bucket.Copy(context.TODO(), newBlob, oldBlob, nil); err != nil {
			return errors.Wrap(err, "copying history file")
		}
		if err := c.bucket.Delete(context.TODO(), oldBlob); err != nil {
			return errors.Wrap(err, "deleting existing history file")
		}
	}

	return nil
}

// addToHistory saves the UpdateInfo and makes a copy of the current Checkpoint file.
func (c *fileClient) addToHistory(id backend.StackIdentifier, update backend.UpdateInfo) error {
	contract.Require(c.stackName(id) != "", "id")

	dir := c.historyDirectory(id)

	// Prefix for the update and checkpoint files.
	pathPrefix := path.Join(dir, fmt.Sprintf("%s-%d", c.stackName(id), time.Now().UnixNano()))

	// Save the history file.
	byts, err := json.MarshalIndent(&update, "", "    ")
	if err != nil {
		return err
	}

	historyFile := fmt.Sprintf("%s.history.json", pathPrefix)
	if err = c.bucket.WriteAll(context.TODO(), historyFile, byts, nil); err != nil {
		return err
	}

	// Make a copy of the checkpoint file. (Assuming it already exists.)
	checkpointFile := fmt.Sprintf("%s.checkpoint.json", pathPrefix)
	return c.bucket.Copy(context.TODO(), checkpointFile, c.stackPath(id), nil)
}

type stackIteratorFunc func(id backend.StackIdentifier, snapshot *deploy.Snapshot, path string) error

func (c *fileClient) iterateLocalStacks(foreach stackIteratorFunc) error {
	// Read the stack directory.
	path := c.stackPath(backend.StackIdentifier{})

	files, err := listBucket(c.bucket, path)
	if err != nil {
		return errors.Wrap(err, "error listing stacks")
	}

	for _, file := range files {
		// Ignore directories.
		if file.IsDir {
			continue
		}

		// Skip files without valid extensions (e.g., *.bak files).
		stackfn := objectName(file)
		ext := filepath.Ext(stackfn)
		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this stack's information.
		id := backend.StackIdentifier{Stack: stackfn[:len(stackfn)-len(ext)]}
		snapshot, path, err := c.getStack(id)
		if err != nil {
			logging.V(5).Infof("error reading stack: %v (%v) skipping", c.stackName(id), err)
			continue // failure reading the stack information.
		}
		if err = foreach(id, snapshot, path); err != nil {
			return err
		}
	}

	return nil
}
