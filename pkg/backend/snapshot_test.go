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

package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type MockRegisterResourceEvent struct {
	deploy.SourceEvent
}

func (m MockRegisterResourceEvent) Goal() *resource.Goal               { return nil }
func (m MockRegisterResourceEvent) Done(result *deploy.RegisterResult) {}

type MockStackPersister struct {
	SavedSnapshots []*deploy.Snapshot
}

func (m *MockStackPersister) Save(snap *deploy.Snapshot) error {
	m.SavedSnapshots = append(m.SavedSnapshots, snap)
	return nil
}

func (m *MockStackPersister) SecretsManager() secrets.Manager {
	return b64.NewBase64SecretsManager()
}

func (m *MockStackPersister) LastSnap() *deploy.Snapshot {
	return m.SavedSnapshots[len(m.SavedSnapshots)-1]
}

func MockSetup(t *testing.T, baseSnap *deploy.Snapshot) (*SnapshotManager, *MockStackPersister) {
	err := baseSnap.VerifyIntegrity()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	sp := &MockStackPersister{}
	return NewSnapshotManager(sp, baseSnap), sp
}

func NewResourceWithDeps(name string, deps []resource.URN) *resource.State {
	return &resource.State{
		Type:         tokens.Type("test"),
		URN:          resource.URN(name),
		Inputs:       make(resource.PropertyMap),
		Outputs:      make(resource.PropertyMap),
		Dependencies: deps,
	}
}

func NewResourceWithInputs(name string, inputs resource.PropertyMap) *resource.State {
	return &resource.State{
		Type:         tokens.Type("test"),
		URN:          resource.URN(name),
		Inputs:       inputs,
		Outputs:      make(resource.PropertyMap),
		Dependencies: []resource.URN{},
	}
}

func NewResource(name string, deps ...resource.URN) *resource.State {
	return NewResourceWithDeps(name, deps)
}

func NewSnapshot(resources []*resource.State) *deploy.Snapshot {
	return deploy.NewSnapshot(deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, b64.NewBase64SecretsManager(), resources, nil)
}

func TestIdenticalSames(t *testing.T) {
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

	assert.NotEmpty(t, sp.SavedSnapshots)
	assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

	// Our same resource should be the first entry in the snapshot list.
	inSnapshot := sp.SavedSnapshots[0].Resources[0]
	assert.Equal(t, sameState.URN, inSnapshot.URN)
}

func TestSamesWithEmptyDependencies(t *testing.T) {
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
	assert.Len(t, sp.SavedSnapshots, 0, "expected no snapshots to be saved for same step")
}

func TestSamesWithEmptyArraysInInputs(t *testing.T) {
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
	assert.Len(t, sp.SavedSnapshots, 0, "expected no snapshots to be saved for same step")
}

// This test challenges the naive approach of mutating resources
// that are the targets of Same steps in-place by changing the dependencies
// of two resources in the snapshot, which is perfectly legal in our system
// (and in fact is done by the `dependency_steps` integration test as well).
//
// The correctness of the `snap` function in snapshot.go is tested here.
func TestSamesWithDependencyChanges(t *testing.T) {
	resourceA := NewResource("a-unique-urn-resource-a")
	resourceB := NewResource("a-unique-urn-resource-b", resourceA.URN)

	// The setup: the snapshot contains two resources, A and B, where
	// B depends on A. We're going to begin a mutation in which B no longer
	// depends on A and appears first in program order.
	snap := NewSnapshot([]*resource.State{
		resourceA,
		resourceB,
	})

	manager, sp := MockSetup(t, snap)

	resourceBUpdated := NewResource(string(resourceB.URN))
	// note: no dependencies

	resourceAUpdated := NewResource(string(resourceA.URN), resourceBUpdated.URN)
	// note: now depends on B

	// The engine first generates a Same for b:
	bSame := deploy.NewSameStep(nil, nil, resourceB, resourceBUpdated)
	mutation, err := manager.BeginMutation(bSame)
	assert.NoError(t, err)
	err = mutation.End(bSame, true)
	assert.NoError(t, err)

	// The snapshot should now look like this:
	//   snapshot
	//    resources
	//     b
	//     a
	// where b does not depend on anything and neither does a.
	firstSnap := sp.SavedSnapshots[0]
	assert.Len(t, firstSnap.Resources, 2)
	assert.Equal(t, resourceB.URN, firstSnap.Resources[0].URN)
	assert.Len(t, firstSnap.Resources[0].Dependencies, 0)
	assert.Equal(t, resourceA.URN, firstSnap.Resources[1].URN)
	assert.Len(t, firstSnap.Resources[1].Dependencies, 0)

	// The engine then generates a Same for a:
	aSame := deploy.NewSameStep(nil, nil, resourceA, resourceAUpdated)
	mutation, err = manager.BeginMutation(aSame)
	assert.NoError(t, err)
	err = mutation.End(aSame, true)
	assert.NoError(t, err)

	// The snapshot should now look like this:
	//   snapshot
	//    resources
	//     b
	//     a
	// where b does not depend on anything and a depends on b.
	secondSnap := sp.SavedSnapshots[1]
	assert.Len(t, secondSnap.Resources, 2)
	assert.Equal(t, resourceB.URN, secondSnap.Resources[0].URN)
	assert.Len(t, secondSnap.Resources[0].Dependencies, 0)
	assert.Equal(t, resourceA.URN, secondSnap.Resources[1].URN)
	assert.Len(t, secondSnap.Resources[1].Dependencies, 1)
	assert.Equal(t, resourceB.URN, secondSnap.Resources[1].Dependencies[0])
}

// This test exercises same steps with meaningful changes to properties _other_ than `Dependencies` in order to ensure
// that the snapshot is written.
func TestSamesWithOtherMeaningfulChanges(t *testing.T) {
	provider := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgA::provider")
	provider.Custom, provider.Type, provider.ID = true, "pulumi:providers:pkgA", "id"

	resourceP := NewResource("a-unique-urn-resource-p")
	resourceA := NewResource("a-unique-urn-resource-a")

	var changes []*resource.State

	// Change the "custom" bit.
	changes = append(changes, NewResource(string(resourceA.URN)))
	changes[0].Custom, changes[0].Provider = true, "urn:pulumi:foo::bar::pulumi:providers:pkgA::provider::id"

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

		assert.NotEmpty(t, sp.SavedSnapshots)
		assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

		inSnapshot := sp.SavedSnapshots[0].Resources[2]
		assert.Equal(t, c, inSnapshot)

		err = manager.Close()
		assert.NoError(t, err)
	}

	// Set up a second provider and change the resource's provider reference.
	provider2 := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgA::provider2")
	provider2.Custom, provider2.Type, provider2.ID = true, "pulumi:providers:pkgA", "id2"

	resourceA.Custom, resourceA.ID, resourceA.Provider =
		true, "id", "urn:pulumi:foo::bar::pulumi:providers:pkgA::provider::id"

	snap = NewSnapshot([]*resource.State{
		provider,
		provider2,
		resourceA,
	})

	changes = []*resource.State{NewResource(string(resourceA.URN))}
	changes[0].Custom, changes[0].Provider = true, "urn:pulumi:foo::bar::pulumi:providers:pkgA::provider2::id2"

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

		assert.NotEmpty(t, sp.SavedSnapshots)
		assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

		inSnapshot := sp.SavedSnapshots[0].Resources[2]
		assert.Equal(t, c, inSnapshot)

		err = manager.Close()
		assert.NoError(t, err)
	}
}

// This test exercises the merge operation with a particularly vexing deployment
// state that was useful in shaking out bugs.
func TestVexingDeployment(t *testing.T) {
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

	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	assert.Len(t, lastSnap.Resources, 6)
	res := lastSnap.Resources

	// Here's what the merged snapshot should look like:
	// B should be first, and it should depend on nothing
	assert.Equal(t, b.URN, res[0].URN)
	assert.Len(t, res[0].Dependencies, 0)

	// cPrime should be next, and it should depend on B
	assert.Equal(t, c.URN, res[1].URN)
	assert.Len(t, res[1].Dependencies, 1)
	assert.Equal(t, b.URN, res[1].Dependencies[0])

	// d should be next, and it should depend on cPrime
	assert.Equal(t, d.URN, res[2].URN)
	assert.Len(t, res[2].Dependencies, 1)
	assert.Equal(t, c.URN, res[2].Dependencies[0])

	// a should be next, and it should depend on nothing
	assert.Equal(t, a.URN, res[3].URN)
	assert.Len(t, res[3].Dependencies, 0)

	// c should be next, it should depend on A and B and should be pending deletion
	// this is a critical operation of snap and the crux of this test:
	// merge MUST put c after a in the snapshot, despite never having seen a in the current plan
	assert.Equal(t, c.URN, res[4].URN)
	assert.True(t, res[4].Delete)
	assert.Len(t, res[4].Dependencies, 2)
	assert.Contains(t, res[4].Dependencies, a.URN)
	assert.Contains(t, res[4].Dependencies, b.URN)

	// e should be last, it should depend on C and still be live
	assert.Equal(t, e.URN, res[5].URN)
	assert.Len(t, res[5].Dependencies, 1)
	assert.Equal(t, c.URN, res[5].Dependencies[0])
}

func TestDeletion(t *testing.T) {
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

	// the end mutation should mark the resource as "done".
	// snap should then not put resourceA in the merged snapshot, since it has been deleted.
	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	assert.Len(t, lastSnap.Resources, 0)
}

func TestFailedDelete(t *testing.T) {
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

	// since we marked the mutation as not successful, the snapshot should still contain
	// the resource we failed to delete.
	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	assert.Len(t, lastSnap.Resources, 1)
	assert.Equal(t, resourceA.URN, lastSnap.Resources[0].URN)
}

func TestRecordingCreateSuccess(t *testing.T) {
	resourceA := NewResource("a")
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewCreateStep(nil, &MockRegisterResourceEvent{}, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Beginning the create step mutation should have placed a pending "creating" operation
	// into the operations list
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeCreating, snap.PendingOperations[0].Type)

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A successful creation should remove the "creating" operation from the operations list
	// and persist the created resource in the snapshot.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestRecordingCreateFailure(t *testing.T) {
	resourceA := NewResource("a")
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewCreateStep(nil, &MockRegisterResourceEvent{}, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Beginning the create step mutation should have placed a pending "creating" operation
	// into the operations list
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeCreating, snap.PendingOperations[0].Type)

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A failed creation should remove the "creating" operation from the operations list
	// and not persist the created resource in the snapshot.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 0)
}

func TestRecordingUpdateSuccess(t *testing.T) {
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

	// Beginning the update mutation should have placed a pending "updating" operation into
	// the operations list, with the resource's new inputs.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeUpdating, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewStringProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])

	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Completing the update should place the resource with the new inputs into the snapshot and clear the in
	// flight operation.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewStringProperty("new"), snap.Resources[0].Inputs["key"])
}

func TestRecordingUpdateFailure(t *testing.T) {
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

	// Beginning the update mutation should have placed a pending "updating" operation into
	// the operations list, with the resource's new inputs.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeUpdating, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewStringProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])

	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Failing the update should keep the old resource with old inputs in the snapshot while clearing the
	// in flight operation.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewStringProperty("old"), snap.Resources[0].Inputs["key"])
}

func TestRecordingDeleteSuccess(t *testing.T) {
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

	// Beginning the delete mutation should have placed a pending "deleting" operation into the operations list.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeDeleting, snap.PendingOperations[0].Type)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A successful delete should remove the in flight operation and deleted resource from the snapshot.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 0)
}

func TestRecordingDeleteFailure(t *testing.T) {
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

	// Beginning the delete mutation should have placed a pending "deleting" operation into the operations list.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeDeleting, snap.PendingOperations[0].Type)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A failed delete should remove the in flight operation but leave the resource in the snapshot.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestRecordingReadSuccessNoPreviousResource(t *testing.T) {
	resourceA := NewResource("a")
	resourceA.External = true
	resourceA.Custom = true
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A successful read should clear the in flight operation and put the new resource into the snapshot
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestRecordingReadSuccessPreviousResource(t *testing.T) {
	resourceA := NewResource("a")
	resourceA.External = true
	resourceA.Custom = true
	resourceA.Inputs["key"] = resource.NewStringProperty("old")
	resourceANew := NewResource("a")
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

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list
	// with the inputs of the new read
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewStringProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewStringProperty("old"), snap.Resources[0].Inputs["key"])
	err = mutation.End(step, true /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A successful read should clear the in flight operation and replace the existing resource in the snapshot.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewStringProperty("new"), snap.Resources[0].Inputs["key"])
}

func TestRecordingReadFailureNoPreviousResource(t *testing.T) {
	resourceA := NewResource("a")
	resourceA.External = true
	resourceA.Custom = true
	snap := NewSnapshot(nil)
	manager, sp := MockSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A failed read should clear the in flight operation and leave the snapshot empty.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 0)
	assert.Len(t, snap.PendingOperations, 0)
}

func TestRecordingReadFailurePreviousResource(t *testing.T) {
	resourceA := NewResource("a")
	resourceA.External = true
	resourceA.Custom = true
	resourceA.Inputs["key"] = resource.NewStringProperty("old")
	resourceANew := NewResource("a")
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

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list
	// with the inputs of the new read
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewStringProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewStringProperty("old"), snap.Resources[0].Inputs["key"])
	err = mutation.End(step, false /* successful */)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// A failed read should clear the in flight operation and leave the existing read in the snapshot with the
	// old inputs.
	snap = sp.LastSnap()
	assert.Len(t, snap.Resources, 1)
	assert.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewStringProperty("old"), snap.Resources[0].Inputs["key"])
}

func TestRegisterOutputs(t *testing.T) {
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

	// The RegisterResourceOutputs should have caused a snapshot to be written.
	assert.Len(t, sp.SavedSnapshots, 1)

	// It should be identical to what has already been written.
	lastSnap := sp.LastSnap()
	assert.Len(t, lastSnap.Resources, 1)
	assert.Equal(t, resourceA.URN, lastSnap.Resources[0].URN)
}
