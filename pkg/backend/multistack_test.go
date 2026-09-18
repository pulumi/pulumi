// Copyright 2026, Pulumi Corporation.
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

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func rootStackState(project, stackName string) *pkgresource.State {
	return &pkgresource.State{
		Type: resource.RootStackType,
		URN:  resource.DefaultRootStackURN(tokens.QName(stackName), tokens.PackageName(project)),
	}
}

func stackReferenceState(project, stackName, refFQN string) *pkgresource.State {
	return &pkgresource.State{
		Type: "pulumi:pulumi:StackReference",
		URN: resource.NewURN(
			tokens.QName(stackName), tokens.PackageName(project), "", "pulumi:pulumi:StackReference", "ref"),
		Custom: true,
		Inputs: resource.PropertyMap{
			"name": resource.NewProperty(refFQN),
		},
	}
}

func testManifest() deploy.Manifest {
	m := deploy.Manifest{Version: "test"}
	m.Magic = m.NewMagic()
	return m
}

// TestLoadMultistackSnapshotToleratesSyntheticCrossStackDependency is the regression test for
// the checkpoint a real multistack UPDATE leaves behind: deploy.MergeSnapshots (called by
// engine.MultistackUpdate for the shared-engine update path) intentionally mutates a co-deployed
// StackReference's Dependencies in place to add the referenced stack's root URN, for destroy
// ordering. Because it mutates the shared, per-stack State pointers rather than cloning them
// (see MergeSnapshots's own doc comment for why), that synthetic dependency edge is exactly what
// ends up persisted into the referencing stack's own checkpoint. A later ordinary,
// integrity-checked read of that checkpoint then fails: the referenced root belongs to a
// different stack and is absent from this one's own resource list. loadMultistackSnapshot must
// still succeed, by reading through UnverifiedSnapshotStack when the stack's backend implements
// it, exactly as a real multistack PreviewMany does against checkpoints written by a real
// multistack update.
func TestLoadMultistackSnapshotToleratesSyntheticCrossStackDependency(t *testing.T) {
	t.Parallel()

	manifest := testManifest()
	rootA := rootStackState("proj-a", "dev")
	refA := stackReferenceState("proj-a", "dev", "organization/proj-b/dev")
	rootB := rootStackState("proj-b", "dev")

	snapA := &deploy.Snapshot{Manifest: manifest, Resources: []*pkgresource.State{rootA, refA}}
	snapB := &deploy.Snapshot{Manifest: manifest, Resources: []*pkgresource.State{rootB}}

	// Simulate the real multistack UPDATE path: engine.MultistackUpdate merges the members'
	// snapshots for cross-stack destroy ordering, mutating refA's Dependencies in place.
	merged := deploy.MergeSnapshots([]*deploy.Snapshot{snapA, snapB}, map[string]bool{
		"organization/proj-b/dev": true,
	})
	require.NotNil(t, merged)
	require.Contains(t, refA.Dependencies, rootB.URN, "MergeSnapshots must add the cross-stack dependency in place")

	// Confirm the poisoning is real: snapA (exactly what gets persisted back to stack A's own
	// checkpoint) now fails ordinary integrity verification, since rootB's URN is not one of
	// snapA's own resources.
	verifyErr := snapA.VerifyIntegrity()
	require.Error(t, verifyErr)
	assert.Contains(t, verifyErr.Error(), "refers to missing resource")

	// loadMultistackSnapshot must still succeed: the Stack implements UnverifiedSnapshotStack, so
	// it takes the unchecked branch instead of failing the way an ordinary checked load would.
	stackA := &MockStack{
		SnapshotF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			t.Fatal("loadMultistackSnapshot must not use the checked Snapshot path when SnapshotUnchecked is available")
			return nil, nil
		},
		SnapshotUncheckedF: func(context.Context, secrets.Provider) (*deploy.Snapshot, error) {
			return snapA, nil
		},
	}

	got, err := loadMultistackSnapshot(t.Context(), stackA, nil)
	require.NoError(t, err)
	assert.Same(t, snapA, got)
}
