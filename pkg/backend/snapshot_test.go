// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import (
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/version"
	"github.com/stretchr/testify/assert"
)

type MockRegisterResourceEvent struct {
	deploy.SourceEvent
}

func (m MockRegisterResourceEvent) Goal() *resource.Goal               { return nil }
func (m MockRegisterResourceEvent) Done(result *deploy.RegisterResult) {}

type MockStackPersister struct {
	Valid          bool
	SavedSnapshots []*deploy.Snapshot
}

func (m *MockStackPersister) Save(snap *deploy.Snapshot) error {
	m.Valid = true
	m.SavedSnapshots = append(m.SavedSnapshots, snap)
	return nil
}

func (m *MockStackPersister) Invalidate() error {
	m.Valid = false
	return nil
}

func MockSetup(t *testing.T, baseSnap *deploy.Snapshot) (*SnapshotManager, *MockStackPersister) {
	err := baseSnap.VerifyIntegrity()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	sp := &MockStackPersister{}
	return NewSnapshotManager(sp, baseSnap), sp
}

func NewResource(name string, deps ...resource.URN) *resource.State {
	return &resource.State{
		Type:         tokens.Type("test"),
		URN:          resource.URN(name),
		Inputs:       make(resource.PropertyMap),
		Outputs:      make(resource.PropertyMap),
		Dependencies: deps,
	}
}

func NewSnapshot(resources []*resource.State) *deploy.Snapshot {
	return deploy.NewSnapshot(deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}, resources)
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

	// The snapshot manager invalidated the stored snapshot
	assert.False(t, sp.Valid)

	// No mutation was made
	assert.Empty(t, sp.SavedSnapshots)

	err = mutation.End(same, true)
	assert.NoError(t, err)
	assert.True(t, sp.Valid)

	// Sames `do` cause a snapshot mutation as part of `End`.
	assert.NotEmpty(t, sp.SavedSnapshots)
	assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

	// Our same resource should be the first entry in the snapshot list.
	inSnapshot := sp.SavedSnapshots[0].Resources[0]
	assert.Equal(t, sameState.URN, inSnapshot.URN)
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
	assert.False(t, sp.Valid)
	err = mutation.End(bSame, true)
	assert.NoError(t, err)
	assert.True(t, sp.Valid)

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
	assert.False(t, sp.Valid)
	err = mutation.End(aSame, true)
	assert.NoError(t, err)
	assert.True(t, sp.Valid)

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
	createReplacement := deploy.NewCreateReplacementStep(nil, MockRegisterResourceEvent{}, c, cPrime, nil, true)
	replace := deploy.NewReplaceStep(nil, c, cPrime, nil, true)
	c.Delete = true

	applyStep(createReplacement)
	applyStep(replace)

	// cPrime now exists, c is now pending deletion
	// dPrime now depends on cPrime, which got replaced
	dPrime := NewResource(string(d.URN), cPrime.URN)
	applyStep(deploy.NewUpdateStep(nil, MockRegisterResourceEvent{}, d, dPrime, nil))

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
