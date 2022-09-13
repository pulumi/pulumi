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

// this file is copied from snapshot_test.go

package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestIdenticalSamesUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	sameState := NewResource("a-unique-urn")
	snap := NewSnapshot([]*resource.State{
		sameState,
	})

	manager, sp := MockSetup(t, snap)

	// The engine generates a SameStep on sameState.
	engineGeneratedSame := NewResource(string(sameState.URN))
	same := deploy.NewSameStep(nil, nil, sameState, engineGeneratedSame)

	mutation, err := manager.BeginMutation(same)
	assert.NoError(t, err)
	// No mutation was made
	assert.Empty(t, sp.SavedSnapshots)

	err = mutation.End(same, true)
	assert.NoError(t, err)

	// Identical sames do not cause a snapshot mutation as part of `End`.
	assert.Empty(t, sp.SavedSnapshots)

	// Close must write the snapshot.
	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestSamesWithEmptyDependenciesUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	res := NewResourceWithDeps("a-unique-urn-resource-a", nil)
	snap := NewSnapshot([]*resource.State{
		res,
	})
	manager, sp := MockSetup(t, snap)
	resUpdated := NewResourceWithDeps(string(res.URN), []resource.URN{})
	same := deploy.NewSameStep(nil, nil, res, resUpdated)
	mutation, err := manager.BeginMutation(same)
	assert.NoError(t, err)
	err = mutation.End(same, true)
	assert.NoError(t, err)
	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestSamesWithEmptyArraysInInputsUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	// Model reading from state file
	state := map[string]interface{}{"defaults": []interface{}{}}
	inputs, err := stack.DeserializeProperties(state, config.NopDecrypter, config.NopEncrypter)
	assert.NoError(t, err)

	res := NewResourceWithInputs("a-unique-urn-resource-a", inputs)
	snap := NewSnapshot([]*resource.State{
		res,
	})
	manager, sp := MockSetup(t, snap)

	// Model passing into and back out of RPC layer (e.g. via `Check`)
	marshalledInputs, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{})
	assert.NoError(t, err)
	inputsUpdated, err := plugin.UnmarshalProperties(marshalledInputs, plugin.MarshalOptions{})
	assert.NoError(t, err)

	resUpdated := NewResourceWithInputs(string(res.URN), inputsUpdated)
	same := deploy.NewSameStep(nil, nil, res, resUpdated)
	mutation, err := manager.BeginMutation(same)
	assert.NoError(t, err)
	err = mutation.End(same, true)
	assert.NoError(t, err)
	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

// This test exercises same steps with meaningful changes to properties _other_ than `Dependencies` in order to ensure
// that the snapshot is written.
func TestSamesWithOtherMeaningfulChangesUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	provider := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgB::provider")
	provider.Custom, provider.Type, provider.ID = true, "pulumi:providers:pkgB", "id"

	resourceP := NewResource("a-unique-urn-resource-p")
	resourceA := NewResource("a-unique-urn-resource-a")

	var changes []*resource.State

	// Change the "custom" bit.
	changes = append(changes, NewResource(string(resourceA.URN)))
	changes[0].Custom, changes[0].Provider = true, "urn:pulumi:foo::bar::pulumi:providers:pkgB::provider::id"

	// Change the parent.
	changes = append(changes, NewResource(string(resourceA.URN)))
	changes[1].Parent = resourceP.URN

	// Change the "protect" bit.
	changes = append(changes, NewResource(string(resourceA.URN)))
	changes[2].Protect = !resourceA.Protect

	// Change the resource outputs.
	changes = append(changes, NewResource(string(resourceA.URN)))
	changes[3].Outputs = resource.PropertyMap{"foo": resource.NewStringProperty("bar")}

	snap := NewSnapshot([]*resource.State{
		provider,
		resourceP,
		resourceA,
	})

	for _, c := range changes {
		manager, sp := MockSetup(t, snap)

		// Generate a same for the provider.
		provUpdated := NewResource(string(provider.URN))
		provUpdated.Custom, provUpdated.Type = true, provider.Type
		provSame := deploy.NewSameStep(nil, nil, provider, provUpdated)
		mutation, err := manager.BeginMutation(provSame)
		assert.NoError(t, err)
		_, _, err = provSame.Apply(false)
		assert.NoError(t, err)
		err = mutation.End(provSame, true)
		assert.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for p. This is not a meaningful change, so the snapshot is not written.
		pUpdated := NewResource(string(resourceP.URN))
		pSame := deploy.NewSameStep(nil, nil, resourceP, pUpdated)
		mutation, err = manager.BeginMutation(pSame)
		assert.NoError(t, err)
		err = mutation.End(pSame, true)
		assert.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for a. Because this is a meaningful change, the snapshot is written:
		aSame := deploy.NewSameStep(nil, nil, resourceA, c)
		mutation, err = manager.BeginMutation(aSame)
		assert.NoError(t, err)
		err = mutation.End(aSame, true)
		assert.NoError(t, err)

		err = manager.Close()
		assert.NoError(t, err)
		inSnapshot := sp.SavedSnapshots[0].Resources[2]
		assert.Equal(t, c, inSnapshot)

		assert.Len(t, sp.SavedSnapshots, 1)
	}

	// Set up a second provider and change the resource's provider reference.
	provider2 := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgB::provider2")
	provider2.Custom, provider2.Type, provider2.ID = true, "pulumi:providers:pkgB", "id2"

	resourceA.Custom, resourceA.ID, resourceA.Provider =
		true, "id", "urn:pulumi:foo::bar::pulumi:providers:pkgB::provider::id"

	snap = NewSnapshot([]*resource.State{
		provider,
		provider2,
		resourceA,
	})

	changes = []*resource.State{NewResource(string(resourceA.URN))}
	changes[0].Custom, changes[0].Provider = true, "urn:pulumi:foo::bar::pulumi:providers:pkgB::provider2::id2"

	for _, c := range changes {
		manager, sp := MockSetup(t, snap)

		// Generate sames for the providers.
		provUpdated := NewResource(string(provider.URN))
		provUpdated.Custom, provUpdated.Type = true, provider.Type
		provSame := deploy.NewSameStep(nil, nil, provider, provUpdated)
		mutation, err := manager.BeginMutation(provSame)
		assert.NoError(t, err)
		_, _, err = provSame.Apply(false)
		assert.NoError(t, err)
		err = mutation.End(provSame, true)
		assert.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for p. This is not a meaningful change, so the snapshot is not written.
		prov2Updated := NewResource(string(provider2.URN))
		prov2Updated.Custom, prov2Updated.Type = true, provider.Type
		prov2Same := deploy.NewSameStep(nil, nil, provider2, prov2Updated)
		mutation, err = manager.BeginMutation(prov2Same)
		assert.NoError(t, err)
		_, _, err = prov2Same.Apply(false)
		assert.NoError(t, err)
		err = mutation.End(prov2Same, true)
		assert.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for a. Because this is a meaningful change, the snapshot is written:
		aSame := deploy.NewSameStep(nil, nil, resourceA, c)
		mutation, err = manager.BeginMutation(aSame)
		assert.NoError(t, err)
		_, _, err = aSame.Apply(false)
		assert.NoError(t, err)
		err = mutation.End(aSame, true)
		assert.NoError(t, err)

		err = manager.Close()
		assert.NoError(t, err)
		assert.Len(t, sp.SavedSnapshots, 1)
	}
}

// This test exercises the merge operation with a particularly vexing deployment
// state that was useful in shaking out bugs.
func TestVexingDeploymentUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	// This is the dependency graph we are going for in the base snapshot:
	//
	//       +-+
	//  +--> |A|
	//  |    +-+
	//  |     ^
	//  |    +-+
	//  |    |B|
	//  |    +-+
	//  |     ^
	//  |    +-+
	//  +--+ |C| <---+
	//       +-+     |
	//        ^      |
	//       +-+     |
	//       |D|     |
	//       +-+     |
	//               |
	//       +-+     |
	//       |E| +---+
	//       +-+
	a := NewResource("a")
	b := NewResource("b", a.URN)
	c := NewResource("c", a.URN, b.URN)
	d := NewResource("d", c.URN)
	e := NewResource("e", c.URN)
	snap := NewSnapshot([]*resource.State{
		a,
		b,
		c,
		d,
		e,
	})

	manager, sp := MockSetup(t, snap)

	// This is the sequence of events that come out of the engine:
	//   B - Same, depends on nothing
	//   C - CreateReplacement, depends on B
	//   C - Replace
	//   D - Update, depends on new C

	// This produces the following dependency graph in the new snapshot:
	//        +-+
	//  +---> |B|
	//  |     +++
	//  |      ^
	//  |     +++
	//  |     |C| <----+
	//  |     +-+      |
	//  |              |
	//  |     +-+      |
	//  +---+ |C| +-------------> A (not in graph!)
	//        +-+      |
	//                 |
	//        +-+      |
	//        |D|  +---+
	//        +-+
	//
	// Conceptually, this is a plan that deletes A. However, we have not yet observed the
	// deletion of A, presumably because the engine can't know for sure that it's been deleted
	// until the eval source completes. Of note in this snapshot is that the replaced C is still in the graph,
	// because it has not yet been deleted, and its dependency A is not in the graph because it
	// has not been seen.
	//
	// Since axiomatically we assume that steps come in in a valid topological order of the dependency graph,
	// we can logically assume that A is going to be deleted. (If A were not being deleted, it must have been
	// the target of a Step that came before C, which depends on it.)
	applyStep := func(step deploy.Step) {
		mutation, err := manager.BeginMutation(step)
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		err = mutation.End(step, true)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}

	// b now depends on nothing
	bPrime := NewResource(string(b.URN))
	applyStep(deploy.NewSameStep(nil, MockRegisterResourceEvent{}, b, bPrime))

	// c now only depends on b
	cPrime := NewResource(string(c.URN), bPrime.URN)

	// mocking out the behavior of a provider indicating that this resource needs to be deleted
	createReplacement := deploy.NewCreateReplacementStep(nil, MockRegisterResourceEvent{}, c, cPrime, nil, nil, nil, true)
	replace := deploy.NewReplaceStep(nil, c, cPrime, nil, nil, nil, true)
	c.Delete = true

	applyStep(createReplacement)
	applyStep(replace)

	// cPrime now exists, c is now pending deletion
	// dPrime now depends on cPrime, which got replaced
	dPrime := NewResource(string(d.URN), cPrime.URN)
	applyStep(deploy.NewUpdateStep(nil, MockRegisterResourceEvent{}, d, dPrime, nil, nil, nil, nil))

	err := manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestDeletionUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockSetup(t, snap)
	step := deploy.NewDeleteStep(nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, true)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestFailedDeleteUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockSetup(t, snap)
	step := deploy.NewDeleteStep(nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingCreateSuccessUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewCreateStep(nil, &MockRegisterResourceEvent{}, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingCreateFailureUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewCreateStep(nil, &MockRegisterResourceEvent{}, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingUpdateSuccessUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	resourceA.Inputs["key"] = resource.NewStringProperty("old")
	resourceANew := NewResource("a")
	resourceANew.Inputs["key"] = resource.NewStringProperty("new")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockSetup(t, snap)
	step := deploy.NewUpdateStep(nil, &MockRegisterResourceEvent{}, resourceA, resourceANew, nil, nil, nil, nil)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingUpdateFailureUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	resourceA.Inputs["key"] = resource.NewStringProperty("old")
	resourceANew := NewResource("a")
	resourceANew.Inputs["key"] = resource.NewStringProperty("new")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockSetup(t, snap)
	step := deploy.NewUpdateStep(nil, &MockRegisterResourceEvent{}, resourceA, resourceANew, nil, nil, nil, nil)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingDeleteSuccessUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockSetup(t, snap)
	step := deploy.NewDeleteStep(nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingDeleteFailureUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockSetup(t, snap)
	step := deploy.NewDeleteStep(nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingReadSuccessNoPreviousResourceUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("b")
	resourceA.ID = "some-b"
	resourceA.External = true
	resourceA.Custom = true
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingReadSuccessPreviousResourceUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("c")
	resourceA.ID = "some-c"
	resourceA.External = true
	resourceA.Custom = true
	resourceA.Inputs["key"] = resource.NewStringProperty("old")
	resourceANew := NewResource("c")
	resourceANew.ID = "some-other-c"
	resourceANew.External = true
	resourceANew.Custom = true
	resourceANew.Inputs["key"] = resource.NewStringProperty("new")

	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, resourceA, resourceANew)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingReadFailureNoPreviousResourceUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("d")
	resourceA.ID = "some-d"
	resourceA.External = true
	resourceA.Custom = true
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRecordingReadFailurePreviousResourceUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("e")
	resourceA.ID = "some-e"
	resourceA.External = true
	resourceA.Custom = true
	resourceA.Inputs["key"] = resource.NewStringProperty("old")
	resourceANew := NewResource("e")
	resourceANew.ID = "some-new-e"
	resourceANew.External = true
	resourceANew.Custom = true
	resourceANew.Inputs["key"] = resource.NewStringProperty("new")

	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, resourceA, resourceANew)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}

func TestRegisterOutputsUnsafe(t *testing.T) {
	t.Setenv(experimentalSnapshotManagerFlag, "1")
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockSetup(t, snap)

	// There should be zero snaps performed at the start.
	assert.Len(t, sp.SavedSnapshots, 0)

	// The step here is not important.
	step := deploy.NewSameStep(nil, nil, resourceA, resourceA)
	err := manager.RegisterResourceOutputs(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = manager.Close()
	assert.NoError(t, err)
	assert.Len(t, sp.SavedSnapshots, 1)
}
