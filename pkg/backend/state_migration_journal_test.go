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
	base.Resources[2].ID = base.Resources[0].ID
	replayer := NewJournalReplayer(base)

	migrated := apitype.ResourceV3{
		URN:    "urn:pulumi:test::test::pkgA:m:typA::d",
		Type:   "pkgA:m:typA",
		Custom: true,
		ID:     "id-a",
	}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
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
		Version:     1,
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 1,
		RemoveOld:   &removeOld,
	}))

	// The migration replaces "a" (index 0) with "d". After the rewrite, "c" sits at index 1 of the new base;
	// the recorded deletion must follow it there.
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
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

// TestJournalReplayerStateMigrationBasePatchSupersedesOverlay verifies that a prepared base patch incorporates and
// supersedes any earlier outputs overlay for the same retained base resource, even when the migration moves its index.
func TestJournalReplayerStateMigrationBasePatchSupersedesOverlay(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	base.Resources[0].Custom, base.Resources[0].ID = false, ""
	base.Resources[1].Custom, base.Resources[1].ID = false, ""
	replayer := NewJournalReplayer(base)

	staleOverlay := base.Resources[2]
	staleOverlay.Inputs = map[string]any{"reference": string(base.Resources[0].URN)}
	removeOld := int64(2)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:   1,
		Kind:      apitype.JournalEntryKindOutputs,
		State:     &staleOverlay,
		RemoveOld: &removeOld,
	}))

	migrated := stateMigrationResource("d")
	migrated.Custom, migrated.ID = false, ""
	patched := staleOverlay
	patched.Inputs = map[string]any{"reference": string(migrated.URN)}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 1},
		States:     []apitype.ResourceV3{migrated},
		BaseStatePatches: []apitype.JournalBaseStatePatch{{
			Index: 2,
			State: patched,
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.Len(t, deployment.Deployment.Resources, 2)
	assert.Equal(t, migrated, deployment.Deployment.Resources[0])
	assert.Equal(t, patched, deployment.Deployment.Resources[1])
}

// TestJournalReplayerStateMigrationAppliesBasePatches tests that replay installs the exact rewritten states carried
// by a migration entry instead of interpreting its successor mappings.
func TestJournalReplayerStateMigrationAppliesBasePatches(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	base.Resources[2].ID = base.Resources[0].ID
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
	patched := dependent
	patched.Dependencies = []resource.URN{d.URN}
	patched.PropertyDependencies = map[resource.PropertyKey][]resource.URN{"value": {d.URN}}
	patched.DeletedWith = d.URN
	patched.ReplaceWith = []resource.URN{d.URN}
	patched.Inputs = map[string]any{
		"reference": map[string]any{
			resource.SigKey: resource.ResourceReferenceSig,
			"urn":           string(d.URN),
			"id":            string(d.ID),
		},
		"output": map[string]any{
			resource.SigKey: resource.OutputValueSig,
			"value":         "value",
			"dependencies":  []any{string(d.URN)},
		},
	}
	// A migration folds "a" and "c" into "d".
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 2},
		States:     []apitype.ResourceV3{d},
		BaseStatePatches: []apitype.JournalBaseStatePatch{{
			Index: 3,
			State: patched,
		}},
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

func TestJournalReplayerStateMigrationAppliesNewStatePatches(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)

	current := stateMigrationResource("current")
	current.Inputs = map[string]any{"secret": "encrypted-before-migration"}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 42,
		State:       &current,
	}))

	patched := current
	patched.Inputs = map[string]any{"secret": "encrypted-after-typed-rewrite"}
	successor := stateMigrationResource("successor")
	successor.ID = base.Resources[0].ID
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{successor},
		NewStatePatches: []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patched,
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.NotEmpty(t, deployment.Deployment.Resources)
	assert.Equal(t, patched, deployment.Deployment.Resources[0])
}

func TestJournalReplayerStateMigrationAllowsElidedSameBegin(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)
	// Pulumi Cloud may persist an elided Same entry as a Begin without an Operation. Since there is no provider
	// operation or embedded resource state, it does not participate in the migration transaction.
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindBegin,
		OperationID: 42,
	}))

	successor := stateMigrationResource("successor")
	successor.ID = base.Resources[0].ID
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{successor},
	}))
}

func TestJournalReplayerStateMigrationRejectsIncompleteOperation(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindBegin,
		OperationID: 42,
		Operation: &apitype.OperationV2{
			Resource: base.Resources[1],
			Type:     apitype.OperationTypeUpdating,
		},
	}))

	successor := stateMigrationResource("successor")
	successor.ID = base.Resources[0].ID
	err := replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{successor},
	})
	require.ErrorContains(t, err, "cannot be applied with incomplete operation 42")
}

func TestJournalReplayerRejectsMalformedStateMigration(t *testing.T) {
	t.Parallel()

	validEntry := func(base *apitype.DeploymentV3) apitype.JournalEntry {
		successor := stateMigrationResource("successor")
		successor.ID = base.Resources[0].ID
		return apitype.JournalEntry{
			Version:    2,
			Kind:       apitype.JournalEntryKindStateMigration,
			RemoveOlds: []int64{0},
			States:     []apitype.ResourceV3{successor},
		}
	}

	t.Run("wrong version", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.Version = 1
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "must use version 2")
	})

	t.Run("patch changes lifecycle state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		patched := base.Resources[1]
		patched.Protect = !patched.Protect
		entry.BaseStatePatches = []apitype.JournalBaseStatePatch{{Index: 1, State: patched}}
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "changes non-reference resource state")
	})

	t.Run("patch accepts canonical empty timeouts", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		base.Resources[1].CustomTimeouts = &resource.CustomTimeouts{}
		entry := validEntry(base)
		patched := base.Resources[1]
		patched.CustomTimeouts = nil
		entry.BaseStatePatches = []apitype.JournalBaseStatePatch{{Index: 1, State: patched}}
		require.NoError(t, NewJournalReplayer(base).Add(entry))
	})

	t.Run("patch includes earlier base lifecycle markers", func(t *testing.T) {
		t.Parallel()
		for name, mark := range map[string]func(*apitype.JournalEntry, *int64){
			"delete": func(entry *apitype.JournalEntry, index *int64) {
				entry.DeleteOld = index
			},
			"pending replacement": func(entry *apitype.JournalEntry, index *int64) {
				entry.PendingReplacementOld = index
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				base := stateMigrationBase()
				replayer := NewJournalReplayer(base)
				index := int64(1)
				marker := apitype.JournalEntry{Version: 1, Kind: apitype.JournalEntryKindSuccess}
				mark(&marker, &index)
				require.NoError(t, replayer.Add(marker))

				entry := validEntry(base)
				patched := base.Resources[index]
				if name == "delete" {
					patched.Delete = true
				} else {
					patched.PendingReplacement = true
				}
				entry.BaseStatePatches = []apitype.JournalBaseStatePatch{{Index: index, State: patched}}
				require.NoError(t, replayer.Add(entry))

				deployment, err := replayer.GenerateDeployment()
				require.NoError(t, err)
				require.Len(t, deployment.Deployment.Resources, 3)
				if name == "delete" {
					assert.True(t, deployment.Deployment.Resources[1].Delete)
				} else {
					assert.True(t, deployment.Deployment.Resources[1].PendingReplacement)
				}
			})
		}
	})

	t.Run("duplicate resulting urn", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0] = base.Resources[1]
		entry.States[0].ID = base.Resources[0].ID
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "duplicate resource")
	})

	t.Run("inserted pending-delete state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].Delete = true
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "is marked for deletion")
	})

	t.Run("inserted view state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].ViewOf = base.Resources[1].URN
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "is a view of")
	})

	t.Run("inserted custom state without id", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].ID = ""
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "has no physical ID")
	})

	t.Run("inserted component state with id", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].Custom = false
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "custom false but non-empty ID")
	})

	t.Run("missing parent", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].Parent = "urn:pulumi:test::test::pkgA:m:typA::missing"
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "refers to missing parent")
	})

	t.Run("missing dependency", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].Dependencies = []resource.URN{
			"urn:pulumi:test::test::pkgA:m:typA::missing",
		}
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "refers to missing dependency")
	})

	t.Run("malformed provider reference", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].Provider = "not-a-provider-reference"
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "failed to parse provider reference")
	})

	t.Run("unknown extension", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validEntry(base)
		entry.States[0].ExtensionRef = "missing"
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "references unknown extension")
	})

	t.Run("surviving new state duplicate is rejected atomically", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		replayer := NewJournalReplayer(base)
		entry := validEntry(base)

		current := entry.States[0]
		current.Inputs = map[string]any{"value": "before"}
		require.NoError(t, replayer.Add(apitype.JournalEntry{
			Version:     1,
			Kind:        apitype.JournalEntryKindSuccess,
			OperationID: 42,
			State:       &current,
		}))

		patched := current
		patched.Inputs = map[string]any{"value": "after"}
		entry.NewStatePatches = []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patched,
		}}
		err := replayer.Add(entry)
		require.ErrorContains(t, err, "duplicate resource")

		deployment, generateErr := replayer.GenerateDeployment()
		require.NoError(t, generateErr)
		require.Len(t, deployment.Deployment.Resources, 4)
		assert.Equal(t, map[string]any{"value": "before"}, deployment.Deployment.Resources[0].Inputs)
		assert.Equal(t, base.Resources[0].URN, deployment.Deployment.Resources[1].URN)
	})

	t.Run("new-state patch is included in prospective integrity", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		replayer := NewJournalReplayer(base)
		current := stateMigrationResource("current")
		require.NoError(t, replayer.Add(apitype.JournalEntry{
			Version:     1,
			Kind:        apitype.JournalEntryKindSuccess,
			OperationID: 42,
			State:       &current,
		}))

		entry := validEntry(base)
		patched := current
		patched.Dependencies = []resource.URN{
			"urn:pulumi:test::test::pkgA:m:typA::missing",
		}
		entry.NewStatePatches = []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patched,
		}}
		err := replayer.Add(entry)
		require.ErrorContains(t, err, "refers to missing dependency")
	})

	t.Run("unrelated pending-delete duplicate urn", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		pendingDelete := base.Resources[1]
		pendingDelete.Delete = true
		base.Resources = append(base.Resources, pendingDelete)
		entry := validEntry(base)
		require.NoError(t, NewJournalReplayer(base).Add(entry))
	})

}
