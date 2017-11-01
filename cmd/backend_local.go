// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"

	"github.com/pulumi/pulumi/pkg/tokens"
)

type localPulumiBackend struct{}

func (b *localPulumiBackend) CreateStack(stackName tokens.QName) error {
	if _, _, _, err := getStack(stackName); err == nil {
		return fmt.Errorf("stack '%v' already exists", stackName)
	}

	return saveStack(stackName, nil, nil)
}

func (b *localPulumiBackend) GetStacks() ([]stackSummary, error) {
	stacks, err := getStacks()
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
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	var decrypter config.ValueDecrypter = panicCrypter{}

	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return err
		}
	}

	localProvider := localStackProvider{decrypter: decrypter}
	pulumiEngine := engine.Engine{Targets: localProvider, Snapshots: localProvider}

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
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	var decrypter config.ValueDecrypter = panicCrypter{}

	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return err
		}
	}

	localProvider := localStackProvider{decrypter: decrypter}
	pulumiEngine := engine.Engine{Targets: localProvider, Snapshots: localProvider}

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
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return err
	}

	var decrypter config.ValueDecrypter = panicCrypter{}

	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return err
		}
	}

	localProvider := localStackProvider{decrypter: decrypter}
	pulumiEngine := engine.Engine{Targets: localProvider, Snapshots: localProvider}

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
