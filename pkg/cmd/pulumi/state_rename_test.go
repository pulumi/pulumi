// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenameProvider tests that we can rename a provider and produce a valid snapshot.
func TestRenameProvider(t *testing.T) {
	t.Parallel()

	// Define a state with a single provider and a single resource dependent on that provider.
	provURN := resource.URN("urn:pulumi:dev::prog-aws-typescript::pulumi:providers:random::my-provider")
	prov := resource.State{
		URN:  provURN,
		ID:   "81cd12dd-2a90-4f21-a521-f4c71c1f11eb",
		Type: "pulumi:providers:random",
	}

	providerRefString := string(prov.URN) + "::" + string(prov.ID)

	res1 := resource.State{
		URN:      resource.URN("urn:pulumi:dev::prog-aws-typescript::random:index/randomPet:RandomPet::pet-0"),
		Provider: providerRefString,
	}
	snap := &deploy.Snapshot{
		Resources: []*resource.State{
			&prov,
			&res1,
		},
	}

	// Ensure that the snapshot is valid before the rename.
	assert.NoError(t, snap.VerifyIntegrity())

	// Mutates the snapshot.
	err := stateRenameOperation(provURN, "our-provider", display.Options{}, snap)
	assert.NoError(t, err)

	// Ensure that the snapshot is valid after the rename.
	assert.NoError(t, snap.VerifyIntegrity())

	// Check that the snapshot contains the renamed provider as `our-provider`.
	for _, res := range snap.Resources {
		if res.ID == prov.ID {
			assert.Equal(t, "our-provider", res.URN.Name())
		}
	}
}

func TestStateRename_invalidName(t *testing.T) {
	t.Parallel()

	prov := resource.URN("urn:pulumi:dev::xxx-dev::kubernetes::provider")
	res := resource.URN("urn:pulumi:dev::xxx-dev::kubernetes:core/v1:Namespace::amazon_cloudwatchNamespace")

	snap := deploy.Snapshot{
		Resources: []*resource.State{
			{
				URN:  prov,
				ID:   "provider-id",
				Type: "pulumi:provider:kubernetes",
			},
			{
				URN:  res,
				ID:   "res-id",
				Type: "kubernetes:core/v1:Namespace",
			},
		},
	}
	require.NoError(t, snap.VerifyIntegrity(),
		"invalid test: snapshot is already broken")

	// stateRenameOperation accepts newResouceName: a tokens.QName. It assumes that
	// newResourceName is valid but it *must* not invalidate state when given an
	// invalid QName. This is a defensive measure to prevent invalidating state.
	assert.Panicsf(t, func() {
		_ = stateRenameOperation(
			res,
			"urn:pulumi:dev::xxx-dev::eks:index:Cluster$kubernetes:core/v1:Namespace::amazon_cloudwatchNamespace",
			display.Options{},
			&snap,
		)
	}, "swallowed invalid QName")

	// The state must still be valid, and the resource name unchanged.
	require.NoError(t, snap.VerifyIntegrity(), "snapshot is broken after rename")
	assert.Equal(t, res, snap.Resources[1].URN)
}

// Regression test for https://github.com/pulumi/pulumi/issues/13179.
//
// Defines a state with a two resources, one parented to the other,
// and renames the parent.
// The child must have an updated parent reference.
func TestStateRename_updatesChildren(t *testing.T) {
	t.Parallel()

	provider := resource.URN("urn:pulumi:dev::pets::random::provider")
	parent := resource.URN("urn:pulumi:dev::pets::random:index/randomPet:RandomPet::parent")
	child := resource.URN("urn:pulumi:dev::pets::random:index/randomPet:RandomPet$random:index/randomPet:RandomPet::child")

	snap := deploy.Snapshot{
		Resources: []*resource.State{
			{
				URN:  provider,
				ID:   "provider-id",
				Type: "pulumi:provider:random",
			},
			{
				URN:  parent,
				ID:   "parent-id",
				Type: "random:index/randomPet:RandomPet",
			},
			{
				URN:    child,
				ID:     "child-id",
				Type:   "random:index/randomPet:RandomPet",
				Parent: parent,
			},
		},
	}
	require.NoError(t, snap.VerifyIntegrity(),
		"invalid test: snapshot is already broken")

	err := stateRenameOperation(parent, "new-parent", display.Options{}, &snap)
	require.NoError(t, err)

	require.NoError(t, snap.VerifyIntegrity(), "snapshot is broken after rename")

	// VerifyIntegrity checks that the parent reference is updated,
	// but we'll check it explicitly here to be sure.
	var sawChild bool
	for _, res := range snap.Resources {
		if res.URN == child {
			sawChild = true
			assert.Equal(t, "new-parent", res.Parent.Name())
		}
	}
	assert.True(t, sawChild, "child resource not found in snapshot")
}
