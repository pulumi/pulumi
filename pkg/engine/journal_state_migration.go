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
	"errors"
	"fmt"
	"maps"
	"slices"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
)

func (sm *JournalSnapshotManager) SupportsStateMigrations() bool {
	return sm != nil && sm.journalVersion >= 2
}

// StateMigration records a prepared migration transaction in an ordered journal entry. A journal update is normally
// reconstructed by applying operation entries to an immutable base snapshot, but a migration can change both that
// base and resource states produced by earlier entries. One journal-v2 entry therefore records the complete change:
//
//   - RemoveOlds identifies PriorSubtree entries by their indices in the original base snapshot.
//   - ResultStates contains the complete replacement subtree to splice into the base.
//   - BaseStatePatches contains complete replacement states for retained base resources whose references changed.
//   - NewStatePatches does the same for resources produced earlier in the update, identified by operation ID.
//
// The patches are complete resource states, not property-level diffs. Reference rewriting happens while the states
// are still typed and decrypted; normal journal serialization then encrypts the prepared values. Replay applies those
// values without rerunning the callback, interpreting SuccessorURNs, or traversing encrypted property data.
func (sm *JournalSnapshotManager) StateMigration(transaction *deploy.StateMigrationTransaction) error {
	if sm == nil {
		return nil
	}
	if transaction == nil {
		return errors.New("state migration transaction must not be nil")
	}
	if sm.journalVersion < 2 {
		return errors.New("the backend does not support state migrations yet; " +
			"the state migration cannot be applied")
	}

	removedSet := make(map[*pkgresource.State]bool, len(transaction.PriorSubtree))
	for _, res := range transaction.PriorSubtree {
		removedSet[res] = true
	}
	indices := make([]int64, 0, len(transaction.PriorSubtree))
	basePatches := make([]JournalBaseStatePatch, 0)
	for i, res := range sm.baseSnapshot.Resources {
		if removedSet[res] {
			indices = append(indices, int64(i))
			continue
		}
		if rewritten, ok := transaction.RetainedResourceRewrites[res]; ok && rewritten != res {
			basePatches = append(basePatches, JournalBaseStatePatch{
				Index: int64(i),
				State: rewritten.Copy(),
			})
		}
	}
	if len(indices) != len(transaction.PriorSubtree) {
		return fmt.Errorf("state migration: only found %d of %d removed resources in the base snapshot",
			len(indices), len(transaction.PriorSubtree))
	}

	states := make([]*pkgresource.State, len(transaction.ResultSubtree))
	for i, res := range transaction.ResultSubtree {
		states[i] = res.Copy()
	}

	// Patch the exact current state of every resource produced by an earlier operation. This map mirrors replay
	// bookkeeping, so failed, removed, and superseded states cannot produce patches for unknown operation IDs.
	newByOperation := make(map[int64]*pkgresource.State)
	sm.currentNewResources.Range(func(operationID int64, state *pkgresource.State) bool {
		newByOperation[operationID] = state
		return true
	})
	operationIDs := slices.Sorted(maps.Keys(newByOperation))
	newStates := make([]*pkgresource.State, len(operationIDs))
	for i, operationID := range operationIDs {
		newStates[i] = newByOperation[operationID]
	}
	rewrittenNew, err := transaction.RewriteResources(newStates)
	if err != nil {
		return fmt.Errorf("rewriting earlier operation state for migration of %s: %w", transaction.RootURN, err)
	}
	newPatches := make([]JournalNewStatePatch, 0, len(operationIDs))
	for i, operationID := range operationIDs {
		if rewrittenNew[i] == newStates[i] {
			continue
		}
		newPatches = append(newPatches, JournalNewStatePatch{
			OperationID: operationID,
			State:       rewrittenNew[i].Copy(),
		})
	}

	entry := sm.newJournalEntry(JournalEntryStateMigration, 0)
	entry.RemoveOlds = indices
	entry.ResultStates = states
	entry.BaseStatePatches = basePatches
	entry.NewStatePatches = newPatches
	return sm.addJournalEntry(entry)
}
