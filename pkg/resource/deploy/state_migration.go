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
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// ResourceSerializer converts a resource state to its checkpoint (apitype.ResourceV3) representation for a state
// migration callback. Secrets are expected to be serialized in plaintext, under the secret signature envelope.
//
// This is injected via Options by the engine: the serialization logic lives in pkg/resource/stack, which imports
// this package and so cannot be imported from here.
type ResourceSerializer func(ctx context.Context, res *pkgresource.State) (apitype.ResourceV3, error)

// ResourceDeserializer converts a checkpoint (apitype.ResourceV3) representation back into a resource state. It is
// the inverse of ResourceSerializer and is injected via Options for the same reason.
type ResourceDeserializer func(res apitype.ResourceV3) (*pkgresource.State, error)

// applyStateMigrations runs the state migrations attached to a resource registration against the prior state of
// the resource and all prior resources transitively parented to it, and splices the transformed states back into
// the deployment's view of the prior state before any diffing takes place for those resources.
//
// Migrations run against the engine's in-memory view of the prior state only: they issue no provider calls and are
// therefore safe to run during previews. Resources returned by the migration are diffed as usual against the
// program's registrations. Every resource removed from state must name a returned successor, so a migration cannot
// silently leave a physical resource unmanaged.
func (sg *stepGenerator) applyStateMigrations(
	ctx context.Context, event RegisterResourceEvent, urn resource.URN,
) error {
	migrations := event.StateMigrations()
	if len(migrations) == 0 {
		return nil
	}

	// State migrations only run during a normal update. During destroy the prior state is deleted wholesale, so
	// rewriting it first is unnecessary; during refresh it is reconciled against the cloud and must not be rewritten
	// out from under it.
	if sg.mode != updateMode {
		logging.V(5).Infof("StateMigration: skipping migrations for %s outside of update mode", urn)
		return nil
	}

	opts := sg.deployment.opts

	// Migrations only run for targeted registrations: an untargeted resource is not operated on, so its prior
	// state must be left untouched as well.
	if opts.Targets.IsConstrained() && !opts.Targets.Contains(urn) {
		logging.V(5).Infof("StateMigration: skipping migrations for untargeted resource %s", urn)
		return nil
	}
	if opts.Excludes.IsConstrained() && opts.Excludes.Contains(urn) {
		logging.V(5).Infof("StateMigration: skipping migrations for excluded resource %s", urn)
		return nil
	}

	// Resolve the prior state of the registering resource: by URN first and then by any aliases, in the same
	// order used by getOldResource. If there is no prior state this is a fresh install and there is nothing to
	// migrate.
	goal := event.Goal()
	olds := sg.deployment.Olds()
	root, hasOld := olds[urn]
	if !hasOld {
		for _, alias := range sg.generateAliases(goal.Name, goal.Type, goal.Parent, goal.Aliases) {
			if old, ok := olds[alias]; ok {
				root, hasOld = old, true
				break
			}
		}
	}
	if !hasOld {
		logging.V(5).Infof("StateMigration: no prior state for %s, skipping migrations", urn)
		return nil
	}

	if opts.StateSerializer == nil || opts.StateDeserializer == nil {
		return fmt.Errorf("state migration for %s: the deployment is not configured for state migrations", urn)
	}

	// Collect the prior state of the resource and all resources transitively parented to it, in snapshot order.
	// Resources that are pending deletion (mid-replacement leftovers of an earlier, failed update) are not handed
	// to migrations: the callbacks could not meaningfully account for them. They are collected separately so
	// that a migration which changes the state can be rejected explicitly below — splicing the subtree out from
	// around them could leave them referencing states that no longer exist.
	members := []*pkgresource.State{root}
	var pendingDeletes []*pkgresource.State
	for _, child := range sg.deployment.depGraph.ChildrenOf(root) {
		if child.Delete {
			pendingDeletes = append(pendingDeletes, child)
			continue
		}
		members = append(members, child)
	}
	for _, member := range members {
		if member.ViewOf != "" || len(sg.deployment.oldViews[member.URN]) > 0 {
			return fmt.Errorf("state migration for %s: resource %s is or has a view; "+
				"state migrations do not support views", urn, member.URN)
		}
	}

	// Reject migrating prior state that an earlier registration in this deployment already claimed (by URN or
	// alias) — for example a child hoisted out of the component and registered on its own before the component.
	// Splicing such a member out from under the resource that claimed it would corrupt the snapshot, so fail
	// loudly rather than silently double-consume the state. The root's own claim happens later in this same
	// registration, so it is exempt.
	for _, member := range members {
		if member == root {
			continue
		}
		if claimant, ok := sg.aliased[member.URN]; ok {
			return fmt.Errorf("state migration for %s: the prior state of %s was already claimed by %s earlier "+
				"in this deployment (via an alias); it cannot also be migrated as part of %s in the same "+
				"operation", urn, member.URN, claimant, urn)
		}
		if sg.urns[member.URN] {
			return fmt.Errorf("state migration for %s: the prior state of %s was already registered earlier in "+
				"this deployment; it cannot also be migrated as part of %s", urn, member.URN, urn)
		}
	}

	// Serialize the members to their checkpoint representation and hand them to each migration in turn.
	serialized := make([]apitype.ResourceV3, len(members))
	for i, member := range members {
		res, err := opts.StateSerializer(ctx, member)
		if err != nil {
			return fmt.Errorf("state migration for %s: serializing state of %s: %w", urn, member.URN, err)
		}
		serialized[i] = res
	}
	currentJSON, err := json.Marshal(serialized)
	if err != nil {
		return fmt.Errorf("state migration for %s: marshaling prior state: %w", urn, err)
	}
	originalJSON := currentJSON
	// NOTE: currentJSON serializes secrets in plaintext (the callback needs the real values), so it must never
	// be logged. Log a secret-redacted rendering of the states instead.
	if logging.V(9).Enabled() {
		logging.V(9).Infof("StateMigration: prior state for %s:%s", urn, redactStatesForLog(members))
	}

	current := serialized
	allSuccessors := make(map[resource.URN]resource.URN)
	changed := false
	for i, migrate := range migrations {
		logging.V(5).Infof("StateMigration: running state migration (%d of %d) for %s", i+1, len(migrations), urn)

		newJSON, successors, err := migrate(ctx, urn, currentJSON)
		if err != nil {
			return fmt.Errorf("state migration %d of %d for %s: %w", i+1, len(migrations), urn, err)
		}
		if newJSON == nil {
			// No new state means the migration left the state unchanged.
			if len(successors) > 0 {
				return fmt.Errorf("state migration %d of %d for %s: returned successors without "+
					"returning a new state", i+1, len(migrations), urn)
			}
			continue
		}

		var newSet []apitype.ResourceV3
		if err := json.Unmarshal(newJSON, &newSet); err != nil {
			return fmt.Errorf("state migration %d of %d for %s: unmarshaling returned state: %w",
				i+1, len(migrations), urn, err)
		}
		if err := validateStateMigrationAccounting(urn, i, len(migrations), current, newSet, successors); err != nil {
			return err
		}
		for oldURN, successor := range successors {
			if previous, exists := allSuccessors[oldURN]; exists && previous != successor {
				return fmt.Errorf("state migration %d of %d for %s: resource %s has conflicting successors %s and %s",
					i+1, len(migrations), urn, oldURN, previous, successor)
			}
			allSuccessors[oldURN] = successor
		}

		current, currentJSON, changed = newSet, newJSON, true
	}

	if !changed {
		// No migration returned a new state, so the prior state is unchanged.
		logging.V(5).Infof("StateMigration: migrations for %s made no changes", urn)
		return nil
	}
	// A migration that returns state (rather than nil) but leaves it semantically unchanged — the common
	// "check whether already migrated, otherwise return the input" idiom — must not trigger a splice. Compare
	// the returned state against the prior state normalized through the SAME JSON round-trip, so the check is
	// not defeated by serialization asymmetries (typed secrets vs decoded maps, empty vs nil slices, and so on).
	var priorNormalized []apitype.ResourceV3
	if err := json.Unmarshal(originalJSON, &priorNormalized); err != nil {
		return fmt.Errorf("state migration for %s: normalizing prior state: %w", urn, err)
	}
	if reflect.DeepEqual(priorNormalized, current) {
		logging.V(5).Infof("StateMigration: migrations for %s made no changes", urn)
		return nil
	}

	// Compose mappings from chained callbacks (A -> B followed by B -> C becomes A -> C). Canonical mappings have
	// original sources and are persisted; rewrite mappings also include intermediate sources so references returned by
	// later callbacks cannot dangle.
	successors, rewriteSuccessors, err := finalStateMigrationSuccessors(serialized, current, allSuccessors)
	if err != nil {
		return fmt.Errorf("state migration for %s: %w", urn, err)
	}

	// The migrations changed the state; reject the change if the subtree contains resources that are pending
	// deletion. Those states were hidden from the callbacks, so the migration cannot have accounted for them,
	// and rewriting the subtree around them can leave them referencing states that no longer exist. Migrations
	// that make no changes (the already-migrated case above) are still allowed, so an update whose migrations
	// are all no-ops proceeds and reaps the pending deletions as usual.
	if len(pendingDeletes) > 0 {
		pendingURNs := make([]string, len(pendingDeletes))
		for i, pending := range pendingDeletes {
			pendingURNs[i] = string(pending.URN)
		}
		return fmt.Errorf("state migration for %s: the prior state contains resources that are pending deletion "+
			"from a previous update: %s; resolve the pending deletion (for example by completing an update in "+
			"which this migration makes no changes) before migrating %s",
			urn, strings.Join(pendingURNs, ", "), urn)
	}

	migrated := make([]*pkgresource.State, len(current))
	for i, res := range current {
		state, err := opts.StateDeserializer(res)
		if err != nil {
			return fmt.Errorf("state migration for %s: deserializing returned state of %s: %w", urn, res.URN, err)
		}
		migrated[i] = state
	}
	migrated, err = rewriteStateMigrationReferences(migrated, rewriteSuccessors)
	if err != nil {
		return fmt.Errorf("state migration for %s: rewriting successor references: %w", urn, err)
	}
	if logging.V(9).Enabled() {
		logging.V(9).Infof("StateMigration: migrated state for %s:%s", urn, redactStatesForLog(migrated))
	}

	if err := sg.validateMigratedStates(urn, root, members, migrated, successors); err != nil {
		return err
	}
	return sg.commitStateMigration(urn, members, migrated, successors)
}

// redactStatesForLog renders resource states for debug logging with secret values masked. State migrations
// serialize prior and migrated state with plaintext secrets for the callback exchange, so that JSON must never
// reach a log; this renders the same states with secret values replaced by "[secret]".
func redactStatesForLog(states []*pkgresource.State) string {
	var sb strings.Builder
	for _, s := range states {
		fmt.Fprintf(&sb, "\n  %s", s.URN)
		if s.ID != "" {
			fmt.Fprintf(&sb, " id=%s", s.ID)
		}
		if s.Parent != "" {
			fmt.Fprintf(&sb, " parent=%s", s.Parent)
		}
		if len(s.Inputs) > 0 {
			fmt.Fprintf(&sb, " inputs=%s", resource.NewProperty(s.Inputs).RedactSecrets())
		}
		if len(s.Outputs) > 0 {
			fmt.Fprintf(&sb, " outputs=%s", resource.NewProperty(s.Outputs).RedactSecrets())
		}
	}
	return sb.String()
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
		if res.URN == "" {
			return errorf("received a resource with no URN")
		}
		if _, exists := oldStates[res.URN]; exists {
			return errorf("received duplicate resource %s", res.URN)
		}
		oldStates[res.URN] = res
	}
	newStates := make(map[resource.URN]apitype.ResourceV3, len(newSet))
	for _, res := range newSet {
		if res.URN == "" {
			return errorf("returned a resource with no URN")
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

// rewriteStateMigrationReferences returns independent copies of states with every reference to a removed URN
// rewritten to its final successor. This includes structural dependencies and resource references nested in property
// values. Multiple sources may resolve to the same target; dependency lists are deduplicated in that case.
func rewriteStateMigrationReferences(
	states []*pkgresource.State, successors map[resource.URN]resource.URN,
) ([]*pkgresource.State, error) {
	if len(successors) == 0 {
		return states, nil
	}

	fixURN := func(urn resource.URN) (resource.URN, error) {
		if urn == "" {
			return "", nil
		}
		return resolveStateMigrationSuccessor(urn, successors)
	}
	byURN := make(map[resource.URN]*pkgresource.State, len(states))
	for _, state := range states {
		byURN[state.URN] = state
	}
	rewriteURNs := func(urns []resource.URN) ([]resource.URN, error) {
		if len(urns) == 0 {
			return urns, nil
		}
		result := make([]resource.URN, 0, len(urns))
		seen := make(map[resource.URN]bool, len(urns))
		for _, urn := range urns {
			fixed, err := fixURN(urn)
			if err != nil {
				return nil, err
			}
			if !seen[fixed] {
				seen[fixed] = true
				result = append(result, fixed)
			}
		}
		return result, nil
	}

	var rewritePropertyValue func(resource.PropertyValue) (resource.PropertyValue, error)
	rewritePropertyMap := func(properties resource.PropertyMap) (resource.PropertyMap, error) {
		if properties == nil {
			return nil, nil
		}
		result := make(resource.PropertyMap, len(properties))
		for key, value := range properties {
			rewritten, err := rewritePropertyValue(value)
			if err != nil {
				return nil, err
			}
			result[key] = rewritten
		}
		return result, nil
	}
	rewritePropertyValue = func(value resource.PropertyValue) (resource.PropertyValue, error) {
		switch {
		case value.IsArray():
			array := value.ArrayValue()
			result := make([]resource.PropertyValue, len(array))
			for i, element := range array {
				rewritten, err := rewritePropertyValue(element)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				result[i] = rewritten
			}
			return resource.NewProperty(result), nil
		case value.IsObject():
			result, err := rewritePropertyMap(value.ObjectValue())
			return resource.NewProperty(result), err
		case value.IsComputed():
			element, err := rewritePropertyValue(value.Input().Element)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			return resource.MakeComputed(element), nil
		case value.IsOutput():
			output := value.OutputValue()
			element, err := rewritePropertyValue(output.Element)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			dependencies, err := rewriteURNs(output.Dependencies)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			output.Element = element
			output.Dependencies = dependencies
			return resource.NewProperty(output), nil
		case value.IsSecret():
			element, err := rewritePropertyValue(value.SecretValue().Element)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			return resource.MakeSecret(element), nil
		case value.IsResourceReference():
			ref := value.ResourceReferenceValue()
			fixed, err := fixURN(ref.URN)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			if fixed != ref.URN {
				ref.URN = fixed
				ref.Name = fixed.Name()
				ref.Type = string(fixed.Type())
				if target, ok := byURN[fixed]; ok {
					if target.Custom {
						ref.ID = resource.NewProperty(string(target.ID))
					} else {
						ref.ID = resource.NewNullProperty()
					}
				}
			}
			return resource.NewProperty(ref), nil
		default:
			return value, nil
		}
	}

	result := make([]*pkgresource.State, len(states))
	for i, state := range states {
		fixed := state.Copy()
		var err error
		fixed.Parent, err = fixURN(fixed.Parent)
		if err != nil {
			return nil, err
		}
		fixed.Dependencies, err = rewriteURNs(fixed.Dependencies)
		if err != nil {
			return nil, err
		}
		if fixed.PropertyDependencies != nil {
			propertyDependencies := make(map[resource.PropertyKey][]resource.URN, len(fixed.PropertyDependencies))
			for key, dependencies := range fixed.PropertyDependencies {
				propertyDependencies[key], err = rewriteURNs(dependencies)
				if err != nil {
					return nil, err
				}
			}
			fixed.PropertyDependencies = propertyDependencies
		}
		fixed.DeletedWith, err = fixURN(fixed.DeletedWith)
		if err != nil {
			return nil, err
		}
		fixed.ReplaceWith, err = rewriteURNs(fixed.ReplaceWith)
		if err != nil {
			return nil, err
		}
		fixed.ViewOf, err = fixURN(fixed.ViewOf)
		if err != nil {
			return nil, err
		}
		if fixed.Provider != "" {
			ref, err := sdkproviders.ParseReference(fixed.Provider)
			if err != nil {
				return nil, fmt.Errorf("parsing provider reference %q: %w", fixed.Provider, err)
			}
			providerURN, err := fixURN(ref.URN())
			if err != nil {
				return nil, err
			}
			providerID := ref.ID()
			if provider, ok := byURN[providerURN]; ok {
				providerID = provider.ID
			}
			providerRef, err := sdkproviders.NewReference(providerURN, providerID)
			if err != nil {
				return nil, fmt.Errorf("rewriting provider reference %q: %w", fixed.Provider, err)
			}
			fixed.Provider = providerRef.String()
		}
		fixed.Inputs, err = rewritePropertyMap(fixed.Inputs)
		if err != nil {
			return nil, err
		}
		fixed.Outputs, err = rewritePropertyMap(fixed.Outputs)
		if err != nil {
			return nil, err
		}
		replacementTrigger, err := rewritePropertyValue(resource.ToResourcePropertyValue(fixed.ReplacementTrigger))
		if err != nil {
			return nil, err
		}
		fixed.ReplacementTrigger = resource.FromResourcePropertyValue(replacementTrigger)
		if fixed.Parent == state.Parent &&
			reflect.DeepEqual(fixed.Dependencies, state.Dependencies) &&
			reflect.DeepEqual(fixed.PropertyDependencies, state.PropertyDependencies) &&
			fixed.DeletedWith == state.DeletedWith &&
			reflect.DeepEqual(fixed.ReplaceWith, state.ReplaceWith) &&
			fixed.ViewOf == state.ViewOf &&
			fixed.Provider == state.Provider &&
			fixed.Inputs.DeepEquals(state.Inputs) &&
			fixed.Outputs.DeepEquals(state.Outputs) &&
			fixed.ReplacementTrigger.Equals(state.ReplacementTrigger) {
			result[i] = state
		} else {
			result[i] = fixed
		}
	}
	return result, nil
}

// commitStateMigration splices the migrated states into the deployment's view of the prior state. References
// throughout the snapshot are rewritten to explicit successors before the result is verified or persisted.
func (sg *stepGenerator) commitStateMigration(
	urn resource.URN, members, migrated []*pkgresource.State, successors map[resource.URN]resource.URN,
) error {
	d := sg.deployment

	// Pause step execution while the base state is rewritten: executing steps and the snapshot manager read the
	// base snapshot, which is about to be mutated.
	if sg.stepExecLock != nil {
		sg.stepExecLock.Lock()
		defer sg.stepExecLock.Unlock()
	}

	memberSet := make(map[*pkgresource.State]bool, len(members))
	for _, member := range members {
		memberSet[member] = true
	}
	last := members[len(members)-1]

	// Build the new base resource list: the migrated states take the position of the last member so that they appear
	// after anything the prior states could reference.
	candidates := make([]*pkgresource.State, 0, len(d.prev.Resources)-len(members)+len(migrated))
	migratedIndices := make([]int, 0, len(migrated))
	for _, res := range d.prev.Resources {
		if memberSet[res] {
			if res == last {
				for _, state := range migrated {
					migratedIndices = append(migratedIndices, len(candidates))
					candidates = append(candidates, state)
				}
			}
			continue
		}
		candidates = append(candidates, res)
	}

	rewritten, err := rewriteStateMigrationReferences(candidates, successors)
	if err != nil {
		return fmt.Errorf("state migration for %s: rewriting successor references: %w", urn, err)
	}
	rewrittenMigrated := make([]*pkgresource.State, len(migratedIndices))
	for i, index := range migratedIndices {
		rewrittenMigrated[i] = rewritten[index]
	}

	// Verify the spliced state before mutating anything. Reference rewriting operates on copies, so any failure leaves
	// the deployment's state untouched.
	verifySnap := &Snapshot{
		Manifest:  d.prev.Manifest,
		Resources: rewritten,
	}
	if err := verifySnap.VerifyIntegrity(); err != nil {
		return fmt.Errorf("state migration for %s produced an invalid state; no changes were made: %w", urn, err)
	}

	// Notify the snapshot manager before the in-memory base state is mutated: it resolves the removed states
	// against the current (pre-splice) base snapshot.
	if d.events != nil {
		if err := d.events.OnStateMigration(urn, members, rewrittenMigrated, successors); err != nil {
			return fmt.Errorf("state migration for %s: %w", urn, err)
		}
	}

	// Point of no return: splice the verified, rewritten resource list in.
	d.prev.Resources = rewritten

	oldResources, hasRefreshBeforeUpdateResources, olds, allOlds, oldViews, err := buildResourceMaps(d.prev)
	contract.AssertNoErrorf(err, "state migration for %s produced duplicate resources after verification", urn)
	d.hasRefreshBeforeUpdateResources = hasRefreshBeforeUpdateResources
	d.depGraph = graph.NewDependencyGraph(oldResources)
	d.olds = olds
	d.allOlds = allOlds
	d.oldViews = oldViews

	logging.V(5).Infof("StateMigration: %s: %d prior resources migrated to %d resources",
		urn, len(members), len(rewrittenMigrated))
	return nil
}
