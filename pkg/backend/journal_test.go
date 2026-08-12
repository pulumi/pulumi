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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJournalExtensionParameterizeRoundTrip(t *testing.T) {
	t.Parallel()

	ref := apitype.ExtensionRef("ref-1")
	ext := apitype.Extension{Name: "myext", Version: "1.0.0", Value: []byte("Hello")}

	engineEntry := engine.JournalEntry{
		Kind:         engine.JournalEntryExtensionParameterize,
		SequenceID:   1,
		OperationID:  1,
		ExtensionRef: &ref,
		Extension:    &ext,
	}

	serialized, err := SerializeJournalEntry(t.Context(), engineEntry, config.NopEncrypter)
	require.NoError(t, err)
	assert.Equal(t, apitype.JournalEntryKindExtensionParameterize, serialized.Kind)
	require.NotNil(t, serialized.ExtensionRef)
	require.NotNil(t, serialized.Extension)
	assert.Equal(t, ref, *serialized.ExtensionRef)
	assert.Equal(t, ext, *serialized.Extension)

	replayer := NewJournalReplayer(&apitype.DeploymentV3{})
	require.NoError(t, replayer.Add(serialized))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.NotNil(t, deployment.Deployment)
	require.Contains(t, deployment.Deployment.Extensions, ref)
	assert.Equal(t, ext, deployment.Deployment.Extensions[ref])
}

func TestJournalReplayerSeedsExtensionsFromBase(t *testing.T) {
	t.Parallel()

	ref := apitype.ExtensionRef("base-ref")
	ext := apitype.Extension{Name: "base-ext", Version: "1.0.0", Value: []byte("baseline")}

	base := &apitype.DeploymentV3{
		Extensions: map[apitype.ExtensionRef]apitype.Extension{ref: ext},
	}
	replayer := NewJournalReplayer(base)

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.Contains(t, deployment.Deployment.Extensions, ref,
		"extensions from base must survive replay even with no extension journal entries")
	assert.Equal(t, ext, deployment.Deployment.Extensions[ref])
}

// TestJournalReplayerRefreshPrunesReplaceWith tests that a targeted refresh which deletes a resource prunes
// dangling ReplaceWith references to it from resources that were not themselves refreshed, while keeping
// references that are still valid.
func TestJournalReplayerRefreshPrunesReplaceWith(t *testing.T) {
	t.Parallel()

	res := func(name string) apitype.ResourceV3 {
		return apitype.ResourceV3{
			URN:    resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
			Type:   "pkgA:m:typA",
			Custom: true,
			ID:     resource.ID("id-" + name),
		}
	}
	a, b := res("a"), res("b")
	unrelated, dependent := res("unrelated"), res("dependent")
	dependent.ReplaceWith = []resource.URN{a.URN, b.URN}
	base := &apitype.DeploymentV3{Resources: []apitype.ResourceV3{a, b, unrelated, dependent}}

	replayer := NewJournalReplayer(base)

	// Model a targeted refresh of only "b" (index 1 of the base snapshot), where the provider reports that
	// "b" no longer exists. "dependent" is not refreshed, so replay must repair the ReplaceWith list from its
	// old state in the base snapshot.
	removeOld := int64(1)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Kind:        apitype.JournalEntryKindRefreshSuccess,
		OperationID: 1,
		RemoveOld:   &removeOld,
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	byURN := make(map[resource.URN]apitype.ResourceV3, len(deployment.Deployment.Resources))
	for _, r := range deployment.Deployment.Resources {
		byURN[r.URN] = r
	}
	require.NotContains(t, byURN, b.URN)
	require.Contains(t, byURN, dependent.URN)

	// The reference to the deleted "b" is pruned and the reference to the surviving "a" is kept.
	assert.Equal(t, []resource.URN{a.URN}, byURN[dependent.URN].ReplaceWith)
	// The old implementation shadowed the current resource index and could append an empty URN to an unrelated
	// resource instead of repairing the dependent resource.
	assert.Empty(t, byURN[unrelated.URN].ReplaceWith)
	for _, r := range deployment.Deployment.Resources {
		assert.NotContains(t, r.ReplaceWith, resource.URN(""), "resource %s", r.URN)
	}
}
