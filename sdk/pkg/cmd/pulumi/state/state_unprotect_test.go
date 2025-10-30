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
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/pkg/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnprotectResourceWithDeleteTrue(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	resourceURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "myresource")

	// Create a snapshot with both a resource marked for deletion and a normal resource with the same URN
	// This simulates a replacement scenario
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:	resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:	"pulumi:providers:a::default_1_0_0",
			ID:	"provider_id",
			Custom:	true,
		},
		{
			URN:		resourceURN,
			Type:		"a:b:c",
			ID:		"old_id",
			Delete:		true,	// This resource is marked for deletion
			Protect:	true,
		},
		{
			URN:		resourceURN,
			Type:		"a:b:c",
			ID:		"new_id",
			Delete:		false,	// This is the replacement resource
			Protect:	true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to unprotect the resource
	urns := []string{string(resourceURN)}
	resourceCount, errs := unprotectResourcesInSnapshot(snap, urns)

	// Should only unprotect the non-deleted resource
	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	require.Len(t, snap.Resources, 3)
	assert.True(t, snap.Resources[1].Protect)	// Resource marked for deletion should remain protected
	assert.False(t, snap.Resources[2].Protect)	// Replacement resource should be unprotected
}

func TestUnprotectAllResourcesWithDeleteTrue(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	// Create a snapshot with some resources marked for deletion
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:	resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:	"pulumi:providers:a::default_1_0_0",
			ID:	"provider_id",
			Custom:	true,
		},
		{
			URN:		resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource1"),
			Type:		"a:b:c",
			ID:		"id1",
			Delete:		false,
			Protect:	true,
		},
		{
			URN:		resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource2"),
			Type:		"a:b:c",
			ID:		"id2",
			Delete:		true,	// Marked for deletion
			Protect:	true,
		},
		{
			URN:		resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "resource3"),
			Type:		"a:b:c",
			ID:		"id3",
			Delete:		false,
			Protect:	true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to unprotect all resources
	urns := []string{
		string(snap.Resources[1].URN),
		string(snap.Resources[2].URN),
		string(snap.Resources[3].URN),
	}
	resourceCount, errs := unprotectResourcesInSnapshot(snap, urns)

	// Should only unprotect the non-deleted resources
	assert.Equal(t, 2, resourceCount)
	require.Len(t, errs, 1)					// Should have an error for the deleted resource
	assert.Contains(t, errs[0].Error(), "No such resource")	// The deleted resource won't be found in our map
	require.Len(t, snap.Resources, 4)
	assert.False(t, snap.Resources[1].Protect)	// resource1 should be unprotected
	assert.True(t, snap.Resources[2].Protect)	// resource2 marked for deletion should remain protected
	assert.False(t, snap.Resources[3].Protect)	// resource3 should be unprotected
}

func TestUnprotectOnlyDeletedResource(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	deletedURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "deleted")

	// Create a snapshot with only a deleted resource
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:	resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:	"pulumi:providers:a::default_1_0_0",
			ID:	"provider_id",
			Custom:	true,
		},
		{
			URN:		deletedURN,
			Type:		"a:b:c",
			ID:		"id",
			Delete:		true,	// Resource is marked for deletion
			Protect:	true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to unprotect the deleted resource
	urns := []string{string(deletedURN)}
	resourceCount, errs := unprotectResourcesInSnapshot(snap, urns)

	// Should not unprotect the deleted resource and report it as not found
	assert.Equal(t, 0, resourceCount)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "No such resource")
	require.Len(t, snap.Resources, 2)
	assert.True(t, snap.Resources[1].Protect)	// Resource should remain protected
}

func TestUnprotectMultipleResourcesWithSameURNAndDelete(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()

	sharedURN := resource.NewURN("test-stack", "test", "d:e:f", "a:b:c", "shared")

	// Create a snapshot with multiple resources having the same URN
	// but some marked for deletion (replacement scenario)
	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, []*resource.State{
		{
			URN:	resource.NewURN("test-stack", "test", "", "pulumi:providers:a", "default_1_0_0"),
			Type:	"pulumi:providers:a::default_1_0_0",
			ID:	"provider_id",
			Custom:	true,
		},
		{
			URN:		sharedURN,
			Type:		"a:b:c",
			ID:		"old_id_1",
			Delete:		true,	// Old resource marked for deletion
			Protect:	true,
		},
		{
			URN:		sharedURN,
			Type:		"a:b:c",
			ID:		"old_id_2",
			Delete:		true,	// Another old resource marked for deletion
			Protect:	true,
		},
		{
			URN:		sharedURN,
			Type:		"a:b:c",
			ID:		"new_id",
			Delete:		false,	// The current active resource
			Protect:	true,
		},
	}, nil, deploy.SnapshotMetadata{})

	// Try to unprotect the resource
	urns := []string{string(sharedURN)}
	resourceCount, errs := unprotectResourcesInSnapshot(snap, urns)

	// Should only unprotect the active (non-deleted) resource
	assert.Equal(t, 1, resourceCount)
	assert.Empty(t, errs)
	require.Len(t, snap.Resources, 4)
	assert.True(t, snap.Resources[1].Protect)	// First deleted resource should remain protected
	assert.True(t, snap.Resources[2].Protect)	// Second deleted resource should remain protected
	assert.False(t, snap.Resources[3].Protect)	// Active resource should be unprotected
}
