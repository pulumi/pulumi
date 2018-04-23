// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
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
	"github.com/pulumi/pulumi/pkg/workspace"
)

// localBackendURL is fake URL scheme we use to signal we want to use the local backend vs a cloud one.
const localBackendURLPrefix = "local://"

// Backend extends the base backend interface with specific information about local backends.
type Backend interface {
	backend.Backend
	local() // at the moment, no local specific info, so just use a marker function.
}

type localBackend struct {
	d         diag.Sink
	url       string
	stateRoot string
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

func stateRootFromLocalURL(localURL string) string {
	if localURL == localBackendURLPrefix {
		user, err := user.Current()
		contract.AssertNoErrorf(err, "could not determine current user")
		return filepath.Join(user.HomeDir, workspace.BookkeepingDir)
	}

	return localURL[len(localBackendURLPrefix):]
}

func IsLocalBackendURL(url string) bool {
	return strings.HasPrefix(url, localBackendURLPrefix)
}

func New(d diag.Sink, localURL string) Backend {
	return &localBackend{d: d, url: localURL, stateRoot: stateRootFromLocalURL(localURL)}
}

func Login(d diag.Sink, localURL string) (Backend, error) {
	return New(d, localURL), workspace.StoreAccessToken(localURL, "", true)
}

func (b *localBackend) Name() string {
	name, err := os.Hostname()
	contract.IgnoreError(err)
	if name == "" {
		name = "local"
	}
	return name
}

func (b *localBackend) ParseStackReference(stackRefName string) (backend.StackReference, error) {
	return localBackendReference{name: tokens.QName(stackRefName)}, nil
}

func (b *localBackend) local() {}

func (b *localBackend) CreateStack(stackRef backend.StackReference, opts interface{}) (backend.Stack, error) {
	contract.Requiref(opts == nil, "opts", "local stacks do not support any options")

	stackName := stackRef.StackName()
	if stackName == "" {
		return nil, errors.New("invalid empty stack name")
	}

	if _, _, _, err := b.getStack(stackName); err == nil {
		return nil, errors.Errorf("stack '%s' already exists", stackName)
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

	return newStack(stackRef, file, nil, nil, b), nil
}

func (b *localBackend) GetStack(stackRef backend.StackReference) (backend.Stack, error) {
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

func (b *localBackend) ListStacks(projectFilter *tokens.PackageName) ([]backend.Stack, error) {
	stacks, err := b.getLocalStacks()
	if err != nil {
		return nil, err
	}

	var results []backend.Stack
	for _, stackName := range stacks {
		stack, err := b.GetStack(localBackendReference{name: stackName})
		if err != nil {
			return nil, err
		}
		results = append(results, stack)
	}

	return results, nil
}

func (b *localBackend) RemoveStack(stackRef backend.StackReference, force bool) (bool, error) {
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

func (b *localBackend) Update(
	stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts engine.UpdateOptions, displayOpts backend.DisplayOptions, scopes backend.CancellationScopeSource) error {

	stackName := stackRef.StackName()
	// The Pulumi Service will pick up changes to a stack's tags on each update. (e.g. changing the description
	// in Pulumi.yaml.) While this isn't necessary for local updates, we do the validation here to keep
	// parity with stacks managed by the Pulumi Service.
	tags, err := backend.GetStackTags()
	if err != nil {
		return errors.Wrap(err, "getting stack tags")
	}
	if err = backend.ValidateStackProperties(string(stackName), tags); err != nil {
		return errors.Wrap(err, "validating stack properties")
	}

	return b.performEngineOp("updating", backend.DeployUpdate,
		stackName, proj, root, m, opts, displayOpts, opts.Preview, engine.Update, scopes)
}

func (b *localBackend) Refresh(
	stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts engine.UpdateOptions, displayOpts backend.DisplayOptions, scopes backend.CancellationScopeSource) error {

	stackName := stackRef.StackName()
	return b.performEngineOp("refreshing", backend.RefreshUpdate,
		stackName, proj, root, m, opts, displayOpts, opts.Preview, engine.Refresh, scopes)
}

func (b *localBackend) Destroy(
	stackRef backend.StackReference, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts engine.UpdateOptions, displayOpts backend.DisplayOptions, scopes backend.CancellationScopeSource) error {

	stackName := stackRef.StackName()
	return b.performEngineOp("destroying", backend.DestroyUpdate,
		stackName, proj, root, m, opts, displayOpts, opts.Preview, engine.Destroy, scopes)
}

type engineOpFunc func(
	engine.UpdateInfo, *engine.Context, engine.UpdateOptions, bool) (engine.ResourceChanges, error)

func (b *localBackend) performEngineOp(op string, kind backend.UpdateKind,
	stackName tokens.QName, proj *workspace.Project, root string, m backend.UpdateMetadata,
	opts engine.UpdateOptions, displayOpts backend.DisplayOptions, dryRun bool, performEngineOp engineOpFunc,
	scopes backend.CancellationScopeSource) error {

	if !opts.Force && !dryRun {
		return errors.Errorf("--force or --preview must be passed when %s a local stack", op)
	}

	update, err := b.newUpdate(stackName, proj, root)
	if err != nil {
		return err
	}

	manager := b.newSnapshotManager(stackName)

	events := make(chan engine.Event)

	cancelScope := scopes.NewScope(events, dryRun)
	defer cancelScope.Close()

	done := make(chan bool)

	go DisplayEvents(op, events, done, displayOpts)

	// Perform the update
	start := time.Now().Unix()
	engineCtx := &engine.Context{Cancel: cancelScope.Context(), Events: events, SnapshotManager: manager}
	changes, updateErr := performEngineOp(update, engineCtx, opts, dryRun)
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
		return updateErr
	}
	if saveErr != nil {
		// We swallow backupErr as it is less important than the saveErr.
		return errors.Wrap(saveErr, "saving update info")
	}
	return errors.Wrap(backupErr, "saving backup")
}

func (b *localBackend) GetHistory(stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	stackName := stackRef.StackName()
	updates, err := b.getHistory(stackName)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *localBackend) GetLogs(stackRef backend.StackReference,
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

func (b *localBackend) ExportDeployment(stackRef backend.StackReference) (*apitype.UntypedDeployment, error) {
	stackName := stackRef.StackName()
	_, snap, _, err := b.getStack(stackName)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(stack.SerializeDeployment(snap))
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{
		Version:    1,
		Deployment: json.RawMessage(data),
	}, nil
}

func (b *localBackend) ImportDeployment(stackRef backend.StackReference, deployment *apitype.UntypedDeployment) error {
	stackName := stackRef.StackName()
	config, _, _, err := b.getStack(stackName)
	if err != nil {
		return err
	}

	var latest apitype.Deployment
	if err = json.Unmarshal(deployment.Deployment, &latest); err != nil {
		return err
	}

	checkpoint := &apitype.CheckpointV1{
		Stack:  stackName,
		Config: config,
		Latest: &latest,
	}
	snap, err := stack.DeserializeCheckpoint(checkpoint)
	if err != nil {
		return err
	}

	_, err = b.saveStack(stackName, config, snap)
	return err
}

func (b *localBackend) Logout() error {
	return workspace.DeleteAccessToken(b.url)
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
			glog.V(5).Infof("error reading stack: %v (%v) skipping", name, err)
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}
