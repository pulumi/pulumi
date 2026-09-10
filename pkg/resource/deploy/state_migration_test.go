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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
)

func prepareAndCommitStateMigration(
	ctx context.Context,
	sg *stepGenerator,
	urn resource.URN,
	priorSubtree, resultSubtree []*pkgresource.State,
	successors map[resource.URN]resource.URN,
) error {
	registrationURN := urn
	if successor, ok := successors[urn]; ok {
		registrationURN = successor
	}
	transaction, err := sg.prepareStateMigrationTransaction(
		registrationURN, priorSubtree[0], priorSubtree, resultSubtree, successors)
	if err != nil {
		return err
	}
	return sg.commitStateMigration(ctx, transaction)
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

	require.NoError(t, prepareAndCommitStateMigration(t.Context(), sg,
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
	require.Len(t, d.stateMigrationRewrites, 1)
	assert.Equal(t, successorURN, d.stateMigrationRewrites[0].successorURNs[oldURN])
	assert.Equal(t, stateMigrationSuccessorIdentity{custom: true, id: "physical-id"},
		d.stateMigrationRewrites[0].successorIdentities[successorURN])
}
