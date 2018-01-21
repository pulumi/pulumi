// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Backend extends the base backend interface with specific information about local backends.
type Backend interface {
	backend.Backend
	local() // at the moment, no local specific info, so just use a marker function.
}

type localBackend struct {
	d diag.Sink
}

func New(d diag.Sink) Backend {
	return &localBackend{d: d}
}

func (b *localBackend) Name() string {
	// nolint: gas
	name, _ := os.Hostname()
	if name == "" {
		name = "local"
	}
	return name
}

func (b *localBackend) local() {}

func (b *localBackend) CreateStack(stackName tokens.QName, opts interface{}) error {
	contract.Requiref(opts == nil, "opts", "local stacks do not support any options")

	if _, _, _, err := getStack(stackName); err == nil {
		return errors.Errorf("stack '%v' already exists", stackName)
	}

	return saveStack(stackName, nil, nil)
}

func (b *localBackend) GetStack(stackName tokens.QName) (backend.Stack, error) {
	config, snapshot, path, err := getStack(stackName)
	if err != nil {
		return nil, nil
	}
	return newStack(stackName, path, config, snapshot, b), nil
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

func (b *localBackend) Preview(stackName tokens.QName, pkg *pack.Package, root string, debug bool,
	opts engine.UpdateOptions) error {

	update, err := b.newUpdate(stackName, pkg, root)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	go displayEvents(events, done, debug)

	if err = engine.Preview(update, events, opts); err != nil {
		return err
	}

	<-done
	close(events)
	close(done)
	return nil
}

func (b *localBackend) Update(stackName tokens.QName, pkg *pack.Package, root string, debug bool,
	opts engine.UpdateOptions) error {

	update, err := b.newUpdate(stackName, pkg, root)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	go displayEvents(events, done, debug)

	if _, err = engine.Deploy(update, events, opts); err != nil {
		return err
	}

	<-done
	close(events)
	close(done)
	return nil
}

func (b *localBackend) Destroy(stackName tokens.QName, pkg *pack.Package, root string, debug bool,
	opts engine.UpdateOptions) error {

	update, err := b.newUpdate(stackName, pkg, root)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	go displayEvents(events, done, debug)

	if _, err := engine.Destroy(update, events, opts); err != nil {
		return err
	}

	<-done
	close(events)
	close(done)

	return nil
}

func (b *localBackend) GetHistory(stackName tokens.QName) ([]backend.UpdateInfo, error) {
	return nil, errors.New("not yet implemented")
}

func (b *localBackend) GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error) {
	target, err := b.getTarget(stackName)
	if err != nil {
		return nil, err
	}

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

func (b *localBackend) ExportDeployment(stackName tokens.QName) (json.RawMessage, error) {
	_, snap, _, err := getStack(stackName)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(stack.SerializeDeployment(snap))
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (b *localBackend) ImportDeployment(stackName tokens.QName, src json.RawMessage) error {
	config, _, _, err := getStack(stackName)
	if err != nil {
		return err
	}

	var deployment stack.Deployment
	if err = json.Unmarshal(src, &deployment); err != nil {
		return err
	}

	checkpoint := &stack.Checkpoint{
		Stack:  stackName,
		Config: config,
		Latest: &deployment,
	}
	snap, err := stack.DeserializeCheckpoint(checkpoint)
	if err != nil {
		return err
	}

	return saveStack(stackName, config, snap)
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
