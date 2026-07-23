// Copyright 2016, Pulumi Corporation.
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
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack/snapshot"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	utilenv "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

func SerializeJournalEntry(
	ctx context.Context, je engine.JournalEntry, enc config.Encrypter,
) (apitype.JournalEntry, error) {
	var state *apitype.ResourceV3
	var requiresByteString bool

	if je.State != nil {
		s, encodedByteString, err := stack.SerializeResource(ctx, je.State, enc, false)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing resource state: %w", err)
		}
		state = &s
		requiresByteString = requiresByteString || encodedByteString
	}

	var operation *apitype.OperationV2
	if je.Operation != nil {
		op, encodedByteString, err := stack.SerializeOperation(ctx, *je.Operation, enc, false)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing operation: %w", err)
		}
		operation = &op
		requiresByteString = requiresByteString || encodedByteString
	}
	var secretsManager *apitype.SecretsProvidersV1
	if je.SecretsManager != nil {
		secretsManager = &apitype.SecretsProvidersV1{
			Type:  je.SecretsManager.Type(),
			State: je.SecretsManager.State(),
		}
	}

	var snapshot *apitype.DeploymentV3
	if je.NewSnapshot != nil {
		var features []string
		var err error
		snapshot, _, features, err = stack.SerializeDeploymentWithMetadata(ctx, je.NewSnapshot, false)
		if err != nil {
			return apitype.JournalEntry{}, fmt.Errorf("serializing new snapshot: %w", err)
		}
		requiresByteString = requiresByteString || slices.Contains(features, "byteString")
	}
	var snippets []apitype.SnippetV1
	if je.Snippets != nil {
		snippets = make([]apitype.SnippetV1, len(je.Snippets))
		for i, snippet := range je.Snippets {
			snippets[i] = stack.SerializeSnippet(snippet)
		}
	}

	var migratedStates []apitype.ResourceV3
	if je.MigratedStates != nil {
		migratedStates = make([]apitype.ResourceV3, len(je.MigratedStates))
		for i, migrated := range je.MigratedStates {
			s, encodedByteString, err := stack.SerializeResource(ctx, migrated, enc, false)
			if err != nil {
				return apitype.JournalEntry{}, fmt.Errorf("serializing migrated resource state: %w", err)
			}
			migratedStates[i] = s
			requiresByteString = requiresByteString || encodedByteString
		}
	}
	var baseStatePatches []apitype.JournalBaseStatePatch
	if je.BaseStatePatches != nil {
		baseStatePatches = make([]apitype.JournalBaseStatePatch, len(je.BaseStatePatches))
		for i, patch := range je.BaseStatePatches {
			state, encodedByteString, err := stack.SerializeResource(ctx, patch.State, enc, false)
			if err != nil {
				return apitype.JournalEntry{}, fmt.Errorf("serializing migrated base resource state: %w", err)
			}
			baseStatePatches[i] = apitype.JournalBaseStatePatch{
				Index: patch.Index,
				State: state,
			}
			requiresByteString = requiresByteString || encodedByteString
		}
	}
	var newStatePatches []apitype.JournalNewStatePatch
	if je.NewStatePatches != nil {
		newStatePatches = make([]apitype.JournalNewStatePatch, len(je.NewStatePatches))
		for i, patch := range je.NewStatePatches {
			state, encodedByteString, err := stack.SerializeResource(ctx, patch.State, enc, false)
			if err != nil {
				return apitype.JournalEntry{}, fmt.Errorf("serializing migrated operation resource state: %w", err)
			}
			newStatePatches[i] = apitype.JournalNewStatePatch{
				OperationID: patch.OperationID,
				State:       state,
			}
			requiresByteString = requiresByteString || encodedByteString
		}
	}
	// State-migration entries carry exact resource patches and removal/insertion indices whose replay semantics only
	// exist in journal format version 2, so stamp them accordingly. Replay applies the prepared patches and does not
	// repeat migration or successor-resolution semantics.
	entryVersion := 1
	if je.Kind == engine.JournalEntryStateMigration {
		entryVersion = 2
	}

	serializedEntry := apitype.JournalEntry{
		Version:               entryVersion,
		Kind:                  apitype.JournalEntryKind(je.Kind),
		SequenceID:            je.SequenceID,
		OperationID:           je.OperationID,
		RemoveOld:             je.RemoveOld,
		RemoveNew:             je.RemoveNew,
		State:                 state,
		Operation:             operation,
		SecretsProvider:       secretsManager,
		PendingReplacementOld: je.PendingReplacementOld,
		PendingReplacementNew: je.PendingReplacementNew,
		DeleteOld:             je.DeleteOld,
		DeleteNew:             je.DeleteNew,
		IsRefresh:             je.IsRefresh,
		NewSnapshot:           snapshot,
		ExtensionRef:          je.ExtensionRef,
		Extension:             je.Extension,
		Snippets:              snippets,
		RequiresByteString:    requiresByteString,
		RemoveOlds:            je.RemoveOlds,
		States:                migratedStates,
		BaseStatePatches:      baseStatePatches,
		NewStatePatches:       newStatePatches,
	}

	return serializedEntry, nil
}

type JournalReplayer struct {
	// toRemove tracks operation IDs of resources that are to be removed.
	toRemove map[int64]struct{}
	// toDeleteInSnapshot tracks the indices of resources in the snapshot that are to be deleted.
	toDeleteInSnapshot map[int64]struct{}
	// toReplaceInSnapshot tracks indices of resources in the snapshot that are to be replaced.
	toReplaceInSnapshot map[int64]*apitype.ResourceV3
	// markAsDeletion tracks indices of resources in the snapshot that are to be marked for deletion.
	markAsDeletion map[int64]struct{}
	// markAsPendingReplacement tracks indices of resources in the snapshot that are to be marked for pending replacement.
	markAsPendingReplacement map[int64]struct{}

	// operationIDToResourceIndex maps operation IDs to resource indices in the new resource list.
	// This is used to replace resources that are being replaced, and remove new resources that are being deleted.
	operationIDToResourceIndex map[int64]int64

	// incompleteOps tracks operations that have begun but not yet completed.
	incompleteOps map[int64]apitype.JournalEntry

	// hasRefresh indicates whether any of the journal entries were part of a refresh operation.
	hasRefresh bool

	// requiresByteString indicates whether any applied journal entry encoded strings containing
	// non-UTF8 bytes. It is tracked here because such strings inside secrets cannot be detected from
	// the serialized resources this replayer holds.
	requiresByteString bool
	// index is the current index in the new resource list.
	index int64

	// base is the base snapshot.
	base *apitype.DeploymentV3

	// newResources is the list of new resources created by the current plan.
	newResources []*apitype.ResourceV3

	// extensions accumulates (ref, blob) pairs produced by extension parameterize
	// entries so the rebuilt DeploymentV3.Extensions map survives cancellation/replay.
	extensions map[apitype.ExtensionRef]apitype.Extension
}

func NewJournalReplayer(base *apitype.DeploymentV3) *JournalReplayer {
	replayer := JournalReplayer{
		toRemove:                   make(map[int64]struct{}),
		toDeleteInSnapshot:         make(map[int64]struct{}),
		toReplaceInSnapshot:        make(map[int64]*apitype.ResourceV3),
		markAsDeletion:             make(map[int64]struct{}),
		markAsPendingReplacement:   make(map[int64]struct{}),
		operationIDToResourceIndex: make(map[int64]int64),
		incompleteOps:              make(map[int64]apitype.JournalEntry),
		newResources:               make([]*apitype.ResourceV3, 0),
		extensions:                 make(map[apitype.ExtensionRef]apitype.Extension),
		base:                       base,
	}
	return &replayer
}

func (r *JournalReplayer) Add(entry apitype.JournalEntry) error {
	if entry.Version <= 0 || int64(entry.Version) > apitype.LatestJournalVersion {
		return fmt.Errorf("unsupported journal entry version %d", entry.Version)
	}
	if entry.Kind == apitype.JournalEntryKindStateMigration && entry.Version != 2 {
		return fmt.Errorf("state migration journal entry must use version 2, got %d", entry.Version)
	}
	// A state-migration entry is validated transactionally below. Delay this bookkeeping bit as well so a rejected
	// migration cannot affect a later deployment generated from the replayer.
	if entry.RequiresByteString && entry.Kind != apitype.JournalEntryKindStateMigration {
		r.requiresByteString = true
	}
	switch entry.Kind {
	case apitype.JournalEntryKindBegin:
		r.incompleteOps[entry.OperationID] = entry
	case apitype.JournalEntryKindSuccess:
		delete(r.incompleteOps, entry.OperationID)
		// If this is a success, we need to add the resource to the list of resources.
		if entry.State != nil {
			r.newResources = append(r.newResources, entry.State)
			r.operationIDToResourceIndex[entry.OperationID] = r.index
			r.index++
		}
		if entry.RemoveOld != nil {
			r.toDeleteInSnapshot[*entry.RemoveOld] = struct{}{}
		}
		if entry.RemoveNew != nil {
			if _, _, err := r.newResourceForOperation(*entry.RemoveNew, "remove-new"); err != nil {
				return err
			}
			r.toRemove[*entry.RemoveNew] = struct{}{}
		}
		if entry.DeleteOld != nil {
			r.markAsDeletion[*entry.DeleteOld] = struct{}{}
		}
		if entry.DeleteNew != nil {
			_, state, err := r.newResourceForOperation(*entry.DeleteNew, "delete-new")
			if err != nil {
				return err
			}
			state.Delete = true
		}
		if entry.PendingReplacementOld != nil {
			r.markAsPendingReplacement[*entry.PendingReplacementOld] = struct{}{}
		}
		if entry.PendingReplacementNew != nil {
			_, state, err := r.newResourceForOperation(*entry.PendingReplacementNew, "pending-replacement-new")
			if err != nil {
				return err
			}
			state.PendingReplacement = true
		}

		if entry.IsRefresh {
			r.hasRefresh = true
		}
	case apitype.JournalEntryKindRefreshSuccess:
		delete(r.incompleteOps, entry.OperationID)
		r.hasRefresh = true
		if entry.RemoveOld != nil {
			if entry.State == nil {
				r.toDeleteInSnapshot[*entry.RemoveOld] = struct{}{}
			} else {
				r.toReplaceInSnapshot[*entry.RemoveOld] = entry.State
			}
		}
		if entry.RemoveNew != nil {
			index, _, err := r.newResourceForOperation(*entry.RemoveNew, "refresh remove-new")
			if err != nil {
				return err
			}
			if entry.State == nil {
				r.toRemove[*entry.RemoveNew] = struct{}{}
			} else {
				r.newResources[index] = entry.State
			}
		}
	case apitype.JournalEntryKindFailure:
		delete(r.incompleteOps, entry.OperationID)
	case apitype.JournalEntryKindOutputs:
		if entry.State != nil && entry.RemoveOld != nil {
			r.toReplaceInSnapshot[*entry.RemoveOld] = entry.State
		}
		if entry.RemoveNew != nil {
			index, _, err := r.newResourceForOperation(*entry.RemoveNew, "outputs remove-new")
			if err != nil {
				return err
			}
			if entry.State != nil {
				r.newResources[index] = entry.State
			}
		}
	case apitype.JournalEntryKindWrite:
		// Overwrite the base snapshot. Note that we expect this to happen before any other
		// journal entries that modify the snapshot.
		r.base = entry.NewSnapshot
	case apitype.JournalEntryKindSecretsManager:
		// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
		// the Snapshot (checkpoint file) in the HTTP backend. We will reuse the snapshot's secrets manager when possible
		// to ensure that secrets are not re-encrypted on each update.
		secretsProvider := entry.SecretsProvider
		if r.base.SecretsProviders != nil &&
			(secretsProvider.Type == r.base.SecretsProviders.Type &&
				bytes.Equal(secretsProvider.State, r.base.SecretsProviders.State)) {
			return nil
		}

		r.base.SecretsProviders = entry.SecretsProvider
	case apitype.JournalEntryKindSnippets:
		r.base.Snippets = entry.Snippets
	case apitype.JournalEntryKindRebuiltBaseState:
		// We need to build the snapshot from the current state here and discard the
		// current journal entries. This happens after a refresh operation.
		deployment, err := r.GenerateDeployment()
		if err != nil {
			return err
		}
		r.base = deployment.Deployment
		r.toRemove = make(map[int64]struct{})
		r.toDeleteInSnapshot = make(map[int64]struct{})
		r.toReplaceInSnapshot = make(map[int64]*apitype.ResourceV3)
		r.markAsDeletion = make(map[int64]struct{})
		r.markAsPendingReplacement = make(map[int64]struct{})
		r.operationIDToResourceIndex = make(map[int64]int64)
		r.incompleteOps = make(map[int64]apitype.JournalEntry)
		r.newResources = make([]*apitype.ResourceV3, 0)
		r.extensions = make(map[apitype.ExtensionRef]apitype.Extension)
	case apitype.JournalEntryKindExtensionParameterize:
		r.extensions[*entry.ExtensionRef] = *entry.Extension
	case apitype.JournalEntryKindStateMigration:
		if err := r.applyStateMigration(entry); err != nil {
			return err
		}
		if entry.RequiresByteString {
			r.requiresByteString = true
		}
	default:
		return fmt.Errorf("unsupported journal entry kind %d", entry.Kind)
	}
	return nil
}

func (r *JournalReplayer) newResourceForOperation(
	operationID int64, field string,
) (int64, *apitype.ResourceV3, error) {
	index, ok := r.operationIDToResourceIndex[operationID]
	if !ok {
		return 0, nil, fmt.Errorf("journal %s references unknown operation %d", field, operationID)
	}
	if index < 0 || index >= int64(len(r.newResources)) {
		return 0, nil, fmt.Errorf(
			"journal %s operation %d resolves to invalid resource index %d", field, operationID, index)
	}
	state := r.newResources[index]
	if state == nil {
		return 0, nil, fmt.Errorf("journal %s operation %d resolves to a nil resource state", field, operationID)
	}
	return index, state, nil
}

// currentBaseResource materializes the state of a base resource after applying journal entries that update it
// without replacing the base snapshot itself. This is the state observed by the live engine at the same point.
func (r *JournalReplayer) currentBaseResource(index int64) apitype.ResourceV3 {
	state := r.base.Resources[index]
	if replacement, ok := r.toReplaceInSnapshot[index]; ok {
		state = *replacement
	}
	if _, ok := r.markAsDeletion[index]; ok {
		state.Delete = true
	}
	if _, ok := r.markAsPendingReplacement[index]; ok {
		state.PendingReplacement = true
	}
	return state
}

// applyStateMigration applies a prepared state-migration transaction. Every reference-bearing state that changed was
// rewritten while it was still typed and decrypted, then serialized into this entry. Replay only installs those exact
// states; it does not reinterpret successor mappings.
func validateStateMigrationPatch(original, patched apitype.ResourceV3) error {
	if original.URN != patched.URN {
		return fmt.Errorf("changes resource URN from %s to %s", original.URN, patched.URN)
	}
	if original.Type != patched.Type || original.Custom != patched.Custom || original.ID != patched.ID {
		return errors.New("changes resource type, custom kind, or physical ID")
	}

	timeEqual := func(left, right *time.Time) bool {
		if left == nil || right == nil {
			return left == nil && right == nil
		}
		return left.Equal(*right)
	}
	timeoutsEqual := func(left, right *resource.CustomTimeouts) bool {
		var leftValue, rightValue resource.CustomTimeouts
		if left != nil {
			leftValue = *left
		}
		if right != nil {
			rightValue = *right
		}
		return leftValue == rightValue
	}
	pathsEqual := func(left, right []resource.PropertyPath) bool {
		return slices.EqualFunc(left, right, func(left, right resource.PropertyPath) bool {
			return left.String() == right.String()
		})
	}
	hooksEqual := func(
		left, right map[resource.HookType][]string,
	) bool {
		return maps.EqualFunc(left, right, slices.Equal)
	}

	// Reference rewriting may alter Inputs, Outputs, Parent, Dependencies, Provider, PropertyDependencies,
	// DeletedWith, ReplaceWith, ReplacementTrigger, and ViewOf. Compare every other field semantically. This avoids
	// rejecting benign checkpoint canonicalization (for example {} custom timeouts becoming nil) while preventing a
	// prepared patch from changing resource identity, lifecycle, or registration metadata.
	if original.Delete != patched.Delete ||
		original.Protect != patched.Protect ||
		original.Taint != patched.Taint ||
		original.External != patched.External ||
		!slices.Equal(original.InitErrors, patched.InitErrors) ||
		original.PendingReplacement != patched.PendingReplacement ||
		!slices.Equal(original.AdditionalSecretOutputs, patched.AdditionalSecretOutputs) ||
		!slices.Equal(original.Aliases, patched.Aliases) ||
		!timeoutsEqual(original.CustomTimeouts, patched.CustomTimeouts) ||
		original.ImportID != patched.ImportID ||
		original.RetainOnDelete != patched.RetainOnDelete ||
		!timeEqual(original.Created, patched.Created) ||
		!timeEqual(original.Modified, patched.Modified) ||
		original.SourcePosition != patched.SourcePosition ||
		!slices.Equal(original.StackTrace, patched.StackTrace) ||
		!slices.Equal(original.IgnoreChanges, patched.IgnoreChanges) ||
		!pathsEqual(original.HideDiff, patched.HideDiff) ||
		!slices.Equal(original.ReplaceOnChanges, patched.ReplaceOnChanges) ||
		original.RefreshBeforeUpdate != patched.RefreshBeforeUpdate ||
		!hooksEqual(original.ResourceHooks, patched.ResourceHooks) ||
		original.ExtensionRef != patched.ExtensionRef ||
		original.SnippetID != patched.SnippetID {
		return errors.New("changes non-reference resource state")
	}
	return nil
}

func (r *JournalReplayer) validateProspectiveStateMigration(resources []apitype.ResourceV3) error {
	if r.hasRefresh {
		// GenerateDeployment applies this same repair when a refresh has removed resources. Validate the state that
		// would actually be persisted rather than rejecting a pre-existing dangling edge that refresh replay prunes.
		rebuildDependencies(resources)
	}

	extensions := make(map[apitype.ExtensionRef]apitype.Extension, len(r.base.Extensions)+len(r.extensions))
	maps.Copy(extensions, r.extensions)
	maps.Copy(extensions, r.base.Extensions)

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
			if _, ok := extensions[state.ExtensionRef]; !ok {
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

	prospective := *r.base
	prospective.Resources = resources
	prospective.PendingOperations = nil
	prospective.Extensions = extensions
	if err := snapshot.VerifyIntegrity(&prospective); err != nil {
		return err
	}
	return nil
}

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
		// embedded resource state outside the prepared patch transaction, so reject it.
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

	type resolvedNewPatch struct {
		index int64
		state apitype.ResourceV3
	}
	newPatches := make([]resolvedNewPatch, 0, len(entry.NewStatePatches))
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
		newPatches = append(newPatches, resolvedNewPatch{index: index, state: patch.State})
	}

	insertedURNs := make(map[resource.URN]apitype.ResourceV3, len(entry.States))
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
		insertedURNs[state.URN] = state
	}

	last := entry.RemoveOlds[len(entry.RemoveOlds)-1]
	newIndices := make(map[int64]int64, len(baseResources))
	resources := make([]apitype.ResourceV3, 0, len(baseResources)-len(entry.RemoveOlds)+len(entry.States))
	finalBaseResources := make([]apitype.ResourceV3, 0, cap(resources))
	for i, res := range baseResources {
		if _, ok := removed[int64(i)]; ok {
			if int64(i) == last {
				resources = append(resources, entry.States...)
				finalBaseResources = append(finalBaseResources, entry.States...)
			}
			continue
		}
		newIndices[int64(i)] = int64(len(resources))
		resources = append(resources, res)
		if _, deleted := r.toDeleteInSnapshot[int64(i)]; deleted {
			continue
		}
		if _, patched := patchedBase[int64(i)]; patched {
			finalBaseResources = append(finalBaseResources, res)
		} else {
			finalBaseResources = append(finalBaseResources, r.currentBaseResource(int64(i)))
		}
	}

	patchedNewByIndex := make(map[int64]apitype.ResourceV3, len(newPatches))
	for _, patch := range newPatches {
		patchedNewByIndex[patch.index] = patch.state
	}
	removedNewIndices := make(map[int64]struct{}, len(r.toRemove))
	for operationID := range r.toRemove {
		index, _, err := r.newResourceForOperation(operationID, "state migration removed new state")
		if err != nil {
			return err
		}
		removedNewIndices[index] = struct{}{}
	}

	finalResources := make([]apitype.ResourceV3, 0, len(r.newResources)+len(finalBaseResources))
	for i, current := range r.newResources {
		if _, removed := removedNewIndices[int64(i)]; removed {
			continue
		}
		if current == nil {
			return fmt.Errorf("state migration new resource at index %d is nil", i)
		}
		state := *current
		if patched, ok := patchedNewByIndex[int64(i)]; ok {
			state = patched
		}
		finalResources = append(finalResources, state)
	}
	finalResources = append(finalResources, finalBaseResources...)
	if err := r.validateProspectiveStateMigration(finalResources); err != nil {
		return fmt.Errorf("state migration produces invalid snapshot: %w", err)
	}

	for _, patch := range newPatches {
		state := patch.state
		r.newResources[patch.index] = &state
	}

	// The base snapshot object may be shared with the journaler, which replays all entries from scratch on
	// every save: rebase onto a copy rather than mutating it in place, mirroring what Write entries do.
	newBase := *r.base
	newBase.Resources = resources
	r.base = &newBase

	remapSet := func(set map[int64]struct{}) map[int64]struct{} {
		remapped := make(map[int64]struct{}, len(set))
		for index := range set {
			if newIndex, ok := newIndices[index]; ok {
				remapped[newIndex] = struct{}{}
			}
		}
		return remapped
	}
	r.toDeleteInSnapshot = remapSet(r.toDeleteInSnapshot)
	r.markAsDeletion = remapSet(r.markAsDeletion)
	r.markAsPendingReplacement = remapSet(r.markAsPendingReplacement)

	toReplace := make(map[int64]*apitype.ResourceV3, len(r.toReplaceInSnapshot))
	for index, state := range r.toReplaceInSnapshot {
		// The prepared base patch is the exact state after applying both the earlier
		// operation and this migration. Do not let the earlier refresh/outputs
		// overlay replace it when the deployment is assembled.
		if _, patched := patchedBase[index]; patched {
			continue
		}
		if newIndex, ok := newIndices[index]; ok {
			toReplace[newIndex] = state
		}
	}
	r.toReplaceInSnapshot = toReplace
	return nil
}

// rebuildDependencies rebuilds the dependencies of the resources in the snapshot based on the
// resources that are present in the snapshot. This is necessary if a refresh happens, because
// refreshes may delete resources, even if other resources still depend on them.
//
// This function is similar to 'rebuildBaseState' in the engine, but doesn't take care of
// rebuilding the resource list, since that's already done correctly by the journal.
//
// Note that this function assumes that resources are in reverse-dependency order.
func rebuildDependencies(resources []apitype.ResourceV3) {
	referenceable := make(map[resource.URN]bool)
	for i := range resources {
		newDeps := []resource.URN{}
		newPropDeps := make(map[resource.PropertyKey][]resource.URN)
		for _, dep := range resources[i].Dependencies {
			if referenceable[dep] {
				newDeps = append(newDeps, dep)
			}
		}
		for k := range resources[i].PropertyDependencies {
			for _, dep := range resources[i].PropertyDependencies[k] {
				if referenceable[dep] {
					newPropDeps[k] = append(newPropDeps[k], dep)
				}
			}
		}
		newReplaceWith := []resource.URN{}
		for _, r := range resources[i].ReplaceWith {
			if referenceable[r] {
				newReplaceWith = append(newReplaceWith, r)
			}
		}
		if len(resources[i].ReplaceWith) > 0 {
			resources[i].ReplaceWith = newReplaceWith
		}
		if !referenceable[resources[i].DeletedWith] {
			resources[i].DeletedWith = ""
		}
		if len(resources[i].Dependencies) > 0 {
			resources[i].Dependencies = newDeps
		}
		if len(resources[i].PropertyDependencies) > 0 {
			resources[i].PropertyDependencies = newPropDeps
		}
		referenceable[resources[i].URN] = true
	}

	undangleParentResources(referenceable, resources)
}

func undangleParentResources(undeleted map[resource.URN]bool, resources []apitype.ResourceV3) {
	availableParents := map[resource.URN]resource.URN{}
	for i, r := range resources {
		if _, ok := undeleted[r.Parent]; !ok {
			// Since existing must obey a topological sort, we have already addressed
			// p.Parent. Since we know that it doesn't dangle, and that r.Parent no longer
			// exists, we set r.Parent as r.Parent.Parent.
			resources[i].Parent = availableParents[r.Parent]
		}
		availableParents[r.URN] = r.Parent
	}
}

func (r *JournalReplayer) GenerateDeployment() (apitype.TypedDeployment, error) {
	features := make(map[string]bool)
	removeIndices := make(map[int64]struct{})
	for operationID := range r.toRemove {
		index, _, err := r.newResourceForOperation(operationID, "remove-new")
		if err != nil {
			return apitype.TypedDeployment{}, err
		}
		removeIndices[index] = struct{}{}
	}

	resources := make([]apitype.ResourceV3, 0)
	for i, res := range r.newResources {
		if _, ok := removeIndices[int64(i)]; !ok {
			resources = append(resources, *res)
			stack.ApplyFeatures(*res, r.requiresByteString, features)
		}
	}

	// Append any resources from the base plan that were not produced by the current plan.
	if r.base != nil {
		for i := range r.base.Resources {
			if _, ok := r.toDeleteInSnapshot[int64(i)]; !ok {
				state := r.currentBaseResource(int64(i))
				resources = append(resources, state)
				stack.ApplyFeatures(state, r.requiresByteString, features)
			}
		}
	}

	// Record any pending operations, if there are any outstanding that have not completed yet.
	var operations []apitype.OperationV2
	for _, op := range r.incompleteOps {
		if op.Operation != nil {
			operations = append(operations, *op.Operation)
			stack.ApplyFeatures(op.Operation.Resource, r.requiresByteString, features)
		}
	}

	// Track pending create operations from the base snapshot
	// and propagate them to the new snapshot: we don't want to clear pending CREATE operations
	// because these must require user intervention to be cleared or resolved.
	if base := r.base; base != nil {
		for _, pendingOperation := range base.PendingOperations {
			if pendingOperation.Type == apitype.OperationTypeCreating {
				operations = append(operations, pendingOperation)
				stack.ApplyFeatures(pendingOperation.Resource, r.requiresByteString, features)
			}
		}
	}

	if r.hasRefresh {
		// Refreshes can delete resources without exact typed patches for their dependents, so prune dangling
		// dependencies. State migrations carry exact prepared patches and deliberately require mechanical replay.
		rebuildDependencies(resources)
	}

	manifest := deploy.Manifest{
		Time:    time.Now(),
		Version: version.Version,
		// Plugins: sm.plugins, - Explicitly dropped, since we don't use the plugin list in the manifest anymore.
	}
	manifest.Magic = manifest.NewMagic()

	deployment := &apitype.DeploymentV3{}
	deployment.SecretsProviders = r.base.SecretsProviders
	deployment.Resources = resources
	deployment.PendingOperations = operations
	deployment.Metadata = r.base.Metadata
	deployment.Snippets = r.base.Snippets
	deployment.Manifest = manifest.Serialize()
	// Carry extensions forward from the base, plus any this plan produced.
	extensions := maps.Clone(r.extensions)
	maps.Copy(extensions, r.base.Extensions)
	if len(extensions) > 0 {
		deployment.Extensions = extensions
	}

	if len(deployment.Snippets) > 0 {
		features["snippets-prototype"] = true
	}

	version := apitype.DeploymentSchemaVersionCurrent
	if len(features) > 0 {
		version = apitype.DeploymentSchemaVersionLatest
	}

	deployment, err := deployment.NormalizeURNReferences()
	if err != nil {
		return apitype.TypedDeployment{}, fmt.Errorf("failed to normalize URN references: %w", err)
	}

	return apitype.TypedDeployment{
		Deployment: deployment,
		Version:    version,
		Features:   slices.Sorted(maps.Keys(features)),
	}, nil
}

// snap produces a new Snapshot given the base snapshot and a list of resources that the current
// plan has created.
func (sj *SnapshotJournaler) snap() (apitype.TypedDeployment, error) {
	// At this point we have two resource DAGs. One of these is the base DAG for this plan; the other is the current DAG
	// for this plan. Any resource r may be present in both DAGs. In order to produce a snapshot, we need to merge these
	// DAGs such that all resource dependencies are correctly preserved. Conceptually, the merge proceeds as follows:
	//
	// - Begin with an empty merged DAG.
	// - For each resource r in the current DAG, insert r and its outgoing edges into the merged DAG.
	// - For each resource r in the base DAG:
	//     - If r is in the merged DAG, we are done: if the resource is in the merged DAG, it must have been in the
	//       current DAG, which accurately captures its current dependencies.
	//     - If r is not in the merged DAG, insert it and its outgoing edges into the merged DAG.
	//
	// Physically, however, each DAG is represented as list of resources without explicit dependency edges. In place of
	// edges, it is assumed that the list represents a valid topological sort of its source DAG. Thus, any resource r at
	// index i in a list L must be assumed to be dependent on all resources in L with index j s.t. j < i. Due to this
	// representation, we implement the algorithm above as follows to produce a merged list that represents a valid
	// topological sort of the merged DAG:
	//
	// - Begin with an empty merged list.
	// - For each resource r in the current list, append r to the merged list. r must be in a correct location in the
	//   merged list, as its position relative to its assumed dependencies has not changed.
	// - For each resource r in the base list:
	//     - If r is in the merged list, we are done by the logic given in the original algorithm.
	//     - If r is not in the merged list, append r to the merged list. r must be in a correct location in the merged
	//       list:
	//         - If any of r's dependencies were in the current list, they must already be in the merged list and their
	//           relative order w.r.t. r has not changed.
	//         - If any of r's dependencies were not in the current list, they must already be in the merged list, as
	//           they would have been appended to the list before r.
	snap := sj.snapshot

	replayer := NewJournalReplayer(snap)

	// Record any pending operations, if there are any outstanding that have not completed yet.
	for i, entry := range sj.journalEntries {
		if err := replayer.Add(entry); err != nil {
			return apitype.TypedDeployment{}, err
		}
		// If the entry we're adding is a RebuiltBaseState entry, it's only valid if
		// there are no new resources, or the journal entry is the last entry. Validate
		// that, and add a validation error if this is not the case. For a detailed
		// example see https://github.com/pulumi/pulumi/pull/20596#discussion_r2392049682
		if entry.Kind == apitype.JournalEntryKindRebuiltBaseState &&
			len(replayer.newResources) > 0 && i < len(sj.journalEntries)-1 {
			sj.errors = append(sj.errors,
				fmt.Errorf("invalid RebuiltBaseState journal entry. Have %d new resources, but entry is not last in journal",
					len(replayer.newResources)))
		}
	}

	return replayer.GenerateDeployment()
}

// saveSnapshot persists the current snapshot. If integrity checking is enabled,
// the snapshot's integrity is also verified. If the snapshot is invalid,
// metadata about this write operation is added to the snapshot before it is
// written, in order to aid debugging should future operations fail with an
// error.
func (sj *SnapshotJournaler) saveSnapshot() error {
	deployment, err := sj.snap()
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// In order to persist metadata about snapshot integrity issues, we check the
	// snapshot's validity *before* we write it. However, should an error occur,
	// we will only raise this *after* the write has completed. In the event that
	// integrity checking is disabled, we still actually perform the check (and
	// write metadata appropriately), but we will not raise the error following a
	// successful write.
	//
	// If the actual write fails for any reason, this error will supersede any
	// integrity error. This matches behaviour prior to when integrity metadata
	// writing was introduced.
	//
	// Metadata will be cleared out by a successful operation (even if integrity
	// checking is being enforced).
	integrityError := snapshot.VerifyIntegrity(deployment.Deployment)
	if integrityError == nil {
		deployment.Deployment.Metadata.IntegrityErrorMetadata = nil
	} else {
		deployment.Deployment.Metadata.IntegrityErrorMetadata = &apitype.SnapshotIntegrityErrorMetadataV1{
			Version: strconv.FormatInt(int64(deployment.Version), 10),
			Command: strings.Join(os.Args, " "),
			Error:   integrityError.Error(),
			EnvVars: utilenv.ConfiguredVariables(),
		}
	}
	persister := sj.persister
	if err := persister.Save(deployment); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}
	if !DisableIntegrityChecking && integrityError != nil {
		return fmt.Errorf("failed to verify snapshot: %w", integrityError)
	}
	return nil
}

// defaultServiceLoop saves a Snapshot whenever a mutation occurs
func (sj *SnapshotJournaler) defaultServiceLoop(
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	// True if we have elided writes since the last actual write.
	hasElidedWrites := true

	// Service each mutation request in turn.
serviceLoop:
	for {
		select {
		case request := <-journalEvents:
			sj.journalEntries = append(sj.journalEntries, request.journalEntry)
			if request.journalEntry.SequenceID == 0 {
				sj.errors = append(sj.errors, fmt.Errorf("journal entry has no sequence ID %v", request.journalEntry))
			}
			if request.elideWrite {
				hasElidedWrites = true
				if request.result != nil {
					request.result <- nil
				}
				continue
			}
			hasElidedWrites = false
			request.result <- sj.saveSnapshot()
		case <-sj.cancel:
			break serviceLoop
		}
	}

	// If we still have elided writes once the channel has closed, flush the snapshot.
	var err error
	if hasElidedWrites {
		logging.V(9).Infof("SnapshotManager: flushing elided writes...")
		err = sj.saveSnapshot()
	}
	done <- err
}

// unsafeServiceLoop doesn't save Snapshots when mutations occur and instead saves Snapshots when
// SnapshotManager.Close() is invoked. It trades reliability for speed as every mutation does not
// cause a Snapshot to be serialized to the user's state backend.
func (sj *SnapshotJournaler) unsafeServiceLoop(
	journalEvents chan writeJournalEntryRequest, done chan error,
) {
	for {
		select {
		case request := <-journalEvents:
			sj.journalEntries = append(sj.journalEntries, request.journalEntry)
			request.result <- nil
		case <-sj.cancel:
			done <- sj.saveSnapshot()
			return
		}
	}
}

type SnapshotJournaler struct {
	ctx             context.Context
	persister       SnapshotPersister
	snapshot        *apitype.DeploymentV3
	journalEvents   chan writeJournalEntryRequest
	journalEntries  []apitype.JournalEntry
	cancel          chan bool
	done            chan error
	secretsManager  secrets.Manager
	secretsProvider secrets.Provider
	errors          []error
}

// NewSnapshotJournaler creates a new Journal that uses a SnapshotPersister to persist the
// snapshot created from the journal entries.
//
// The snapshot code works on journal entries. Each resource step produces new journal entries
// for beginning and finishing an operation. These journal entries can then be replayed
// in conjunction with the immutable base snapshot, to rebuild the new snapshot.
//
// Currently the backend only supports saving full snapshots, in which case only one journal
// entry is allowed to be processed at a time. In the future journal entries will be processed
// asynchronously in the cloud backend, allowing for better throughput for independent operations.
//
// Serialization is performed by pushing the journal entries onto a channel, where another
// goroutine is polling the channel and creating new snapshots using the entries as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
//
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` field. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
func NewSnapshotJournaler(
	ctx context.Context,
	persister SnapshotPersister,
	secretsManager secrets.Manager,
	secretsProvider secrets.Provider,
	baseSnap *deploy.Snapshot,
) (*SnapshotJournaler, error) {
	snapCopy := &deploy.Snapshot{}
	if baseSnap != nil {
		snapCopy = &deploy.Snapshot{
			Manifest:          baseSnap.Manifest,
			SecretsManager:    baseSnap.SecretsManager,
			Resources:         make([]*pkgresource.State, 0),
			PendingOperations: make([]pkgresource.Operation, 0),
			Metadata:          baseSnap.Metadata,
			Snippets:          baseSnap.Snippets,
		}
		// Copy the resources from the base snapshot to the new snapshot.
		for _, res := range baseSnap.Resources {
			snapCopy.Resources = append(snapCopy.Resources, res.Copy())
		}
		// Copy the pending operations from the base snapshot to the new snapshot.
		for _, op := range baseSnap.PendingOperations {
			snapCopy.PendingOperations = append(snapCopy.PendingOperations, op.Copy())
		}

		if snapCopy.SecretsManager == nil {
			snapCopy.SecretsManager = secretsManager
		}
	}

	journalEvents := make(chan writeJournalEntryRequest)
	done, cancel := make(chan error), make(chan bool)

	var deployment *apitype.DeploymentV3
	if baseSnap != nil {
		var err error
		deployment, _, _, err = stack.SerializeDeploymentWithMetadata(
			ctx, snapCopy, false)
		if err != nil {
			return nil, err
		}
	} else {
		deployment = &apitype.DeploymentV3{}
	}

	journaler := SnapshotJournaler{
		ctx:             ctx,
		persister:       persister,
		snapshot:        deployment,
		journalEvents:   journalEvents,
		journalEntries:  make([]apitype.JournalEntry, 0),
		secretsManager:  secretsManager,
		secretsProvider: secretsProvider,
		cancel:          cancel,
		done:            done,
	}

	serviceLoop := journaler.defaultServiceLoop

	if env.SkipCheckpoints.Value() {
		serviceLoop = journaler.unsafeServiceLoop
	}

	go serviceLoop(journalEvents, done)

	return &journaler, nil
}

func (sj *SnapshotJournaler) Entries() []apitype.JournalEntry {
	return sj.journalEntries
}

type writeJournalEntryRequest struct {
	journalEntry apitype.JournalEntry
	elideWrite   bool
	result       chan error
}

func (sj *SnapshotJournaler) journalMutation(entry engine.JournalEntry) error {
	serializedEntry, err := stack.BatchEncrypt(
		sj.ctx,
		sj.secretsManager,
		func(ctx context.Context, enc config.Encrypter) (apitype.JournalEntry, error) {
			return SerializeJournalEntry(ctx, entry, enc)
		})
	if err != nil {
		return fmt.Errorf("failed to serialize journal entry: %w", err)
	}

	result := make(chan error)
	select {
	case sj.journalEvents <- writeJournalEntryRequest{
		journalEntry: serializedEntry,
		elideWrite:   entry.ElideWrite,
		result:       result,
	}:
		// We don't need to check for cancellation here, as the service loop guarantees
		// that it will return a result for every journal entry that it processes.
		return <-result
	case <-sj.cancel:
		return errors.New("snapshot manager closed")
	}
}

func (sj *SnapshotJournaler) AddJournalEntry(entry engine.JournalEntry) error {
	return sj.journalMutation(entry)
}

func (sj SnapshotJournaler) Errors() []error {
	return sj.errors
}

func (sj SnapshotJournaler) Close() error {
	sj.cancel <- true
	return <-sj.done
}

type journaler struct {
	ctx            context.Context
	persister      JournalPersister
	snapshot       *apitype.DeploymentV3
	secretsManager secrets.Manager
}

// A JournalPersister implements persistence of journal entries in some store.
type JournalPersister interface {
	// Append appends a new entry to the journal.
	Append(ctx context.Context, entry apitype.JournalEntry) error
}

// NewSnapshotJournaler creates a new Journal that uses a SnapshotPersister to persist the
// snapshot created from the journal entries.
//
// The snapshot code works on journal entries. Each resource step produces new journal entries
// for beginning and finishing an operation. These journal entries can then be replayed
// in conjunction with the immutable base snapshot, to rebuild the new snapshot.
//
// Currently the backend only supports saving full snapshots, in which case only one journal
// entry is allowed to be processed at a time. In the future journal entries will be processed
// asynchronously in the cloud backend, allowing for better throughput for independent operations.
//
// Serialization is performed by pushing the journal entries onto a channel, where another
// goroutine is polling the channel and creating new snapshots using the entries as they come.
// This function optionally verifies the integrity of the snapshot before and after mutation.
//
// Each journal entry may indicate that its corresponding checkpoint write may be safely elided by
// setting the `ElideWrite` field. As of this writing, we only elide writes after same steps with no
// meaningful changes (see sameSnapshotMutation.mustWrite for details). Any elided writes
// are flushed by the next non-elided write or the next call to Close.
func NewJournaler(
	ctx context.Context,
	persister JournalPersister,
	secretsManager secrets.Manager,
	baseSnap *deploy.Snapshot,
) (engine.Journal, error) {
	snapCopy := &deploy.Snapshot{}
	if baseSnap != nil {
		snapCopy = &deploy.Snapshot{
			Manifest:          baseSnap.Manifest,
			SecretsManager:    baseSnap.SecretsManager,
			Resources:         make([]*pkgresource.State, 0),
			PendingOperations: make([]pkgresource.Operation, 0),
			Metadata:          baseSnap.Metadata,
		}
		// Copy the resources from the base snapshot to the new snapshot.
		for _, res := range baseSnap.Resources {
			snapCopy.Resources = append(snapCopy.Resources, res.Copy())
		}
		// Copy the pending operations from the base snapshot to the new snapshot.
		for _, op := range baseSnap.PendingOperations {
			snapCopy.PendingOperations = append(snapCopy.PendingOperations, op.Copy())
		}

		if snapCopy.SecretsManager == nil {
			snapCopy.SecretsManager = secretsManager
		}
	}

	var deployment *apitype.DeploymentV3
	if baseSnap != nil {
		var err error
		deployment, _, _, err = stack.SerializeDeploymentWithMetadata(ctx, snapCopy, false)
		if err != nil {
			return nil, err
		}
	} else {
		deployment = &apitype.DeploymentV3{}
	}

	return &journaler{
		ctx:            ctx,
		persister:      persister,
		snapshot:       deployment,
		secretsManager: secretsManager,
	}, nil
}

func (sj *journaler) AddJournalEntry(entry engine.JournalEntry) error {
	serializedEntry, err := stack.BatchEncrypt(
		sj.ctx,
		sj.secretsManager,
		func(ctx context.Context, enc config.Encrypter) (apitype.JournalEntry, error) {
			return SerializeJournalEntry(ctx, entry, enc)
		})
	if err != nil {
		return fmt.Errorf("failed to serialize journal entry: %w", err)
	}
	return sj.persister.Append(sj.ctx, serializedEntry)
}

func (sj journaler) Close() error {
	return nil
}
