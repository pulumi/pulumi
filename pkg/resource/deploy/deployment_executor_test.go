// Copyright 2016, Pulumi Corporation.
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

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blang/semver"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebuildBaseStateDanglingParentsSimple(t *testing.T) {
	t.Parallel()

	steps, ex := makeStepsAndExecutor(
		&pkgresource.State{URN: "B", Parent: "A"},
	)

	ex.rebuildBaseState(steps)

	assert.EqualValues(t, map[resource.URN]*pkgresource.State{
		"B": {URN: "B"},
	}, ex.deployment.olds)
}

func TestRebuildBaseStateDanglingParentsTree(t *testing.T) {
	t.Parallel()

	steps, ex := makeStepsAndExecutor(
		&pkgresource.State{URN: "A"},
		&pkgresource.State{URN: "C", Parent: "A", Delete: true},
		&pkgresource.State{URN: "F", Parent: "A"},

		&pkgresource.State{URN: "D", Parent: "A"},
		&pkgresource.State{URN: "G", Parent: "D"},
		&pkgresource.State{URN: "H", Parent: "D", Delete: true},

		&pkgresource.State{URN: "B", Delete: true},
		&pkgresource.State{URN: "E", Parent: "B", Delete: true},
		&pkgresource.State{URN: "I", Parent: "E"},
	)

	ex.rebuildBaseState(steps)

	assert.EqualValues(t, map[resource.URN]*pkgresource.State{
		"A": {URN: "A"},
		"I": {URN: "I", Parent: "E"},
		"F": {URN: "F", Parent: "A"},
		"G": {URN: "G", Parent: "D"},
		"D": {URN: "D", Parent: "A"},
	}, ex.deployment.olds)
}

func TestRebuildBaseStateDependencies(t *testing.T) {
	t.Parallel()

	// Arrange.
	steps, ex := makeStepsAndExecutor(
		// "A" is missing.
		&pkgresource.State{URN: "B", Dependencies: []resource.URN{"A"}},
		&pkgresource.State{URN: "C", Dependencies: []resource.URN{"A"}},

		// "D" is missing.

		&pkgresource.State{URN: "E"},
		// "F" is missing.
		&pkgresource.State{URN: "G", Parent: "E", Dependencies: []resource.URN{"F"}},
	)

	// Act.
	ex.rebuildBaseState(steps)

	// Assert.
	assert.EqualValues(t, map[resource.URN]*pkgresource.State{
		"B": {URN: "B", Dependencies: []resource.URN{}},
		"C": {URN: "C", Dependencies: []resource.URN{}},

		"E": {URN: "E"},
		"G": {URN: "G", Parent: "E", Dependencies: []resource.URN{}},
	}, ex.deployment.olds)
}

func TestRebuildBaseStateDeletedWith(t *testing.T) {
	t.Parallel()

	// Arrange.
	steps, ex := makeStepsAndExecutor(
		// "A" is missing.
		&pkgresource.State{URN: "B", DeletedWith: "A"},
		&pkgresource.State{URN: "C", DeletedWith: "A"},

		// "D" is missing.

		&pkgresource.State{URN: "E"},
		// "F" is missing.
		&pkgresource.State{URN: "G", Parent: "E", DeletedWith: "F"},
	)

	// Act.
	ex.rebuildBaseState(steps)

	// Assert.
	assert.EqualValues(t, map[resource.URN]*pkgresource.State{
		"B": {URN: "B"},
		"C": {URN: "C"},

		"E": {URN: "E"},
		"G": {URN: "G", Parent: "E"},
	}, ex.deployment.olds)
}

func TestRebuildBaseStateReplaceWith(t *testing.T) {
	t.Parallel()

	steps, ex := makeStepsAndExecutor(
		&pkgresource.State{URN: "A"},
		// "B" is missing.
		&pkgresource.State{URN: "C", ReplaceWith: []resource.URN{"A", "B"}},
	)

	ex.rebuildBaseState(steps)

	assert.EqualValues(t, map[resource.URN]*pkgresource.State{
		"A": {URN: "A"},
		"C": {URN: "C", ReplaceWith: []resource.URN{"A"}},
	}, ex.deployment.olds)
}

func TestRebuildBaseStatePropertyDependencies(t *testing.T) {
	t.Parallel()

	// Arrange.
	steps, ex := makeStepsAndExecutor(
		// "A" is missing.
		&pkgresource.State{URN: "B", PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"propB1": {"A"},
		}},

		&pkgresource.State{URN: "C", PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"propC1": {"A"},
			"propC2": {"B"},
		}},

		// "D" is missing.

		&pkgresource.State{URN: "E"},
		// "F" is missing.
		&pkgresource.State{URN: "G", Parent: "E", PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"propG1": {"F"},
			"propG2": {"E"},
			"propG3": {"F"},
		}},
	)

	// Act.
	ex.rebuildBaseState(steps)

	// Assert.
	assert.EqualValues(t, map[resource.URN]*pkgresource.State{
		"B": {URN: "B", PropertyDependencies: map[resource.PropertyKey][]resource.URN{}},
		"C": {URN: "C", PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"propC2": {"B"},
		}},

		"E": {URN: "E"},
		"G": {URN: "G", Parent: "E", PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"propG2": {"E"},
		}},
	}, ex.deployment.olds)
}

func makeStepsAndExecutor(states ...*pkgresource.State) (map[*pkgresource.State]Step, *deploymentExecutor) {
	steps := make(map[*pkgresource.State]Step, len(states))
	for _, state := range states {
		steps[state] = &RefreshStep{old: state, new: state}
	}

	ex := &deploymentExecutor{
		deployment: &Deployment{
			prev: &Snapshot{
				Resources: states,
			},
		},
	}

	return steps, ex
}

type source struct {
	iterator SourceIterator
}

func (src *source) Close() error                { return nil }
func (src *source) Project() tokens.PackageName { return "project" }
func (src *source) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	return src.iterator, nil
}

type iterator struct {
	closed      bool
	returnError bool
}

func (iter *iterator) Cancel(context.Context) error {
	iter.closed = true
	return nil
}

func (iter *iterator) Next() (SourceEvent, error) {
	if iter.returnError {
		return nil, errors.New("error")
	}
	return nil, nil
}

type eventIterator struct {
	events     []SourceEvent
	next       int
	beforeNext func(int) error
}

func (iter *eventIterator) Cancel(context.Context) error { return nil }

func (iter *eventIterator) Next() (SourceEvent, error) {
	if iter.beforeNext != nil {
		if err := iter.beforeNext(iter.next); err != nil {
			return nil, err
		}
	}
	if iter.next == len(iter.events) {
		return nil, nil
	}
	event := iter.events[iter.next]
	iter.next++
	return event, nil
}

func TestStateMigrationDrainsAsyncPlannerEvents(t *testing.T) {
	t.Parallel()

	const timeout = 10 * time.Second
	wait := func(ch <-chan struct{}, description string) error {
		select {
		case <-ch:
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("timed out waiting for %s", description)
		}
	}

	stack := tokens.MustParseStackName("test")
	project := tokens.PackageName("project")
	providerType := sdkproviders.MakeProviderType("pkgA")
	providerURN := resource.NewURN(stack.Q(), project, "", providerType, "provider")
	providerID := resource.ID("provider-id")

	componentType := tokens.Type("example:m:Component")
	componentURN := resource.NewURN(stack.Q(), project, "", componentType, "component")

	oldProviderInputs := resource.PropertyMap{
		"version": resource.NewProperty("1.0.0"),
		"value":   resource.NewProperty("old"),
	}
	newProviderInputs := resource.PropertyMap{
		"version": resource.NewProperty("1.0.0"),
		"value":   resource.NewProperty("new"),
	}
	oldComponentInputs := resource.PropertyMap{"version": resource.NewProperty("old")}
	newComponentInputs := resource.PropertyMap{"version": resource.NewProperty("new")}

	providerState := &pkgresource.State{
		Type: providerType, URN: providerURN, Custom: true, ID: providerID,
		Inputs: oldProviderInputs, Outputs: oldProviderInputs,
	}
	componentState := &pkgresource.State{
		Type: componentType, URN: componentURN,
		Inputs: oldComponentInputs, Outputs: oldComponentInputs,
	}

	newEvent := func(
		typ tokens.Type, name string, custom bool, inputs resource.PropertyMap, provider string,
		migrations ...StateMigrationFunction,
	) *registerResourceEvent {
		return &registerResourceEvent{
			goal: &pkgresource.Goal{
				Type: typ, Name: name, Custom: custom,
				Properties: resource.FromResourcePropertyMap(inputs), Provider: provider,
			},
			done:            make(chan *RegisterResult, 1),
			stateMigrations: migrations,
		}
	}

	providerEvent := newEvent(providerType, "provider", true, newProviderInputs, "")
	var migrationCalls atomic.Int32
	migrationEvent := newEvent(componentType, "component", false, newComponentInputs, "",
		func(_ context.Context, urn resource.URN, oldState []byte) ([]byte, map[resource.URN]resource.URN, error) {
			migrationCalls.Add(1)
			assert.Equal(t, componentURN, urn)

			var states []apitype.ResourceV3
			if err := json.Unmarshal(oldState, &states); err != nil {
				return nil, nil, err
			}
			require.Len(t, states, 1)
			states[0].Inputs["version"] = "new"
			states[0].Outputs["version"] = "new"
			migrated, err := json.Marshal(states)
			return migrated, nil, err
		})

	diffStarted := make(chan struct{})
	migrationDelivered := make(chan struct{})
	iter := &eventIterator{
		events: []SourceEvent{providerEvent, migrationEvent},
		beforeNext: func(index int) error {
			switch index {
			case 1:
				return wait(diffStarted, "parallel diff to start")
			case 2:
				// The executor's source goroutine cannot ask for this event until the main loop has accepted the
				// migration registration. Releasing the diff here deterministically puts the migration between the
				// diff operation and its planner continuation.
				close(migrationDelivered)
			}
			return nil
		},
	}

	var diffCalls atomic.Int32
	loader := deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
		return &deploytest.Provider{
			DiffConfigF: func(context.Context, plugin.DiffConfigRequest) (plugin.DiffResult, error) {
				if diffCalls.Add(1) == 1 {
					close(diffStarted)
					if err := wait(migrationDelivered, "migration registration to reach the executor"); err != nil {
						return plugin.DiffResult{}, err
					}
				}
				return plugin.DiffResult{
					Changes:     plugin.DiffSome,
					ChangedKeys: []resource.PropertyKey{"value"},
				}, nil
			},
		}, nil
	})

	sink := &deploytest.NoopSink{}
	host := deploytest.NewPluginHost(sink, sink, nil, loader)
	plugctx, err := plugin.NewContext(t.Context(), sink, sink, host, nil, "", nil, false, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, plugctx.Close()) }()

	serialize := func(_ context.Context, state *pkgresource.State) (apitype.ResourceV3, error) {
		return apitype.ResourceV3{
			URN: state.URN, Type: state.Type, Custom: state.Custom, ID: state.ID,
			Inputs: state.Inputs.Mappable(), Outputs: state.Outputs.Mappable(),
			Parent: state.Parent, Provider: state.Provider,
		}, nil
	}
	deserialize := func(state apitype.ResourceV3) (*pkgresource.State, error) {
		return &pkgresource.State{
			URN: state.URN, Type: state.Type, Custom: state.Custom, ID: state.ID,
			Inputs: resource.NewPropertyMapFromMap(state.Inputs), Outputs: resource.NewPropertyMapFromMap(state.Outputs),
			Parent: state.Parent, Provider: state.Provider,
		}, nil
	}

	events := &mockEvents{
		OnResourceStepPreF:  func(Step) (any, error) { return nil, nil },
		OnResourceStepPostF: func(any, Step, resource.Status, error) error { return nil },
		OnResourceOutputsF:  func(Step) error { return nil },
	}
	prev := &Snapshot{Resources: []*pkgresource.State{providerState, componentState}}
	deployment, err := NewDeployment(
		plugctx,
		&Options{Parallel: 2, ParallelDiff: true, StateSerializer: serialize, StateDeserializer: deserialize},
		events,
		&Target{Name: stack, Snapshot: prev},
		prev,
		nil,
		&source{iterator: iter},
		nil,
		nil,
	)
	require.NoError(t, err)

	_, err = (&deploymentExecutor{deployment: deployment}).Execute(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int32(1), diffCalls.Load(),
		"the migration must not invalidate a completed provider configuration diff")
	assert.Equal(t, int32(1), migrationCalls.Load())
	assert.Equal(t, resource.NewProperty("new"), deployment.olds[componentURN].Inputs["version"])
}

func TestSourceIteratorClose(t *testing.T) {
	t.Parallel()
	iter := &iterator{}
	ex := &deploymentExecutor{
		deployment: &Deployment{
			source: &source{iter},
			opts:   &Options{},
			ctx: &plugin.Context{
				Diag: &deploytest.NoopSink{},
				Host: deploytest.NewPluginHost(nil, nil, nil),
			},
			newPlans: &resourcePlans{},
		},
		stepGen: &stepGenerator{},
	}

	_, err := ex.Execute(t.Context())
	require.NoError(t, err)
	require.True(t, iter.closed, "The source iterator should be closed after execution")
}

// If we run into an error, bail out and don't attempt to close the iterator.
func TestSourceIteratorNoCloseOnError(t *testing.T) {
	t.Parallel()
	iter := &iterator{returnError: true}
	ex := &deploymentExecutor{
		deployment: &Deployment{
			source: &source{iter},
			opts:   &Options{},
			ctx: &plugin.Context{
				Diag: &deploytest.NoopSink{},
				Host: deploytest.NewPluginHost(nil, nil, nil),
			},
			newPlans: &resourcePlans{},
		},
		stepGen: &stepGenerator{},
	}

	_, err := ex.Execute(t.Context())
	require.ErrorContains(t, err, "BAIL")
	require.False(t, iter.closed)
}
