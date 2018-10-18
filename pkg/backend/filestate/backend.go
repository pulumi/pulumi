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
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/filestate/storage"
	"github.com/pulumi/pulumi/pkg/backend/filestate/storage/azure"
	"github.com/pulumi/pulumi/pkg/backend/filestate/storage/local"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/workspace"
)

var backends = []string{
	local.URLPrefix,
	azure.URLPrefix,
}

// Backend extends the base backend interface with specific information about local backends.
type Backend interface {
	backend.Backend
	info() // at the moment, no specific info, so just use a marker function.
}

type fileBackend struct {
	d      diag.Sink
	url    string
	bucket storage.Bucket
}

type fileBackendReference struct {
	name tokens.QName
}

func (r fileBackendReference) String() string {
	return string(r.name)
}

func (r fileBackendReference) Name() tokens.QName {
	return r.name
}

func loginAndNew(ctx context.Context, d diag.Sink, url string, logIn bool) (Backend, error) {
	switch {
	case isMatchingBackendURL(url, local.URLPrefix):
		// Local file system file provider
		if logIn {
			if err := local.Login(ctx, url); err != nil {
				return nil, err
			}
		}
		return newBackend(d, url, local.NewBucket)
	case isMatchingBackendURL(url, azure.URLPrefix):
		// Azure blob storage provider
		if logIn {
			if err := azure.Login(ctx, url); err != nil {
				return nil, err
			}
		}
		return newBackend(d, url, azure.NewBucket)
	default:
		return nil,
			errors.Errorf("backend URL %s has an illegal prefix; expected one of %v",
				url, backends)
	}
}

// New creates a new Pulumi backend for the given file provider
func New(d diag.Sink, url string) (Backend, error) {
	return loginAndNew(context.Background(), d, url, false)
}

// Login logs in an creates a new Pulumi backend
func Login(ctx context.Context, d diag.Sink, url string) (Backend, error) {
	return loginAndNew(ctx, d, url, true)
}

// IsFileBackendURL returns whether or not the URL pattern matches
// any of the provided file implementations.
func IsFileBackendURL(url string) bool {
	for _, be := range backends {
		if isMatchingBackendURL(url, be) {
			return true
		}
	}
	return false
}

func isMatchingBackendURL(url, prefix string) bool {
	return strings.HasPrefix(url, prefix)
}

func newBackend(d diag.Sink, url string, createrFn storage.BucketCreater) (Backend, error) {
	accessToken, err := workspace.GetAccessToken(url)
	if err != nil {
		return nil, errors.Wrap(err, "getting stored credentials")
	}

	bucket, err := createrFn(url, accessToken)
	if err != nil {
		return nil, err
	}

	return &fileBackend{
		d:      d,
		url:    url,
		bucket: bucket,
	}, nil
}

func (b *fileBackend) info() {}

func (b *fileBackend) Name() string {
	name, err := os.Hostname()
	contract.IgnoreError(err)
	if name == "" {
		name = "file"
	}
	return name
}

func (b *fileBackend) URL() string {
	return b.url
}

func (b *fileBackend) Dir() string {
	var path string
	// For local backends return a suitable local directory.
	// For all remote file providers just return an empty string.
	if isMatchingBackendURL(b.url, local.URLPrefix) {
		path = b.url[len(local.URLPrefix):]
		if path == "~" {
			user, err := user.Current()
			contract.AssertNoErrorf(err, "could not determine current user")
			path = user.HomeDir
		} else if path == "." {
			pwd, err := os.Getwd()
			contract.AssertNoErrorf(err, "could not determine current working directory")
			path = pwd
		}
	}
	return path
}

func (b *fileBackend) StateDir() string {
	dir := b.Dir()
	return filepath.Join(dir, workspace.BookkeepingDir)
}

func (b *fileBackend) ParseStackReference(stackRefName string) (backend.StackReference, error) {
	return fileBackendReference{name: tokens.QName(stackRefName)}, nil
}

func (b *fileBackend) CreateStack(ctx context.Context, stackRef backend.StackReference,
	opts interface{}) (backend.Stack, error) {

	contract.Requiref(opts == nil, "opts", "file stacks do not support any options")

	stackName := stackRef.Name()
	if stackName == "" {
		return nil, errors.New("invalid empty stack name")
	}

	if _, _, _, err := b.getStack(ctx, stackName); err == nil {
		return nil, &backend.StackAlreadyExistsError{StackName: string(stackName)}
	}

	tags, err := backend.GetStackTags()
	if err != nil {
		return nil, errors.Wrap(err, "getting stack tags")
	}
	if err = backend.ValidateStackProperties(string(stackName), tags); err != nil {
		return nil, errors.Wrap(err, "validating stack properties")
	}

	file, err := b.saveStack(ctx, stackName, nil, nil)
	if err != nil {
		return nil, err
	}

	stack := newStack(stackRef, file, nil, nil, b)
	fmt.Printf("Created stack '%s'\n", stack.Ref())

	return stack, nil
}

func (b *fileBackend) GetStack(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
	stackName := stackRef.Name()
	config, snapshot, path, err := b.getStack(ctx, stackName)
	switch {
	case b.bucket.IsNotExist(errors.Cause(err)):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return newStack(stackRef, path, config, snapshot, b), nil
	}
}

func (b *fileBackend) ListStacks(
	ctx context.Context, projectFilter *tokens.PackageName) ([]backend.StackSummary, error) {
	stacks, err := b.getFileStacks(ctx)
	if err != nil {
		return nil, err
	}

	var results []backend.StackSummary
	for _, stackName := range stacks {
		stack, err := b.GetStack(ctx, fileBackendReference{name: stackName})
		if err != nil {
			return nil, err
		}
		fileStack, ok := stack.(*fileStack)
		contract.Assertf(ok, "fileBackend GetStack returned non-fileStack")
		results = append(results, newFileStackSummary(fileStack))
	}

	return results, nil
}

func (b *fileBackend) RemoveStack(ctx context.Context, stackRef backend.StackReference, force bool) (bool, error) {
	stackName := stackRef.Name()
	_, snapshot, _, err := b.getStack(ctx, stackName)
	if err != nil {
		return false, err
	}

	// Don't remove stacks that still have resources.
	if !force && snapshot != nil && len(snapshot.Resources) > 0 {
		return true, errors.New("refusing to remove stack because it still contains resources")
	}

	return false, b.removeStack(ctx, stackName)
}

func (b *fileBackend) GetStackCrypter(stackRef backend.StackReference) (config.Crypter, error) {
	return symmetricCrypter(stackRef.Name())
}

func (b *fileBackend) GetLatestConfiguration(ctx context.Context,
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

func (b *fileBackend) Preview(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, error) {
	// Get the stack.
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}

	// We can skip PreviewThenPromptThenExecute and just go straight to Execute.
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(ctx, apitype.PreviewUpdate, stack, op, opts, nil /*events*/)
}

func (b *fileBackend) Update(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, error) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply)
}

func (b *fileBackend) Refresh(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, error) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply)
}

func (b *fileBackend) Destroy(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, error) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply)
}

// apply actually performs the provided type of update on a locally hosted stack.
func (b *fileBackend) apply(ctx context.Context, kind apitype.UpdateKind, stack backend.Stack,
	op backend.UpdateOperation, opts backend.ApplierOptions, events chan<- engine.Event) (engine.ResourceChanges, error) {
	stackRef := stack.Ref()
	stackName := stackRef.Name()

	// Print a banner so it's clear this is a local deployment.
	actionLabel := backend.ActionLabel(kind, opts.DryRun)
	fmt.Printf(op.Opts.Display.Color.Colorize(
		colors.SpecHeadline+"%s (%s):"+colors.Reset+"\n"), actionLabel, stackRef)

	// Start the update.
	update, err := b.newUpdate(ctx, stackName, op.Proj, op.Root)
	if err != nil {
		return nil, err
	}

	// Spawn a display loop to show events on the CLI.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go display.ShowEvents(
		strings.ToLower(actionLabel), kind, stackName, op.Proj.Name, displayEvents, displayDone, op.Opts.Display)

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
	persister := b.newSnapshotPersister(ctx, stackName)
	manager := backend.NewSnapshotManager(persister, update.GetTarget().Snapshot)
	engineCtx := &engine.Context{Cancel: scope.Context(), Events: engineEvents, SnapshotManager: manager}

	// Perform the update
	start := time.Now().Unix()
	var changes engine.ResourceChanges
	var updateErr error
	switch kind {
	case apitype.PreviewUpdate:
		changes, updateErr = engine.Update(update, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		changes, updateErr = engine.Update(update, engineCtx, op.Opts.Engine, opts.DryRun)
	case apitype.RefreshUpdate:
		changes, updateErr = engine.Refresh(update, engineCtx, op.Opts.Engine, opts.DryRun)
	case apitype.DestroyUpdate:
		changes, updateErr = engine.Destroy(update, engineCtx, op.Opts.Engine, opts.DryRun)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}
	end := time.Now().Unix()

	// Wait for the display to finish showing all the events.
	<-displayDone
	scope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	close(displayDone)
	contract.IgnoreClose(manager)

	// Make sure the goroutine writing to displayEvents and events has exited before proceeding.
	<-eventsDone
	close(displayEvents)

	// Save update results.
	result := backend.SucceededResult
	if updateErr != nil {
		result = backend.FailedResult
	}
	info := backend.UpdateInfo{
		Kind:        kind,
		StartTime:   start,
		Message:     op.M.Message,
		Environment: op.M.Environment,
		Config:      update.GetTarget().Config,
		Result:      result,
		EndTime:     end,
		// IDEA: it would be nice to populate the *Deployment, so that addToHistory below doesn't need to
		//     rudely assume it knows where the checkpoint file is on disk as it makes a copy of it.  This isn't
		//     trivial to achieve today given the event driven nature of plan-walking, however.
		ResourceChanges: changes,
	}

	var saveErr error
	var backupErr error
	if !opts.DryRun {
		saveErr = b.addToHistory(ctx, stackName, info)
		backupErr = b.backupStack(ctx, stackName)
	}

	if updateErr != nil {
		// We swallow saveErr and backupErr as they are less important than the updateErr.
		return changes, updateErr
	}

	if saveErr != nil {
		// We swallow backupErr as it is less important than the saveErr.
		return changes, errors.Wrap(saveErr, "saving update info")
	}

	if backupErr != nil {
		return changes, errors.Wrap(backupErr, "saving backup")
	}

	// Make sure to print a link to the stack's checkpoint before exiting.
	if opts.ShowLink {
		fmt.Printf(
			op.Opts.Display.Color.Colorize(
				colors.SpecHeadline+"Permalink: "+
					colors.Underline+colors.BrightBlue+"file://%s"+colors.Reset+"\n"), stack.(*fileStack).Path())
	}

	return changes, nil
}

func (b *fileBackend) GetHistory(ctx context.Context, stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	stackName := stackRef.Name()
	updates, err := b.getHistory(ctx, stackName)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *fileBackend) GetLogs(ctx context.Context, stackRef backend.StackReference,
	query operations.LogQuery) ([]operations.LogEntry, error) {

	stackName := stackRef.Name()
	target, err := b.getTarget(ctx, stackName)
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

func (b *fileBackend) ExportDeployment(ctx context.Context,
	stackRef backend.StackReference) (*apitype.UntypedDeployment, error) {

	stackName := stackRef.Name()
	_, snap, _, err := b.getStack(ctx, stackName)
	if err != nil {
		return nil, err
	}

	if snap == nil {
		snap = deploy.NewSnapshot(deploy.Manifest{}, nil, nil)
	}

	data, err := json.Marshal(stack.SerializeDeployment(snap))
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{
		Version:    2,
		Deployment: json.RawMessage(data),
	}, nil
}

func (b *fileBackend) ImportDeployment(ctx context.Context, stackRef backend.StackReference,
	deployment *apitype.UntypedDeployment) error {

	stackName := stackRef.Name()
	config, _, _, err := b.getStack(ctx, stackName)
	if err != nil {
		return err
	}

	snap, err := stack.DeserializeUntypedDeployment(deployment)
	if err != nil {
		return err
	}

	_, err = b.saveStack(ctx, stackName, config, snap)
	return err
}

func (b *fileBackend) Logout() error {
	return workspace.DeleteAccessToken(b.url)
}

func (b *fileBackend) CurrentUser() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return user.Username, nil
}

func (b *fileBackend) getFileStacks(ctx context.Context) ([]tokens.QName, error) {
	var stacks []tokens.QName

	// Read the stack directory.
	path := b.stackPath("")

	files, err := b.bucket.ListFiles(ctx, path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Errorf("could not read stacks: %v", err)
	}

	for _, file := range files {

		// Skip files without valid extensions (e.g., *.bak files).
		stackfn := filepath.Base(file)
		ext := filepath.Ext(stackfn)
		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this stack's information.
		name := tokens.QName(stackfn[:len(stackfn)-len(ext)])
		_, _, _, err := b.getStack(ctx, name)
		if err != nil {
			logging.V(5).Infof("error reading stack: %v (%v) skipping", name, err)
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}
