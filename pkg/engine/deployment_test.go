// Copyright 2016-2023, Pulumi Corporation.
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

package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type updateInfo struct {
	project workspace.Project
	target  deploy.Target
}

func (u *updateInfo) GetRoot() string {
	return ""
}

func (u *updateInfo) GetProject() *workspace.Project {
	return &u.project
}

func (u *updateInfo) GetTarget() *deploy.Target {
	return &u.target
}

func makeUpdateInfo() *updateInfo {
	return &updateInfo{
		project: workspace.Project{
			Name:    "test",
			Runtime: workspace.NewProjectRuntimeInfo("test", nil),
		},
		target: deploy.Target{Name: tokens.MustParseStackName("test")},
	}
}

type testContext struct {
	Context
	wg      sync.WaitGroup
	events  chan Event
	journal *Journal

	firedEvents []Event
}

func makeTestContext(t testing.TB, cancelCtx *cancel.Context) *testContext {
	events := make(chan Event)
	journal := NewJournal()

	ctx := &testContext{
		Context: Context{
			Cancel:          cancelCtx,
			Events:          events,
			SnapshotManager: journal,
			BackendClient:   nil,
		},
		events:  events,
		journal: journal,
	}

	// Begin draining events.
	ctx.wg.Add(1)
	go func() {
		for e := range events {
			ctx.firedEvents = append(ctx.firedEvents, e)
		}
		ctx.wg.Done()
	}()

	return ctx
}

func (ctx *testContext) makeEventEmitter(t testing.TB) eventEmitter {
	emitter, err := makeQueryEventEmitter(ctx.events)
	assert.NoError(t, err)
	return emitter
}

func (ctx *testContext) Close() error {
	contract.IgnoreClose(ctx.journal)
	close(ctx.events)
	return nil
}

func makePluginHost(t testing.TB, program deploytest.ProgramFunc) plugin.Host {
	sink := diagtest.LogSink(t)
	statusSink := diagtest.LogSink(t)
	lang := deploytest.NewLanguageRuntime(program)
	return deploytest.NewPluginHost(sink, statusSink, lang)
}

// Tests cancellation during early stage of deployment, e.g. plugin installation.
func TestSourceFuncCancellation(t *testing.T) {
	t.Parallel()

	// Set up a cancelable context for the operation.
	cancelCtx, cancelSrc := cancel.NewContext(context.Background())

	// Wait for our source func, then cancel.
	ops := make(chan bool)
	go func() {
		<-ops
		cancelSrc.Cancel()
	}()

	// Create a source func that waits for cancellation.
	sourceF := func(ctx context.Context,
		client deploy.BackendClient, opts *deploymentOptions, proj *workspace.Project, pwd, main, projectRoot string,
		target *deploy.Target, plugctx *plugin.Context, dryRun bool,
	) (deploy.Source, error) {
		// Send ops completion then wait for the cancellation signal.
		close(ops)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
		return nil
	}

	ctx := makeTestContext(t, cancelCtx)
	defer ctx.Close()

	info, err := newDeploymentContext(makeUpdateInfo(), "test", nil)
	assert.NoError(t, err)
	defer info.Close()

	host := makePluginHost(t, program)
	defer host.Close()

	opts := &deploymentOptions{
		UpdateOptions: UpdateOptions{
			Host: host,
		},
		SourceFunc: sourceF,
		Events:     ctx.makeEventEmitter(t),
		Diag:       diagtest.LogSink(t),
		StatusDiag: diagtest.LogSink(t),
	}

	_, err = newDeployment(&ctx.Context, info, opts, false)
	if !assert.ErrorIs(t, err, context.Canceled) {
		t.FailNow()
	}
}
