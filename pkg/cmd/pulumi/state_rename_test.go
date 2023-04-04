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
			assert.Equal(t, "our-provider", res.URN.Name().String())
		}
	}
}
