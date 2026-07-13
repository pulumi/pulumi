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

func stateMigrationResource(name string) apitype.ResourceV3 {
	return apitype.ResourceV3{
		URN:    resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
		Type:   "pkgA:m:typA",
		Custom: true,
		ID:     resource.ID("id-" + name),
	}
}

func stateMigrationBase() *apitype.DeploymentV3 {
	return &apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			stateMigrationResource("a"),
			stateMigrationResource("b"),
			stateMigrationResource("c"),
		},
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
		Successors: map[string]string{
			string(base.Resources[0].URN): string(migrated.URN),
			string(base.Resources[2].URN): string(migrated.URN),
		},
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
		Successors: map[string]string{
			string(base.Resources[0].URN): "urn:pulumi:test::test::pkgA:m:typA::d",
		},
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

// TestJournalReplayerStateMigrationRewritesReferences tests that a state migration rewrites references from a
// removed resource to its successor when the deployment is rebuilt.
func TestJournalReplayerStateMigrationRewritesReferences(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	oldAURN := base.Resources[0].URN
	oldCURN := base.Resources[2].URN
	dependent := stateMigrationResource("e")
	dependent.Dependencies = []resource.URN{oldAURN, oldCURN}
	dependent.PropertyDependencies = map[resource.PropertyKey][]resource.URN{"value": {oldAURN, oldCURN}}
	dependent.DeletedWith = oldAURN
	dependent.ReplaceWith = []resource.URN{oldAURN, oldCURN}
	dependent.Inputs = map[string]any{
		"reference": map[string]any{
			resource.SigKey: resource.ResourceReferenceSig,
			"urn":           string(oldCURN),
			"id":            "id-c",
		},
		"output": map[string]any{
			resource.SigKey: resource.OutputValueSig,
			"value":         "value",
			"dependencies": []any{
				string(oldAURN),
				string(oldCURN),
			},
		},
	}
	base.Resources = append(base.Resources, dependent)
	replayer := NewJournalReplayer(base)

	d := apitype.ResourceV3{
		URN:    "urn:pulumi:test::test::pkgA:m:typA::d",
		Type:   "pkgA:m:typA",
		Custom: true,
		ID:     "id-a",
	}
	// A migration folds "a" and "c" into "d".
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 2},
		States:     []apitype.ResourceV3{d},
		Successors: map[string]string{
			string(oldAURN): string(d.URN),
			string(oldCURN): string(d.URN),
		},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	require.Len(t, deployment.Deployment.Resources, 3)
	got := deployment.Deployment.Resources[2]
	assert.Equal(t, []resource.URN{d.URN}, got.Dependencies)
	assert.Equal(t, []resource.URN{d.URN}, got.PropertyDependencies["value"])
	assert.Equal(t, d.URN, got.DeletedWith)
	assert.Equal(t, []resource.URN{d.URN}, got.ReplaceWith)
	ref := got.Inputs["reference"].(map[string]any)
	assert.Equal(t, string(d.URN), ref["urn"])
	assert.Equal(t, string(d.ID), ref["id"])
	output := got.Inputs["output"].(map[string]any)
	assert.Equal(t, []any{string(d.URN)}, output["dependencies"])
}
