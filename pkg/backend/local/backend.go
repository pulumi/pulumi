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

package local

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
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

// localBackendURL is fake URL scheme we use to signal we want to use the local backend vs a cloud one.
const localBackendURLPrefix = "file://"

// Backend extends the base backend interface with specific information about local backends.
type Backend interface {
	backend.Backend
	local() // at the moment, no local specific info, so just use a marker function.
}

type localBackend struct {
	d   diag.Sink
	url string
}

type localBackendReference struct {
	name tokens.QName
}

func (r localBackendReference) String() string {
	return string(r.name)
}

func (r localBackendReference) StackName() tokens.QName {
	return r.name
}

func IsLocalBackendURL(url string) bool {
	return strings.HasPrefix(url, localBackendURLPrefix)
}

func New(d diag.Sink, url string) (Backend, error) {
	if !IsLocalBackendURL(url) {
		return nil, errors.Errorf("local URL %s has an illegal prefix; expected %s", url, localBackendURLPrefix)
	}
	return &localBackend{
		d:   d,
		url: url,
	}, nil
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

func (b *localBackend) Dir() string {
	path := b.url[len(localBackendURLPrefix):]
	if path == "~" {
		user, err := user.Current()
		contract.AssertNoErrorf(err, "could not determine current user")
		path = user.HomeDir
	} else if path == "." {
		pwd, err := os.Getwd()
		contract.AssertNoErrorf(err, "could not determine current working directory")
		path = pwd
	}
	return path
}

func (b *localBackend) StateDir() string {
	dir := b.Dir()
	return filepath.Join(dir, workspace.BookkeepingDir)
}

func (b *localBackend) ParseStackReference(stackRefName string) (backend.StackReference, error) {
	return localBackendReference{name: tokens.QName(stackRefName)}, nil
}

func (b *localBackend) CreateStack(ctx context.Context, stackRef backend.StackReference,
	opts interface{}) (backend.Stack, error) {

	contract.Requiref(opts == nil, "opts", "local stacks do not support any options")

	stackName := stackRef.StackName()
	if stackName == "" {
		return nil, errors.New("invalid empty stack name")
	}

	if _, _, _, err := b.getStack(stackName); err == nil {
		return nil, &backend.StackAlreadyExistsError{StackName: string(stackName)}
	}

	tags, err := backend.GetStackTags()
	if err != nil {
		return nil, errors.Wrap(err, "getting stack tags")
	}
	if err = backend.ValidateStackProperties(string(stackName), tags); err != nil {
		return nil, errors.Wrap(err, "validating stack properties")
	}

	file, err := b.saveStack(stackName, nil, nil)
	if err != nil {
		return nil, err
	}

	stack := newStack(stackRef, file, nil, nil, b)
	fmt.Printf("Created stack '%s'.\n", stack.Name())

	return stack, nil
}

func (b *localBackend) GetStack(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
	stackName := stackRef.StackName()
	config, snapshot, path, err := b.getStack(stackName)
	switch {
	case os.IsNotExist(errors.Cause(err)):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return newStack(stackRef, path, config, snapshot, b), nil
	}
}

func (b *localBackend) ListStacks(ctx context.Context, projectFilter *tokens.PackageName) ([]backend.Stack, error) {
	stacks, err := b.getLocalStacks()
	if err != nil {
		return nil, err
	}

	var results []backend.Stack
	for _, stackName := range stacks {
		stack, err := b.GetStack(ctx, localBackendReference{name: stackName})
		if err != nil {
			return nil, err
		}
		results = append(results, stack)
	}

	return results, nil
}

func (b *localBackend) RemoveStack(ctx context.Context, stackRef backend.StackReference, force bool) (bool, error) {
	stackName := stackRef.StackName()
	_, snapshot, _, err := b.getStack(stackName)
	if err != nil {
		return false, err
	}

	// Don't remove stacks that still have resources.
	if !force && snapshot != nil && len(snapshot.Resources) > 0 {
		return true, errors.New("refusing to remove stack because it still contains resources")
	}

	return false, b.removeStack(stackName)
}

func (b *localBackend) GetStackCrypter(stackRef backend.StackReference) (config.Crypter, error) {
	return symmetricCrypter(stackRef.StackName())
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

func (b *localBackend) Preview(
	_ context.Context, stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return b.performEngineOp("previewing", apitype.PreviewUpdate,
		stackRef.StackName(), proj, root, m, opts, scopes, engine.Update)
}

func (b *localBackend) Update(
	_ context.Context, stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {

	// The Pulumi Service will pick up changes to a stack's tags on each update. (e.g. changing the description
	// in Pulumi.yaml.) While this isn't necessary for local updates, we do the validation here to keep
	// parity with stacks managed by the Pulumi Service.
	tags, err := backend.GetStackTags()
	if err != nil {
		return nil, errors.Wrap(err, "getting stack tags")
	}
	stackName := stackRef.StackName()
	if err = backend.ValidateStackProperties(string(stackName), tags); err != nil {
		return nil, errors.Wrap(err, "validating stack properties")
	}
	return b.performEngineOp("updating", apitype.UpdateUpdate,
		stackName, proj, root, m, opts, scopes, engine.Update)
}

func (b *localBackend) Refresh(
	_ context.Context, stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return b.performEngineOp("refreshing", apitype.RefreshUpdate,
		stackRef.StackName(), proj, root, m, opts, scopes, engine.Refresh)
}

func (b *localBackend) Destroy(
	_ context.Context, stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts backend.UpdateOptions, scopes backend.CancellationScopeSource) (engine.ResourceChanges, error) {
	return b.performEngineOp("destroying", apitype.DestroyUpdate,
		stackRef.StackName(), proj, root, m, opts, scopes, engine.Destroy)
}

type engineOpFunc func(engine.UpdateInfo, *engine.Context, engine.UpdateOptions, bool) (engine.ResourceChanges, error)

func (b *localBackend) performEngineOp(op string, kind apitype.UpdateKind,
	stackName tokens.QName, proj *workspace.Project, root string, m backend.UpdateMetadata, opts backend.UpdateOptions,
	scopes backend.CancellationScopeSource, performEngineOp engineOpFunc) (engine.ResourceChanges, error) {

	update, err := b.newUpdate(stackName, proj, root)
	if err != nil {
		return nil, err
	}

	events := make(chan engine.Event)
	dryRun := (kind == apitype.PreviewUpdate)

	cancelScope := scopes.NewScope(events, dryRun)
	defer cancelScope.Close()

	done := make(chan bool)
	go DisplayEvents(op, kind, events, done, opts.Display)

	// Create the management machinery.
	persister := b.newSnapshotPersister(stackName)
	manager := backend.NewSnapshotManager(persister, update.GetTarget().Snapshot)
	engineCtx := &engine.Context{Cancel: cancelScope.Context(), Events: events, SnapshotManager: manager}

	// Perform the update
	start := time.Now().Unix()
	changes, updateErr := performEngineOp(update, engineCtx, opts.Engine, dryRun)
	end := time.Now().Unix()

	<-done
	close(events)
	close(done)
	contract.IgnoreClose(manager)

	// Save update results.
	result := backend.SucceededResult
	if updateErr != nil {
		result = backend.FailedResult
	}
	info := backend.UpdateInfo{
		Kind:        kind,
		StartTime:   start,
		Message:     m.Message,
		Environment: m.Environment,
		Config:      update.GetTarget().Config,
		Result:      result,
		EndTime:     end,
		// IDEA: it would be nice to populate the *Deployment, so that addToHistory below doens't need to
		//     rudely assume it knows where the checkpoint file is on disk as it makes a copy of it.  This isn't
		//     trivial to achieve today given the event driven nature of plan-walking, however.
		ResourceChanges: changes,
	}
	var saveErr error
	var backupErr error
	if !dryRun {
		saveErr = b.addToHistory(stackName, info)
		backupErr = b.backupStack(stackName)
	}

	if updateErr != nil {
		// We swallow saveErr and backupErr as they are less important than the updateErr.
		return changes, updateErr
	}
	if saveErr != nil {
		// We swallow backupErr as it is less important than the saveErr.
		return changes, errors.Wrap(saveErr, "saving update info")
	}
	return changes, errors.Wrap(backupErr, "saving backup")
}

func (b *localBackend) GetHistory(ctx context.Context, stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	stackName := stackRef.StackName()
	updates, err := b.getHistory(stackName)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *localBackend) GetLogs(ctx context.Context, stackRef backend.StackReference,
	query operations.LogQuery) ([]operations.LogEntry, error) {

	stackName := stackRef.StackName()
	target, err := b.getTarget(stackName)
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

	stackName := stackRef.StackName()
	_, snap, _, err := b.getStack(stackName)
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

func (b *localBackend) ImportDeployment(ctx context.Context, stackRef backend.StackReference,
	deployment *apitype.UntypedDeployment) error {

	stackName := stackRef.StackName()
	config, _, _, err := b.getStack(stackName)
	if err != nil {
		return err
	}

	snap, err := stack.DeserializeUntypedDeployment(deployment)
	if err != nil {
		return err
	}

	_, err = b.saveStack(stackName, config, snap)
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

	files, err := ioutil.ReadDir(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Errorf("could not read stacks: %v", err)
	}

	for _, file := range files {
		// Ignore directories.
		if file.IsDir() {
			continue
		}

		// Skip files without valid extensions (e.g., *.bak files).
		stackfn := file.Name()
		ext := filepath.Ext(stackfn)
		if _, has := encoding.Marshalers[ext]; !has {
			continue
		}

		// Read in this stack's information.
		name := tokens.QName(stackfn[:len(stackfn)-len(ext)])
		_, _, _, err := b.getStack(name)
		if err != nil {
			logging.V(5).Infof("error reading stack: %v (%v) skipping", name, err)
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}
