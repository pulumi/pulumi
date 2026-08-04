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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack/snapshot"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// stateMigrationNonReferenceState returns the canonical serialized form of every field that reference rewriting is
// not allowed to change. New ResourceV3 fields are included by default, so they fail closed until deliberately added
// to the reference-bearing allow-list below.
func stateMigrationNonReferenceState(state apitype.ResourceV3) ([]byte, error) {
	state.Inputs = nil
	state.Outputs = nil
	state.Parent = ""
	state.Dependencies = nil
	state.Provider = ""
	state.PropertyDependencies = nil
	state.DeletedWith = ""
	state.ReplaceWith = nil
	state.ReplacementTrigger = nil
	state.ViewOf = ""

	// Typed state canonicalizes an empty timeout block to nil. Times may also carry equivalent locations or monotonic
	// clock readings. Normalize both representations before comparing serialized checkpoint state.
	if state.CustomTimeouts != nil && !state.CustomTimeouts.IsNotEmpty() {
		state.CustomTimeouts = nil
	}
	normalizeTime := func(value *time.Time) *time.Time {
		if value == nil {
			return nil
		}
		normalized := value.Round(0).UTC()
		return &normalized
	}
	state.Created = normalizeTime(state.Created)
	state.Modified = normalizeTime(state.Modified)

	// At the checkpoint boundary nil and empty collections are equivalent. JSON omitempty canonicalizes top-level
	// collections; normalize empty hook-name lists nested inside the non-empty hooks map as well.
	if len(state.ResourceHooks) == 0 {
		state.ResourceHooks = nil
	} else {
		hooks := make(map[resource.HookType][]string, len(state.ResourceHooks))
		for hook, names := range state.ResourceHooks {
			if len(names) == 0 {
				names = nil
			}
			hooks[hook] = names
		}
		state.ResourceHooks = hooks
	}

	return json.Marshal(state)
}

func validateStateMigrationPatch(original, patched apitype.ResourceV3) error {
	originalState, err := stateMigrationNonReferenceState(original)
	if err != nil {
		return fmt.Errorf("serializing original non-reference resource state: %w", err)
	}
	patchedState, err := stateMigrationNonReferenceState(patched)
	if err != nil {
		return fmt.Errorf("serializing patched non-reference resource state: %w", err)
	}
	if !bytes.Equal(originalState, patchedState) {
		return errors.New("changes non-reference resource state")
	}
	return nil
}

func validateProspectiveStateMigration(prospective *apitype.DeploymentV3) error {
	resources := prospective.Resources
	referenceable := make(map[resource.URN]struct{}, len(resources))
	for i, state := range resources {
		if !state.URN.IsValid() {
			return fmt.Errorf("resource at index %d has invalid URN %q", i, state.URN)
		}
		if state.Type != state.URN.Type() {
			return fmt.Errorf("resource %s has type %s, which does not match its URN type", state.URN, state.Type)
		}
		if !state.Delete {
			referenceable[state.URN] = struct{}{}
		}
		if state.ExtensionRef != "" {
			if _, ok := prospective.Extensions[state.ExtensionRef]; !ok {
				return fmt.Errorf("resource %s references unknown extension %s", state.URN, state.ExtensionRef)
			}
		}
	}
	for _, state := range resources {
		if state.ViewOf != "" {
			if _, ok := referenceable[state.ViewOf]; !ok {
				return fmt.Errorf("view resource %s refers to missing resource %s", state.URN, state.ViewOf)
			}
		}
		for _, urn := range state.ReplaceWith {
			if _, ok := referenceable[urn]; !ok {
				return fmt.Errorf("resource %s has missing replace-with resource %s", state.URN, urn)
			}
		}
	}

	return snapshot.VerifyIntegrity(prospective)
}

// applyStateMigration applies a prepared state-migration transaction. Every reference-bearing state that changed was
// rewritten while it was still typed and decrypted, then serialized into this entry. Replay applies the prepared
// states without reinterpreting successor mappings.
func (r *JournalReplayer) applyStateMigration(entry apitype.JournalEntry) error {
	if r.base == nil {
		return errors.New("state migration journal entry has no base snapshot")
	}
	if len(r.base.PendingOperations) != 0 {
		return errors.New("state migration journal entry cannot be applied with pending base operations")
	}
	for operationID, incomplete := range r.incompleteOps {
		// Cloud can persist an elided Same as a Begin without an Operation. It does not represent an in-flight
		// provider operation and is safe to carry across the migration. A Begin with an Operation would leave its
		// embedded resource state outside the prepared transaction, so reject it.
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

	// Stage every mutation until the deployment assembled from it passes integrity validation. The migration
	// replaces entries in newResources but does not mutate the pointed-to states, so cloning the slice is sufficient;
	// all affected index maps and the base deployment are rebuilt below before assignment.
	staged := *r
	staged.newResources = slices.Clone(r.newResources)

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
		current := r.currentBaseResource(patch.Index)
		if err := validateStateMigrationPatch(current, patch.State); err != nil {
			return fmt.Errorf("state migration base patch at index %d %w", patch.Index, err)
		}
		patchedBase[patch.Index] = struct{}{}
		baseResources[patch.Index] = patch.State
	}

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
		if err := validateStateMigrationPatch(*current, patch.State); err != nil {
			return fmt.Errorf("state migration new-state patch for operation %d %w", patch.OperationID, err)
		}
		patchedNew[patch.OperationID] = struct{}{}
		state := patch.State
		staged.newResources[index] = &state
	}

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

	// The base snapshot object may be shared with the journaler, which replays all entries from scratch on
	// every save: rebase onto a copy rather than mutating it in place, mirroring what Write entries do.
	newBase := *r.base
	newBase.Resources = resources
	staged.base = &newBase

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

	toReplace := make(map[int64]*apitype.ResourceV3, len(r.toReplaceInSnapshot))
	for index, state := range r.toReplaceInSnapshot {
		// The prepared base patch is the state after applying both the earlier operation and this migration. Do not
		// let the earlier refresh/outputs overlay replace it when the deployment is assembled.
		if _, patched := patchedBase[index]; patched {
			continue
		}
		if newIndex, ok := newIndices[index]; ok {
			toReplace[newIndex] = state
		}
	}
	staged.toReplaceInSnapshot = toReplace

	prospective, err := staged.generateDeployment()
	if err != nil {
		return fmt.Errorf("assembling prospective state migration snapshot: %w", err)
	}
	if err := validateProspectiveStateMigration(prospective.Deployment); err != nil {
		return fmt.Errorf("state migration produces invalid snapshot: %w", err)
	}

	*r = staged
	return nil
}
