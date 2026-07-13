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
		sm, err := NewJournalSnapshotManager(journal, base, b64.NewBase64SecretsManager(), 1)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sm.Close()) })

		err = sm.StateMigration([]*pkgresource.State{a}, []*pkgresource.State{newState("d")}, map[resource.URN]resource.URN{
			a.URN: "urn:pulumi:test::test::pkgA:m:typA::d",
		})
		assert.ErrorContains(t, err, "does not support state migrations")
	})

	t.Run("journaled on journal version 2", func(t *testing.T) {
		t.Parallel()
		journal := &captureJournal{}
		sm, err := NewJournalSnapshotManager(journal, base, b64.NewBase64SecretsManager(), apitype.LatestJournalVersion)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sm.Close()) })

		d := newState("d")
		successors := map[resource.URN]resource.URN{a.URN: d.URN, c.URN: d.URN}
		require.NoError(t, sm.StateMigration([]*pkgresource.State{a, c}, []*pkgresource.State{d}, successors))

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
		assert.Equal(t, successors, entry.Successors)
	})
}
