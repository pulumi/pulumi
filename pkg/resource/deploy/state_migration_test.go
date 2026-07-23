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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
)

func TestRedactStatesForLog(t *testing.T) {
	t.Parallel()

	states := []*pkgresource.State{{
		URN:  "urn:pulumi:test::test::pkg:m:t::res",
		Type: "pkg:m:t",
		ID:   "id-1",
		Inputs: resource.PropertyMap{
			"plain":  resource.NewProperty("visible"),
			"secret": resource.MakeSecret(resource.NewProperty("s3cr3t")),
		},
	}}
	out := redactStatesForLog(states)
	assert.NotContains(t, out, "s3cr3t", "secret plaintext must not appear in the debug rendering")
	assert.Contains(t, out, "[secret]")
	assert.Contains(t, out, "visible")
	assert.Contains(t, out, "urn:pulumi:test::test::pkg:m:t::res")
}

func TestCommitStateMigrationPreservesLivePointers(t *testing.T) {
	t.Parallel()

	const (
		oldURN       = resource.URN("urn:pulumi:test::test::pkgA:m:Resource::old")
		successorURN = resource.URN("urn:pulumi:test::test::pkgB:m:Resource::successor")
		consumerURN  = resource.URN("urn:pulumi:test::test::consumer:m:Resource::consumer")
		currentURN   = resource.URN("urn:pulumi:test::test::consumer:m:Resource::current")
	)
	reference := func(urn resource.URN) resource.PropertyValue {
		return resource.MakeSecret(resource.MakeCustomResourceReference(urn, "physical-id", "1.0.0"))
	}
	old := &pkgresource.State{URN: oldURN, Type: oldURN.Type(), Custom: true, ID: "physical-id"}
	consumer := &pkgresource.State{
		URN:          consumerURN,
		Type:         consumerURN.Type(),
		Custom:       true,
		ID:           "consumer-id",
		Dependencies: []resource.URN{oldURN},
		Outputs:      resource.PropertyMap{"reference": reference(oldURN)},
	}
	current := &pkgresource.State{
		URN:     currentURN,
		Type:    currentURN.Type(),
		Custom:  true,
		ID:      "current-id",
		Outputs: resource.PropertyMap{"reference": reference(oldURN)},
	}
	successor := &pkgresource.State{
		URN: successorURN, Type: successorURN.Type(), Custom: true, ID: old.ID,
	}
	news := &gsync.Map[resource.URN, *pkgresource.State]{}
	news.Store(current.URN, current)
	d := &Deployment{
		prev:   &Snapshot{Resources: []*pkgresource.State{old, consumer}},
		news:   news,
		events: &mockEvents{},
	}
	sg := &stepGenerator{deployment: d}

	require.NoError(t, sg.commitStateMigration(
		oldURN,
		[]*pkgresource.State{old},
		[]*pkgresource.State{successor},
		map[resource.URN]resource.URN{oldURN: successorURN},
	))

	require.Len(t, d.prev.Resources, 2)
	assert.Same(t, successor, d.prev.Resources[0])
	assert.Same(t, consumer, d.prev.Resources[1])
	assert.Equal(t, []resource.URN{successorURN}, consumer.Dependencies)
	assert.Equal(t, successorURN,
		consumer.Outputs["reference"].SecretValue().Element.ResourceReferenceValue().URN)
	assert.Equal(t, successorURN,
		current.Outputs["reference"].SecretValue().Element.ResourceReferenceValue().URN)
	require.Len(t, d.stateMigrations, 1)
	assert.Equal(t, successorURN, d.stateMigrations[0].successorURNs[oldURN])
	assert.Equal(t, stateMigrationTarget{custom: true, id: "physical-id"},
		d.stateMigrations[0].targets[successorURN])
}

func TestCommitStateMigrationUsesLiveSuccessorIdentity(t *testing.T) {
	t.Parallel()

	const (
		oldURN       = resource.URN("urn:pulumi:test::test::pkgA:m:Resource::old")
		successorURN = resource.URN("urn:pulumi:test::test::pkgB:m:Resource::successor")
		consumerURN  = resource.URN("urn:pulumi:test::test::consumer:m:Resource::consumer")
	)
	old := &pkgresource.State{URN: oldURN, Type: oldURN.Type(), Custom: true, ID: "live-id"}
	successor := &pkgresource.State{
		URN: successorURN, Type: successorURN.Type(), Custom: true, ID: old.ID,
	}
	pendingDelete := &pkgresource.State{
		URN: successorURN, Type: successorURN.Type(), Custom: true, ID: "stale-id", Delete: true,
	}
	consumer := &pkgresource.State{
		URN:     consumerURN,
		Type:    consumerURN.Type(),
		Custom:  true,
		ID:      "consumer-id",
		Outputs: resource.PropertyMap{"reference": resource.MakeCustomResourceReference(oldURN, "live-id", "")},
	}
	d := &Deployment{
		prev:   &Snapshot{Resources: []*pkgresource.State{old, pendingDelete, consumer}},
		events: &mockEvents{},
	}
	sg := &stepGenerator{deployment: d}

	require.NoError(t, sg.commitStateMigration(
		oldURN,
		[]*pkgresource.State{old},
		[]*pkgresource.State{successor},
		map[resource.URN]resource.URN{oldURN: successorURN},
	))

	ref := consumer.Outputs["reference"].ResourceReferenceValue()
	assert.Equal(t, successorURN, ref.URN)
	assert.Equal(t, "live-id", ref.ID.StringValue(),
		"a pending-deletion state with the successor URN must not supply the live reference identity")
}

func TestCommitStateMigrationRejectsProviderReferenceRewrite(t *testing.T) {
	t.Parallel()

	const (
		oldURN       = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		successorURN = resource.URN("urn:pulumi:test::test::pkg:m:Resource::successor")
	)
	providerURN := resource.NewURN("test", "test", "", "pulumi:providers:pkg", "default")
	provider := &pkgresource.State{
		URN:     providerURN,
		Type:    providerURN.Type(),
		Custom:  true,
		ID:      "provider-id",
		Inputs:  resource.PropertyMap{"reference": resource.MakeCustomResourceReference(oldURN, "old-id", "")},
		Outputs: resource.PropertyMap{},
	}
	old := &pkgresource.State{URN: oldURN, Type: oldURN.Type(), Custom: true, ID: "old-id"}
	successor := &pkgresource.State{
		URN: successorURN, Type: successorURN.Type(), Custom: true, ID: old.ID,
	}
	sg := &stepGenerator{deployment: &Deployment{
		prev:   &Snapshot{Resources: []*pkgresource.State{provider, old}},
		events: &mockEvents{},
	}}

	err := sg.commitStateMigration(
		oldURN,
		[]*pkgresource.State{old},
		[]*pkgresource.State{successor},
		map[resource.URN]resource.URN{oldURN: successorURN},
	)
	require.ErrorContains(t, err, "rewrites references in provider state")
	assert.Equal(t, oldURN, provider.Inputs["reference"].ResourceReferenceValue().URN)
}

func TestCommitStateMigrationRejectsCurrentProviderReferenceRewrite(t *testing.T) {
	t.Parallel()

	const (
		oldURN       = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		successorURN = resource.URN("urn:pulumi:test::test::pkg:m:Resource::successor")
	)
	providerURN := resource.NewURN("test", "test", "", "pulumi:providers:pkg", "default")
	provider := &pkgresource.State{
		URN:     providerURN,
		Type:    providerURN.Type(),
		Custom:  true,
		ID:      "provider-id",
		Inputs:  resource.PropertyMap{"reference": resource.MakeCustomResourceReference(oldURN, "old-id", "")},
		Outputs: resource.PropertyMap{},
	}
	old := &pkgresource.State{URN: oldURN, Type: oldURN.Type(), Custom: true, ID: "old-id"}
	successor := &pkgresource.State{
		URN: successorURN, Type: successorURN.Type(), Custom: true, ID: old.ID,
	}
	news := &gsync.Map[resource.URN, *pkgresource.State]{}
	news.Store(providerURN, provider)
	sg := &stepGenerator{deployment: &Deployment{
		prev:   &Snapshot{Resources: []*pkgresource.State{old}},
		news:   news,
		events: &mockEvents{},
	}}

	err := sg.commitStateMigration(
		oldURN,
		[]*pkgresource.State{old},
		[]*pkgresource.State{successor},
		map[resource.URN]resource.URN{oldURN: successorURN},
	)
	require.ErrorContains(t, err, "created or updated earlier in this deployment")
	assert.Equal(t, oldURN, provider.Inputs["reference"].ResourceReferenceValue().URN)
}
