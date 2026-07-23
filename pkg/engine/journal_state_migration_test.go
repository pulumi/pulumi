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

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// TestJournalStateMigrationVersionGate tests that a state migration is rejected when the negotiated journal
// version is too old to encode it, and journaled with the removed base indices when it is supported.
func TestJournalStateMigrationVersionGate(t *testing.T) {
	t.Parallel()

	newState := func(name string) *pkgresource.State {
		return &pkgresource.State{
			URN:    resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
			Type:   "pkgA:m:typA",
			Custom: true,
			ID:     resource.ID("id-" + name),
		}
	}
	a, b, c := newState("a"), newState("b"), newState("c")
	base := &deploy.Snapshot{Resources: []*pkgresource.State{a, b, c}}

	t.Run("rejected on journal version 1", func(t *testing.T) {
		t.Parallel()
		journal := &captureJournal{}
		sm, err := NewJournalSnapshotManagerWithVersion(journal, base, b64.NewBase64SecretsManager(), 1)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sm.Close()) })
		assert.False(t, sm.SupportsStateMigrations())

		d := newState("d")
		err = sm.StateMigration(&deploy.StateMigrationPlan{
			RootURN:           a.URN,
			RemovedResources:  []*pkgresource.State{a},
			MigratedResources: []*pkgresource.State{d},
			SuccessorURNs:     map[resource.URN]resource.URN{a.URN: d.URN},
			BaseResources:     []*pkgresource.State{d, b, c},
			RetainedResources: map[*pkgresource.State]*pkgresource.State{b: b, c: c},
		})
		assert.ErrorContains(t, err, "does not support state migrations")
	})

	t.Run("journaled on journal version 2", func(t *testing.T) {
		t.Parallel()
		journal := &captureJournal{}
		sm, err := NewJournalSnapshotManagerWithVersion(
			journal, base, b64.NewBase64SecretsManager(), apitype.LatestJournalVersion)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sm.Close()) })
		assert.True(t, sm.SupportsStateMigrations())

		d := newState("d")
		successors := map[resource.URN]resource.URN{a.URN: d.URN, c.URN: d.URN}
		require.NoError(t, sm.StateMigration(&deploy.StateMigrationPlan{
			RootURN:           a.URN,
			RemovedResources:  []*pkgresource.State{a, c},
			MigratedResources: []*pkgresource.State{d},
			SuccessorURNs:     successors,
			BaseResources:     []*pkgresource.State{b, d},
			RetainedResources: map[*pkgresource.State]*pkgresource.State{b: b},
		}))

		var entry *JournalEntry
		for i := range journal.entries {
			if journal.entries[i].Kind == JournalEntryStateMigration {
				entry = &journal.entries[i]
			}
		}
		require.NotNil(t, entry, "expected a state migration journal entry")
		assert.Equal(t, []int64{0, 2}, entry.RemoveOlds)
		require.Len(t, entry.MigratedStates, 1)
		assert.Equal(t, d.URN, entry.MigratedStates[0].URN)
	})
}

func TestJournalStateMigrationPersistsTypedPatches(t *testing.T) {
	t.Parallel()

	newState := func(name string) *pkgresource.State {
		return &pkgresource.State{
			URN:    resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
			Type:   "pkgA:m:typA",
			Custom: true,
			ID:     resource.ID("id-" + name),
		}
	}
	secretReference := func(target *pkgresource.State) resource.PropertyValue {
		return resource.MakeSecret(resource.MakeCustomResourceReference(target.URN, target.ID, "1.0.0"))
	}
	assertReference := func(t *testing.T, state, target *pkgresource.State) {
		t.Helper()
		ref := state.Outputs["reference"].SecretValue().Element.ResourceReferenceValue()
		assert.Equal(t, target.URN, ref.URN)
		assert.Equal(t, string(target.ID), ref.ID.StringValue())
		assert.Empty(t, ref.PackageVersion)
	}

	old := newState("old")
	retained := newState("retained")
	retained.Outputs = resource.PropertyMap{"reference": secretReference(old)}
	base := &deploy.Snapshot{Resources: []*pkgresource.State{old, retained}}

	journal := &captureJournal{}
	sm, err := NewJournalSnapshotManagerWithVersion(
		journal, base, b64.NewBase64SecretsManager(), apitype.LatestJournalVersion)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sm.Close()) })

	// This state represents an operation that completed before the migration. Its original journal entry may already
	// have encrypted the secret, so the migration entry must carry a complete replacement keyed by operation ID.
	earlier := newState("earlier")
	earlier.Outputs = resource.PropertyMap{"reference": secretReference(old)}
	require.NoError(t, sm.addJournalEntry(JournalEntry{
		Kind:        JournalEntrySuccess,
		OperationID: 42,
		State:       earlier,
	}))
	unrelated := newState("unrelated")
	require.NoError(t, sm.addJournalEntry(JournalEntry{
		Kind:        JournalEntrySuccess,
		OperationID: 43,
		State:       unrelated,
	}))
	failed := newState("failed")
	failed.Outputs = resource.PropertyMap{"reference": secretReference(old)}
	require.NoError(t, sm.addJournalEntry(JournalEntry{
		Kind:        JournalEntryFailure,
		OperationID: 99,
		State:       failed,
	}))

	successor := newState("successor")
	rewrittenRetained := retained.Copy()
	rewrittenRetained.Outputs = resource.PropertyMap{"reference": resource.MakeSecret(
		resource.MakeCustomResourceReference(successor.URN, successor.ID, ""))}
	plan := &deploy.StateMigrationPlan{
		RootURN:           old.URN,
		RemovedResources:  []*pkgresource.State{old},
		MigratedResources: []*pkgresource.State{successor},
		SuccessorURNs:     map[resource.URN]resource.URN{old.URN: successor.URN},
		BaseResources:     []*pkgresource.State{successor, rewrittenRetained},
		RetainedResources: map[*pkgresource.State]*pkgresource.State{retained: rewrittenRetained},
	}
	require.NoError(t, sm.StateMigration(plan))

	var migration *JournalEntry
	for i := range journal.entries {
		if journal.entries[i].Kind == JournalEntryStateMigration {
			migration = &journal.entries[i]
		}
	}
	require.NotNil(t, migration)
	require.Len(t, migration.BaseStatePatches, 1)
	assert.Equal(t, int64(1), migration.BaseStatePatches[0].Index)
	assertReference(t, migration.BaseStatePatches[0].State, successor)
	require.Len(t, migration.NewStatePatches, 1)
	assert.Equal(t, int64(42), migration.NewStatePatches[0].OperationID)
	assertReference(t, migration.NewStatePatches[0].State, successor)
}
