// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import (
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

func (m *MockUpdateInfo) GetRoot() string {
	return "mocked"
}

func (m *MockUpdateInfo) GetProject() *workspace.Project {
	return nil
}

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

func TestIdenticalSames(t *testing.T) {
	sameState := &resource.State{
		Type:    tokens.Type("test"),
		URN:     resource.URN("a-unique-urn"),
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
		Status:  resource.ResourceStatusCreated,
	}

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
	resourceA := &resource.State{
		Type:    tokens.Type("test"),
		URN:     resource.URN("a-unique-urn-resource-a"),
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
		Status:  resource.ResourceStatusCreated,
	}

	resourceB := &resource.State{
		Type:    tokens.Type("test"),
		URN:     resource.URN("a-unique-urn-resource-b"),
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
		Status:  resource.ResourceStatusCreated,
		Dependencies: []resource.URN{
			resourceA.URN,
		},
	}

	// The setup: the snapshot contains two resources, A and B, where
	// B depends on A. We're going to begin a mutation in which B no longer
	// depends on A and appears first in program order.
	manager, sp := MockSetup(t, "test", []*resource.State{
		resourceA,
		resourceB,
	})

	resourceBUpdated := &resource.State{
		Type:    tokens.Type("test"),
		URN:     resourceB.URN,
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
		Status:  resource.ResourceStatusCreated,
		// note: no dependencies
	}

	resourceAUpdated := &resource.State{
		Type:    tokens.Type("test"),
		URN:     resourceA.URN,
		Inputs:  make(resource.PropertyMap),
		Outputs: make(resource.PropertyMap),
		Status:  resource.ResourceStatusCreated,
		// note: now depends on B
		Dependencies: []resource.URN{
			resourceBUpdated.URN,
		},
	}

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
