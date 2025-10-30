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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func MockJournalSetup(t *testing.T, baseSnap *deploy.Snapshot) (engine.SnapshotManager, *MockStackPersister) {
	err := baseSnap.VerifyIntegrity()
	require.NoError(t, err)

	sp := &MockStackPersister{}

	secretsProvider := stack.Base64SecretsProvider{}
	journal, err := NewSnapshotJournaler(
		context.Background(), sp, baseSnap.SecretsManager, secretsProvider, baseSnap)
	require.NoError(t, err)
	snap, err := engine.NewJournalSnapshotManager(journal, baseSnap, baseSnap.SecretsManager)
	require.NoError(t, err)
	return snap, sp
}

func TestIdenticalSamesJournaling(t *testing.T) {
	t.Parallel()

	sameState := NewResource(aUniqueUrn)
	snap := NewSnapshot([]*resource.State{
		sameState,
	})

	manager, sp := MockJournalSetup(t, snap)

	// The engine generates a SameStep on sameState.
	engineGeneratedSame := NewResource(sameState.URN)
	same := deploy.NewSameStep(nil, nil, sameState, engineGeneratedSame)

	mutation, err := manager.BeginMutation(same)
	require.NoError(t, err)
	// No mutation was made
	assert.Empty(t, sp.SavedSnapshots)

	err = mutation.End(same, true)
	require.NoError(t, err)

	// Identical sames do not cause a snapshot mutation as part of `End`.
	assert.Empty(t, sp.SavedSnapshots)

	// Close must write the snapshot.
	err = manager.Close()
	require.NoError(t, err)

	assert.NotEmpty(t, sp.SavedSnapshots)
	assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

	// Our same resource should be the first entry in the snapshot list.
	inSnapshot := sp.SavedSnapshots[0].Resources[0]
	assert.Equal(t, sameState.URN, inSnapshot.URN)
}

func TestSamesWithEmptyDependenciesJournaling(t *testing.T) {
	t.Parallel()

	res := NewResourceWithDeps(aUniqueUrnResourceA, nil)
	snap := NewSnapshot([]*resource.State{
		res,
	})
	manager, sp := MockJournalSetup(t, snap)
	resUpdated := NewResourceWithDeps(res.URN, []resource.URN{})
	same := deploy.NewSameStep(nil, nil, res, resUpdated)
	mutation, err := manager.BeginMutation(same)
	require.NoError(t, err)
	err = mutation.End(same, true)
	require.NoError(t, err)
	require.Len(t, sp.SavedSnapshots, 0, "expected no snapshots to be saved for same step")
}

func TestSamesWithEmptyArraysInInputsJournaling(t *testing.T) {
	t.Parallel()

	// Model reading from state file
	state := map[string]any{"defaults": []any{}}
	inputs, err := stack.DeserializeProperties(state, config.NopDecrypter)
	require.NoError(t, err)

	res := NewResourceWithInputs(aUniqueUrnResourceA, inputs)
	snap := NewSnapshot([]*resource.State{
		res,
	})
	manager, sp := MockJournalSetup(t, snap)

	// Model passing into and back out of RPC layer (e.g. via `Check`)
	marshalledInputs, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{})
	require.NoError(t, err)
	inputsUpdated, err := plugin.UnmarshalProperties(marshalledInputs, plugin.MarshalOptions{})
	require.NoError(t, err)

	resUpdated := NewResourceWithInputs(res.URN, inputsUpdated)
	same := deploy.NewSameStep(nil, nil, res, resUpdated)
	mutation, err := manager.BeginMutation(same)
	require.NoError(t, err)
	err = mutation.End(same, true)
	require.NoError(t, err)
	require.Len(t, sp.SavedSnapshots, 0, "expected no snapshots to be saved for same step")
}

// This test challenges the naive approach of mutating resources
// that are the targets of Same steps in-place by changing the dependencies
// of two resources in the snapshot, which is perfectly legal in our system
// (and in fact is done by the `dependency_steps` integration test as well).
//
// The correctness of the `snap` function in snapshot.go is tested here.
func TestSamesWithDependencyChangesJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource(aUniqueUrnResourceA)
	resourceB := NewResource(aUniqueUrnResourceB, resourceA.URN)

	// The setup: the snapshot contains two resources, A and B, where
	// B depends on A. We're going to begin a mutation in which B no longer
	// depends on A and appears first in program order.
	snap := NewSnapshot([]*resource.State{
		resourceA,
		resourceB,
	})

	manager, sp := MockJournalSetup(t, snap)

	resourceBUpdated := NewResource(resourceB.URN)
	// note: no dependencies

	resourceAUpdated := NewResource(resourceA.URN, resourceBUpdated.URN)
	// note: now depends on B

	// The engine first generates a Same for b:
	bSame := deploy.NewSameStep(nil, nil, resourceB, resourceBUpdated)
	mutation, err := manager.BeginMutation(bSame)
	require.NoError(t, err)
	err = mutation.End(bSame, true)
	require.NoError(t, err)

	// The snapshot should now look like this:
	//   snapshot
	//    resources
	//     b
	//     a
	// where b does not depend on anything and neither does a.
	firstSnap := sp.SavedSnapshots[0]
	require.Len(t, firstSnap.Resources, 2)
	assert.Equal(t, resourceB.URN, firstSnap.Resources[0].URN)
	require.Len(t, firstSnap.Resources[0].Dependencies, 0)
	assert.Equal(t, resourceA.URN, firstSnap.Resources[1].URN)
	require.Len(t, firstSnap.Resources[1].Dependencies, 0)

	// The engine then generates a Same for a:
	aSame := deploy.NewSameStep(nil, nil, resourceA, resourceAUpdated)
	mutation, err = manager.BeginMutation(aSame)
	require.NoError(t, err)
	err = mutation.End(aSame, true)
	require.NoError(t, err)

	// The snapshot should now look like this:
	//   snapshot
	//    resources
	//     b
	//     a
	// where b does not depend on anything and a depends on b.
	secondSnap := sp.SavedSnapshots[1]
	require.Len(t, secondSnap.Resources, 2)
	assert.Equal(t, resourceB.URN, secondSnap.Resources[0].URN)
	require.Len(t, secondSnap.Resources[0].Dependencies, 0)
	assert.Equal(t, resourceA.URN, secondSnap.Resources[1].URN)
	require.Len(t, secondSnap.Resources[1].Dependencies, 1)
	assert.Equal(t, resourceB.URN, secondSnap.Resources[1].Dependencies[0])
}

// This test checks that we only write the Checkpoint once whether or
// not there are important changes when asked to via
// env.SkipCheckpoints.
func TestWriteCheckpointOnceUnsafeJournaling(t *testing.T) {
	t.Setenv(env.SkipCheckpoints.Var().Name(), "1")

	provider := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgUnsafe::provider")
	provider.Custom, provider.Type, provider.ID = true, "pulumi:providers:pkgUnsafe", "id"

	resourceP := NewResource("a-unique-urn-resource-p")
	resourceA := NewResource("a-unique-urn-resource-a")

	snap := NewSnapshot([]*resource.State{
		provider,
		resourceP,
		resourceA,
	})

	manager, sp := MockJournalSetup(t, snap)

	// Generate a same for the provider.
	provUpdated := NewResource(provider.URN)
	provUpdated.Custom, provUpdated.Type = true, provider.Type
	provSame := deploy.NewSameStep(nil, nil, provider, provUpdated)
	mutation, err := manager.BeginMutation(provSame)
	require.NoError(t, err)
	_, _, err = provSame.Apply()
	require.NoError(t, err)
	err = mutation.End(provSame, true)
	require.NoError(t, err)

	// The engine generates a meaningful change, the DEFAULT behavior is that a snapshot is written:
	pUpdated := NewResource(resourceP.URN)
	pUpdated.Protect = !resourceP.Protect
	pSame := deploy.NewSameStep(nil, nil, resourceP, pUpdated)
	mutation, err = manager.BeginMutation(pSame)
	require.NoError(t, err)
	err = mutation.End(pSame, true)
	require.NoError(t, err)

	// The engine generates a meaningful change, the DEFAULT behavior is that a snapshot is written:
	aUpdated := NewResource(resourceA.URN)
	aUpdated.Protect = !resourceA.Protect
	aSame := deploy.NewSameStep(nil, nil, resourceA, aUpdated)
	mutation, err = manager.BeginMutation(aSame)
	require.NoError(t, err)
	err = mutation.End(aSame, true)
	require.NoError(t, err)

	// a `Close()` call is required to write back the snapshots.
	// It is called in all of the references to SnapshotManager.
	err = manager.Close()
	require.NoError(t, err)

	// DEFAULT behavior would cause more than 1 snapshot to be written,
	// but the provided flag should only create 1 Snapshot
	require.Len(t, sp.SavedSnapshots, 1)
}

// This test exercises same steps with meaningful changes to properties _other_ than `Dependencies` in order to ensure
// that the snapshot is written.
func TestSamesWithOtherMeaningfulChangesJournaling(t *testing.T) {
	t.Parallel()

	provider := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgA::provider")
	provider.Custom, provider.Type, provider.ID = true, "pulumi:providers:pkgA", "id"

	resourceP := NewResource(aUniqueUrnResourceP)
	resourceA := NewResource(aUniqueUrnResourceA)

	var changes []*resource.State

	// Change the "custom" bit.
	changes = append(changes, NewResource(resourceA.URN))
	changes[0].Custom, changes[0].Provider = true, "urn:pulumi:foo::bar::pulumi:providers:pkgA::provider::id"

	// Change the parent, this also has to change the URN.
	changes = append(changes, NewResource(resourceA.URN))
	changes[1].URN = resource.NewURN(
		resourceA.URN.Stack(), resourceA.URN.Project(),
		resourceP.URN.QualifiedType(), resourceA.URN.Type(),
		resourceA.URN.Name())
	changes[1].Parent = resourceP.URN

	// Change the "protect" bit.
	changes = append(changes, NewResource(resourceA.URN))
	changes[2].Protect = !resourceA.Protect

	// Change the resource outputs.
	changes = append(changes, NewResource(resourceA.URN))
	changes[3].Outputs = resource.PropertyMap{"foo": resource.NewProperty("bar")}

	snap := NewSnapshot([]*resource.State{
		provider,
		resourceP,
		resourceA,
	})

	for _, c := range changes {
		manager, sp := MockJournalSetup(t, snap)

		// Generate a same for the provider.
		provUpdated := NewResource(provider.URN)
		provUpdated.Custom, provUpdated.Type = true, provider.Type
		provSame := deploy.NewSameStep(nil, nil, provider, provUpdated)
		mutation, err := manager.BeginMutation(provSame)
		require.NoError(t, err)
		_, _, err = provSame.Apply()
		require.NoError(t, err)
		err = mutation.End(provSame, true)
		require.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for p. This is not a meaningful change, so the snapshot is not written.
		pUpdated := NewResource(resourceP.URN)
		pSame := deploy.NewSameStep(nil, nil, resourceP, pUpdated)
		mutation, err = manager.BeginMutation(pSame)
		require.NoError(t, err)
		err = mutation.End(pSame, true)
		require.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for a. Because this is a meaningful change, the snapshot is written:
		aSame := deploy.NewSameStep(nil, nil, resourceA, c)
		mutation, err = manager.BeginMutation(aSame)
		require.NoError(t, err)
		err = mutation.End(aSame, true)
		require.NoError(t, err)

		assert.NotEmpty(t, sp.SavedSnapshots)
		assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

		inSnapshot := sp.SavedSnapshots[0].Resources[2]
		// The snapshot might edit the URN so don't check against that
		c.URN = inSnapshot.URN
		assert.Equal(t, c, inSnapshot)

		err = manager.Close()
		require.NoError(t, err)
	}

	// Source position is not a meaningful change, and we batch them up for performance reasons
	manager, sp := MockJournalSetup(t, snap)
	sourceUpdated := NewResource(resourceA.URN)
	sourceUpdated.SourcePosition = "project:///foo.ts#1,2"
	sourceUpdatedSame := deploy.NewSameStep(nil, nil, resourceA, sourceUpdated)
	mutation, err := manager.BeginMutation(sourceUpdatedSame)
	require.NoError(t, err)
	_, _, err = sourceUpdatedSame.Apply()
	require.NoError(t, err)
	err = mutation.End(sourceUpdatedSame, true)
	require.NoError(t, err)
	assert.Empty(t, sp.SavedSnapshots)

	// It should still write on close
	err = manager.Close()
	require.NoError(t, err)

	assert.NotEmpty(t, sp.SavedSnapshots)
	assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)
	inSnapshot := sp.SavedSnapshots[0].Resources[0]
	assert.Equal(t, sourceUpdated, inSnapshot)

	// Set up a second provider and change the resource's provider reference.
	provider2 := NewResource("urn:pulumi:foo::bar::pulumi:providers:pkgA::provider2")
	provider2.Custom, provider2.Type, provider2.ID = true, "pulumi:providers:pkgA", "id2"

	resourceA.Custom = true
	resourceA.ID = "id"
	resourceA.Provider = "urn:pulumi:foo::bar::pulumi:providers:pkgA::provider::id"

	snap = NewSnapshot([]*resource.State{
		provider,
		provider2,
		resourceA,
	})

	changes = []*resource.State{NewResource(resourceA.URN)}
	changes[0].Custom, changes[0].Provider = true, "urn:pulumi:foo::bar::pulumi:providers:pkgA::provider2::id2"

	for _, c := range changes {
		manager, sp := MockJournalSetup(t, snap)

		// Generate sames for the providers.
		provUpdated := NewResource(provider.URN)
		provUpdated.Custom, provUpdated.Type = true, provider.Type
		provSame := deploy.NewSameStep(nil, nil, provider, provUpdated)
		mutation, err := manager.BeginMutation(provSame)
		require.NoError(t, err)
		_, _, err = provSame.Apply()
		require.NoError(t, err)
		err = mutation.End(provSame, true)
		require.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for p. This is not a meaningful change, so the snapshot is not written.
		prov2Updated := NewResource(provider2.URN)
		prov2Updated.Custom, prov2Updated.Type = true, provider.Type
		prov2Same := deploy.NewSameStep(nil, nil, provider2, prov2Updated)
		mutation, err = manager.BeginMutation(prov2Same)
		require.NoError(t, err)
		_, _, err = prov2Same.Apply()
		require.NoError(t, err)
		err = mutation.End(prov2Same, true)
		require.NoError(t, err)
		assert.Empty(t, sp.SavedSnapshots)

		// The engine generates a Same for a. Because this is a meaningful change, the snapshot is written:
		aSame := deploy.NewSameStep(nil, nil, resourceA, c)
		mutation, err = manager.BeginMutation(aSame)
		require.NoError(t, err)
		_, _, err = aSame.Apply()
		require.NoError(t, err)
		err = mutation.End(aSame, true)
		require.NoError(t, err)

		assert.NotEmpty(t, sp.SavedSnapshots)
		assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

		inSnapshot := sp.SavedSnapshots[0].Resources[2]
		assert.Equal(t, c, inSnapshot)

		err = manager.Close()
		require.NoError(t, err)
	}
}

// This test exercises the merge operation with a particularly vexing deployment
// state that was useful in shaking out bugs.
func TestVexingDeploymentJournaling(t *testing.T) {
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

	manager, sp := MockJournalSetup(t, snap)

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
		require.NoError(t, err)

		err = mutation.End(step, true)
		require.NoError(t, err)
	}

	// b now depends on nothing
	bPrime := NewResource(b.URN)
	applyStep(deploy.NewSameStep(nil, MockRegisterResourceEvent{}, b, bPrime))

	// c now only depends on b
	cPrime := NewResource(c.URN, bPrime.URN)

	// mocking out the behavior of a provider indicating that this resource needs to be deleted
	createReplacement := deploy.NewCreateReplacementStep(nil, MockRegisterResourceEvent{}, c, cPrime, nil, nil, nil, true)
	replace := deploy.NewReplaceStep(nil, c, cPrime, nil, nil, nil, true)
	c.Delete = true

	applyStep(createReplacement)
	applyStep(replace)

	// cPrime now exists, c is now pending deletion
	// dPrime now depends on cPrime, which got replaced
	dPrime := NewResource(d.URN, cPrime.URN)
	applyStep(deploy.NewUpdateStep(nil, MockRegisterResourceEvent{}, d, dPrime, nil, nil, nil, nil, nil))

	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	require.Len(t, lastSnap.Resources, 6)
	res := lastSnap.Resources

	// Here's what the merged snapshot should look like:
	// B should be first, and it should depend on nothing
	assert.Equal(t, b.URN, res[0].URN)
	require.Len(t, res[0].Dependencies, 0)

	// cPrime should be next, and it should depend on B
	assert.Equal(t, c.URN, res[1].URN)
	require.Len(t, res[1].Dependencies, 1)
	assert.Equal(t, b.URN, res[1].Dependencies[0])

	// d should be next, and it should depend on cPrime
	assert.Equal(t, d.URN, res[2].URN)
	require.Len(t, res[2].Dependencies, 1)
	assert.Equal(t, c.URN, res[2].Dependencies[0])

	// a should be next, and it should depend on nothing
	assert.Equal(t, a.URN, res[3].URN)
	require.Len(t, res[3].Dependencies, 0)

	// c should be next, it should depend on A and B and should be pending deletion
	// this is a critical operation of snap and the crux of this test:
	// merge MUST put c after a in the snapshot, despite never having seen a in the current plan
	assert.Equal(t, c.URN, res[4].URN)
	assert.True(t, res[4].Delete)
	require.Len(t, res[4].Dependencies, 2)
	assert.Contains(t, res[4].Dependencies, a.URN)
	assert.Contains(t, res[4].Dependencies, b.URN)

	// e should be last, it should depend on C and still be live
	assert.Equal(t, e.URN, res[5].URN)
	require.Len(t, res[5].Dependencies, 1)
	assert.Equal(t, c.URN, res[5].Dependencies[0])
}

func TestDeletionJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewDeleteStep(nil, map[resource.URN]bool{}, resourceA, nil)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	err = mutation.End(step, true)
	require.NoError(t, err)

	// the end mutation should mark the resource as "done".
	// snap should then not put resourceA in the merged snapshot, since it has been deleted.
	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	require.Len(t, lastSnap.Resources, 0)
}

func TestFailedDeleteJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewDeleteStep(nil, map[resource.URN]bool{}, resourceA, nil)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// since we marked the mutation as not successful, the snapshot should still contain
	// the resource we failed to delete.
	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	require.Len(t, lastSnap.Resources, 1)
	assert.Equal(t, resourceA.URN, lastSnap.Resources[0].URN)
}

func TestRecordingCreateSuccessJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot(nil)
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewCreateStep(nil, &MockRegisterResourceEvent{}, resourceA)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the create step mutation should have placed a pending "creating" operation
	// into the operations list
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeCreating, snap.PendingOperations[0].Type)

	err = mutation.End(step, true /* successful */)
	require.NoError(t, err)

	// A successful creation should remove the "creating" operation from the operations list
	// and persist the created resource in the snapshot.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestRecordingCreateFailureJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot(nil)
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewCreateStep(nil, &MockRegisterResourceEvent{}, resourceA)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the create step mutation should have placed a pending "creating" operation
	// into the operations list
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeCreating, snap.PendingOperations[0].Type)

	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// A failed creation should remove the "creating" operation from the operations list
	// and not persist the created resource in the snapshot.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 0)
}

func TestRecordingUpdateSuccessJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	resourceA.Inputs["key"] = resource.NewProperty("old")
	resourceANew := NewResource("a")
	resourceANew.Inputs["key"] = resource.NewProperty("new")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewUpdateStep(nil, &MockRegisterResourceEvent{}, resourceA, resourceANew, nil, nil, nil, nil, nil)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the update mutation should have placed a pending "updating" operation into
	// the operations list, with the resource's new inputs.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeUpdating, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])

	err = mutation.End(step, true /* successful */)
	require.NoError(t, err)

	// Completing the update should place the resource with the new inputs into the snapshot and clear the in
	// flight operation.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewProperty("new"), snap.Resources[0].Inputs["key"])
}

func TestRecordingUpdateFailureJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	resourceA.Inputs["key"] = resource.NewProperty("old")
	resourceANew := NewResource("a")
	resourceANew.Inputs["key"] = resource.NewProperty("new")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})

	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewUpdateStep(nil, &MockRegisterResourceEvent{}, resourceA, resourceANew, nil, nil, nil, nil, nil)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the update mutation should have placed a pending "updating" operation into
	// the operations list, with the resource's new inputs.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeUpdating, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])

	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// Failing the update should keep the old resource with old inputs in the snapshot while clearing the
	// in flight operation.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewProperty("old"), snap.Resources[0].Inputs["key"])
}

func TestRecordingDeleteSuccessJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewDeleteStep(nil, map[resource.URN]bool{}, resourceA, nil)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the delete mutation should have placed a pending "deleting" operation into the operations list.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeDeleting, snap.PendingOperations[0].Type)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	err = mutation.End(step, true /* successful */)
	require.NoError(t, err)

	// A successful delete should remove the in flight operation and deleted resource from the snapshot.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 0)
}

func TestRecordingDeleteFailureJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewDeleteStep(nil, map[resource.URN]bool{}, resourceA, nil)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the delete mutation should have placed a pending "deleting" operation into the operations list.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeDeleting, snap.PendingOperations[0].Type)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// A failed delete should remove the in flight operation but leave the resource in the snapshot.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestRecordingReadSuccessNoPreviousResourceJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("b")
	resourceA.ID = "some-b"
	resourceA.External = true
	resourceA.Custom = true
	snap := NewSnapshot(nil)
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	err = mutation.End(step, true /* successful */)
	require.NoError(t, err)

	// A successful read should clear the in flight operation and put the new resource into the snapshot
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestRecordingReadSuccessPreviousResourceJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("c")
	resourceA.ID = "some-c"
	resourceA.External = true
	resourceA.Custom = true
	resourceA.Inputs["key"] = resource.NewProperty("old")
	resourceANew := NewResource("c")
	resourceANew.ID = "some-other-c"
	resourceANew.External = true
	resourceANew.Custom = true
	resourceANew.Inputs["key"] = resource.NewProperty("new")

	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, resourceA, resourceANew)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list
	// with the inputs of the new read
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewProperty("old"), snap.Resources[0].Inputs["key"])
	err = mutation.End(step, true /* successful */)
	require.NoError(t, err)

	// A successful read should clear the in flight operation and replace the existing resource in the snapshot.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewProperty("new"), snap.Resources[0].Inputs["key"])
}

func TestRecordingReadFailureNoPreviousResourceJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("d")
	resourceA.ID = "some-d"
	resourceA.External = true
	resourceA.Custom = true
	snap := NewSnapshot(nil)
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// A failed read should clear the in flight operation and leave the snapshot empty.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 0)
	require.Len(t, snap.PendingOperations, 0)
}

func TestRecordingReadFailurePreviousResourceJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("e")
	resourceA.ID = "some-e"
	resourceA.External = true
	resourceA.Custom = true
	resourceA.Inputs["key"] = resource.NewProperty("old")
	resourceANew := NewResource("e")
	resourceANew.ID = "some-new-e"
	resourceANew.External = true
	resourceANew.Custom = true
	resourceANew.Inputs["key"] = resource.NewProperty("new")

	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewReadStep(nil, nil, resourceA, resourceANew)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// Beginning the read mutation should have placed a pending "reading" operation into the operations list
	// with the inputs of the new read
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 1)
	assert.Equal(t, resourceA.URN, snap.PendingOperations[0].Resource.URN)
	assert.Equal(t, resource.OperationTypeReading, snap.PendingOperations[0].Type)
	assert.Equal(t, resource.NewProperty("new"), snap.PendingOperations[0].Resource.Inputs["key"])
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewProperty("old"), snap.Resources[0].Inputs["key"])
	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// A failed read should clear the in flight operation and leave the existing read in the snapshot with the
	// old inputs.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.NewProperty("old"), snap.Resources[0].Inputs["key"])
}

func TestRegisterOutputsJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockJournalSetup(t, snap)

	// There should be zero snaps performed at the start.
	require.Empty(t, sp.SavedSnapshots)

	// The step here is not important.
	resACopy := resourceA.Copy()
	step := deploy.NewSameStep(nil, nil, resourceA, resACopy)
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)
	require.NoError(t, mutation.End(step, true))
	err = manager.RegisterResourceOutputs(step)
	require.NoError(t, err)

	// The RegisterResourceOutputs should not have caused a snapshot to be written.
	require.Empty(t, sp.SavedSnapshots)

	// Now, change the outputs and issue another RRO.
	resourceA2 := NewResource("a")
	resourceA2.Outputs = resource.PropertyMap{"hello": resource.NewProperty("world")}
	step = deploy.NewSameStep(nil, nil, resACopy, resourceA2)
	mutation, err = manager.BeginMutation(step)
	require.NoError(t, err)
	require.NoError(t, mutation.End(step, true))
	err = manager.RegisterResourceOutputs(step)
	require.NoError(t, err)

	// The new outputs should have been saved.
	require.Len(t, sp.SavedSnapshots, 2)

	// It should be identical to what has already been written.
	lastSnap := sp.LastSnap()
	require.Len(t, lastSnap.Resources, 1)
	assert.Equal(t, resourceA.URN, lastSnap.Resources[0].URN)
	assert.Equal(t, resourceA2.Outputs, lastSnap.Resources[0].Outputs)
}

func TestRecordingSameFailureJournaling(t *testing.T) {
	t.Parallel()

	resourceA := NewResource("a")
	snap := NewSnapshot([]*resource.State{
		resourceA,
	})
	manager, sp := MockJournalSetup(t, snap)
	step := deploy.NewSameStep(nil, nil, resourceA, resourceA.Copy())
	mutation, err := manager.BeginMutation(step)
	require.NoError(t, err)

	// There should be zero snaps performed at the start.
	require.Len(t, sp.SavedSnapshots, 0)

	err = mutation.End(step, false /* successful */)
	require.NoError(t, err)

	// A failed same should leave the resource in the snapshot.
	snap = sp.LastSnap()
	require.Len(t, snap.Resources, 1)
	require.Len(t, snap.PendingOperations, 0)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
}

func TestSnapshotIntegrityErrorMetadataIsWrittenForInvalidSnapshotsJournaling(t *testing.T) {
	t.Parallel()

	// The dependency "b" does not exist in the snapshot, so we'll get a missing
	// dependency error when we try to save the snapshot.
	r := NewResource("a", "b")
	snap := NewSnapshot([]*resource.State{r})
	sp := &MockStackPersister{}
	secretsProvider := stack.Base64SecretsProvider{}
	journal, err := NewSnapshotJournaler(
		context.Background(), sp, snap.SecretsManager, secretsProvider, snap)
	require.NoError(t, err)

	sm, err := engine.NewJournalSnapshotManager(journal, snap, snap.SecretsManager)
	require.NoError(t, err)

	err = sm.Close()

	assert.ErrorContains(t, err, "failed to verify snapshot")
	require.NotNil(t, sp.LastSnap().Metadata.IntegrityErrorMetadata)
}

func TestSnapshotIntegrityErrorMetadataIsClearedForValidSnapshotsJournaling(t *testing.T) {
	t.Parallel()

	r := NewResource("a")

	snap := NewSnapshot([]*resource.State{r})
	snap.Metadata.IntegrityErrorMetadata = &deploy.SnapshotIntegrityErrorMetadata{}

	sp := &MockStackPersister{}
	secretsProvider := stack.Base64SecretsProvider{}
	journal, err := NewSnapshotJournaler(
		context.Background(), sp, snap.SecretsManager, secretsProvider, snap)
	require.NoError(t, err)

	sm, err := engine.NewJournalSnapshotManager(journal, snap, snap.SecretsManager)
	require.NoError(t, err)

	err = sm.Close()

	require.NoError(t, err)
	assert.Nil(t, sp.LastSnap().Metadata.IntegrityErrorMetadata)
}

//nolint:paralleltest // mutates global state
func TestSnapshotIntegrityErrorMetadataIsWrittenForInvalidSnapshotsChecksDisabledJournaling(t *testing.T) {
	old := DisableIntegrityChecking
	DisableIntegrityChecking = true
	defer func() { DisableIntegrityChecking = old }()

	// The dependency "b" does not exist in the snapshot, so we'll get a missing
	// dependency error when we try to save the snapshot.
	r := NewResource("a", "b")
	snap := NewSnapshot([]*resource.State{r})
	sp := &MockStackPersister{}
	secretsProvider := stack.Base64SecretsProvider{}
	journal, err := NewSnapshotJournaler(
		context.Background(), sp, snap.SecretsManager, secretsProvider, snap)
	require.NoError(t, err)
	sm, err := engine.NewJournalSnapshotManager(journal, snap, snap.SecretsManager)
	require.NoError(t, err)

	err = sm.Close()

	require.NoError(t, err)
	require.NotNil(t, sp.LastSnap().Metadata.IntegrityErrorMetadata)
}

//nolint:paralleltest // mutates global state
func TestSnapshotIntegrityErrorMetadataIsClearedForValidSnapshotsChecksDisabledJournaling(t *testing.T) {
	old := DisableIntegrityChecking
	DisableIntegrityChecking = true
	defer func() { DisableIntegrityChecking = old }()

	// The dependency "b" does not exist in the snapshot, so we'll get a missing
	// dependency error when we try to save the snapshot.
	r := NewResource("a")
	snap := NewSnapshot([]*resource.State{r})
	sp := &MockStackPersister{}
	secretsProvider := stack.Base64SecretsProvider{}
	journal, err := NewSnapshotJournaler(
		context.Background(), sp, snap.SecretsManager, secretsProvider, snap)
	require.NoError(t, err)
	sm, err := engine.NewJournalSnapshotManager(journal, snap, snap.SecretsManager)
	require.NoError(t, err)

	err = sm.Close()

	require.NoError(t, err)
	assert.Nil(t, sp.LastSnap().Metadata.IntegrityErrorMetadata)
}
