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
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // driver for azblob://
	_ "gocloud.dev/blob/fileblob"  // driver for file://
	_ "gocloud.dev/blob/gcsblob"   // driver for gs://
	_ "gocloud.dev/blob/s3blob"    // driver for s3://

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/edit"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/util/validation"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Backend extends the base backend interface with specific information about local backends.
type Backend interface {
	backend.Backend
	local() // at the moment, no local specific info, so just use a marker function.
}

type localBackend struct {
	d      diag.Sink
	url    string
	bucket Bucket
}

type localBackendReference struct {
	name tokens.QName
}

func (r localBackendReference) String() string {
	return string(r.name)
}

func (r localBackendReference) Name() tokens.QName {
	return r.name
}

func IsFileStateBackendURL(urlstr string) bool {
	u, err := url.Parse(urlstr)
	if err != nil {
		return false
	}

	return blob.DefaultURLMux().ValidBucketScheme(u.Scheme)
}

const FilePathPrefix = "file://"

func New(d diag.Sink, url string) (Backend, error) {
	if !IsFileStateBackendURL(url) {
		return nil, errors.Errorf("local URL %s has an illegal prefix; expected one of: %s",
			url, strings.Join(blob.DefaultURLMux().BucketSchemes(), ", "))
	}

	url, err := massageBlobPath(url)
	if err != nil {
		return nil, err
	}

	bucket, err := blob.OpenBucket(context.TODO(), url)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open bucket %s", url)
	}

	return &localBackend{
		d:      d,
		url:    url,
		bucket: &wrappedBucket{bucket: bucket},
	}, nil
}

// massageBlobPath takes the path the user provided and converts it to an appropriate form go-cloud
// can support.  Importantly, s3/azblob/gs paths should not be be touched. This will only affect
// file:// paths which have a few oddities around them that we want to ensure work properly.
func massageBlobPath(path string) (string, error) {
	if !strings.HasPrefix(path, FilePathPrefix) {
		// not a file:// path.  Keep this untouched and pass directly to gocloud.
		return path, nil
	}

	// Strip off the "file://"" portion so we can examine and determine what to do with the rest.
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
		return "", errors.Wrap(err, "An IO error occurred during the current operation")
	}

	// Using example from https://godoc.org/gocloud.dev/blob/fileblob#example-package--OpenBucket
	// On Windows, convert "\" to "/" and add a leading "/":
	path = filepath.ToSlash(path)
	if os.PathSeparator != '/' && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return FilePathPrefix + path, nil
}

func Login(d diag.Sink, url string) (Backend, error) {
	be, err := New(d, url)
	if err != nil {
		return nil, err
	}
	return be, workspace.StoreAccessToken(url, "", true)
}

func (b *localBackend) local() {}

func (b *localBackend) Name() string {
	name, err := os.Hostname()
	contract.IgnoreError(err)
	if name == "" {
		name = "local"
	}
	return name
}

func (b *localBackend) URL() string {
	return b.url
}

func (b *localBackend) StateDir() string {
	return workspace.BookkeepingDir
}

func (b *localBackend) ParseStackReference(stackRefName string) (backend.StackReference, error) {
	return localBackendReference{name: tokens.QName(stackRefName)}, nil
}

func (b *localBackend) CreateStack(ctx context.Context, stackRef backend.StackReference,
	opts interface{}) (backend.Stack, error) {

	contract.Requiref(opts == nil, "opts", "local stacks do not support any options")

	stackName := stackRef.Name()
	if stackName == "" {
		return nil, errors.New("invalid empty stack name")
	}

	if _, _, err := b.getStack(stackName); err == nil {
		return nil, &backend.StackAlreadyExistsError{StackName: string(stackName)}
	}

	tags, err := backend.GetEnvironmentTagsForCurrentStack()
	if err != nil {
		return nil, errors.Wrap(err, "getting stack tags")
	}
	if err = validation.ValidateStackProperties(string(stackName), tags); err != nil {
		return nil, errors.Wrap(err, "validating stack properties")
	}

	file, err := b.saveStack(stackName, nil, nil)
	if err != nil {
		return nil, err
	}

	stack := newStack(stackRef, file, nil, b)
	fmt.Printf("Created stack '%s'\n", stack.Ref())

	return stack, nil
}

func (b *localBackend) GetStack(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
	stackName := stackRef.Name()
	snapshot, path, err := b.getStack(stackName)
	switch {
	case os.IsNotExist(errors.Cause(err)):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return newStack(stackRef, path, snapshot, b), nil
	}
}

func (b *localBackend) ListStacks(
	ctx context.Context, projectFilter *tokens.PackageName) ([]backend.StackSummary, error) {
	stacks, err := b.getLocalStacks()
	if err != nil {
		return nil, err
	}

	var results []backend.StackSummary
	for _, stackName := range stacks {
		stack, err := b.GetStack(ctx, localBackendReference{name: stackName})
		if err != nil {
			return nil, err
		}
		localStack, ok := stack.(*localStack)
		contract.Assertf(ok, "localBackend GetStack returned non-localStack")
		results = append(results, newLocalStackSummary(localStack))
	}

	return results, nil
}

func (b *localBackend) RemoveStack(ctx context.Context, stackRef backend.StackReference, force bool) (bool, error) {
	stackName := stackRef.Name()
	snapshot, _, err := b.getStack(stackName)
	if err != nil {
		return false, err
	}

	// Don't remove stacks that still have resources.
	if !force && snapshot != nil && len(snapshot.Resources) > 0 {
		return true, errors.New("refusing to remove stack because it still contains resources")
	}

	return false, b.removeStack(stackName)
}

func (b *localBackend) RenameStack(ctx context.Context, stackRef backend.StackReference, newName tokens.QName) error {
	stackName := stackRef.Name()
	snap, _, err := b.getStack(stackName)
	if err != nil {
		return err
	}

	// Ensure the destination stack does not already exist.
	_, err = os.Stat(b.stackPath(newName))
	if err == nil {
		return errors.Errorf("a stack named %s already exists", newName)
	} else if !os.IsNotExist(err) {
		return err
	}

	// Rewrite the checkpoint and save it with the new name.
	if err = edit.RenameStack(snap, newName); err != nil {
		return err
	}

	if _, err = b.saveStack(newName, snap, snap.SecretsManager); err != nil {
		return err
	}

	// To remove the old stack, just make a backup of the file and don't write out anything new.
	file := b.stackPath(stackName)
	backupTarget(b.bucket, file)

	// And move the history over as well.
	oldHistoryDir := b.historyDirectory(stackName)
	newHistoryDir := b.historyDirectory(newName)

	return os.Rename(oldHistoryDir, newHistoryDir)
}

func (b *localBackend) GetLatestConfiguration(ctx context.Context,
	stackRef backend.StackReference) (config.Map, error) {

	hist, err := b.GetHistory(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	if len(hist) == 0 {
		return nil, backend.ErrNoPreviousDeployment
	}

	return hist[0].Config, nil
}

func (b *localBackend) Preview(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	// Get the stack.
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}

	// We can skip PreviewThenPromptThenExecute and just go straight to Execute.
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(ctx, apitype.PreviewUpdate, stack, op, opts, nil /*events*/)
}

func (b *localBackend) Update(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply)
}

func (b *localBackend) Refresh(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply)
}

func (b *localBackend) Destroy(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply)
}

func (b *localBackend) Query(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) result.Result {

	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return result.FromError(err)
	}

	return b.query(ctx, stack, op, nil /*events*/)
}

// apply actually performs the provided type of update on a locally hosted stack.
func (b *localBackend) apply(
	ctx context.Context, kind apitype.UpdateKind, stack backend.Stack,
	op backend.UpdateOperation, opts backend.ApplierOptions,
	events chan<- engine.Event) (engine.ResourceChanges, result.Result) {

	stackRef := stack.Ref()
	stackName := stackRef.Name()
	actionLabel := backend.ActionLabel(kind, opts.DryRun)

	if !op.Opts.Display.JSONDisplay {
		// Print a banner so it's clear this is a local deployment.
		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"%s (%s):"+colors.Reset+"\n"), actionLabel, stackRef)
	}

	// Start the update.
	update, err := b.newUpdate(stackName, op)
	if err != nil {
		return nil, result.FromError(err)
	}

	// Spawn a display loop to show events on the CLI.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go display.ShowEvents(
		strings.ToLower(actionLabel), kind, stackName, op.Proj.Name,
		displayEvents, displayDone, op.Opts.Display, opts.DryRun)

	// Create a separate event channel for engine events that we'll pipe to both listening streams.
	engineEvents := make(chan engine.Event)

	scope := op.Scopes.NewScope(engineEvents, opts.DryRun)
	eventsDone := make(chan bool)
	go func() {
		// Pull in all events from the engine and send them to the two listeners.
		for e := range engineEvents {
			displayEvents <- e

			// If the caller also wants to see the events, stream them there also.
			if events != nil {
				events <- e
			}
		}

		close(eventsDone)
	}()

	// Create the management machinery.
	persister := b.newSnapshotPersister(stackName, op.SecretsManager)
	manager := backend.NewSnapshotManager(persister, update.GetTarget().Snapshot)
	engineCtx := &engine.Context{
		Cancel:          scope.Context(),
		Events:          engineEvents,
		SnapshotManager: manager,
		BackendClient:   backend.NewBackendClient(b),
	}

	// Perform the update
	start := time.Now().Unix()
	var changes engine.ResourceChanges
	var updateRes result.Result
	switch kind {
	case apitype.PreviewUpdate:
		changes, updateRes = engine.Update(update, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		changes, updateRes = engine.Update(update, engineCtx, op.Opts.Engine, opts.DryRun)
	case apitype.RefreshUpdate:
		changes, updateRes = engine.Refresh(update, engineCtx, op.Opts.Engine, opts.DryRun)
	case apitype.DestroyUpdate:
		changes, updateRes = engine.Destroy(update, engineCtx, op.Opts.Engine, opts.DryRun)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}
	end := time.Now().Unix()

	// Wait for the display to finish showing all the events.
	<-displayDone
	scope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	contract.IgnoreClose(manager)

	// Make sure the goroutine writing to displayEvents and events has exited before proceeding.
	<-eventsDone
	close(displayEvents)

	// Save update results.
	backendUpdateResult := backend.SucceededResult
	if updateRes != nil {
		backendUpdateResult = backend.FailedResult
	}
	info := backend.UpdateInfo{
		Kind:        kind,
		StartTime:   start,
		Message:     op.M.Message,
		Environment: op.M.Environment,
		Config:      update.GetTarget().Config,
		Result:      backendUpdateResult,
		EndTime:     end,
		// IDEA: it would be nice to populate the *Deployment, so that addToHistory below doesn't need to
		//     rudely assume it knows where the checkpoint file is on disk as it makes a copy of it.  This isn't
		//     trivial to achieve today given the event driven nature of plan-walking, however.
		ResourceChanges: changes,
	}

	var saveErr error
	var backupErr error
	if !opts.DryRun {
		saveErr = b.addToHistory(stackName, info)
		backupErr = b.backupStack(stackName)
	}

	if updateRes != nil {
		// We swallow saveErr and backupErr as they are less important than the updateErr.
		return changes, updateRes
	}

	if saveErr != nil {
		// We swallow backupErr as it is less important than the saveErr.
		return changes, result.FromError(errors.Wrap(saveErr, "saving update info"))
	}

	if backupErr != nil {
		return changes, result.FromError(errors.Wrap(backupErr, "saving backup"))
	}

	// Make sure to print a link to the stack's checkpoint before exiting.
	if opts.ShowLink && !op.Opts.Display.JSONDisplay {
		// Note we get a real signed link for aws/azure/gcp links.  But no such option exists for
		// file:// links so we manually create the link ourselves.
		var link string
		if strings.HasPrefix(b.url, FilePathPrefix) {
			u, _ := url.Parse(b.url)
			u.Path = filepath.ToSlash(path.Join(u.Path, b.stackPath(stackName)))
			link = u.String()
		} else {
			link, err = b.bucket.SignedURL(context.TODO(), b.stackPath(stackName), nil)
			if err != nil {
				return changes, result.FromError(errors.Wrap(err, "Could not get signed url for stack location"))
			}
		}

		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"Permalink: "+
				colors.Underline+colors.BrightBlue+"%s"+colors.Reset+"\n"), link)
	}

	return changes, nil
}

// query executes a query program against the resource outputs of a locally hosted stack.
func (b *localBackend) query(ctx context.Context, stack backend.Stack, op backend.UpdateOperation,
	events chan<- engine.Event) result.Result {

	// TODO: Consider implementing this for local backend. We left it out for the initial cut
	// because we weren't sure we wanted to commit to it.
	return result.Error("Local backend does not support querying over the state")
}

func (b *localBackend) GetHistory(ctx context.Context, stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	stackName := stackRef.Name()
	updates, err := b.getHistory(stackName)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *localBackend) GetLogs(ctx context.Context, stackRef backend.StackReference, cfg backend.StackConfiguration,
	query operations.LogQuery) ([]operations.LogEntry, error) {

	stackName := stackRef.Name()
	target, err := b.getTarget(stackName, cfg.Config, cfg.Decrypter)
	if err != nil {
		return nil, err
	}

	return GetLogsForTarget(target, query)
}

// GetLogsForTarget fetches stack logs using the config, decrypter, and checkpoint in the given target.
func GetLogsForTarget(target *deploy.Target, query operations.LogQuery) ([]operations.LogEntry, error) {
	contract.Assert(target != nil)
	contract.Assert(target.Snapshot != nil)

	config, err := target.Config.Decrypt(target.Decrypter)
	if err != nil {
		return nil, err
	}

	components := operations.NewResourceTree(target.Snapshot.Resources)
	ops := components.OperationsProvider(config)
	logs, err := ops.GetLogs(query)
	if logs == nil {
		return nil, err
	}
	return *logs, err
}

func (b *localBackend) ExportDeployment(ctx context.Context,
	stackRef backend.StackReference) (*apitype.UntypedDeployment, error) {

	stackName := stackRef.Name()
	snap, _, err := b.getStack(stackName)
	if err != nil {
		return nil, err
	}

	if snap == nil {
		snap = deploy.NewSnapshot(deploy.Manifest{}, nil, nil, nil)
	}

	sdep, err := stack.SerializeDeployment(snap, snap.SecretsManager)
	if err != nil {
		return nil, errors.Wrap(err, "serializing deployment")
	}

	data, err := json.Marshal(sdep)
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}, nil
}

func (b *localBackend) ImportDeployment(ctx context.Context, stackRef backend.StackReference,
	deployment *apitype.UntypedDeployment) error {

	stackName := stackRef.Name()
	_, _, err := b.getStack(stackName)
	if err != nil {
		return err
	}

	snap, err := stack.DeserializeUntypedDeployment(deployment)
	if err != nil {
		return err
	}

	_, err = b.saveStack(stackName, snap, snap.SecretsManager)
	return err
}

func (b *localBackend) Logout() error {
	return workspace.DeleteAccessToken(b.url)
}

func (b *localBackend) CurrentUser() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return user.Username, nil
}

func (b *localBackend) getLocalStacks() ([]tokens.QName, error) {
	var stacks []tokens.QName

	// Read the stack directory.
	path := b.stackPath("")

	files, err := listBucket(b.bucket, path)
	if err != nil {
		return nil, errors.Wrap(err, "error listing stacks")
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
		name := tokens.QName(stackfn[:len(stackfn)-len(ext)])
		_, _, err := b.getStack(name)
		if err != nil {
			logging.V(5).Infof("error reading stack: %v (%v) skipping", name, err)
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}

// GetStackTags fetches the stack's existing tags.
func (b *localBackend) GetStackTags(ctx context.Context,
	stackRef backend.StackReference) (map[apitype.StackTagName]string, error) {

	// The local backend does not currently persist tags.
	return nil, errors.New("stack tags not supported in --local mode")
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (b *localBackend) UpdateStackTags(ctx context.Context,
	stackRef backend.StackReference, tags map[apitype.StackTagName]string) error {

	// The local backend does not currently persist tags.
	return errors.New("stack tags not supported in --local mode")
}
