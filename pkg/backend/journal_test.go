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
	"fmt"
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

	for _, version := range []int{0, int(apitype.LatestJournalVersion + 1)} {
		t.Run(fmt.Sprintf("version %d", version), func(t *testing.T) {
			t.Parallel()

			err := NewJournalReplayer(&apitype.DeploymentV3{}).Add(apitype.JournalEntry{
				Version: version,
				Kind:    apitype.JournalEntryKindBegin,
			})
			require.ErrorContains(t, err, fmt.Sprintf("unsupported journal entry version %d", version))
		})
	}
}

func TestJournalReplayerRejectsUnknownEntryKind(t *testing.T) {
	t.Parallel()

	err := NewJournalReplayer(&apitype.DeploymentV3{}).Add(apitype.JournalEntry{
		Version: 1,
		Kind:    apitype.JournalEntryKind(999),
	})
	require.ErrorContains(t, err, "unsupported journal entry kind 999")
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
		"payload": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
			"nestedReference": resource.MakeCustomResourceReference(
				predecessor.URN, predecessor.ID, "1.2.3"),
		})),
	}
	operationConsumer := consumer.Copy()
	operationConsumer.URN = resource.URN("urn:pulumi:test::test::pkgA:m:typA::operation-consumer")
	operationConsumer.ID = "id-operation-consumer"

	transaction := &deploy.StateMigrationTransaction{
		SuccessorURNs:          map[resource.URN]resource.URN{predecessor.URN: successor.URN},
		PreparedPriorResources: []*pkgresource.State{successor, consumer},
	}
	rewritten, err := transaction.RewriteResources([]*pkgresource.State{consumer, operationConsumer})
	require.NoError(t, err)
	require.NotSame(t, consumer, rewritten[0])
	require.NotSame(t, operationConsumer, rewritten[1])

	secretsManager := b64.NewBase64SecretsManager()
	entry, err := SerializeJournalEntry(t.Context(), engine.JournalEntry{
		Kind:         engine.JournalEntryStateMigration,
		RemoveOlds:   []int64{0},
		ResultStates: []*pkgresource.State{successor},
		BaseStatePatches: []engine.JournalBaseStatePatch{{
			Index: 1,
			State: rewritten[0],
		}},
		NewStatePatches: []engine.JournalNewStatePatch{{
			OperationID: 42,
			State:       rewritten[1],
		}},
	}, secretsManager.Encrypter())
	require.NoError(t, err)
	require.Len(t, entry.BaseStatePatches, 1)
	require.Len(t, entry.NewStatePatches, 1)

	patchSecret := func(state apitype.ResourceV3) *apitype.SecretV1 {
		secret, ok := state.Outputs["payload"].(*apitype.SecretV1)
		require.True(t, ok)
		assert.Empty(t, secret.Plaintext)
		assert.NotEmpty(t, secret.Ciphertext)
		assert.NotContains(t, secret.Ciphertext, string(predecessor.URN))
		assert.NotContains(t, secret.Ciphertext, string(successor.URN))
		return secret
	}
	basePatchSecret := patchSecret(entry.BaseStatePatches[0].State)
	newPatchSecret := patchSecret(entry.NewStatePatches[0].State)

	serializeBaseState := func(state *pkgresource.State) apitype.ResourceV3 {
		serialized, _, err := stack.SerializeResource(t.Context(), state, secretsManager.Encrypter(), false)
		require.NoError(t, err)
		return serialized
	}
	replayer := NewJournalReplayer(&apitype.DeploymentV3{Resources: []apitype.ResourceV3{
		serializeBaseState(predecessor),
		serializeBaseState(consumer),
	}})
	operationState := serializeBaseState(operationConsumer)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 42,
		State:       &operationState,
	}))
	require.NoError(t, replayer.Add(entry))
	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	byURN := make(map[resource.URN]apitype.ResourceV3, len(deployment.Deployment.Resources))
	for _, state := range deployment.Deployment.Resources {
		byURN[state.URN] = state
	}
	for _, expected := range []struct {
		urn        resource.URN
		ciphertext string
	}{
		{consumer.URN, basePatchSecret.Ciphertext},
		{operationConsumer.URN, newPatchSecret.Ciphertext},
	} {
		replayed := byURN[expected.urn]
		replayedSecret, ok := replayed.Outputs["payload"].(*apitype.SecretV1)
		require.True(t, ok)
		assert.Equal(t, expected.ciphertext, replayedSecret.Ciphertext,
			"replay should install the prepared ciphertext without interpreting it")

		roundTripped, err := stack.DeserializeResource(replayed, secretsManager.Decrypter())
		require.NoError(t, err)
		payload := roundTripped.Outputs["payload"]
		require.True(t, payload.IsSecret())
		reference := payload.SecretValue().Element.ObjectValue()["nestedReference"].ResourceReferenceValue()
		assert.Equal(t, successor.URN, reference.URN)
		assert.Equal(t, string(successor.ID), reference.ID.StringValue())
		assert.Empty(t, reference.PackageVersion)
	}
}
