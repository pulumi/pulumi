// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/component"
	"github.com/pulumi/pulumi/pkg/encoding"
	"github.com/pulumi/pulumi/pkg/pulumiframework"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"

	"github.com/pulumi/pulumi/pkg/tokens"
)

type localPulumiBackend struct {
	engineCache map[tokens.QName]engine.Engine
}

func (b *localPulumiBackend) CreateStack(stackName tokens.QName, opts StackCreationOptions) error {
	contract.Requiref(opts.Cloud == "", "cloud", "local backend does not support clouds, cloud must be empty")

	if _, _, _, err := getStack(stackName); err == nil {
		return errors.Errorf("stack '%v' already exists", stackName)
	}

	return saveStack(stackName, nil, nil)
}

func (b *localPulumiBackend) GetStacks() ([]stackSummary, error) {
	stacks, err := getLocalStacks()
	if err != nil {
		return nil, err
	}

	var summaries []stackSummary
	for _, stack := range stacks {
		summary := stackSummary{
			Name:          stack,
			LastDeploy:    "n/a",
			ResourceCount: "n/a",
		}

		// Ignore errors, just leave display settings as "n/a".
		_, _, snapshot, err := getStack(stack)
		if err == nil && snapshot != nil {
			summary.LastDeploy = snapshot.Time.String()
			summary.ResourceCount = strconv.Itoa(len(snapshot.Resources))
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (b *localPulumiBackend) RemoveStack(stackName tokens.QName, force bool) error {
	name, _, snapshot, err := getStack(stackName)
	if err != nil {
		return err
	}

	// Don't remove stacks that still have resources.
	if !force && snapshot != nil && len(snapshot.Resources) > 0 {
		return errHasResources
	}

	return removeStack(name)
}

func (b *localPulumiBackend) Preview(stackName tokens.QName, debug bool, opts engine.PreviewOptions) error {
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

func (b *localPulumiBackend) Update(stackName tokens.QName, debug bool, opts engine.DeployOptions) error {
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

func (b *localPulumiBackend) Destroy(stackName tokens.QName, debug bool, opts engine.DestroyOptions) error {
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

func (b *localPulumiBackend) GetLogs(stackName tokens.QName, query component.LogQuery) ([]component.LogEntry, error) {
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

	// TODO[pulumi/pulumi#54]: replace this with a call into a generalized operations provider.
	components := pulumiframework.NewResource(snap.Resources)
	ops := pulumiframework.NewResourceOperations(target.Config, components)

	return ops.GetLogs(query)
}

func (b *localPulumiBackend) getEngine(stackName tokens.QName) (engine.Engine, error) {
	if b.engineCache == nil {
		b.engineCache = make(map[tokens.QName]engine.Engine)
	}

	if engine, has := b.engineCache[stackName]; has {
		return engine, nil
	}

	cfg, err := getConfiguration(stackName)
	if err != nil {
		return engine.Engine{}, err
	}

	var decrypter config.ValueDecrypter = panicCrypter{}

	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return engine.Engine{}, err
		}
	}

	localProvider := localStackProvider{decrypter: decrypter}
	pulumiEngine := engine.Engine{Targets: localProvider, Snapshots: localProvider}
	b.engineCache[stackName] = pulumiEngine
	return pulumiEngine, nil
}

func getLocalStacks() ([]tokens.QName, error) {
	var stacks []tokens.QName

	w, err := newWorkspace()
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
			continue // failure reading the stack information.
		}

		stacks = append(stacks, name)
	}

	return stacks, nil
}
