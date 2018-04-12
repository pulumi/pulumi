// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import (
	"fmt"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/version"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

type MockRegisterResourceEvent struct {
	deploy.SourceEvent
}

func (m MockRegisterResourceEvent) Goal() *resource.Goal               { return nil }
func (m MockRegisterResourceEvent) Done(result *deploy.RegisterResult) {}

type MockStackPersister struct {
	SavedSnapshots []*deploy.Snapshot
}

func (m *MockStackPersister) SaveSnapshot(snap *deploy.Snapshot) error {
	m.SavedSnapshots = append(m.SavedSnapshots, snap)
	return nil
}

type MockUpdateInfo struct {
	StackName string
	Snapshot  *deploy.Snapshot
}

func (m *MockUpdateInfo) GetRoot() string                { return "mocked" }
func (m *MockUpdateInfo) GetProject() *workspace.Project { return nil }
func (m *MockUpdateInfo) GetTarget() *deploy.Target {
	return &deploy.Target{
		Name:      tokens.QName(m.StackName),
		Config:    make(config.Map),
		Decrypter: config.NopDecrypter,
		Snapshot:  m.Snapshot,
	}
}

func MockSetup(t *testing.T, name string, initialResources []*resource.State) (*SnapshotManager, *MockStackPersister) {
	manifest := deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		Plugins: nil,
	}
	manifest.Magic = manifest.NewMagic()
	ui := &MockUpdateInfo{
		StackName: name,
		Snapshot:  deploy.NewSnapshot(tokens.QName(name), manifest, initialResources),
	}

	err := ui.Snapshot.VerifyIntegrity()
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	sp := &MockStackPersister{}
	return NewSnapshotManager(sp, ui), sp
}

func NewResource(name string, status resource.MutationStatus, deps ...resource.URN) *resource.State {
	return &resource.State{
		Type:         tokens.Type("test"),
		URN:          resource.URN(name),
		Inputs:       make(resource.PropertyMap),
		Outputs:      make(resource.PropertyMap),
		Status:       status,
		Dependencies: deps,
	}
}

func TestIdenticalSames(t *testing.T) {
	sameState := NewResource("a-unique-urn", resource.ResourceStatusCreated)
	manager, sp := MockSetup(t, "test", []*resource.State{
		sameState,
	})

	same := deploy.NewSameStep(nil, nil, sameState.Clone(), sameState.Clone())

	mutation, err := manager.BeginMutation(same)
	assert.NoError(t, err)

	// Sames do not cause a snapshot mutation as part of `BeginMutation.`
	assert.Empty(t, sp.SavedSnapshots)

	err = mutation.End(same)
	assert.NoError(t, err)

	// Sames `do` cause a snapshot mutation as part of `End`.
	assert.NotEmpty(t, sp.SavedSnapshots)
	assert.NotEmpty(t, sp.SavedSnapshots[0].Resources)

	// Our same resource should be the first entry in the snapshot list.
	inSnapshot := sp.SavedSnapshots[0].Resources[0]
	assert.Equal(t, sameState.URN, inSnapshot.URN)
	assert.Equal(t, sameState.Status, inSnapshot.Status)
}

// This test challenges the naive approach of mutating resources
// that are the targets of Same steps in-place by changing the dependencies
// of two resources in the snapshot, which is perfectly legal in our system
// (and in fact is done by the `dependency_steps` integration test as well).
//
// The correctness of the `merge` function in snapshot.go is tested here.
func TestSamesWithDependencyChanges(t *testing.T) {
	resourceA := NewResource("a-unique-urn-resource-a", resource.ResourceStatusCreated)
	resourceB := NewResource("a-unique-urn-resource-b", resource.ResourceStatusCreated, resourceA.URN)

	// The setup: the snapshot contains two resources, A and B, where
	// B depends on A. We're going to begin a mutation in which B no longer
	// depends on A and appears first in program order.
	manager, sp := MockSetup(t, "test", []*resource.State{
		resourceA,
		resourceB,
	})

	resourceBUpdated := NewResource(string(resourceB.URN), resource.ResourceStatusCreated)
	// note: no dependencies

	resourceAUpdated := NewResource(string(resourceA.URN), resource.ResourceStatusCreated, resourceBUpdated.URN)
	// note: now depends on B

	// The engine first generates a Same for b:
	bSame := deploy.NewSameStep(nil, nil, resourceB, resourceBUpdated)
	mutation, err := manager.BeginMutation(bSame)
	assert.NoError(t, err)
	err = mutation.End(bSame)
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
	err = mutation.End(aSame)
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
	a := NewResource("a", resource.ResourceStatusCreated)
	b := NewResource("b", resource.ResourceStatusCreated, a.URN)
	c := NewResource("c", resource.ResourceStatusCreated, a.URN, b.URN)
	d := NewResource("d", resource.ResourceStatusCreated, c.URN)
	e := NewResource("e", resource.ResourceStatusCreated, c.URN)
	manager, sp := MockSetup(t, "test", []*resource.State{
		a,
		b,
		c,
		d,
		e,
	})

	// This is the sequence of events that come out of the engine:
	//   B - Same, depends on nothing
	//   C - CreateReplacement, depends on B
	//   C - Replace
	//   D - Update, depends on new C

	// This produces the following dependency graph in the new snapshot:
	//        +-+
	//  +---> |B|          (state: Created)
	//  |     +++
	//  |      ^
	//  |     +++
	//  |     |C| <----+   (state: Created)
	//  |     +-+      |
	//  |              |
	//  |     +-+      |
	//  +---+ |C| +-------------> A (not in graph!) (state: Pending Deletion)
	//        +-+      |
	//                 |
	//        +-+      |
	//        |D|  +---+   (state: Updated)
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

		err = mutation.End(step)
		if !assert.NoError(t, err) {
			t.FailNow()
		}
	}

	// b now depends on nothing
	bPrime := NewResource(string(b.URN), resource.ResourceStatusCreated)
	applyStep(deploy.NewSameStep(nil, MockRegisterResourceEvent{}, b, bPrime))

	// c now only depends on b
	cPrime := NewResource(string(c.URN), resource.ResourceStatusCreated, bPrime.URN)
	cCondemned := c.Clone()

	// mocking out the behavior of a provider indicating that this resource needs to be deleted
	cCondemned.Delete = true
	applyStep(deploy.NewCreateReplacementStep(nil, MockRegisterResourceEvent{}, cCondemned, cPrime, nil, true))
	applyStep(deploy.NewReplaceStep(nil, c, cPrime, nil, true))

	// cPrime now exists, c is now pending-delete
	// dPrime now depends on cPrime, which got replaced
	dPrime := NewResource(string(d.URN), resource.ResourceStatusUpdated, cPrime.URN)
	applyStep(deploy.NewUpdateStep(nil, MockRegisterResourceEvent{}, d, dPrime, nil))

	lastSnap := sp.SavedSnapshots[len(sp.SavedSnapshots)-1]
	assert.Len(t, lastSnap.Resources, 6)
	res := lastSnap.Resources

	// Here's what the merged snapshot should look like:
	// B should be first, and it should depend on nothing
	assert.Equal(t, b.URN, res[0].URN)
	assert.Equal(t, resource.ResourceStatusCreated, res[0].Status)
	assert.Len(t, res[0].Dependencies, 0)

	// cPrime should be next, and it should depend on B
	assert.Equal(t, c.URN, res[1].URN)
	assert.Equal(t, resource.ResourceStatusCreated, res[1].Status)
	assert.Len(t, res[1].Dependencies, 1)
	assert.Equal(t, b.URN, res[1].Dependencies[0])

	// d should be next, and it should depend on cPrime
	assert.Equal(t, d.URN, res[2].URN)
	assert.Equal(t, resource.ResourceStatusUpdated, res[2].Status)
	assert.Len(t, res[2].Dependencies, 1)
	assert.Equal(t, c.URN, res[2].Dependencies[0])

	// a should be next, and it should depend on nothing
	assert.Equal(t, a.URN, res[3].URN)
	assert.Equal(t, resource.ResourceStatusCreated, res[3].Status)
	assert.Len(t, res[3].Dependencies, 0)

	// c should be next, it should depend on A and B and should be pending deletion
	// this is a critical operation of merge and the crux of this test:
	// merge MUST put c after a in the snapshot, despite never having seen a in the current plan
	assert.Equal(t, c.URN, res[4].URN)
	assert.Equal(t, resource.ResourceStatusPendingDeletion, res[4].Status)
	assert.Len(t, res[4].Dependencies, 2)
	assert.Contains(t, res[4].Dependencies, a.URN)
	assert.Contains(t, res[4].Dependencies, b.URN)

	// e should be last, it should depend on C and still be live
	assert.Equal(t, e.URN, res[5].URN)
	assert.Equal(t, resource.ResourceStatusCreated, res[5].Status)
	assert.Len(t, res[5].Dependencies, 1)
	assert.Equal(t, c.URN, res[5].Dependencies[0])
}

func TestDeletion(t *testing.T) {
	resourceA := NewResource("a", resource.ResourceStatusCreated)
	manager, sp := MockSetup(t, "test", []*resource.State{
		resourceA,
	})

	step := deploy.NewDeleteStep(nil, resourceA)
	mutation, err := manager.BeginMutation(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// the begin mutation should persist a snapshot with a having the
	// state "deleting"
	snap := sp.SavedSnapshots[0]
	assert.Len(t, snap.Resources, 1)
	assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
	assert.Equal(t, resource.ResourceStatusDeleting, snap.Resources[0].Status)

	err = mutation.End(step)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// the end mutation should set resourceA to the "deleted" state in the new snapshot.
	// merge should then not put resourceA in the merged snapshot, since it has been deleted.
	snap = sp.SavedSnapshots[1]
	assert.Len(t, snap.Resources, 0)
}

func TestCondemnedInBaseSnapshot(t *testing.T) {
	condemnedStates := []resource.MutationStatus{
		resource.ResourceStatusPendingDeletion,
		resource.ResourceStatusDeleting,
	}

	for _, state := range condemnedStates {
		t.Run(fmt.Sprintf("Condemned-%s", state), func(t *testing.T) {
			// a is a resource that's condemned. this is a similar setup to the previous
			// test, but tests a slightly different code path.
			resourceABase := NewResource("a", resource.ResourceStatusCreated)
			resourceA := NewResource("a", state)
			manager, sp := MockSetup(t, "test", []*resource.State{
				resourceABase,
				resourceA,
			})

			// do a Same step for a so that we don't think it's deleted
			same := deploy.NewSameStep(nil, MockRegisterResourceEvent{}, resourceABase.Clone(), resourceABase.Clone())
			mutation, err := manager.BeginMutation(same)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			err = mutation.End(same)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			step := deploy.NewDeleteStep(nil, resourceA)
			mutation, err = manager.BeginMutation(step)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			// the begin mutation should persist a snapshot with the still-live A and the
			// pending-delete A marked for deletion
			snap := sp.SavedSnapshots[1]
			assert.Len(t, snap.Resources, 2)
			assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
			assert.Equal(t, resource.ResourceStatusCreated, snap.Resources[0].Status)
			assert.Equal(t, resourceA.URN, snap.Resources[1].URN)
			assert.Equal(t, resource.ResourceStatusDeleting, snap.Resources[1].Status)

			err = mutation.End(step)
			if !assert.NoError(t, err) {
				t.FailNow()
			}

			// the end mutation should set the pending delete resourceA to the "deleted" state in the new snapshot.
			// merge should then only put the live resourceA into the snapshot.
			snap = sp.SavedSnapshots[2]
			assert.Len(t, snap.Resources, 1)
			assert.Equal(t, resourceA.URN, snap.Resources[0].URN)
			assert.Equal(t, resource.ResourceStatusCreated, snap.Resources[0].Status)
		})
	}
}
