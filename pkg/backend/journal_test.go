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
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
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
		Version:     1,
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

func TestJournalReplayerRejectsUnsupportedEntryVersion(t *testing.T) {
	t.Parallel()

	err := NewJournalReplayer(&apitype.DeploymentV3{}).Add(apitype.JournalEntry{
		Version: 0,
		Kind:    apitype.JournalEntryKindBegin,
	})
	require.ErrorContains(t, err, "unsupported journal entry version 0")
}

func TestJournalReplayerRejectsUnknownNewOperationReferences(t *testing.T) {
	t.Parallel()

	operationID := int64(42)
	tests := map[string]apitype.JournalEntry{
		"remove new": {
			Version:   1,
			Kind:      apitype.JournalEntryKindSuccess,
			RemoveNew: &operationID,
		},
		"delete new": {
			Version:   1,
			Kind:      apitype.JournalEntryKindSuccess,
			DeleteNew: &operationID,
		},
		"pending replacement new": {
			Version:               1,
			Kind:                  apitype.JournalEntryKindSuccess,
			PendingReplacementNew: &operationID,
		},
		"refresh remove new": {
			Version:   1,
			Kind:      apitype.JournalEntryKindRefreshSuccess,
			RemoveNew: &operationID,
		},
		"outputs remove new": {
			Version:   1,
			Kind:      apitype.JournalEntryKindOutputs,
			RemoveNew: &operationID,
		},
	}
	for name, entry := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := NewJournalReplayer(&apitype.DeploymentV3{}).Add(entry)
			require.ErrorContains(t, err, "references unknown operation 42")
		})
	}
}

func TestSerializeJournalEntryStateMigrationPatches(t *testing.T) {
	t.Parallel()

	targetURN := resource.URN("urn:pulumi:test::test::pkgA:m:typA::target")
	newState := func(name string, id resource.ID) *pkgresource.State {
		urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name)
		return &pkgresource.State{
			Type:   urn.Type(),
			URN:    urn,
			Custom: true,
			ID:     id,
			Inputs: resource.PropertyMap{
				"name": resource.NewProperty(name),
			},
			Outputs: resource.PropertyMap{
				"reference": resource.MakeSecret(
					resource.MakeCustomResourceReference(targetURN, "target-id", "6.0.0")),
			},
		}
	}

	baseState := newState("base-consumer", "base-id")
	operationState := newState("operation-consumer", "operation-id")
	entry := engine.JournalEntry{
		Kind:       engine.JournalEntryStateMigration,
		SequenceID: 17,
		BaseStatePatches: []engine.JournalBaseStatePatch{{
			Index: 4,
			State: baseState,
		}},
		NewStatePatches: []engine.JournalNewStatePatch{{
			OperationID: 23,
			State:       operationState,
		}},
	}

	secretsManager := b64.NewBase64SecretsManager()
	serialized, err := SerializeJournalEntry(t.Context(), entry, secretsManager.Encrypter())
	require.NoError(t, err)

	assert.Equal(t, 2, serialized.Version)
	assert.Equal(t, apitype.JournalEntryKindStateMigration, serialized.Kind)
	require.Len(t, serialized.BaseStatePatches, 1)
	require.Len(t, serialized.NewStatePatches, 1)
	assert.Equal(t, int64(4), serialized.BaseStatePatches[0].Index)
	assert.Equal(t, int64(23), serialized.NewStatePatches[0].OperationID)

	assertPatchState := func(expected *pkgresource.State, serialized apitype.ResourceV3) {
		assert.Equal(t, expected.URN, serialized.URN)
		assert.Equal(t, expected.ID, serialized.ID)

		secret, ok := serialized.Outputs["reference"].(*apitype.SecretV1)
		require.True(t, ok)
		assert.Empty(t, secret.Plaintext)
		assert.NotEmpty(t, secret.Ciphertext)

		roundTripped, err := stack.DeserializeResource(serialized, secretsManager.Decrypter())
		require.NoError(t, err)
		assert.Equal(t, expected.URN, roundTripped.URN)
		assert.Equal(t, expected.ID, roundTripped.ID)
		assert.Equal(t, expected.Inputs, roundTripped.Inputs)

		reference := roundTripped.Outputs["reference"]
		require.True(t, reference.IsSecret())
		typedReference := reference.SecretValue().Element
		require.True(t, typedReference.IsResourceReference())
		assert.Equal(t, targetURN, typedReference.ResourceReferenceValue().URN)
		assert.Equal(t, "target-id", typedReference.ResourceReferenceValue().ID.StringValue())
		assert.Equal(t, "6.0.0", typedReference.ResourceReferenceValue().PackageVersion)
	}

	assertPatchState(baseState, serialized.BaseStatePatches[0].State)
	assertPatchState(operationState, serialized.NewStatePatches[0].State)
}

func TestStateMigrationSecretReferencePatchRoundTrip(t *testing.T) {
	t.Parallel()

	newState := func(name string) *pkgresource.State {
		urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name)
		return &pkgresource.State{
			Type:   urn.Type(),
			URN:    urn,
			Custom: true,
			ID:     resource.ID("id-" + name),
		}
	}
	predecessor := newState("predecessor")
	successor := newState("successor")
	successor.ID = predecessor.ID
	consumer := newState("consumer")
	consumer.Outputs = resource.PropertyMap{
		"payload": resource.MakeSecret(resource.NewObjectProperty(resource.PropertyMap{
			"nestedReference": resource.MakeCustomResourceReference(
				predecessor.URN, predecessor.ID, "1.2.3"),
		})),
	}

	plan := &deploy.StateMigrationPlan{
		SuccessorURNs: map[resource.URN]resource.URN{predecessor.URN: successor.URN},
		BaseResources: []*pkgresource.State{successor, consumer},
	}
	rewritten, err := plan.RewriteResources([]*pkgresource.State{consumer})
	require.NoError(t, err)
	require.NotSame(t, consumer, rewritten[0])

	secretsManager := b64.NewBase64SecretsManager()
	entry, err := SerializeJournalEntry(t.Context(), engine.JournalEntry{
		Kind:           engine.JournalEntryStateMigration,
		RemoveOlds:     []int64{0},
		MigratedStates: []*pkgresource.State{successor},
		BaseStatePatches: []engine.JournalBaseStatePatch{{
			Index: 1,
			State: rewritten[0],
		}},
	}, secretsManager.Encrypter())
	require.NoError(t, err)
	require.Len(t, entry.BaseStatePatches, 1)

	patchSecret, ok := entry.BaseStatePatches[0].State.Outputs["payload"].(*apitype.SecretV1)
	require.True(t, ok)
	assert.Empty(t, patchSecret.Plaintext)
	assert.NotEmpty(t, patchSecret.Ciphertext)
	assert.NotContains(t, patchSecret.Ciphertext, string(predecessor.URN))
	assert.NotContains(t, patchSecret.Ciphertext, string(successor.URN))

	serializeBaseState := func(state *pkgresource.State) apitype.ResourceV3 {
		serialized, _, err := stack.SerializeResource(t.Context(), state, secretsManager.Encrypter(), false)
		require.NoError(t, err)
		return serialized
	}
	replayer := NewJournalReplayer(&apitype.DeploymentV3{Resources: []apitype.ResourceV3{
		serializeBaseState(predecessor),
		serializeBaseState(consumer),
	}})
	require.NoError(t, replayer.Add(entry))
	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.Len(t, deployment.Deployment.Resources, 2)

	replayedConsumer := deployment.Deployment.Resources[1]
	replayedSecret, ok := replayedConsumer.Outputs["payload"].(*apitype.SecretV1)
	require.True(t, ok)
	assert.Equal(t, patchSecret.Ciphertext, replayedSecret.Ciphertext,
		"replay should install the prepared ciphertext without interpreting it")

	roundTripped, err := stack.DeserializeResource(replayedConsumer, secretsManager.Decrypter())
	require.NoError(t, err)
	payload := roundTripped.Outputs["payload"]
	require.True(t, payload.IsSecret())
	nestedReference := payload.SecretValue().Element.ObjectValue()["nestedReference"]
	require.True(t, nestedReference.IsResourceReference())
	reference := nestedReference.ResourceReferenceValue()
	assert.Equal(t, successor.URN, reference.URN)
	assert.Equal(t, string(successor.ID), reference.ID.StringValue())
	assert.Empty(t, reference.PackageVersion)
}
