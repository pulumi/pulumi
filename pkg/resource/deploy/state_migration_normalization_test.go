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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
)

func TestDeploymentNormalizesStateAcrossMigrations(t *testing.T) {
	t.Parallel()

	const (
		aURN = resource.URN("urn:pulumi:test::test::pkgA:m:Resource::a")
		bURN = resource.URN("urn:pulumi:test::test::pkgB:m:Resource::b")
		cURN = resource.URN("urn:pulumi:test::test::pkgC:m:Resource::c")
	)
	d := &Deployment{stateMigrationRewrites: []*stateMigrationRewrite{
		newStateMigrationRewrite(aURN,
			map[resource.URN]resource.URN{aURN: bURN},
			[]*pkgresource.State{{URN: bURN, Custom: true, ID: "physical-id"}}),
		newStateMigrationRewrite(bURN,
			map[resource.URN]resource.URN{bURN: cURN},
			[]*pkgresource.State{{URN: cURN, Custom: true, ID: "physical-id"}}),
	}}
	state := &pkgresource.State{
		URN:     "urn:pulumi:test::test::consumer:m:Resource::consumer",
		Protect: true,
		Outputs: resource.PropertyMap{
			"reference": resource.MakeSecret(resource.MakeCustomResourceReference(aURN, "physical-id", "1.2.3")),
		},
	}

	rewritten, err := d.rewriteStateMigrationState(state)
	require.NoError(t, err)
	assert.NotSame(t, state, rewritten)
	ref := rewritten.Outputs["reference"].SecretValue().Element.ResourceReferenceValue()
	assert.Equal(t, cURN, ref.URN)
	assert.Equal(t, "physical-id", ref.ID.StringValue())
	assert.Empty(t, ref.PackageVersion)
	assert.True(t, rewritten.Protect)
	assert.Equal(t, cURN, d.rewriteStateMigrationURN(aURN))
}

func TestDeploymentNormalizesStateMigrationSourceEvents(t *testing.T) {
	t.Parallel()

	const (
		predecessorURN         = resource.URN("urn:pulumi:test::test::pkg:m:Resource::a")
		successorURN           = resource.URN("urn:pulumi:test::test::pkg:m:Resource::b")
		predecessorProviderURN = resource.URN("urn:pulumi:test::test::pulumi:providers:pkg::provider-a")
		successorProviderURN   = resource.URN("urn:pulumi:test::test::pulumi:providers:pkg::provider-b")
		consumerURN            = resource.URN("urn:pulumi:test::test::pkg:m:Resource::consumer")
	)
	predecessorProvider, err := sdkproviders.NewReference(predecessorProviderURN, "provider-a-id")
	require.NoError(t, err)
	d := &Deployment{stateMigrationRewrites: []*stateMigrationRewrite{
		newStateMigrationRewrite(
			"urn:pulumi:test::test::pkg:m:Component::component",
			map[resource.URN]resource.URN{
				predecessorURN:         successorURN,
				predecessorProviderURN: successorProviderURN,
			},
			[]*pkgresource.State{
				{URN: successorURN, Custom: true, ID: "physical-id"},
				{URN: successorProviderURN, Custom: true, ID: "provider-b-id"},
			},
		),
	}}

	t.Run("read", func(t *testing.T) {
		t.Parallel()

		event := &readResourceEvent{
			id:           "read-id",
			name:         "consumer",
			baseType:     consumerURN.Type(),
			parent:       predecessorURN,
			provider:     predecessorProvider.String(),
			dependencies: []resource.URN{predecessorURN},
			props: resource.PropertyMap{
				"reference": resource.MakeCustomResourceReference(predecessorURN, "physical-id", "1.2.3"),
			},
		}

		normalized, err := d.normalizeStateMigrationSourceEvent(event)
		require.NoError(t, err)
		read, ok := normalized.(ReadResourceEvent)
		require.True(t, ok)
		assert.NotSame(t, event, normalized)
		assert.Equal(t, event.ID(), read.ID())
		assert.Equal(t, event.Name(), read.Name())
		assert.Equal(t, event.Type(), read.Type())
		assert.Equal(t, successorURN, read.Parent())
		assert.Equal(t, []resource.URN{successorURN}, read.Dependencies())

		provider, err := sdkproviders.ParseReference(read.Provider())
		require.NoError(t, err)
		assert.Equal(t, successorProviderURN, provider.URN())
		assert.Equal(t, resource.ID("provider-b-id"), provider.ID())

		ref := read.Properties()["reference"].ResourceReferenceValue()
		assert.Equal(t, successorURN, ref.URN)
		assert.Equal(t, "physical-id", ref.ID.StringValue())
		assert.Empty(t, ref.PackageVersion)
	})

	t.Run("outputs", func(t *testing.T) {
		t.Parallel()

		event := &registerResourceOutputsEvent{
			urn: consumerURN,
			outputs: resource.PropertyMap{
				"reference": resource.MakeCustomResourceReference(predecessorURN, "physical-id", "1.2.3"),
			},
		}

		normalized, err := d.normalizeStateMigrationSourceEvent(event)
		require.NoError(t, err)
		outputs, ok := normalized.(RegisterResourceOutputsEvent)
		require.True(t, ok)
		assert.NotSame(t, event, normalized)
		assert.Equal(t, consumerURN, outputs.URN())
		assert.Equal(t, successorURN, outputs.Outputs()["reference"].ResourceReferenceValue().URN)
	})

	t.Run("engine and registration events pass through", func(t *testing.T) {
		t.Parallel()

		registration := &registerResourceEvent{goal: &pkgresource.Goal{}}
		normalized, err := d.normalizeStateMigrationSourceEvent(registration)
		require.NoError(t, err)
		assert.Same(t, registration, normalized)

		continuation := &continueResourceRefreshEvent{RegisterResourceEvent: registration}
		normalized, err = d.normalizeStateMigrationSourceEvent(continuation)
		require.NoError(t, err)
		assert.Same(t, continuation, normalized)
	})

	t.Run("no rewrites preserves event identity", func(t *testing.T) {
		t.Parallel()

		event := &readResourceEvent{name: "unchanged"}
		normalized, err := (&Deployment{}).normalizeStateMigrationSourceEvent(event)
		require.NoError(t, err)
		assert.Same(t, event, normalized)
	})
}

func TestGenerateResourceStepsNormalizesGoalAfterMigration(t *testing.T) {
	t.Parallel()

	const (
		predecessorURN         = resource.URN("urn:pulumi:test::test::pkg:m:Resource::a")
		successorURN           = resource.URN("urn:pulumi:test::test::pkg:m:Resource::b")
		predecessorProviderURN = resource.URN("urn:pulumi:test::test::pulumi:providers:pkg::provider-a")
		successorProviderURN   = resource.URN("urn:pulumi:test::test::pulumi:providers:pkg::provider-b")
		consumerURN            = resource.URN("urn:pulumi:test::test::pkg:m:Component::consumer")
	)
	predecessorProvider, err := sdkproviders.NewReference(predecessorProviderURN, "provider-a-id")
	require.NoError(t, err)
	d := &Deployment{
		ctx:   &plugin.Context{Diag: &diag.MockSink{}},
		opts:  &Options{},
		goals: &gsync.Map[resource.URN, *pkgresource.Goal]{},
		stateMigrationRewrites: []*stateMigrationRewrite{
			newStateMigrationRewrite(
				consumerURN,
				map[resource.URN]resource.URN{
					predecessorURN:         successorURN,
					predecessorProviderURN: successorProviderURN,
				},
				[]*pkgresource.State{
					{URN: successorURN, Custom: true, ID: "physical-id"},
					{URN: successorProviderURN, Custom: true, ID: "provider-b-id"},
				},
			),
		},
	}
	sg := newStepGenerator(d, false, updateMode, nil)
	goal := &pkgresource.Goal{
		Type: consumerURN.Type(),
		Name: consumerURN.Name(),
		Properties: resource.FromResourcePropertyMap(resource.PropertyMap{
			"reference": resource.MakeCustomResourceReference(predecessorURN, "physical-id", "1.2.3"),
		}),
		Parent:       predecessorURN,
		Dependencies: []resource.URN{predecessorURN},
		Provider:     predecessorProvider.String(),
		PropertyDependencies: map[resource.PropertyKey][]resource.URN{
			"reference": {predecessorURN},
		},
		DeletedWith: predecessorURN,
		ReplaceWith: []resource.URN{predecessorURN},
		ReplacementTrigger: resource.FromResourcePropertyValue(
			resource.MakeCustomResourceReference(predecessorURN, "physical-id", "1.2.3")),
	}

	steps, async, err := sg.generateResourceSteps(t.Context(), &registerResourceEvent{goal: goal}, consumerURN)
	require.NoError(t, err)
	assert.False(t, async)
	require.NotEmpty(t, steps)
	newState := steps[0].New()
	require.NotNil(t, newState)
	assert.Equal(t, successorURN, newState.Parent)
	assert.Equal(t, []resource.URN{successorURN}, newState.Dependencies)
	assert.Equal(t, []resource.URN{successorURN}, newState.PropertyDependencies["reference"])
	assert.Equal(t, successorURN, newState.DeletedWith)
	assert.Equal(t, []resource.URN{successorURN}, newState.ReplaceWith)
	assert.Equal(t, successorURN, newState.Inputs["reference"].ResourceReferenceValue().URN)
	assert.Equal(t, successorURN,
		resource.ToResourcePropertyValue(newState.ReplacementTrigger).ResourceReferenceValue().URN)
	provider, err := sdkproviders.ParseReference(newState.Provider)
	require.NoError(t, err)
	assert.Equal(t, successorProviderURN, provider.URN())
	assert.Equal(t, resource.ID("provider-b-id"), provider.ID())

	assert.Equal(t, successorURN, goal.Parent)
	assert.Equal(t, []resource.URN{successorURN}, goal.Dependencies)
	assert.Equal(t, []resource.URN{successorURN}, goal.PropertyDependencies["reference"])
	assert.Equal(t, successorURN, goal.DeletedWith)
	assert.Equal(t, []resource.URN{successorURN}, goal.ReplaceWith)
	assert.Equal(t, successorURN,
		resource.ToResourcePropertyMap(goal.Properties)["reference"].ResourceReferenceValue().URN)
	assert.Equal(t, successorURN,
		resource.ToResourcePropertyValue(goal.ReplacementTrigger).ResourceReferenceValue().URN)
	provider, err = sdkproviders.ParseReference(goal.Provider)
	require.NoError(t, err)
	assert.Equal(t, successorProviderURN, provider.URN())
	assert.Equal(t, resource.ID("provider-b-id"), provider.ID())
}

func TestDeploymentRejectsStateMigrationPredecessorURN(t *testing.T) {
	t.Parallel()

	const (
		aURN       = resource.URN("urn:pulumi:test::test::pkgA:m:Resource::a")
		bURN       = resource.URN("urn:pulumi:test::test::pkgB:m:Resource::b")
		cURN       = resource.URN("urn:pulumi:test::test::pkgC:m:Resource::c")
		unrelated  = resource.URN("urn:pulumi:test::test::pkgA:m:Resource::unrelated")
		firstRoot  = resource.URN("urn:pulumi:test::test::pkgA:m:Component::first")
		secondRoot = resource.URN("urn:pulumi:test::test::pkgB:m:Component::second")
	)
	d := &Deployment{stateMigrationRewrites: []*stateMigrationRewrite{
		newStateMigrationRewrite(firstRoot,
			map[resource.URN]resource.URN{aURN: bURN},
			[]*pkgresource.State{{URN: bURN}}),
		newStateMigrationRewrite(secondRoot,
			map[resource.URN]resource.URN{bURN: cURN},
			[]*pkgresource.State{{URN: cURN}}),
	}}

	require.Error(t, d.rejectStateMigrationPredecessorURN(aURN))
	require.Error(t, d.rejectStateMigrationPredecessorURN(bURN))
	require.NoError(t, d.rejectStateMigrationPredecessorURN(cURN), "a canonical successor URN remains claimable")
	require.NoError(t, d.rejectStateMigrationPredecessorURN(unrelated))
}
