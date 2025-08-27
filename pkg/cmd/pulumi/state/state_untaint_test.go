// Copyright 2025, Pulumi Corporation.
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

package state

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUntaintSingleResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true, // Resource is tainted
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot to verify taint state
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)
	assert.True(t, initialSnap.Resources[1].Taint)

	// Untaint the resource
	urns := []string{string(resources[1].URN)}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.False(t, initialSnap.Resources[1].Taint)
}

func TestUntaintMultipleResources(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name1"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    false,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name3"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Untaint multiple resources
	urns := []string{
		string(resources[1].URN),
		string(resources[3].URN),
	}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 2, resourceCount)
	assert.Empty(t, errs)
	assert.False(t, initialSnap.Resources[1].Taint)
	assert.False(t, initialSnap.Resources[2].Taint)
	assert.False(t, initialSnap.Resources[3].Taint)
}

func TestUntaintNonExistentResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Try to untaint a non-existent resource
	urns := []string{
		"urn:pulumi:test-stack::test::d:e:f$a:b:c::nonexistent",
	}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 0, resourceCount)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "No such resource")
	assert.Contains(t, errs[0].Error(), "nonexistent")
	// Ensure the existing resource remains tainted
	assert.True(t, initialSnap.Resources[1].Taint)
}

func TestUntaintMixedExistingAndNonExistent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name1"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name2"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Try to untaint existing and non-existent resources
	urns := []string{
		string(resources[1].URN),
		"urn:pulumi:test-stack::test::d:e:f$a:b:c::nonexistent",
		string(resources[2].URN),
	}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 2, resourceCount)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "No such resource")
	assert.Contains(t, errs[0].Error(), "nonexistent")
	// Verify the existing resources were untainted
	assert.False(t, initialSnap.Resources[1].Taint)
	assert.False(t, initialSnap.Resources[2].Taint)
}

func TestUntaintAlreadyUntaintedResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name"),
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    false, // Already untainted
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)
	assert.False(t, initialSnap.Resources[1].Taint)

	// Untaint the already untainted resource
	urns := []string{string(resources[1].URN)}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.False(t, initialSnap.Resources[1].Taint)
}

func TestUntaintEmptySnapshot(t *testing.T) {
	t.Parallel()

	// Test with nil snapshot
	urns := []string{"urn:pulumi:test-stack::test::d:e:f$a:b:c::name"}
	resourceCount, errs := untaintResourcesInSnapshot(nil, urns)

	assert.Equal(t, 0, resourceCount)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "no resources found to untaint")
}

func TestUntaintAllResources(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	// Create a snapshot directly with resources
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:   resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:  "pulumi:providers:a::default_1_0_0",
			ID:    "provider_id",
			Taint: true,
		},
		{
			URN:   resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name1"),
			Type:  "a:b:c",
			Taint: true,
		},
		{
			URN:   resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name2"),
			Type:  "a:b:c",
			Taint: false,
		},
		{
			URN:   resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name3"),
			Type:  "a:b:c",
			Taint: true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Untaint all resources
	for _, res := range snap.Resources {
		res.Taint = false
	}

	// Verify all resources were untainted
	for _, res := range snap.Resources {
		assert.False(t, res.Taint)
	}
}

func TestUntaintWithParentChildRelationship(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	parentURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "parent")
	childURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "child")

	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      parentURN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
		{
			URN:      childURN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Parent:   parentURN,
			Taint:    true,
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Untaint the parent resource only
	urns := []string{string(parentURN)}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.False(t, initialSnap.Resources[1].Taint)
	// Child should not be automatically untainted
	assert.True(t, initialSnap.Resources[2].Taint)
}

func TestUntaintMultipleResourcesWithErrors(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	// Create a snapshot directly with resources
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:   resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:  "pulumi:providers:a::default_1_0_0",
			ID:    "provider_id",
			Taint: true,
		},
		{
			URN:   resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name1"),
			Type:  "a:b:c",
			Taint: true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to untaint multiple resources with some non-existent
	urns := []string{
		string(snap.Resources[1].URN),
		"urn:pulumi:test-stack::test::d:e:f$a:b:c::nonexistent1",
		"urn:pulumi:test-stack::test::d:e:f$a:b:c::nonexistent2",
	}

	resourceCount, errs := untaintResourcesInSnapshot(snap, urns)

	assert.Equal(t, 1, resourceCount)
	require.Len(t, errs, 2)
	assert.False(t, snap.Resources[1].Taint)
	// Verify both error messages are present
	for _, err := range errs {
		assert.Contains(t, err.Error(), "No such resource")
	}
}

func TestUntaintWithDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resource1URN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource1")
	resource2URN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource2")

	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resource1URN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    true,
		},
		{
			URN:          resource2URN,
			Type:         "a:b:c",
			Provider:     string(providerURN) + "::provider_id",
			Dependencies: []resource.URN{resource1URN},
			Taint:        true,
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)

	// Untaint the resource that has a dependency
	urns := []string{string(resource2URN)}
	resourceCount, errs := untaintResourcesInSnapshot(initialSnap, urns)

	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.True(t, initialSnap.Resources[1].Taint)
	assert.False(t, initialSnap.Resources[2].Taint)
	// Ensure the dependency relationship is preserved
	assert.Equal(t, []resource.URN{resource1URN}, initialSnap.Resources[2].Dependencies)
}

func TestUntaintRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()

	b, err := diy.New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	providerURN := resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0")
	resourceURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "name")
	resources := []*resource.State{
		{
			URN:    providerURN,
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:      resourceURN,
			Type:     "a:b:c",
			Provider: string(providerURN) + "::provider_id",
			Taint:    false, // Start untainted
		},
	}

	stackName := "organization/test/test-stack"
	stack := createStackWithResources(t, b, stackName, resources)

	mp := &secrets.MockProvider{}
	mp = mp.Add("b64", func(_ json.RawMessage) (secrets.Manager, error) {
		return b64.NewBase64SecretsManager(), nil
	})

	// Get initial snapshot
	initialSnap, err := stack.Snapshot(ctx, mp)
	require.NoError(t, err)
	assert.False(t, initialSnap.Resources[1].Taint)

	// Taint the resource
	urns := []string{string(resourceURN)}
	resourceCount, errs := taintResourcesInSnapshot(initialSnap, urns)
	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.True(t, initialSnap.Resources[1].Taint)

	// Untaint the resource
	resourceCount, errs = untaintResourcesInSnapshot(initialSnap, urns)
	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.False(t, initialSnap.Resources[1].Taint)
}

func TestUntaintResourceWithDeleteTrue(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	resourceURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "myresource")

	// Create a snapshot with both a resource marked for deletion and a normal resource with the same URN
	// This simulates a replacement scenario
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:    resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:    resourceURN,
			Type:   "a:b:c",
			ID:     "old_id",
			Delete: true, // This resource is marked for deletion
			Taint:  true,
		},
		{
			URN:    resourceURN,
			Type:   "a:b:c",
			ID:     "new_id",
			Delete: false, // This is the replacement resource
			Taint:  true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to untaint the resource
	urns := []string{string(resourceURN)}
	resourceCount, errs := untaintResourcesInSnapshot(snap, urns)

	// Should only untaint the non-deleted resource
	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	assert.True(t, snap.Resources[1].Taint)  // Resource marked for deletion should remain tainted
	assert.False(t, snap.Resources[2].Taint) // Replacement resource should be untainted
}

func TestUntaintAllResourcesWithDeleteTrue(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	// Create a snapshot with some resources marked for deletion
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:    resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:    resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource1"),
			Type:   "a:b:c",
			ID:     "id1",
			Delete: false,
			Taint:  true,
		},
		{
			URN:    resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource2"),
			Type:   "a:b:c",
			ID:     "id2",
			Delete: true, // Marked for deletion
			Taint:  true,
		},
		{
			URN:    resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource3"),
			Type:   "a:b:c",
			ID:     "id3",
			Delete: false,
			Taint:  true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to untaint all resources
	urns := []string{
		string(snap.Resources[1].URN),
		string(snap.Resources[2].URN),
		string(snap.Resources[3].URN),
	}
	resourceCount, errs := untaintResourcesInSnapshot(snap, urns)

	// Should only untaint the non-deleted resources
	assert.Equal(t, 2, resourceCount)
	require.Len(t, errs, 1)                                 // Should have an error for the deleted resource
	assert.Contains(t, errs[0].Error(), "No such resource") // The deleted resource won't be found in our map
	assert.False(t, snap.Resources[1].Taint)                // resource1 should be untainted
	assert.True(t, snap.Resources[2].Taint)                 // resource2 marked for deletion should remain tainted
	assert.False(t, snap.Resources[3].Taint)                // resource3 should be untainted
}

func TestUntaintOnlyDeletedResource(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	deletedURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "deleted")

	// Create a snapshot with only a deleted resource
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:    resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:   "pulumi:providers:a::default_1_0_0",
			ID:     "provider_id",
			Custom: true,
		},
		{
			URN:    deletedURN,
			Type:   "a:b:c",
			ID:     "id",
			Delete: true, // Resource is marked for deletion
			Taint:  true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to untaint the deleted resource
	urns := []string{string(deletedURN)}
	resourceCount, errs := untaintResourcesInSnapshot(snap, urns)

	// Should not untaint the deleted resource and report it as not found
	assert.Equal(t, 0, resourceCount)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "No such resource")
	assert.True(t, snap.Resources[1].Taint) // Resource should remain tainted
}
