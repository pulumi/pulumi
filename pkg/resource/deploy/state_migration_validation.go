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
	"errors"
	"fmt"
	"reflect"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

var ErrStateMigrationsUnsupported = errors.New("state migrations are not supported by this deployment")

// validateStateMigrationContext rejects state migrations during partial updates, or when pending operations or
// persisted snippets prevent the snapshot from being rewritten safely.
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
			"operation(s); resolve them before migrating %s, by running 'pulumi refresh'", urn, len(snap.PendingOperations), urn)
	}
	// Snippet references are persisted separately from resource state and are not part of StateMigrationTransaction.
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
// different object its successor. A successor mapping may change a resource's logical Pulumi identity (its URN and
// type), but a migration only rewrites checkpoint state, it does not ask a provider to import, read, or replace the
// underlying object. A custom resource's physical ID, provider and extension identity, whether Pulumi owns its
// lifecycle (External), and its PendingReplacement and Taint flags must therefore follow its successor. Protect and
// RetainOnDelete are engine-owned safety metadata and must be inherited conservatively by any custom or component
// successor: when multiple resources are folded together, the successor must carry either flag if any predecessor did.
// The provider identity restrictions do not apply to components because they have no provider-managed physical
// identity.
func validateStateMigrationManagedIdentity(
	urn resource.URN,
	original, final []apitype.ResourceV3,
	successors map[resource.URN]resource.URN,
) error {
	finalByURN := make(map[resource.URN]apitype.ResourceV3, len(final))
	for _, state := range final {
		finalByURN[state.URN] = state
	}

	type inheritedState struct {
		hasCustomPredecessor bool
		protect              bool
		retainOnDelete       bool
		pendingReplacement   bool
		taint                bool
	}
	inheritedByURN := make(map[resource.URN]inheritedState, len(final))
	for _, old := range original {
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

		inherited := inheritedByURN[targetURN]
		inherited.protect = inherited.protect || old.Protect
		inherited.retainOnDelete = inherited.retainOnDelete || old.RetainOnDelete
		if !old.Custom {
			inheritedByURN[targetURN] = inherited
			continue
		}

		inherited.hasCustomPredecessor = true
		if !target.Custom {
			return fmt.Errorf("state migration for %s maps managed custom resource %s to component successor %s; "+
				"migrations must preserve managed resource identity", urn, old.URN, targetURN)
		}
		if old.ID != target.ID {
			return fmt.Errorf("state migration for %s changes the physical ID of managed custom resource %s "+
				"from %q to %q on successor %s; migrations must preserve managed resource identity",
				urn, old.URN, old.ID, target.ID, targetURN)
		}
		if old.Provider != target.Provider {
			return fmt.Errorf("state migration for %s changes the provider reference of managed custom resource %s "+
				"from %q to %q on successor %s; migrations must preserve managed resource provider identity",
				urn, old.URN, old.Provider, target.Provider, targetURN)
		}
		if old.ExtensionRef != target.ExtensionRef {
			return fmt.Errorf("state migration for %s changes the extension reference of managed custom resource %s "+
				"from %q to %q on successor %s; migrations must preserve managed resource extension identity",
				urn, old.URN, old.ExtensionRef, target.ExtensionRef, targetURN)
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
			inherited.pendingReplacement = true
		}
		if old.Taint {
			inherited.taint = true
		}
		inheritedByURN[targetURN] = inherited
	}

	// The checks above validate retained custom resources and their successors. Now reject custom states without a
	// managed predecessor, and reject safety or lifecycle flags that were not inherited from their predecessors. This
	// includes states newly introduced by the migration.
	for _, state := range final {
		inherited := inheritedByURN[state.URN]
		if state.PendingReplacement && (!state.Custom || !inherited.pendingReplacement) {
			return fmt.Errorf("state migration for %s returns resource %s with PendingReplacement set "+
				"without a pending-replacement custom predecessor", urn, state.URN)
		}
		if state.Taint && (!state.Custom || !inherited.taint) {
			return fmt.Errorf("state migration for %s returns resource %s with Taint set "+
				"without a tainted custom predecessor", urn, state.URN)
		}
		if state.Custom && !inherited.hasCustomPredecessor {
			return fmt.Errorf("state migration for %s introduces custom resource %s with ID %q without a "+
				"managed custom predecessor; migrations cannot import resources", urn, state.URN, state.ID)
		}
		if state.Protect != inherited.protect {
			return fmt.Errorf("state migration for %s changes Protect for resource %s from inherited value %t to %t; "+
				"migrations must preserve resource protection", urn, state.URN, inherited.protect, state.Protect)
		}
		if state.RetainOnDelete != inherited.retainOnDelete {
			return fmt.Errorf("state migration for %s changes RetainOnDelete for resource %s from inherited value %t "+
				"to %t; migrations must preserve resource retention", urn, state.URN,
				inherited.retainOnDelete, state.RetainOnDelete)
		}
	}
	return nil
}

// validateStateMigrationProviderStates prevents migrations from adding, removing, renaming, or reconfiguring provider
// resources. A provider state corresponds to a configured plugin instance in the deployment's provider registry, which
// custom resources identify by the provider's URN and ID. Rewriting checkpoint state alone cannot make the matching
// registry change. A migration may therefore carry provider state through only with its exact normalized checkpoint
// representation and URN unchanged.
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

// validateStateMigrationResult checks that ResultSubtree is well-formed before using it to rewrite the prior snapshot:
// the registering resource's state must be present, all result states must be parented transitively to it, and no
// result state may introduce an unresolved structural reference. ResourceReference values in inputs and outputs are not
// structural dependencies and may legitimately identify resources in another stack.
//
// registrationURN is the current-program URN derived for the registration. priorRoot is the matched prior-state root;
// it may have a different URN when the registration found it through an alias after a rename.
func (sg *stepGenerator) validateStateMigrationResult(
	registrationURN resource.URN, priorRoot *pkgresource.State, priorSubtree, resultSubtree []*pkgresource.State,
) error {
	errorf := func(format string, args ...any) error {
		prefix := fmt.Sprintf("state migration for %s: ", registrationURN)
		return fmt.Errorf(prefix+format, args...)
	}

	priorURNs := make(map[resource.URN]bool, len(priorSubtree))
	for _, state := range priorSubtree {
		priorURNs[state.URN] = true
	}

	resultByURN := make(map[resource.URN]*pkgresource.State, len(resultSubtree))
	for _, state := range resultSubtree {
		resultByURN[state.URN] = state
	}
	resultRootURN := registrationURN
	rootCount := 0
	if _, ok := resultByURN[registrationURN]; ok {
		rootCount++
	}
	if priorRoot.URN != registrationURN {
		if _, ok := resultByURN[priorRoot.URN]; ok {
			rootCount++
			resultRootURN = priorRoot.URN
		}
	}
	if rootCount == 0 {
		return errorf("the migration result must include the state for %s (the resource being registered) "+
			"under its new URN or its prior URN %s", registrationURN, priorRoot.URN)
	}
	if rootCount != 1 {
		return errorf("the migration result includes both the new root %s and its prior root %s; "+
			"it must include exactly one logical root", registrationURN, priorRoot.URN)
	}
	isRoot := func(u resource.URN) bool { return u == resultRootURN }

	// resolvableOutside is the set of prior resources outside PriorSubtree that ResultSubtree may
	// still reference (providers, dependencies, and so on).
	resolvable := func(target resource.URN) bool {
		if _, ok := resultByURN[target]; ok {
			return true
		}
		if priorURNs[target] {
			// Successor references have already been rewritten. A missing former member at this point would dangle.
			return false
		}
		_, ok := sg.deployment.Olds()[target]
		return ok
	}

	for _, state := range resultSubtree {
		if !state.URN.IsValid() {
			return errorf("returned resource has invalid URN %q", state.URN)
		}
		if state.URN.Stack() != registrationURN.Stack() || state.URN.Project() != registrationURN.Project() {
			return errorf("returned resource %s belongs to stack %q and project %q; migration results must remain "+
				"in registration stack %q and project %q", state.URN, state.URN.Stack(), state.URN.Project(),
				registrationURN.Stack(), registrationURN.Project())
		}
		if state.URN != registrationURN {
			if claimant, claimed := sg.aliased[state.URN]; claimed {
				return errorf("returned resource %s was already claimed as an alias by %s earlier in this deployment",
					state.URN, claimant)
			}
			if sg.urns[state.URN] {
				return errorf("returned resource %s was already registered earlier in this deployment", state.URN)
			}
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
		if state.Parent != "" && !state.Parent.IsValid() {
			return errorf("returned resource %s has invalid parent URN %q", state.URN, state.Parent)
		}
		expectedQualifiedType := state.Type
		if state.Parent != "" && state.Parent.QualifiedType() != resource.RootStackType {
			expectedQualifiedType = state.Parent.QualifiedType() + resource.URNTypeDelimiter + state.Type
		}
		if state.URN.QualifiedType() != expectedQualifiedType {
			return errorf("resource %s has parent %s but qualified type %s; expected %s",
				state.URN, state.Parent, state.URN.QualifiedType(), expectedQualifiedType)
		}

		if isRoot(state.URN) {
			if state.Parent != priorRoot.Parent {
				return errorf("the parent of %s may not be changed by a migration (got %q, expected %q)",
					state.URN, state.Parent, priorRoot.Parent)
			}
		} else {
			// Walk the parent chain within ResultSubtree: it must terminate at the root.
			seen := 0
			parent := state.Parent
			for !isRoot(parent) {
				if seen++; seen > len(resultSubtree) {
					return errorf("resource %s has a cyclic parent chain", state.URN)
				}
				next, ok := resultByURN[parent]
				if !ok {
					return errorf("resource %s is parented to %s which is not part of the migration result",
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

	return nil
}
