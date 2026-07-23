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

package deploy

import (
	"fmt"
	"reflect"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// StateMigrationPlan is the validated, fully prepared transaction produced by a state migration.
//
// BaseResources contains the exact prepared resource values for the post-migration base. RetainedResources maps each
// retained resource in the old base to its prepared value. The engine commits equivalent values while preserving
// pointer identities.
type StateMigrationPlan struct {
	RootURN           resource.URN
	RemovedResources  []*pkgresource.State
	MigratedResources []*pkgresource.State
	SuccessorURNs     map[resource.URN]resource.URN
	BaseResources     []*pkgresource.State
	RetainedResources map[*pkgresource.State]*pkgresource.State
}

// validateStateMigrationAccounting checks that a single migration invocation accounted for every resource it was
// handed: each input resource must either remain in the returned state or name a returned successor. This prevents
// omission from becoming an implicit "forget" operation.
func validateStateMigrationAccounting(
	urn resource.URN, index, total int, oldSet, newSet []apitype.ResourceV3,
	successors map[resource.URN]resource.URN,
) error {
	errorf := func(format string, args ...any) error {
		prefix := fmt.Sprintf("state migration %d of %d for %s: ", index+1, total, urn)
		return fmt.Errorf(prefix+format, args...)
	}

	oldStates := make(map[resource.URN]apitype.ResourceV3, len(oldSet))
	for _, res := range oldSet {
		if !res.URN.IsValid() {
			return errorf("received a resource with invalid URN %q", res.URN)
		}
		if _, exists := oldStates[res.URN]; exists {
			return errorf("received duplicate resource %s", res.URN)
		}
		oldStates[res.URN] = res
	}
	newStates := make(map[resource.URN]apitype.ResourceV3, len(newSet))
	for _, res := range newSet {
		if !res.URN.IsValid() {
			return errorf("returned a resource with invalid URN %q", res.URN)
		}
		if _, exists := newStates[res.URN]; exists {
			return errorf("returned duplicate resource %s", res.URN)
		}
		if res.Type != res.URN.Type() {
			return errorf("returned resource %s with type %s, which does not match its URN type %s",
				res.URN, res.Type, res.URN.Type())
		}
		newStates[res.URN] = res
	}

	for source, target := range successors {
		if source == "" || target == "" {
			return errorf("returned an empty successor mapping %q -> %q", source, target)
		}
		if _, ok := oldStates[source]; !ok {
			return errorf("returned successor for resource %s, which is not part of the migrated state", source)
		}
		if _, ok := newStates[source]; ok {
			return errorf("resource %s is both returned and assigned successor %s", source, target)
		}
		if _, ok := newStates[target]; !ok {
			return errorf("successor %s for resource %s is not present in the returned state", target, source)
		}
	}

	for oldURN, old := range oldStates {
		newState, retained := newStates[oldURN]
		if !retained {
			if _, ok := successors[oldURN]; !ok {
				return errorf("did not account for resource %s: it must either be returned in the new state or "+
					"have a successor", oldURN)
			}
			continue
		}

		// Keeping a URN while changing the identity behind it is an implicit unmanage/import pair. Require callers
		// to express identity changes with a distinct successor URN instead.
		if old.Custom != newState.Custom {
			return errorf("resource %s changed between custom and component state without changing URN", oldURN)
		}
		if old.Custom && old.ID != newState.ID {
			return errorf("resource %s changed ID from %q to %q without changing URN", oldURN, old.ID, newState.ID)
		}
	}
	return nil
}

// resolveStateMigrationSuccessor follows a successor chain to its final returned URN.
func resolveStateMigrationSuccessor(
	urn resource.URN, successors map[resource.URN]resource.URN,
) (resource.URN, error) {
	seen := make(map[resource.URN]bool)
	for {
		if seen[urn] {
			return "", fmt.Errorf("successor mappings contain a cycle at %s", urn)
		}
		seen[urn] = true
		next, ok := successors[urn]
		if !ok {
			return urn, nil
		}
		urn = next
	}
}

// finalStateMigrationSuccessors composes mappings returned by chained callbacks and validates them against the
// original and final state sets. It returns both canonical mappings sourced from the original state and a complete
// map that also rewrites references to intermediate callback states.
func finalStateMigrationSuccessors(
	original, final []apitype.ResourceV3, all map[resource.URN]resource.URN,
) (map[resource.URN]resource.URN, map[resource.URN]resource.URN, error) {
	// Resolve every mapping, including intermediate sources, so cycles are rejected even if none of their sources
	// appeared in the original state.
	composed := make(map[resource.URN]resource.URN, len(all))
	for source := range all {
		resolved, err := resolveStateMigrationSuccessor(source, all)
		if err != nil {
			return nil, nil, err
		}
		composed[source] = resolved
	}

	finalURNs := make(map[resource.URN]bool, len(final))
	for _, state := range final {
		finalURNs[state.URN] = true
	}
	for source, target := range composed {
		if finalURNs[source] {
			return nil, nil, fmt.Errorf("resource %s is present in the final state and also has successor %s",
				source, target)
		}
		if !finalURNs[target] {
			return nil, nil, fmt.Errorf("successor %s for resource %s is not present in the final state", target, source)
		}
	}

	result := make(map[resource.URN]resource.URN)
	for _, state := range original {
		if finalURNs[state.URN] {
			continue
		}
		target, ok := composed[state.URN]
		if !ok {
			return nil, nil, fmt.Errorf("did not account for resource %s: it must either be returned in the final state or "+
				"have a successor", state.URN)
		}
		result[state.URN] = target
	}
	return result, composed, nil
}

// validateStateMigrationContext rejects state-changing migrations in update contexts whose state is deliberately
// only partially operated on, or which contain recovery records that the migration callback cannot update. No-op
// migrations return before this check, so permanently attached idempotent callbacks remain usable in these contexts.
func validateStateMigrationContext(
	urn resource.URN, opts *Options, snap *Snapshot, successors map[resource.URN]resource.URN,
) error {
	if opts.Targets.IsConstrained() || opts.Excludes.IsConstrained() ||
		opts.ReplaceTargets.IsConstrained() || len(opts.TargetSnippets) > 0 {
		return fmt.Errorf("state migration for %s cannot change state during a targeted or excluded update; "+
			"run a full update without --target, --exclude, --replace, or --target-snippet", urn)
	}
	if len(snap.PendingOperations) > 0 {
		return fmt.Errorf("state migration for %s cannot change state while the snapshot has %d pending "+
			"operation(s); resolve them before migrating %s", urn, len(snap.PendingOperations), urn)
	}
	for _, snippet := range snap.Snippets {
		for name, referenced := range snippet.References {
			predecessor := resource.URN(referenced)
			if successor, ok := successors[predecessor]; ok {
				return fmt.Errorf("state migration for %s cannot rewrite snippet %q reference %q from %s to %s; "+
					"remove or update the persisted snippet before migrating %s",
					urn, snippet.UUID, name, predecessor, successor, urn)
			}
		}
	}
	return nil
}

// validateStateMigrationManagedIdentity ensures a migration cannot abandon one managed object and silently make a
// different object (or a component) its successor. A custom resource's physical ID, ownership, and the engine's record
// that its provider deletion has already happened follow its final successor after callback chaining. Components do
// not carry these constraints and may fold freely.
func validateStateMigrationManagedIdentity(
	urn resource.URN,
	original, final []apitype.ResourceV3,
	successors map[resource.URN]resource.URN,
) error {
	finalByURN := make(map[resource.URN]apitype.ResourceV3, len(final))
	for _, state := range final {
		finalByURN[state.URN] = state
	}

	inheritedPendingReplacement := make(map[resource.URN]bool)
	inheritedTaint := make(map[resource.URN]bool)
	for _, old := range original {
		if !old.Custom {
			continue
		}

		targetURN := old.URN
		target, retained := finalByURN[targetURN]
		if !retained {
			var ok bool
			targetURN, ok = successors[old.URN]
			if !ok {
				continue
			}
			target, ok = finalByURN[targetURN]
			if !ok {
				continue
			}
		}

		if !target.Custom {
			return fmt.Errorf("state migration for %s maps managed custom resource %s to component successor %s; "+
				"migrations must preserve managed resource identity", urn, old.URN, targetURN)
		}
		if old.ID != target.ID {
			return fmt.Errorf("state migration for %s changes the physical ID of managed custom resource %s "+
				"from %q to %q on successor %s; migrations must preserve managed resource identity",
				urn, old.URN, old.ID, target.ID, targetURN)
		}
		if old.External != target.External {
			return fmt.Errorf("state migration for %s changes ownership of custom resource %s "+
				"from external=%t to external=%t on successor %s; migrations must preserve managed resource ownership",
				urn, old.URN, old.External, target.External, targetURN)
		}
		if old.PendingReplacement != target.PendingReplacement {
			return fmt.Errorf("state migration for %s changes PendingReplacement for custom resource %s "+
				"from %t to %t on successor %s; migrations must preserve provider lifecycle state",
				urn, old.URN, old.PendingReplacement, target.PendingReplacement, targetURN)
		}
		if old.Taint != target.Taint {
			return fmt.Errorf("state migration for %s changes Taint for custom resource %s "+
				"from %t to %t on successor %s; migrations must preserve provider lifecycle state",
				urn, old.URN, old.Taint, target.Taint, targetURN)
		}
		if old.PendingReplacement {
			inheritedPendingReplacement[targetURN] = true
		}
		if old.Taint {
			inheritedTaint[targetURN] = true
		}
	}

	for _, state := range final {
		if state.PendingReplacement && (!state.Custom || !inheritedPendingReplacement[state.URN]) {
			return fmt.Errorf("state migration for %s returns resource %s with PendingReplacement set "+
				"without a pending-replacement custom predecessor", urn, state.URN)
		}
		if state.Taint && (!state.Custom || !inheritedTaint[state.URN]) {
			return fmt.Errorf("state migration for %s returns resource %s with Taint set "+
				"without a tainted custom predecessor", urn, state.URN)
		}
	}
	return nil
}

// validateStateMigrationProviderStates keeps provider registration and configuration outside the scope of the raw
// state migration API. Provider resources participate in separate engine registries and lifecycle rules that cannot
// be reconstructed by rewriting checkpoint state alone. A provider in the migrated subtree may therefore be carried
// through, but it must retain its exact normalized checkpoint representation and URN.
func validateStateMigrationProviderStates(
	urn resource.URN,
	original, final []apitype.ResourceV3,
) error {
	originalProviders := make(map[resource.URN]apitype.ResourceV3)
	for _, state := range original {
		if sdkproviders.IsProviderType(state.Type) {
			originalProviders[state.URN] = state
		}
	}

	finalProviders := make(map[resource.URN]apitype.ResourceV3)
	for _, state := range final {
		if sdkproviders.IsProviderType(state.Type) {
			finalProviders[state.URN] = state
		}
	}

	for providerURN, originalState := range originalProviders {
		finalState, ok := finalProviders[providerURN]
		if !ok {
			return fmt.Errorf("state migration for %s removes or renames provider state %s; "+
				"provider states must be returned unchanged", urn, providerURN)
		}
		if !reflect.DeepEqual(originalState, finalState) {
			return fmt.Errorf("state migration for %s changes provider state %s; "+
				"provider states must be returned unchanged", urn, providerURN)
		}
	}
	for providerURN := range finalProviders {
		if _, ok := originalProviders[providerURN]; !ok {
			return fmt.Errorf("state migration for %s introduces provider state %s; "+
				"provider states cannot be created by a state migration", urn, providerURN)
		}
	}
	return nil
}

// validateMigratedStates checks that the final set of migrated states is well-formed before it is spliced into
// the base state: the registering resource's state must be present, all states must be parented (transitively)
// to it, and no state may introduce references that cannot be resolved.
func (sg *stepGenerator) validateMigratedStates(
	urn resource.URN, root *pkgresource.State, members, migrated []*pkgresource.State,
	successors map[resource.URN]resource.URN,
) error {
	errorf := func(format string, args ...any) error {
		prefix := fmt.Sprintf("state migration for %s: ", urn)
		return fmt.Errorf(prefix+format, args...)
	}

	memberURNs := make(map[resource.URN]bool, len(members))
	for _, member := range members {
		memberURNs[member.URN] = true
	}

	migratedByURN := make(map[resource.URN]*pkgresource.State, len(migrated))
	for _, state := range migrated {
		migratedByURN[state.URN] = state
	}
	isRoot := func(u resource.URN) bool { return u == urn || u == root.URN }
	if _, ok := migratedByURN[urn]; !ok {
		if _, ok := migratedByURN[root.URN]; !ok {
			return errorf("the migrated state must include the state for %s (the resource being registered) "+
				"under its new URN or its prior URN %s", urn, root.URN)
		}
	}

	// resolvableOutside is the set of prior resources outside the migrated subtree that migrated states may
	// still reference (providers, dependencies, and so on).
	resolvable := func(target resource.URN) bool {
		if _, ok := migratedByURN[target]; ok {
			return true
		}
		if memberURNs[target] {
			// Successor references have already been rewritten. A missing former member at this point would dangle.
			return false
		}
		_, ok := sg.deployment.Olds()[target]
		return ok
	}

	for _, state := range migrated {
		if !state.URN.IsValid() {
			return errorf("returned resource has invalid URN %q", state.URN)
		}
		if state.URN != urn {
			if claimant, claimed := sg.aliased[state.URN]; claimed {
				return errorf("returned resource %s was already claimed as an alias by %s earlier in this deployment",
					state.URN, claimant)
			}
			if sg.urns[state.URN] {
				return errorf("returned resource %s was already registered earlier in this deployment", state.URN)
			}
		}
		if state.Type != state.URN.Type() {
			return errorf("returned resource %s with type %s, which does not match its URN type %s",
				state.URN, state.Type, state.URN.Type())
		}
		if state.Delete {
			return errorf("returned resource %s is marked for pending deletion; migrations may not return "+
				"pending-delete resources", state.URN)
		}
		if state.ViewOf != "" {
			return errorf("returned resource %s is a view; state migrations do not support views", state.URN)
		}
		if state.Custom && state.ID == "" {
			return errorf("returned custom resource %s has no ID", state.URN)
		}
		if state.ExtensionRef != "" {
			if _, ok := sg.deployment.prev.Extensions[state.ExtensionRef]; !ok {
				return errorf("returned resource %s references unknown extension %s",
					state.URN, state.ExtensionRef)
			}
		}

		if isRoot(state.URN) {
			if state.Parent != root.Parent {
				return errorf("the parent of %s may not be changed by a migration (got %q, expected %q)",
					state.URN, state.Parent, root.Parent)
			}
		} else {
			// Walk the parent chain within the migrated set: it must terminate at the root.
			seen := 0
			parent := state.Parent
			for !isRoot(parent) {
				if seen++; seen > len(migrated) {
					return errorf("resource %s has a cyclic parent chain", state.URN)
				}
				next, ok := migratedByURN[parent]
				if !ok {
					return errorf("resource %s is parented to %s which is not part of the migrated state",
						state.URN, parent)
				}
				parent = next.Parent
			}
		}

		provider, allDeps := state.GetAllDependencies()
		if provider != "" {
			ref, err := sdkproviders.ParseReference(provider)
			if err != nil {
				return errorf("resource %s has an invalid provider reference %q: %w", state.URN, provider, err)
			}
			if !resolvable(ref.URN()) {
				return errorf("resource %s refers to unknown provider %s", state.URN, ref)
			}
		}
		for _, dep := range allDeps {
			if dep.Type == pkgresource.ResourceParent {
				continue
			}
			if !resolvable(dep.URN) {
				return errorf("resource %s refers to unknown resource %s", state.URN, dep.URN)
			}
		}
	}

	// Warn about additional states that have neither a prior state with the same URN nor an explicit predecessor.
	// Explicit successor targets are authorized continuations (including type changes), while genuinely added states
	// are effectively imported without provider verification.
	successorTargets := make(map[resource.URN]bool, len(successors))
	for _, target := range successors {
		successorTargets[target] = true
	}
	for _, state := range migrated {
		if state.Custom && !memberURNs[state.URN] && !successorTargets[state.URN] {
			sg.deployment.Diag().Warningf(diag.Message(urn,
				"state migration for %s introduces resource %s with ID %q without an explicit predecessor; "+
					"the engine cannot verify that it matches a live resource"), urn, state.URN, state.ID)
		}
	}
	return nil
}
