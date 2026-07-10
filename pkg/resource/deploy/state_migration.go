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

	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// ResourceSerializer converts a resource state to its checkpoint (apitype.ResourceV3) representation for a state
// migration callback. Secrets are expected to be serialized in plaintext, under the secret signature envelope.
//
// This is injected via Options by the engine: the serialization logic lives in pkg/resource/stack, which imports
// this package and so cannot be imported from here.
type ResourceSerializer func(ctx context.Context, res *resource.State) (apitype.ResourceV3, error)

// ResourceDeserializer converts a checkpoint (apitype.ResourceV3) representation back into a resource state. It is
// the inverse of ResourceSerializer and is injected via Options for the same reason.
type ResourceDeserializer func(res apitype.ResourceV3) (*resource.State, error)

// StateMigrationSummary classifies what a state migration changed, for display and user-facing warnings.
type StateMigrationSummary struct {
	// Added holds the URNs of state entries introduced by the migration (present in the migrated state but not
	// the prior state).
	Added []resource.URN
	// Removed holds the URNs that left the state — renames (the resource reappears under a new URN) as well as
	// forgets.
	Removed []resource.URN
	// Unmanaged holds the subset of Removed whose resource identity (type + ID) left the state entirely: the
	// underlying cloud resources are no longer managed by Pulumi. They are NOT deleted.
	Unmanaged []resource.URN
}

// stateIdentity identifies a custom resource by its type and provider-assigned ID. It distinguishes a rename
// (the same underlying resource reappearing under a new URN) from a forget (the resource's identity leaving the
// state), keyed by (type, ID) rather than ID alone so that resources of different types that happen to share an
// ID are not conflated.
type stateIdentity struct {
	typ tokens.Type
	id  resource.ID
}

// SummarizeStateMigration classifies the difference between the prior (members) and migrated states into the
// Added / Removed / Unmanaged sets. It is used by the engine to warn about resources left unmanaged, and by the
// display layer to populate the state-migration engine event; keeping it here means both classify identically.
func SummarizeStateMigration(members, migrated []*resource.State) StateMigrationSummary {
	migratedURNs := make(map[resource.URN]bool, len(migrated))
	migratedIdentities := make(map[stateIdentity]bool, len(migrated))
	for _, state := range migrated {
		migratedURNs[state.URN] = true
		if state.Custom && state.ID != "" {
			migratedIdentities[stateIdentity{state.Type, state.ID}] = true
		}
	}
	memberURNs := make(map[resource.URN]bool, len(members))
	var summary StateMigrationSummary
	for _, member := range members {
		memberURNs[member.URN] = true
		if migratedURNs[member.URN] {
			continue
		}
		summary.Removed = append(summary.Removed, member.URN)
		if member.Custom && member.ID != "" && !migratedIdentities[stateIdentity{member.Type, member.ID}] {
			summary.Unmanaged = append(summary.Unmanaged, member.URN)
		}
	}
	for _, state := range migrated {
		if !memberURNs[state.URN] {
			summary.Added = append(summary.Added, state.URN)
		}
	}
	return summary
}

// applyStateMigrations runs the state migrations attached to a resource registration against the prior state of
// the resource and all prior resources transitively parented to it, and splices the transformed states back into
// the deployment's view of the prior state before any diffing takes place for those resources.
//
// Migrations run against the engine's in-memory view of the prior state only: they issue no provider calls and are
// therefore safe to run during previews. Resources returned by the migration are diffed as usual against the
// program's registrations; resources acknowledged as forgotten are removed from the base state without deleting
// the underlying cloud resources.
func (sg *stepGenerator) applyStateMigrations(
	ctx context.Context, event RegisterResourceEvent, urn resource.URN,
) error {
	migrations := event.StateMigrations()
	if len(migrations) == 0 {
		return nil
	}

	// State migrations only run during a normal update. During destroy the prior state is deleted wholesale, so
	// rewriting it first is unnecessary and a "forget" would silently exempt a resource from deletion; during a
	// refresh the prior state is reconciled against the cloud and must not be rewritten out from under it.
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
	members := []*resource.State{root}
	for _, child := range sg.deployment.depGraph.ChildrenOf(root) {
		if child.Delete {
			// Resources that are pending deletion are mid-replacement; they are not handed to migrations and
			// are left untouched in the base state.
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
	changed := false
	for i, migrate := range migrations {
		logging.V(5).Infof("StateMigration: running state migration (%d of %d) for %s", i+1, len(migrations), urn)

		newJSON, forgotten, err := migrate(ctx, urn, currentJSON)
		if err != nil {
			return fmt.Errorf("state migration %d of %d for %s: %w", i+1, len(migrations), urn, err)
		}
		if newJSON == nil {
			// No new state means the migration left the state unchanged.
			if len(forgotten) > 0 {
				return fmt.Errorf("state migration %d of %d for %s: returned forgotten resources without "+
					"returning a new state", i+1, len(migrations), urn)
			}
			continue
		}

		var newSet []apitype.ResourceV3
		if err := json.Unmarshal(newJSON, &newSet); err != nil {
			return fmt.Errorf("state migration %d of %d for %s: unmarshaling returned state: %w",
				i+1, len(migrations), urn, err)
		}
		if err := validateStateMigrationAccounting(urn, i, len(migrations), current, newSet, forgotten); err != nil {
			return err
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

	migrated := make([]*resource.State, len(current))
	for i, res := range current {
		state, err := opts.StateDeserializer(res)
		if err != nil {
			return fmt.Errorf("state migration for %s: deserializing returned state of %s: %w", urn, res.URN, err)
		}
		migrated[i] = state
	}
	if logging.V(9).Enabled() {
		logging.V(9).Infof("StateMigration: migrated state for %s:%s", urn, redactStatesForLog(migrated))
	}

	if err := sg.validateMigratedStates(urn, root, members, migrated); err != nil {
		return err
	}
	return sg.commitStateMigration(urn, members, migrated)
}

// redactStatesForLog renders resource states for debug logging with secret values masked. State migrations
// serialize prior and migrated state with plaintext secrets for the callback exchange, so that JSON must never
// reach a log; this renders the same states with secret values replaced by "[secret]".
func redactStatesForLog(states []*resource.State) string {
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
// handed: each input resource must either be present in the returned state or be explicitly acknowledged as
// forgotten. This ensures a migration cannot silently drop a resource from state by forgetting to handle it.
func validateStateMigrationAccounting(
	urn resource.URN, index, total int, oldSet, newSet []apitype.ResourceV3, forgotten []resource.URN,
) error {
	errorf := func(format string, args ...any) error {
		prefix := fmt.Sprintf("state migration %d of %d for %s: ", index+1, total, urn)
		return fmt.Errorf(prefix+format, args...)
	}

	oldStates := make(map[resource.URN]apitype.ResourceV3, len(oldSet))
	for _, res := range oldSet {
		oldStates[res.URN] = res
	}
	newURNs := make(map[resource.URN]bool, len(newSet))
	for _, res := range newSet {
		if res.URN == "" {
			return errorf("returned a resource with no URN")
		}
		if newURNs[res.URN] {
			return errorf("returned duplicate resource %s", res.URN)
		}
		newURNs[res.URN] = true
	}

	forgottenURNs := make(map[resource.URN]bool, len(forgotten))
	for _, f := range forgotten {
		old, ok := oldStates[f]
		if !ok {
			return errorf("forgot resource %s which is not part of the migrated state", f)
		}
		if newURNs[f] {
			return errorf("resource %s is both returned and forgotten", f)
		}
		if old.Protect {
			return errorf("cannot forget protected resource %s; protected resources must be unprotected "+
				"before they can be removed from state", f)
		}
		forgottenURNs[f] = true
	}

	for _, old := range oldSet {
		if !newURNs[old.URN] && !forgottenURNs[old.URN] {
			return errorf("did not account for resource %s: it must either be returned in the new state or "+
				"be acknowledged as forgotten", old.URN)
		}
	}
	return nil
}

// validateMigratedStates checks that the final set of migrated states is well-formed before it is spliced into
// the base state: the registering resource's state must be present, all states must be parented (transitively)
// to it, and no state may introduce references that cannot be resolved.
func (sg *stepGenerator) validateMigratedStates(
	urn resource.URN, root *resource.State, members, migrated []*resource.State,
) error {
	errorf := func(format string, args ...any) error {
		prefix := fmt.Sprintf("state migration for %s: ", urn)
		return fmt.Errorf(prefix+format, args...)
	}

	memberURNs := make(map[resource.URN]bool, len(members))
	oldIDs := make(map[resource.ID]bool, len(members))
	for _, member := range members {
		memberURNs[member.URN] = true
		if member.ID != "" {
			oldIDs[member.ID] = true
		}
	}

	migratedByURN := make(map[resource.URN]*resource.State, len(migrated))
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
			// The target was part of the subtree but is not part of the migrated state: it is being forgotten,
			// so references to it would dangle.
			return false
		}
		_, ok := sg.deployment.Olds()[target]
		return ok
	}

	for _, state := range migrated {
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
			if dep.Type == resource.ResourceParent {
				continue
			}
			if !resolvable(dep.URN) {
				return errorf("resource %s refers to unknown resource %s", state.URN, dep.URN)
			}
		}
	}

	// Warn about states whose IDs were not present in the prior state: the engine has no way to verify that
	// these correspond to real resources; they are effectively imported without verification.
	for _, state := range migrated {
		if state.Custom && !oldIDs[state.ID] {
			sg.deployment.Diag().Warningf(diag.Message(urn,
				"state migration for %s introduces resource %s with ID %q that was not present in the prior "+
					"state; the engine cannot verify that it matches a live resource"), urn, state.URN, state.ID)
		}
	}
	return nil
}

// commitStateMigration splices the migrated states into the deployment's view of the prior state. The members are
// removed from the base snapshot and the migrated states are inserted at the position of the last member; the
// spliced state is verified before any of the deployment's state is mutated, and the snapshot manager is notified
// (via the OnStateMigration event) so that persisted state stays consistent with the in-memory view.
func (sg *stepGenerator) commitStateMigration(urn resource.URN, members, migrated []*resource.State) error {
	d := sg.deployment

	// Pause step execution while the base state is rewritten: executing steps and the snapshot manager read the
	// base snapshot, which is about to be mutated.
	if sg.stepExecLock != nil {
		sg.stepExecLock.Lock()
		defer sg.stepExecLock.Unlock()
	}

	memberSet := make(map[*resource.State]bool, len(members))
	for _, member := range members {
		memberSet[member] = true
	}
	last := members[len(members)-1]

	// Build the new base resource list: the migrated states take the position of the last member so that they
	// appear after anything the prior states could reference. Resources that appear between subtree members and
	// depend on them will fail verification below; such interleavings cannot be expressed by this splice.
	candidates := make([]*resource.State, 0, len(d.prev.Resources)-len(members)+len(migrated))
	for _, res := range d.prev.Resources {
		if memberSet[res] {
			if res == last {
				candidates = append(candidates, migrated...)
			}
			continue
		}
		candidates = append(candidates, res)
	}

	// Determine which URNs are still present; references to URNs that are gone (forgotten resources) must be
	// pruned from the surviving resources, mirroring what a refresh does when it deletes resources.
	present := make(map[resource.URN]bool, len(candidates))
	for _, res := range candidates {
		present[res.URN] = true
	}

	type repair struct {
		state   *resource.State
		removed []resource.StateDependency
		apply   func(target *resource.State)
	}
	var repairs []repair
	for _, res := range candidates {
		if _, allDeps := res.GetAllDependencies(); len(allDeps) > 0 {
			var removed []resource.StateDependency
			for _, dep := range allDeps {
				if dep.Type != resource.ResourceParent && !present[dep.URN] {
					removed = append(removed, dep)
				}
			}
			if len(removed) > 0 {
				repairs = append(repairs, repair{
					state:   res,
					removed: removed,
					apply: func(target *resource.State) {
						pruneStateDependencies(target, present)
					},
				})
			}
		}
	}

	// Verify the spliced state before mutating anything. Repairs are applied to copies for verification so that
	// a verification failure leaves the deployment's state untouched.
	verifyResources := candidates
	if len(repairs) > 0 {
		needsRepair := make(map[*resource.State]repair, len(repairs))
		for _, r := range repairs {
			needsRepair[r.state] = r
		}
		verifyResources = make([]*resource.State, len(candidates))
		for i, res := range candidates {
			if r, ok := needsRepair[res]; ok {
				repaired := res.Copy()
				r.apply(repaired)
				verifyResources[i] = repaired
			} else {
				verifyResources[i] = res
			}
		}
	}
	verifySnap := &Snapshot{
		Manifest:  d.prev.Manifest,
		Resources: verifyResources,
	}
	if err := verifySnap.VerifyIntegrity(); err != nil {
		return fmt.Errorf("state migration for %s produced an invalid state; no changes were made: %w", urn, err)
	}

	// Classify the change once, for the warnings below. The display layer runs the same classification
	// (SummarizeStateMigration) to populate its event, so the two never disagree.
	summary := SummarizeStateMigration(members, migrated)

	// Notify the snapshot manager before the in-memory base state is mutated: it resolves the removed states
	// against the current (pre-splice) base snapshot.
	if d.events != nil {
		if err := d.events.OnStateMigration(urn, members, migrated); err != nil {
			return fmt.Errorf("state migration for %s: %w", urn, err)
		}
	}

	// Point of no return: apply the repairs to the real states and splice the new resource list in.
	for _, r := range repairs {
		r.apply(r.state)
		for _, dep := range r.removed {
			d.Diag().Warningf(diag.Message(urn,
				"state migration for %s: removed the dependency of %s on %s, which is no longer in the state"),
				urn, r.state.URN, dep.URN)
		}
	}
	d.prev.Resources = candidates

	oldResources, hasRefreshBeforeUpdateResources, olds, allOlds, oldViews, err := buildResourceMaps(d.prev)
	contract.AssertNoErrorf(err, "state migration for %s produced duplicate resources after verification", urn)
	d.hasRefreshBeforeUpdateResources = hasRefreshBeforeUpdateResources
	d.depGraph = graph.NewDependencyGraph(oldResources)
	d.olds = olds
	d.allOlds = allOlds
	d.oldViews = oldViews

	// Warn about resources whose identity left the state entirely: their cloud resources are now unmanaged.
	for _, unmanaged := range summary.Unmanaged {
		d.Diag().Warningf(diag.Message(urn,
			"state migration for %s removed %s from the state; the underlying cloud resource will NOT be "+
				"deleted"), urn, unmanaged)
	}
	logging.V(5).Infof("StateMigration: %s: %d prior resources migrated to %d resources (%d added, %d removed)",
		urn, len(members), len(migrated), len(summary.Added), len(summary.Removed))
	return nil
}

// pruneStateDependencies removes all dependency-style references (dependencies, property dependencies,
// deletedWith and replaceWith) from the given state that refer to URNs that are not present.
func pruneStateDependencies(state *resource.State, present map[resource.URN]bool) {
	if len(state.Dependencies) > 0 {
		deps := make([]resource.URN, 0, len(state.Dependencies))
		for _, dep := range state.Dependencies {
			if present[dep] {
				deps = append(deps, dep)
			}
		}
		state.Dependencies = deps
	}
	if len(state.PropertyDependencies) > 0 {
		propDeps := make(map[resource.PropertyKey][]resource.URN, len(state.PropertyDependencies))
		for key, urns := range state.PropertyDependencies {
			kept := make([]resource.URN, 0, len(urns))
			for _, dep := range urns {
				if present[dep] {
					kept = append(kept, dep)
				}
			}
			propDeps[key] = kept
		}
		state.PropertyDependencies = propDeps
	}
	if state.DeletedWith != "" && !present[state.DeletedWith] {
		state.DeletedWith = ""
	}
	if len(state.ReplaceWith) > 0 {
		replaceWith := make([]resource.URN, 0, len(state.ReplaceWith))
		for _, dep := range state.ReplaceWith {
			if present[dep] {
				replaceWith = append(replaceWith, dep)
			}
		}
		state.ReplaceWith = replaceWith
	}
}
