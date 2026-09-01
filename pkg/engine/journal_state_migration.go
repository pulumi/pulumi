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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func (sm *JournalSnapshotManager) SupportsStateMigrations() bool {
	return sm != nil && sm.journalVersion >= 2
}

// StateMigration records a prepared migration transaction in an ordered journal entry. A journal update is normally
// reconstructed by applying operation entries to an immutable base snapshot, but a migration can change both that
// base and resource states produced by earlier entries. One journal-v2 entry therefore records the complete change:
//
//   - Layout lists the complete post-migration base snapshot: retained resources by their index in the current base
//     snapshot and inserted resources by their index in ResultStates. Base resources absent from Layout are removed.
//   - ResultStates contains the complete inserted resources, in Layout order.
//   - BaseStatePatches contains complete replacement states for retained base resources whose references changed.
//   - NewStatePatches does the same for resources produced earlier in the update, identified by operation ID.
func (sm *JournalSnapshotManager) StateMigration(transaction *deploy.StateMigrationTransaction) error {
	if sm == nil {
		return nil
	}
	contract.Assertf(transaction != nil, "state migration transaction must not be nil")
	if sm.journalVersion < 2 {
		return errors.New("the backend does not support state migrations yet; " +
			"the state migration cannot be applied")
	}

	entry := sm.newJournalEntry(JournalEntryStateMigration, 0)
	if err := layOutStateMigration(&entry, sm.baseSnapshot.Resources, transaction); err != nil {
		return err
	}

	// Rewrite surviving resources produced earlier in this update. JournalReplayer stores these separately from the
	// base snapshot and looks them up by operation ID, so each changed state is recorded as a NewStatePatch for that
	// operation. replayStatesByOperationID contains only the latest states that JournalReplayer would retain.
	newByOperation := make(map[int64]*pkgresource.State)
	sm.replayStatesByOperationID.Range(func(operationID int64, state *pkgresource.State) bool {
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
	entry.NewStatePatches = newPatches
	return sm.addJournalEntry(entry)
}

func layOutStateMigration(
	entry *JournalEntry, base []*pkgresource.State, transaction *deploy.StateMigrationTransaction,
) error {
	baseIndices := make(map[*pkgresource.State]int64, len(base))
	for i, state := range base {
		baseIndices[state] = int64(i)
	}
	removed := make(map[*pkgresource.State]bool, len(transaction.PriorSubtree))
	for _, state := range transaction.PriorSubtree {
		removed[state] = true
	}
	inserted := make(map[*pkgresource.State]bool, len(transaction.ResultSubtree))
	for _, state := range transaction.ResultSubtree {
		inserted[state] = true
	}
	// Retained resources whose references were rewritten appear in the prepared list as copies. Map each copy back to
	// the base resource it replaces.
	retainedOrigins := make(map[*pkgresource.State]*pkgresource.State, len(transaction.RetainedResourceRewrites))
	for original, prepared := range transaction.RetainedResourceRewrites {
		retainedOrigins[prepared] = original
	}

	layout := make([]apitype.JournalLayoutItem, 0, len(transaction.PreparedPriorResources))
	states := make([]*pkgresource.State, 0, len(transaction.ResultSubtree))
	patches := make([]JournalBaseStatePatch, 0, len(transaction.RetainedResourceRewrites))
	placedStates := make(map[*pkgresource.State]bool, len(transaction.ResultSubtree))
	placedBase := make(map[int64]bool, len(base))
	for _, state := range transaction.PreparedPriorResources {
		if inserted[state] {
			if placedStates[state] {
				return fmt.Errorf("state migration: prepared snapshot contains result resource %s more than once",
					state.URN)
			}
			placedStates[state] = true
			states = append(states, state.Copy())
			layout = append(layout, layoutStateItem(int64(len(states)-1)))
			continue
		}

		original, rewritten := retainedOrigins[state]
		if !rewritten {
			original = state
		}
		index, inBase := baseIndices[original]
		if !inBase || removed[original] {
			return fmt.Errorf("state migration: prepared snapshot retains %s, which is not a retained base resource",
				state.URN)
		}
		if placedBase[index] {
			return fmt.Errorf("state migration: prepared snapshot contains base resource %s more than once", state.URN)
		}
		placedBase[index] = true
		if rewritten {
			patches = append(patches, JournalBaseStatePatch{Index: index, State: state.Copy()})
		}
		layout = append(layout, layoutBaseItem(index))
	}
	if len(states) != len(transaction.ResultSubtree) {
		return fmt.Errorf("state migration: only found %d of %d result resources in the prepared snapshot",
			len(states), len(transaction.ResultSubtree))
	}
	retainedCount := 0
	for _, state := range base {
		if !removed[state] {
			retainedCount++
		}
	}
	if len(placedBase) != retainedCount {
		return fmt.Errorf("state migration: only found %d of %d retained base resources in the prepared snapshot",
			len(placedBase), retainedCount)
	}

	entry.Layout, entry.ResultStates, entry.BaseStatePatches = layout, states, patches
	return nil
}

func layoutBaseItem(index int64) apitype.JournalLayoutItem {
	return apitype.JournalLayoutItem{BaseIndex: &index}
}

func layoutStateItem(index int64) apitype.JournalLayoutItem {
	return apitype.JournalLayoutItem{StateIndex: &index}
}
