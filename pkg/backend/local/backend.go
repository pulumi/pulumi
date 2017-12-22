// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package local

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
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
	d           diag.Sink
	engineCache map[tokens.QName]engine.Engine
}

func New(d diag.Sink) Backend {
	return &localBackend{d: d, engineCache: make(map[tokens.QName]engine.Engine)}
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
	return symmetricCrypter()
}

func (b *localBackend) Preview(stackName tokens.QName, debug bool, opts engine.PreviewOptions) error {
	pulumiEngine, err := b.getEngine(stackName)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	go displayEvents(events, done, debug)

	if err = pulumiEngine.Preview(stackName, events, opts); err != nil {
		return err
	}

	<-done
	close(events)
	close(done)
	return nil
}

func (b *localBackend) Update(stackName tokens.QName, debug bool, opts engine.DeployOptions) error {
	pulumiEngine, err := b.getEngine(stackName)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	go displayEvents(events, done, debug)

	if err = pulumiEngine.Deploy(stackName, events, opts); err != nil {
		return err
	}

	<-done
	close(events)
	close(done)
	return nil
}

func (b *localBackend) Destroy(stackName tokens.QName, debug bool, opts engine.DestroyOptions) error {
	pulumiEngine, err := b.getEngine(stackName)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	go displayEvents(events, done, debug)

	if err := pulumiEngine.Destroy(stackName, events, opts); err != nil {
		return err
	}

	<-done
	close(events)
	close(done)

	return nil
}

func (b *localBackend) GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error) {
	pulumiEngine, err := b.getEngine(stackName)
	if err != nil {
		return nil, err
	}

	snap, err := pulumiEngine.Snapshots.GetSnapshot(stackName)
	if err != nil {
		return nil, err
	}

	target, err := pulumiEngine.Targets.GetTarget(stackName)
	if err != nil {
		return nil, err
	}

	contract.Assert(snap != nil)
	contract.Assert(target != nil)

	config, err := target.Config.Decrypt(target.Decrypter)
	if err != nil {
		return nil, err
	}

	components := operations.NewResourceTree(snap.Resources)
	ops := components.OperationsProvider(config)
	logs, err := ops.GetLogs(query)
	if logs == nil {
		return nil, err
	}
	return *logs, err
}

func (b *localBackend) getEngine(stackName tokens.QName) (engine.Engine, error) {
	if b.engineCache == nil {
		b.engineCache = make(map[tokens.QName]engine.Engine)
	}

	if engine, has := b.engineCache[stackName]; has {
		return engine, nil
	}

	cfg, err := state.Configuration(b.d, stackName)
	if err != nil {
		return engine.Engine{}, err
	}

	decrypter, err := defaultCrypter(cfg)
	if err != nil {
		return engine.Engine{}, err
	}

	localProvider := newLocalStackProvider(b.d, decrypter)
	pulumiEngine := engine.Engine{Targets: localProvider, Snapshots: localProvider}
	b.engineCache[stackName] = pulumiEngine
	return pulumiEngine, nil
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
