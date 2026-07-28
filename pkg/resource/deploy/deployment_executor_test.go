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
	"errors"
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
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
func (src *source) Iterate(ctx context.Context, providers ProviderSource, state StateSource) (SourceIterator, error) {
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
