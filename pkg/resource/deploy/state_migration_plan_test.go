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
	canonical, rewrite, err := finalStateMigrationSuccessors(original, final, map[resource.URN]resource.URN{
		"urn:a": "urn:b",
		"urn:b": "urn:c",
	})
	require.NoError(t, err)
	assert.Equal(t, map[resource.URN]resource.URN{"urn:a": "urn:c"}, canonical)
	assert.Equal(t, map[resource.URN]resource.URN{
		"urn:a": "urn:c",
		"urn:b": "urn:c",
	}, rewrite)
}

func TestStateMigrationPlanRewriteResources(t *testing.T) {
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
	plan := &StateMigrationPlan{
		SuccessorURNs:     map[resource.URN]resource.URN{oldURN: newURN},
		MigratedResources: []*pkgresource.State{target, providerTarget},
	}

	rewritten, err := plan.RewriteResources([]*pkgresource.State{consumer, unrelated})
	require.NoError(t, err)
	require.NotSame(t, consumer, rewritten[0])
	require.Same(t, unrelated, rewritten[1])

	ref := rewritten[0].Outputs["reference"].SecretValue().Element.ResourceReferenceValue()
	assert.Equal(t, newURN, ref.URN)
	assert.Equal(t, "new-id", ref.ID.StringValue())
	assert.Empty(t, ref.PackageVersion)
	rewrittenProvider, err := sdkproviders.ParseReference(rewritten[0].Provider)
	require.NoError(t, err)
	assert.Equal(t, resource.ID("current-provider-id"), rewrittenProvider.ID(),
		"an unrelated provider replacement must not be reverted to its migration-time ID")
}

func TestValidateStateMigrationContext(t *testing.T) {
	t.Parallel()

	const (
		rootURN = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		oldURN  = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		newURN  = resource.URN("urn:pulumi:test::test::pkg:m:Resource::new")
	)
	successors := map[resource.URN]resource.URN{oldURN: newURN}

	t.Run("clean full update", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateStateMigrationContext(rootURN, &Options{}, &Snapshot{}, successors))
	})

	for name, opts := range map[string]*Options{
		"target": {
			Targets: NewUrnTargets([]string{string(rootURN)}),
		},
		"exclude": {
			Excludes: NewUrnTargets([]string{string(rootURN)}),
		},
		"replace target": {
			ReplaceTargets: NewUrnTargets([]string{string(rootURN)}),
		},
		"target snippet": {
			TargetSnippets: []string{"snippet-id"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateStateMigrationContext(rootURN, opts, &Snapshot{}, successors)
			require.ErrorContains(t, err, "cannot change state during a targeted or excluded update")
		})
	}

	t.Run("pending operation", func(t *testing.T) {
		t.Parallel()
		snap := &Snapshot{PendingOperations: []pkgresource.Operation{pkgresource.NewOperation(
			&pkgresource.State{URN: "urn:pulumi:test::test::pkg:m:Resource::pending"},
			pkgresource.OperationTypeCreating,
		)}}
		err := validateStateMigrationContext(rootURN, &Options{}, snap, successors)
		require.ErrorContains(t, err, "snapshot has 1 pending operation")
	})

	t.Run("snippet reference", func(t *testing.T) {
		t.Parallel()
		snap := &Snapshot{Snippets: []resource.Snippet{{
			UUID:       "snippet-id",
			References: map[string]string{"dependency": string(oldURN)},
		}}}
		err := validateStateMigrationContext(rootURN, &Options{}, snap, successors)
		require.ErrorContains(t, err, `cannot rewrite snippet "snippet-id" reference "dependency"`)
	})

	t.Run("unrelated snippet reference", func(t *testing.T) {
		t.Parallel()
		snap := &Snapshot{Snippets: []resource.Snippet{{
			UUID: "snippet-id",
			References: map[string]string{
				"dependency": "urn:pulumi:test::test::pkg:m:Resource::unrelated",
			},
		}}}
		require.NoError(t, validateStateMigrationContext(rootURN, &Options{}, snap, successors))
	})
}

func TestValidateStateMigrationManagedIdentity(t *testing.T) {
	t.Parallel()

	const (
		rootURN = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		oldURN  = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		newURN  = resource.URN("urn:pulumi:test::test::pkg:m:Resource::new")
	)
	state := func(urn resource.URN, custom, pending bool, id ...resource.ID) apitype.ResourceV3 {
		resourceID := resource.ID("id")
		if len(id) > 0 {
			resourceID = id[0]
		}
		return apitype.ResourceV3{
			URN:                urn,
			Type:               urn.Type(),
			Custom:             custom,
			ID:                 resourceID,
			PendingReplacement: pending,
		}
	}

	t.Run("renamed successor preserves flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true, true)},
			[]apitype.ResourceV3{state(newURN, true, true)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.NoError(t, err)
	})

	t.Run("cannot clear flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true, true)},
			[]apitype.ResourceV3{state(newURN, true, false)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes PendingReplacement")
	})

	t.Run("cannot forge flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true, false)},
			[]apitype.ResourceV3{state(newURN, true, true)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes PendingReplacement")
	})

	t.Run("new resource cannot invent flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(rootURN, false, false)},
			[]apitype.ResourceV3{state(rootURN, false, false), state(newURN, true, true)},
			nil,
		)
		require.ErrorContains(t, err, "without a pending-replacement custom predecessor")
	})

	t.Run("cannot clear taint", func(t *testing.T) {
		t.Parallel()
		old := state(oldURN, true, false)
		old.Taint = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{old},
			[]apitype.ResourceV3{state(newURN, true, false)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes Taint")
	})

	t.Run("cannot forge taint", func(t *testing.T) {
		t.Parallel()
		successor := state(newURN, true, false)
		successor.Taint = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true, false)},
			[]apitype.ResourceV3{successor},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes Taint")
	})

	t.Run("new resource cannot invent taint", func(t *testing.T) {
		t.Parallel()
		added := state(newURN, true, false)
		added.Taint = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(rootURN, false, false)},
			[]apitype.ResourceV3{state(rootURN, false, false), added},
			nil,
		)
		require.ErrorContains(t, err, "without a tainted custom predecessor")
	})

	t.Run("component does not constrain custom successor", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{
				state(rootURN, false, false),
				state(oldURN, true, true),
			},
			[]apitype.ResourceV3{state(newURN, true, true)},
			map[resource.URN]resource.URN{
				rootURN: newURN,
				oldURN:  newURN,
			},
		)
		require.NoError(t, err)
	})

	t.Run("cannot change physical ID", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true, false, "old-id")},
			[]apitype.ResourceV3{state(newURN, true, false, "new-id")},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes the physical ID")
	})

	t.Run("cannot change ownership", func(t *testing.T) {
		t.Parallel()
		old := state(oldURN, true, false)
		successor := state(newURN, true, false)
		successor.External = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{old},
			[]apitype.ResourceV3{successor},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes ownership")
	})

	t.Run("cannot map custom resource to component", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true, false)},
			[]apitype.ResourceV3{state(newURN, false, false)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "maps managed custom resource")
	})

	t.Run("cannot fold distinct managed IDs", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{
				state(oldURN, true, false, "old-id"),
				state(newURN, true, false, "new-id"),
			},
			[]apitype.ResourceV3{state(rootURN, true, false, "old-id")},
			map[resource.URN]resource.URN{
				oldURN: rootURN,
				newURN: rootURN,
			},
		)
		require.ErrorContains(t, err, "changes the physical ID")
	})
}

func TestValidateStateMigrationProviderStates(t *testing.T) {
	t.Parallel()

	const rootURN = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
	providerURN := resource.NewURN("test", "test", "", "pulumi:providers:pkg", "default")
	provider := apitype.ResourceV3{
		URN:    providerURN,
		Type:   providerURN.Type(),
		Custom: true,
		ID:     "provider-id",
		Inputs: map[string]any{"region": "us-west-2"},
	}

	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateStateMigrationProviderStates(
			rootURN, []apitype.ResourceV3{provider}, []apitype.ResourceV3{provider}))
	})

	t.Run("removed or renamed", func(t *testing.T) {
		t.Parallel()
		renamed := provider
		renamed.URN = resource.NewURN("test", "test", "", "pulumi:providers:pkg", "renamed")
		err := validateStateMigrationProviderStates(
			rootURN, []apitype.ResourceV3{provider}, []apitype.ResourceV3{renamed})
		require.ErrorContains(t, err, "removes or renames provider state")
	})

	t.Run("reconfigured", func(t *testing.T) {
		t.Parallel()
		reconfigured := provider
		reconfigured.Inputs = map[string]any{"region": "eu-west-1"}
		err := validateStateMigrationProviderStates(
			rootURN, []apitype.ResourceV3{provider}, []apitype.ResourceV3{reconfigured})
		require.ErrorContains(t, err, "changes provider state")
	})

	t.Run("introduced", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationProviderStates(rootURN, nil, []apitype.ResourceV3{provider})
		require.ErrorContains(t, err, "introduces provider state")
	})
}

func TestValidateMigratedStatesRejectsAlreadyRegisteredURN(t *testing.T) {
	t.Parallel()

	const (
		rootURN      = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		collisionURN = resource.URN("urn:pulumi:test::test::pkg:m:Component$pkg:m:Component::collision")
	)
	root := &pkgresource.State{URN: rootURN, Type: rootURN.Type()}
	collision := &pkgresource.State{
		URN:    collisionURN,
		Type:   collisionURN.Type(),
		Parent: rootURN,
	}
	sg := &stepGenerator{
		deployment: &Deployment{olds: map[resource.URN]*pkgresource.State{rootURN: root}},
		urns: map[resource.URN]bool{
			rootURN:      true,
			collisionURN: true,
		},
		aliased: map[resource.URN]resource.URN{},
	}

	err := sg.validateMigratedStates(rootURN, root, []*pkgresource.State{root},
		[]*pkgresource.State{root, collision}, nil)
	require.ErrorContains(t, err, "was already registered earlier in this deployment")
}
