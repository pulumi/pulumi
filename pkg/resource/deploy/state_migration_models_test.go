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

package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestFinalStateMigrationSuccessors(t *testing.T) {
	t.Parallel()

	original := []apitype.ResourceV3{{URN: "urn:a"}}
	final := []apitype.ResourceV3{{URN: "urn:c"}}
	originalToFinal, allToFinal, err := finalStateMigrationSuccessors(original, final, map[resource.URN]resource.URN{
		"urn:a": "urn:b",
		"urn:b": "urn:c",
	})
	require.NoError(t, err)
	assert.Equal(t, map[resource.URN]resource.URN{"urn:a": "urn:c"}, originalToFinal)
	assert.Equal(t, map[resource.URN]resource.URN{"urn:a": "urn:c", "urn:b": "urn:c"}, allToFinal)

	result := &pkgresource.State{URN: "urn:c", Dependencies: []resource.URN{"urn:b"}}
	resultStates := []*pkgresource.State{result}
	rewrittenResult, err := rewriteStateMigrationReferences(
		resultStates, allToFinal, stateMigrationSuccessorIdentities(resultStates))
	require.NoError(t, err)
	assert.Equal(t, []resource.URN{"urn:c"}, rewrittenResult[0].Dependencies)

	retained := &pkgresource.State{Dependencies: []resource.URN{"urn:b"}}
	rewrittenRetained, err := rewriteStateMigrationReferences(
		[]*pkgresource.State{retained}, originalToFinal, nil)
	require.NoError(t, err)
	assert.Same(t, retained, rewrittenRetained[0])
}

func TestNewStateMigrationRewriteCopiesSuccessorData(t *testing.T) {
	t.Parallel()

	const (
		oldURN   = resource.URN("urn:old")
		newURN   = resource.URN("urn:new")
		otherURN = resource.URN("urn:other")
	)
	successors := map[resource.URN]resource.URN{oldURN: newURN}
	successor := &pkgresource.State{URN: newURN, Custom: true, ID: "successor-id"}

	rewrite := newStateMigrationRewrite("urn:root", successors, []*pkgresource.State{successor})
	successors[oldURN] = otherURN
	successor.ID = "changed-id"

	assert.Equal(t, newURN, rewrite.successorURNs[oldURN])
	assert.Equal(t, stateMigrationSuccessorIdentity{custom: true, id: "successor-id"},
		rewrite.successorIdentities[newURN])
}

func TestRewriteStateMigrationReferencesRewritesStructuralReferences(t *testing.T) {
	t.Parallel()

	const (
		oldA      = resource.URN("urn:old-a")
		oldB      = resource.URN("urn:old-b")
		successor = resource.URN("urn:successor")
		unrelated = resource.URN("urn:unrelated")
	)
	state := &pkgresource.State{
		Parent:       oldA,
		Dependencies: []resource.URN{oldA, oldB, unrelated},
		PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"value": {oldA, oldB},
		},
		DeletedWith: oldA,
		ReplaceWith: []resource.URN{oldA, oldB},
		ViewOf:      oldB,
	}

	rewritten, err := rewriteStateMigrationReferences(
		[]*pkgresource.State{state},
		map[resource.URN]resource.URN{oldA: successor, oldB: successor},
		nil,
	)
	require.NoError(t, err)
	require.NotSame(t, state, rewritten[0])

	assert.Equal(t, successor, rewritten[0].Parent)
	assert.Equal(t, []resource.URN{successor, unrelated}, rewritten[0].Dependencies)
	assert.Equal(t, map[resource.PropertyKey][]resource.URN{"value": {successor}},
		rewritten[0].PropertyDependencies)
	assert.Equal(t, successor, rewritten[0].DeletedWith)
	assert.Equal(t, []resource.URN{successor}, rewritten[0].ReplaceWith)
	assert.Equal(t, successor, rewritten[0].ViewOf)
}

func TestFinalStateMigrationSuccessorsRejectsCycles(t *testing.T) {
	t.Parallel()

	_, _, err := finalStateMigrationSuccessors(nil, nil, map[resource.URN]resource.URN{
		"urn:a": "urn:b",
		"urn:b": "urn:a",
	})
	require.ErrorContains(t, err, "successor mappings contain a cycle")
}

func TestFinalStateMigrationSuccessorsRejectsUnaccountedOriginal(t *testing.T) {
	t.Parallel()

	_, _, err := finalStateMigrationSuccessors(
		[]apitype.ResourceV3{{URN: "urn:original"}},
		[]apitype.ResourceV3{{URN: "urn:final"}},
		nil,
	)
	require.ErrorContains(t, err, "did not account for resource urn:original")
}

func TestValidateStateMigrationAccountingRejectsAliases(t *testing.T) {
	t.Parallel()

	const (
		rootURN  = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		aliasURN = resource.URN("urn:pulumi:test::test::pkg:m:Resource::unrelated")
	)
	original := apitype.ResourceV3{URN: rootURN, Type: rootURN.Type()}
	returned := original
	returned.Aliases = []resource.URN{aliasURN}

	err := validateStateMigrationAccounting(
		rootURN, []apitype.ResourceV3{original}, []apitype.ResourceV3{returned}, nil)
	require.ErrorContains(t, err, "state migration callbacks must express renames with successor mappings")
}

func TestStateMigrationTransactionRewriteResources(t *testing.T) {
	t.Parallel()

	const (
		oldURN      = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		newURN      = resource.URN("urn:pulumi:test::test::pkg:m:Resource::new")
		providerURN = resource.URN("urn:pulumi:test::test::pulumi:providers:pkg::default")
	)
	target := &pkgresource.State{URN: newURN, Custom: true, ID: "new-id"}
	providerTarget := &pkgresource.State{URN: providerURN, Custom: true, ID: "migration-time-provider-id"}
	providerRef, err := sdkproviders.NewReference(providerURN, "current-provider-id")
	require.NoError(t, err)
	consumer := &pkgresource.State{
		URN:      "urn:pulumi:test::test::pkg:m:Resource::consumer",
		Provider: providerRef.String(),
		Outputs: resource.PropertyMap{
			"reference": resource.MakeSecret(resource.MakeCustomResourceReference(oldURN, "old-id", "1.0.0")),
		},
	}
	unrelated := &pkgresource.State{URN: "urn:pulumi:test::test::pkg:m:Resource::unrelated"}
	transaction := &StateMigrationTransaction{
		SuccessorURNs: map[resource.URN]resource.URN{oldURN: newURN},
		ResultSubtree: []*pkgresource.State{target, providerTarget},
	}

	rewritten, err := transaction.RewriteResources([]*pkgresource.State{consumer, unrelated})
	require.NoError(t, err)
	require.NotSame(t, consumer, rewritten[0])
	require.Same(t, unrelated, rewritten[1])

	ref := rewritten[0].Outputs["reference"].SecretValue().Element.ResourceReferenceValue()
	assert.Equal(t, newURN, ref.URN)
	assert.Equal(t, "new-id", ref.ID.StringValue())
	assert.Empty(t, ref.PackageVersion)
	rewrittenProvider, err := sdkproviders.ParseReference(rewritten[0].Provider)
	require.NoError(t, err)
	assert.Equal(t, resource.ID("current-provider-id"), rewrittenProvider.ID())
}
