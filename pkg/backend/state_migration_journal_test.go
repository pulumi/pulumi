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

func stateMigrationComponent(name string) apitype.ResourceV3 {
	return apitype.ResourceV3{
		URN:  resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
		Type: "pkgA:m:typA",
	}
}

func stateMigrationCustomResource(name string) apitype.ResourceV3 {
	state := stateMigrationComponent(name)
	state.Custom = true
	state.ID = resource.ID("id-" + name)
	return state
}

func stateMigrationResourceReference(urn resource.URN) map[string]any {
	return map[string]any{
		resource.SigKey: resource.ResourceReferenceSig,
		"urn":           string(urn),
	}
}

func stateMigrationComponentBase() *apitype.DeploymentV3 {
	return &apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			stateMigrationComponent("a"),
			stateMigrationComponent("b"),
			stateMigrationComponent("c"),
		},
	}
}

func validComponentStateMigrationEntry() apitype.JournalEntry {
	return apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{stateMigrationComponent("successor")},
	}
}

// TestJournalReplayerStateMigration tests that a state migration journal entry removes the given base indices
// and inserts the migrated states at the position of the last removed resource.
func TestJournalReplayerStateMigration(t *testing.T) {
	t.Parallel()

	base := stateMigrationComponentBase()
	replayer := NewJournalReplayer(base)

	migrated := stateMigrationComponent("d")
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

	base := stateMigrationComponentBase()
	base.Resources[2] = stateMigrationCustomResource("c")
	dependent := stateMigrationComponent("e")
	dependent.Dependencies = []resource.URN{base.Resources[2].URN}
	base.Resources = append(base.Resources, dependent)
	replayer := NewJournalReplayer(base)

	// Refresh deletes base resource "c" (index 2) before the migration runs.
	removeOld := int64(2)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindRefreshSuccess,
		OperationID: 1,
		RemoveOld:   &removeOld,
	}))

	// The earlier refresh recorded that base resource "c" (index 2) should be deleted. The migration folds "a" and "b"
	// into "d", moving "c" to index 1. The pending deletion must follow "c" to its new index. Once "c" is removed,
	// ordinary refresh cleanup prunes "e"'s dangling dependency on it.
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 1},
		States:     []apitype.ResourceV3{stateMigrationComponent("d")},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	urns := make([]resource.URN, len(deployment.Deployment.Resources))
	for i, res := range deployment.Deployment.Resources {
		urns[i] = res.URN
	}
	assert.Equal(t, []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::d",
		"urn:pulumi:test::test::pkgA:m:typA::e",
	}, urns)
	assert.Empty(t, deployment.Deployment.Resources[1].Dependencies)
}

// TestJournalReplayerStateMigrationPatchReplacesEarlierOutputs verifies that a migration patch replaces an earlier
// Outputs entry for the same retained base resource. The patch contains that resource's latest state with its
// references rewritten by the migration.
func TestJournalReplayerStateMigrationPatchReplacesEarlierOutputs(t *testing.T) {
	t.Parallel()

	base := stateMigrationComponentBase()
	replayer := NewJournalReplayer(base)

	// An earlier Outputs entry records a new version of retained resource "c" that still refers to predecessor "a".
	earlierOutputs := base.Resources[2]
	earlierOutputs.Inputs = map[string]any{
		"reference": stateMigrationResourceReference(base.Resources[0].URN),
	}
	removeOld := int64(2)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:   1,
		Kind:      apitype.JournalEntryKindOutputs,
		State:     &earlierOutputs,
		RemoveOld: &removeOld,
	}))

	// The migration replaces "a" and "b" with "d" and records the latest version of "c" with its reference rewritten.
	// This complete patch must replace the earlier Outputs entry when the deployment is assembled.
	migrated := stateMigrationComponent("d")
	migrationPatch := earlierOutputs
	migrationPatch.Inputs = map[string]any{
		"reference": stateMigrationResourceReference(migrated.URN),
	}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 1},
		States:     []apitype.ResourceV3{migrated},
		BaseStatePatches: []apitype.JournalBaseStatePatch{{
			Index: 2,
			State: migrationPatch,
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.Len(t, deployment.Deployment.Resources, 2)
	assert.Equal(t, migrated, deployment.Deployment.Resources[0])
	assert.Equal(t, migrationPatch, deployment.Deployment.Resources[1])
}

// TestJournalReplayerStateMigrationAppliesPatches tests that replay installs exact prepared replacements for both
// retained base resources and resources produced by earlier operations.
func TestJournalReplayerStateMigrationAppliesPatches(t *testing.T) {
	t.Parallel()

	base := stateMigrationComponentBase()
	dependent := stateMigrationComponent("e")
	dependent.Inputs = map[string]any{
		"reference": stateMigrationResourceReference(base.Resources[0].URN),
	}
	base.Resources = append(base.Resources, dependent)
	replayer := NewJournalReplayer(base)

	current := stateMigrationComponent("current")
	current.Inputs = map[string]any{
		"reference": stateMigrationResourceReference(base.Resources[0].URN),
	}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 42,
		State:       &current,
	}))

	d := stateMigrationComponent("d")
	patched := dependent
	patched.Inputs = map[string]any{
		"reference": stateMigrationResourceReference(d.URN),
	}
	patchedCurrent := current
	patchedCurrent.Inputs = map[string]any{
		"reference": stateMigrationResourceReference(d.URN),
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
		NewStatePatches: []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patchedCurrent,
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	byURN := make(map[resource.URN]apitype.ResourceV3, len(deployment.Deployment.Resources))
	for _, state := range deployment.Deployment.Resources {
		byURN[state.URN] = state
	}
	assert.Equal(t, patched, byURN[dependent.URN])
	assert.Equal(t, patchedCurrent, byURN[current.URN])
}

func TestJournalReplayerStateMigrationWithIncompleteEntry(t *testing.T) {
	t.Parallel()

	t.Run("without provider operation", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		replayer := NewJournalReplayer(base)

		// Pulumi Cloud may persist an elided Same entry as a Begin without an Operation. It carries no resource state
		// for the migration to rewrite, so the migration can safely proceed before its matching Success entry.
		require.NoError(t, replayer.Add(apitype.JournalEntry{
			Version:     1,
			Kind:        apitype.JournalEntryKindBegin,
			OperationID: 42,
		}))
		require.NoError(t, replayer.Add(validComponentStateMigrationEntry()))
	})

	t.Run("with provider operation", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		replayer := NewJournalReplayer(base)

		require.NoError(t, replayer.Add(apitype.JournalEntry{
			Version:     1,
			Kind:        apitype.JournalEntryKindBegin,
			OperationID: 42,
			Operation: &apitype.OperationV2{
				Resource: stateMigrationCustomResource("in-flight"),
				Type:     apitype.OperationTypeUpdating,
			},
		}))

		err := replayer.Add(validComponentStateMigrationEntry())
		require.ErrorContains(t, err, "cannot be applied with incomplete operation 42")
	})
}

// TestJournalReplayerStateMigrationPatchPreservesEarlierLifecycleMarkers verifies that a complete migration patch can
// include lifecycle markers recorded by an earlier journal entry for the same retained base resource.
func TestJournalReplayerStateMigrationPatchPreservesEarlierLifecycleMarkers(t *testing.T) {
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
			base := stateMigrationComponentBase()
			replayer := NewJournalReplayer(base)
			index := int64(1)
			marker := apitype.JournalEntry{Version: 1, Kind: apitype.JournalEntryKindSuccess}
			mark(&marker, &index)
			require.NoError(t, replayer.Add(marker))

			entry := validComponentStateMigrationEntry()
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
}

func TestJournalReplayerStateMigrationValidatesNewStatePatches(t *testing.T) {
	t.Parallel()

	base := stateMigrationComponentBase()
	replayer := NewJournalReplayer(base)
	current := stateMigrationComponent("current")
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 42,
		State:       &current,
	}))

	entry := validComponentStateMigrationEntry()
	patched := current
	patched.Dependencies = []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::missing",
	}
	entry.NewStatePatches = []apitype.JournalNewStatePatch{{
		OperationID: 42,
		State:       patched,
	}}
	require.ErrorContains(t, replayer.Add(entry), "refers to missing dependency")
}

func TestJournalReplayerRejectsMalformedStateMigration(t *testing.T) {
	t.Parallel()

	t.Run("wrong version", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		entry := validComponentStateMigrationEntry()
		entry.Version = 1
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "must use version 2")
	})

	t.Run("inserted pending-delete state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		entry := validComponentStateMigrationEntry()
		entry.States[0].Delete = true
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "is marked for deletion")
	})

	t.Run("inserted view state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		entry := validComponentStateMigrationEntry()
		entry.States[0].ViewOf = base.Resources[1].URN
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "is a view of")
	})

	t.Run("inserted custom state without id", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		entry := validComponentStateMigrationEntry()
		entry.States[0].Custom = true
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "has no physical ID")
	})

	t.Run("missing dependency", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		entry := validComponentStateMigrationEntry()
		entry.States[0].Dependencies = []resource.URN{
			"urn:pulumi:test::test::pkgA:m:typA::missing",
		}
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "refers to missing dependency")
	})

	t.Run("unknown extension", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationComponentBase()
		entry := validComponentStateMigrationEntry()
		entry.States[0].ExtensionRef = "missing"
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "references unknown extension")
	})
}
