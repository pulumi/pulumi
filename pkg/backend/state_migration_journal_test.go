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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func stateMigrationBase() *apitype.DeploymentV3 {
	res := func(name string) apitype.ResourceV3 {
		return apitype.ResourceV3{
			URN:    resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
			Type:   "pkgA:m:typA",
			Custom: true,
			ID:     resource.ID("id-" + name),
		}
	}
	return &apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{res("a"), res("b"), res("c")},
	}
}

// TestJournalReplayerStateMigration tests that a state migration journal entry removes the given base indices
// and inserts the migrated states at the position of the last removed resource.
func TestJournalReplayerStateMigration(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)

	migrated := apitype.ResourceV3{
		URN:    "urn:pulumi:test::test::pkgA:m:typA::d",
		Type:   "pkgA:m:typA",
		Custom: true,
		ID:     "id-a",
	}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 2},
		States:     []apitype.ResourceV3{migrated},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	urns := make([]resource.URN, len(deployment.Deployment.Resources))
	for i, res := range deployment.Deployment.Resources {
		urns[i] = res.URN
	}
	// "a" and "c" are removed; "d" takes the position of "c", the last removed resource.
	assert.Equal(t, []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::b",
		"urn:pulumi:test::test::pkgA:m:typA::d",
	}, urns)
}

// TestJournalReplayerStateMigrationRemapsIndices tests that base snapshot indices recorded by entries before a
// state migration keep referring to the same resources after the migration rewrites the base snapshot.
func TestJournalReplayerStateMigrationRemapsIndices(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)

	// Delete base resource "c" (index 2) before the migration runs.
	removeOld := int64(2)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 1,
		RemoveOld:   &removeOld,
	}))

	// The migration replaces "a" (index 0) with "d". After the rewrite, "c" sits at index 1 of the new base;
	// the recorded deletion must follow it there.
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States: []apitype.ResourceV3{{
			URN:    "urn:pulumi:test::test::pkgA:m:typA::d",
			Type:   "pkgA:m:typA",
			Custom: true,
			ID:     "id-a",
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	urns := make([]resource.URN, len(deployment.Deployment.Resources))
	for i, res := range deployment.Deployment.Resources {
		urns[i] = res.URN
	}
	assert.Equal(t, []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::d",
		"urn:pulumi:test::test::pkgA:m:typA::b",
	}, urns)
}

// TestJournalReplayerStateMigrationPrunesReplaceWith tests that a state migration which forgets a resource
// prunes dangling ReplaceWith references from the surviving resources when the deployment is rebuilt — matching
// the engine's in-memory repair — so the replayed checkpoint stays integrity-valid.
func TestJournalReplayerStateMigrationPrunesReplaceWith(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	// "b" is replaced whenever "c" is replaced.
	base.Resources[1].ReplaceWith = []resource.URN{base.Resources[2].URN}
	replayer := NewJournalReplayer(base)

	// A migration forgets "c" (index 2), leaving "a" and "b".
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{2},
		States:     []apitype.ResourceV3{},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	require.Len(t, deployment.Deployment.Resources, 2)
	for _, res := range deployment.Deployment.Resources {
		// "c" is gone, so the dangling ReplaceWith reference to it must have been pruned — not left dangling
		// and not turned into a bogus empty URN.
		assert.NotContains(t, res.ReplaceWith, base.Resources[2].URN)
		assert.NotContains(t, res.ReplaceWith, resource.URN(""))
	}
}
