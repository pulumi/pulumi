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
// version is too old to encode it, and journaled with the post-migration layout when it is supported.
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
		err = sm.StateMigration(&deploy.StateMigrationTransaction{
			RootURN:                a.URN,
			PriorSubtree:           []*pkgresource.State{a},
			ResultSubtree:          []*pkgresource.State{d},
			SuccessorURNs:          map[resource.URN]resource.URN{a.URN: d.URN},
			PreparedPriorResources: []*pkgresource.State{d, b, c},
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
		require.NoError(t, sm.StateMigration(&deploy.StateMigrationTransaction{
			RootURN:                a.URN,
			PriorSubtree:           []*pkgresource.State{a, c},
			ResultSubtree:          []*pkgresource.State{d},
			SuccessorURNs:          successors,
			PreparedPriorResources: []*pkgresource.State{b, d},
		}))

		entry := stateMigrationEntry(t, journal)
		assert.Equal(t, []apitype.JournalLayoutItem{layoutBaseItem(1), layoutStateItem(0)}, entry.Layout)
		require.Len(t, entry.ResultStates, 1)
		assert.Equal(t, d.URN, entry.ResultStates[0].URN)
	})
}

func TestJournalStateMigrationLayout(t *testing.T) {
	t.Parallel()

	newState := func(name string) *pkgresource.State {
		urn := resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name)
		return &pkgresource.State{URN: urn, Type: urn.Type(), Custom: true, ID: resource.ID("id-" + name)}
	}
	root, oldChild, consumer, lastChild :=
		newState("root"), newState("old-child"), newState("consumer"), newState("last-child")
	consumer.Dependencies = []resource.URN{oldChild.URN}
	base := &deploy.Snapshot{Resources: []*pkgresource.State{root, oldChild, consumer, lastChild}}

	newRoot := root.Copy()
	newChild := newState("new-child")
	newLastChild := lastChild.Copy()
	rewrittenConsumer := consumer.Copy()
	rewrittenConsumer.Dependencies = []resource.URN{newChild.URN}

	newTransaction := func(prepared ...*pkgresource.State) *deploy.StateMigrationTransaction {
		return &deploy.StateMigrationTransaction{
			RootURN:                root.URN,
			PriorSubtree:           []*pkgresource.State{root, oldChild, lastChild},
			ResultSubtree:          []*pkgresource.State{newRoot, newChild, newLastChild},
			SuccessorURNs:          map[resource.URN]resource.URN{oldChild.URN: newChild.URN},
			PreparedPriorResources: prepared,
			RetainedResourceRewrites: map[*pkgresource.State]*pkgresource.State{
				consumer: rewrittenConsumer,
			},
		}
	}
	newManager := func(t *testing.T) (*JournalSnapshotManager, *captureJournal) {
		journal := &captureJournal{}
		sm, err := NewJournalSnapshotManagerWithVersion(
			journal, base, b64.NewBase64SecretsManager(), apitype.LatestJournalVersion)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sm.Close()) })
		return sm, journal
	}

	t.Run("retained resource between inserted resources", func(t *testing.T) {
		t.Parallel()
		sm, journal := newManager(t)
		require.NoError(t, sm.StateMigration(newTransaction(newRoot, newChild, rewrittenConsumer, newLastChild)))

		entry := stateMigrationEntry(t, journal)
		assert.Equal(t, []apitype.JournalLayoutItem{
			layoutStateItem(0), layoutStateItem(1), layoutBaseItem(2), layoutStateItem(2),
		}, entry.Layout)
		assertCopiedStates(t, []*pkgresource.State{newRoot, newChild, newLastChild}, entry.ResultStates)
		require.Len(t, entry.BaseStatePatches, 1)
		patch := entry.BaseStatePatches[0]
		assert.Equal(t, int64(2), patch.Index)
		assert.Equal(t, consumer.URN, patch.State.URN)
		assert.Equal(t, []resource.URN{newChild.URN}, patch.State.Dependencies)
		assert.NotSame(t, rewrittenConsumer, patch.State)
	})

	t.Run("inserted resources in prepared order", func(t *testing.T) {
		t.Parallel()
		sm, journal := newManager(t)
		require.NoError(t, sm.StateMigration(newTransaction(newRoot, newLastChild, newChild, rewrittenConsumer)))

		entry := stateMigrationEntry(t, journal)
		assert.Equal(t, []apitype.JournalLayoutItem{
			layoutStateItem(0), layoutStateItem(1), layoutStateItem(2), layoutBaseItem(2),
		}, entry.Layout)
		assertCopiedStates(t, []*pkgresource.State{newRoot, newLastChild, newChild}, entry.ResultStates)
	})

	t.Run("missing result resource", func(t *testing.T) {
		t.Parallel()
		sm, journal := newManager(t)
		err := sm.StateMigration(newTransaction(newRoot, rewrittenConsumer, newLastChild))
		assert.ErrorContains(t, err, "only found 2 of 3 result resources")
		assertNoStateMigrationEntry(t, journal)
	})

	t.Run("missing retained resource", func(t *testing.T) {
		t.Parallel()
		sm, journal := newManager(t)
		err := sm.StateMigration(newTransaction(newRoot, newChild, newLastChild))
		assert.ErrorContains(t, err, "only found 0 of 1 retained base resources")
		assertNoStateMigrationEntry(t, journal)
	})

	t.Run("retains a removed resource", func(t *testing.T) {
		t.Parallel()
		sm, journal := newManager(t)
		err := sm.StateMigration(newTransaction(newRoot, oldChild, newChild, rewrittenConsumer, newLastChild))
		assert.ErrorContains(t, err, "is not a retained base resource")
		assertNoStateMigrationEntry(t, journal)
	})
}

func findStateMigrationEntry(journal *captureJournal) *JournalEntry {
	for i := range journal.entries {
		if journal.entries[i].Kind == JournalEntryStateMigration {
			return &journal.entries[i]
		}
	}
	return nil
}

func stateMigrationEntry(t *testing.T, journal *captureJournal) *JournalEntry {
	t.Helper()
	entry := findStateMigrationEntry(journal)
	require.NotNil(t, entry, "expected a state migration journal entry")
	return entry
}

func assertNoStateMigrationEntry(t *testing.T, journal *captureJournal) {
	t.Helper()
	assert.Nil(t, findStateMigrationEntry(journal), "expected no state migration journal entry")
}

func assertCopiedStates(t *testing.T, expected, actual []*pkgresource.State) {
	t.Helper()
	require.Len(t, actual, len(expected))
	for i, state := range expected {
		assert.Equal(t, state.URN, actual[i].URN)
		assert.Equal(t, state.ID, actual[i].ID)
		assert.NotSame(t, state, actual[i])
	}
}
