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
	"errors"
	"fmt"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack/snapshot"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func (r *JournalReplayer) applyStateMigration(entry apitype.JournalEntry) error {
	if r.base == nil {
		return errors.New("state migration journal entry has no base snapshot")
	}
	if len(r.base.PendingOperations) != 0 {
		return errors.New("state migration journal entry cannot be applied with pending base operations")
	}
	for operationID, incomplete := range r.incompleteOps {
		// Same steps do not run provider operations, so an unmatched Begin entry for one is safe. Any other unmatched
		// Begin entry blocks the migration.
		if incomplete.Operation != nil {
			return fmt.Errorf(
				"state migration journal entry cannot be applied with incomplete operation %d", operationID)
		}
	}
	if len(entry.RemoveOlds) == 0 {
		return errors.New("state migration journal entry removes no resources")
	}
	if len(entry.States) == 0 {
		return errors.New("state migration journal entry inserts no resources")
	}

	removed := make(map[int64]struct{}, len(entry.RemoveOlds))
	var previous int64 = -1
	for _, index := range entry.RemoveOlds {
		if index < 0 || index >= int64(len(r.base.Resources)) {
			return fmt.Errorf("state migration remove index %d is outside base snapshot with %d resources",
				index, len(r.base.Resources))
		}
		if index <= previous {
			return fmt.Errorf("state migration remove indices must be strictly increasing: %v", entry.RemoveOlds)
		}
		previous = index
		removed[index] = struct{}{}
	}

	// Apply changes to a copy so an error leaves the replayer unchanged.
	staged := *r
	staged.newResources = slices.Clone(r.newResources)

	// Update resources that remain in the base snapshot.
	baseResources := slices.Clone(r.base.Resources)
	patchedBase := make(map[int64]struct{}, len(entry.BaseStatePatches))
	for _, patch := range entry.BaseStatePatches {
		if patch.Index < 0 || patch.Index >= int64(len(baseResources)) {
			return fmt.Errorf("state migration base patch index %d is outside base snapshot with %d resources",
				patch.Index, len(baseResources))
		}
		if _, removed := removed[patch.Index]; removed {
			return fmt.Errorf("state migration base patch index %d is also removed", patch.Index)
		}
		if _, duplicate := patchedBase[patch.Index]; duplicate {
			return fmt.Errorf("state migration contains duplicate base patch index %d", patch.Index)
		}
		patchedBase[patch.Index] = struct{}{}
		baseResources[patch.Index] = patch.State
	}

	// Update resources created or changed earlier in this update.
	patchedNew := make(map[int64]struct{}, len(entry.NewStatePatches))
	for _, patch := range entry.NewStatePatches {
		if _, removed := r.toRemove[patch.OperationID]; removed {
			return fmt.Errorf("state migration new-state patch references removed operation %d", patch.OperationID)
		}
		index, ok := r.operationIDToResourceIndex[patch.OperationID]
		if !ok {
			return fmt.Errorf("state migration new-state patch references unknown operation %d", patch.OperationID)
		}
		if index < 0 || index >= int64(len(r.newResources)) {
			return fmt.Errorf("state migration new-state patch for operation %d resolves to invalid index %d",
				patch.OperationID, index)
		}
		if _, duplicate := patchedNew[patch.OperationID]; duplicate {
			return fmt.Errorf("state migration contains duplicate new-state patch for operation %d", patch.OperationID)
		}
		current := r.newResources[index]
		if current == nil {
			return fmt.Errorf("state migration new-state patch for operation %d resolves to a nil state", patch.OperationID)
		}
		patchedNew[patch.OperationID] = struct{}{}
		state := patch.State
		staged.newResources[index] = &state
	}

	// Validate the persisted replacement resources before adding them to the snapshot.
	insertedURNs := make(map[resource.URN]struct{}, len(entry.States))
	for i, state := range entry.States {
		if !state.URN.IsValid() {
			return fmt.Errorf("state migration inserted state %d has invalid URN %q", i, state.URN)
		}
		if state.Type != state.URN.Type() {
			return fmt.Errorf("state migration inserted state %s has type %s", state.URN, state.Type)
		}
		if state.Delete {
			return fmt.Errorf("state migration inserted state %s is marked for deletion", state.URN)
		}
		if state.ViewOf != "" {
			return fmt.Errorf("state migration inserted state %s is a view of %s", state.URN, state.ViewOf)
		}
		if state.Custom && state.ID == "" {
			return fmt.Errorf("state migration inserted custom state %s has no physical ID", state.URN)
		}
		if state.ExtensionRef != "" {
			_, inBase := r.base.Extensions[state.ExtensionRef]
			_, inJournal := r.extensions[state.ExtensionRef]
			if !inBase && !inJournal {
				return fmt.Errorf("state migration inserted state %s references unknown extension %s",
					state.URN, state.ExtensionRef)
			}
		}
		if _, duplicate := insertedURNs[state.URN]; duplicate {
			return fmt.Errorf("state migration inserts duplicate resource %s", state.URN)
		}
		insertedURNs[state.URN] = struct{}{}
	}

	// Replace the old subtree and track the new index of each remaining resource.
	last := entry.RemoveOlds[len(entry.RemoveOlds)-1]
	newIndices := make(map[int64]int64, len(baseResources))
	resources := make([]apitype.ResourceV3, 0, len(baseResources)-len(entry.RemoveOlds)+len(entry.States))
	for i, res := range baseResources {
		if _, ok := removed[int64(i)]; ok {
			if int64(i) == last {
				resources = append(resources, entry.States...)
			}
			continue
		}
		newIndices[int64(i)] = int64(len(resources))
		resources = append(resources, res)
	}

	newBase := *r.base
	newBase.Resources = resources
	staged.base = &newBase

	// Update resource indexes recorded by earlier journal entries.
	remapSet := func(set map[int64]struct{}) map[int64]struct{} {
		remapped := make(map[int64]struct{}, len(set))
		for index := range set {
			if newIndex, ok := newIndices[index]; ok {
				remapped[newIndex] = struct{}{}
			}
		}
		return remapped
	}
	staged.toDeleteInSnapshot = remapSet(r.toDeleteInSnapshot)
	staged.markAsDeletion = remapSet(r.markAsDeletion)
	staged.markAsPendingReplacement = remapSet(r.markAsPendingReplacement)

	// Update indexes for earlier refresh and output changes. Skip resources already covered by a base patch.
	toReplace := make(map[int64]*apitype.ResourceV3, len(r.toReplaceInSnapshot))
	for index, state := range r.toReplaceInSnapshot {
		if _, patched := patchedBase[index]; patched {
			continue
		}
		if newIndex, ok := newIndices[index]; ok {
			toReplace[newIndex] = state
		}
	}
	staged.toReplaceInSnapshot = toReplace

	prospective, err := staged.GenerateDeployment()
	if err != nil {
		return fmt.Errorf("assembling prospective state migration snapshot: %w", err)
	}
	if err := snapshot.VerifyIntegrity(prospective.Deployment); err != nil {
		return fmt.Errorf("state migration produces invalid snapshot: %w", err)
	}

	*r = staged
	return nil
}
