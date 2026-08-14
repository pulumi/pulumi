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

	require.NoError(t, d.rewriteStateMigrationStateInPlace(state))
	ref := state.Outputs["reference"].SecretValue().Element.ResourceReferenceValue()
	assert.Equal(t, cURN, ref.URN)
	assert.Equal(t, "physical-id", ref.ID.StringValue())
	assert.Empty(t, ref.PackageVersion)
	assert.True(t, state.Protect)
	assert.Equal(t, cURN, d.rewriteStateMigrationURN(aURN))
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

	err := d.rejectStateMigrationPredecessorURN(aURN)
	require.ErrorContains(t, err, "resource "+string(aURN)+" cannot be registered or read")
	assert.ErrorContains(t, err, "state migration for "+string(firstRoot))
	assert.ErrorContains(t, err, "replaced it with "+string(cURN))

	err = d.rejectStateMigrationPredecessorURN(bURN)
	require.ErrorContains(t, err, "state migration for "+string(secondRoot))
	assert.ErrorContains(t, err, "replaced it with "+string(cURN))

	require.NoError(t, d.rejectStateMigrationPredecessorURN(cURN), "a canonical successor URN remains claimable")
	require.NoError(t, d.rejectStateMigrationPredecessorURN(unrelated))
}
