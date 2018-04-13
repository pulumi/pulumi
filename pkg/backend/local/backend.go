// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
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

// localBackendURL is fake URL we use to signal we want to use the local backend vs a cloud one.
const localBackendURL = "local://"

// Backend extends the base backend interface with specific information about local backends.
type Backend interface {
	backend.Backend
	local() // at the moment, no local specific info, so just use a marker function.
}

type localBackend struct {
	d diag.Sink
}

func IsLocalBackendURL(url string) bool {
	return url == localBackendURL
}

func New(d diag.Sink) Backend {
	return &localBackend{d: d}
}

func Login(d diag.Sink) (Backend, error) {
	return &localBackend{d: d}, workspace.StoreAccessToken("local://", "", true)
}

func (b *localBackend) Name() string {
	name, err := os.Hostname()
	contract.IgnoreError(err)
	if name == "" {
		name = "local"
	}
	return name
}

func (b *localBackend) local() {}

func (b *localBackend) CreateStack(stackName tokens.QName, opts interface{}) (backend.Stack, error) {
	contract.Requiref(opts == nil, "opts", "local stacks do not support any options")

	if stackName == "" {
		return nil, errors.New("invalid empty stack name")
	}

	if _, _, _, err := getStack(stackName); err == nil {
		return nil, errors.Errorf("stack '%s' already exists", stackName)
	}

	tags, err := backend.GetStackTags()
	if err != nil {
		return nil, errors.Wrap(err, "getting stack tags")
	}
	if err = backend.ValidateStackProperties(string(stackName), tags); err != nil {
		return nil, errors.Wrap(err, "validating stack properties")
	}

	file, err := saveStack(stackName, nil, nil)
	if err != nil {
		return nil, err
	}

	return newStack(stackName, file, nil, nil, b), nil
}

func (b *localBackend) GetStack(stackName tokens.QName) (backend.Stack, error) {
	config, snapshot, path, err := getStack(stackName)
	switch {
	case os.IsNotExist(errors.Cause(err)):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return newStack(stackName, path, config, snapshot, b), nil
	}
}

func (b *localBackend) ListStacks() ([]backend.Stack, error) {
	stacks, err := getLocalStacks()
	if err != nil {
		return nil, err
	}

	var results []backend.Stack
	for _, stackName := range stacks {
		stack, err := b.GetStack(stackName)
		if err != nil {
			return nil, err
		}
		results = append(results, stack)
	}

	return results, nil
}

func (b *localBackend) RemoveStack(stackName tokens.QName, force bool) (bool, error) {
	_, snapshot, _, err := getStack(stackName)
	if err != nil {
		return false, err
	}

	// Don't remove stacks that still have resources.
	if !force && snapshot != nil && len(snapshot.Resources) > 0 {
		return true, errors.New("refusing to remove stack because it still contains resources")
	}

	return false, removeStack(stackName)
}

func (b *localBackend) GetStackCrypter(stackName tokens.QName) (config.Crypter, error) {
	return symmetricCrypter(stackName)
}

func (b *localBackend) Update(
	stackName tokens.QName, proj *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {

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

	if !opts.Force && !opts.Preview {
		return errors.New("--update or --preview must be passed when updating a local stack")
	}

	return b.performEngineOp(
		"updating", backend.DeployUpdate,
		stackName, proj, root, m, opts, displayOpts,
		opts.Preview, engine.Update)
}

func (b *localBackend) Destroy(
	stackName tokens.QName, proj *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {

	if !opts.Force && !opts.Preview {
		return errors.New("--update or --preview must be passed when destroying a local stacks")
	}

	return b.performEngineOp(
		"destroying", backend.DestroyUpdate,
		stackName, proj, root, m, opts, displayOpts,
		opts.Preview, engine.Destroy)
}

func (b *localBackend) performEngineOp(
	op string, kind backend.UpdateKind, stackName tokens.QName, proj *workspace.Project,
	root string, m backend.UpdateMetadata, opts engine.UpdateOptions,
	displayOpts backend.DisplayOptions, dryRun bool,
	performEngineOp func(engine.UpdateInfo, engine.SnapshotManager, chan<- engine.Event, engine.UpdateOptions, bool) (
		engine.ResourceChanges, error)) error {

	update, err := b.newUpdate(stackName, proj, root)
	if err != nil {
		return err
	}

	manager := backend.NewSnapshotManager(b.newSnapshotPersister(), update)
	events := make(chan engine.Event)
	done := make(chan bool)

	go DisplayEvents(op, events, done, displayOpts)

	// Perform the update
	start := time.Now().Unix()
	changes, updateErr := performEngineOp(update, manager, events, opts, dryRun)
	end := time.Now().Unix()

	<-done
	close(events)
	close(done)
	contract.IgnoreError(manager.Close())

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
		saveErr = addToHistory(stackName, info)
		backupErr = backupStack(stackName)
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

func (b *localBackend) GetHistory(stackName tokens.QName) ([]backend.UpdateInfo, error) {
	updates, err := getHistory(stackName)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *localBackend) GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error) {
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

func (b *localBackend) ExportDeployment(stackName tokens.QName) (*apitype.UntypedDeployment, error) {
	_, snap, _, err := getStack(stackName)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(stack.SerializeDeployment(snap))
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{Deployment: json.RawMessage(data)}, nil
}

func (b *localBackend) ImportDeployment(stackName tokens.QName, deployment *apitype.UntypedDeployment) error {
	config, _, _, err := getStack(stackName)
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

	_, err = saveStack(stackName, config, snap)
	return err
}

func (b *localBackend) Logout() error {
	return workspace.DeleteAccessToken(localBackendURL)
}

func getLocalStacks() ([]tokens.QName, error) {
	var stacks []tokens.QName

	w, err := workspace.New()
	if err != nil {
		return nil, err
	}

	// Read the stack directory.
	path := w.StackPath("")

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
		_, _, _, err := getStack(name)
		if err != nil {
			glog.V(5).Infof("error reading stack: %v (%v) skipping", name, err)
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}
