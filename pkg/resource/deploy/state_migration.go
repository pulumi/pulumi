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

	// Resolve the prior state of the registering resource: by URN first and then by any aliases, in the same
	// order used by getOldResource. If there is no prior state this is a fresh install and there is nothing to
	// migrate. Do this before pausing the executor so attaching a migration callback to a newly created resource
	// does not wait for unrelated work.
	goal := event.Goal()
	resolveRoot := func() (*pkgresource.State, bool) {
		olds := sg.deployment.Olds()
		root, hasOld := olds[urn]
		if !hasOld {
			for _, alias := range sg.generateAliases(goal.Name, goal.Type, goal.Parent, goal.Aliases) {
				if old, ok := olds[alias]; ok {
					return old, true
				}
			}
		}
		return root, hasOld
	}
	root, hasOld := resolveRoot()
	if !hasOld {
		logging.V(5).Infof("StateMigration: no prior state for %s, skipping migrations", urn)
		return nil
	}

	// Pause step execution before capturing prior state. A worker completing while the callback runs could otherwise
	// leave the migration validating and committing a stale subtree. Hold the lock through persistence and the
	// in-memory splice so the callback input and committed result are based on the same deployment state.
	if sg.stepExecLock != nil {
		sg.stepExecLock.Lock()
		defer sg.stepExecLock.Unlock()

		// The fast-path lookup above was intentionally outside the barrier. Resolve again after acquiring it so
		// capture never relies on a state that changed while waiting.
		root, hasOld = resolveRoot()
		if !hasOld {
			logging.V(5).Infof("StateMigration: prior state for %s disappeared before capture", urn)
			return nil
		}
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
	// loudly rather than silently double-consume the state.
	if claimant, ok := sg.aliased[urn]; ok {
		return fmt.Errorf("state migration for %s: the registration URN was already claimed as an alias by %s "+
			"earlier in this deployment", urn, claimant)
	}
	for _, member := range members {
		if claimant, ok := sg.aliased[member.URN]; ok {
			return fmt.Errorf("state migration for %s: the prior state of %s was already claimed by %s earlier "+
				"in this deployment (via an alias); it cannot also be migrated as part of %s in the same "+
				"operation", urn, member.URN, claimant, urn)
		}
		// generateURN recorded the current registration before applyStateMigrations was called. Exempt only that
		// exact marker; an aliased old root has a different URN, so a seen marker for it belongs to earlier work.
		if member == root && member.URN == urn {
			continue
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
	if err := validateStateMigrationProviderStates(urn, priorNormalized, current); err != nil {
		return err
	}

	// Compose mappings from chained callbacks (A -> B followed by B -> C becomes A -> C). Canonical mappings have
	// original sources and are persisted; rewrite mappings also include intermediate sources so references returned by
	// later callbacks cannot dangle.
	successors, rewriteSuccessors, err := finalStateMigrationSuccessors(serialized, current, allSuccessors)
	if err != nil {
		return fmt.Errorf("state migration for %s: %w", urn, err)
	}
	if err := validateStateMigrationContext(urn, opts, sg.deployment.prev, successors); err != nil {
		return err
	}
	if err := validateStateMigrationManagedIdentity(urn, serialized, current, successors); err != nil {
		return err
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
	rewrittenMigrated, err := rewriteStateMigrationReferences(migrated, rewriteSuccessors)
	if err != nil {
		return fmt.Errorf("state migration for %s: rewriting successor references: %w", urn, err)
	}
	for i, state := range migrated {
		if sdkproviders.IsProviderType(state.Type) && rewrittenMigrated[i] != state {
			return fmt.Errorf("state migration for %s rewrites references in provider state %s; "+
				"provider states must be returned unchanged", urn, state.URN)
		}
	}
	migrated = rewrittenMigrated
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

// commitStateMigration splices the migrated states into the deployment's view of the prior state. References
// throughout the snapshot are rewritten to explicit successors before the result is verified or persisted.
func (sg *stepGenerator) commitStateMigration(
	urn resource.URN, members, migrated []*pkgresource.State, successors map[resource.URN]resource.URN,
) error {
	d := sg.deployment

	memberSet := make(map[*pkgresource.State]bool, len(members))
	for _, member := range members {
		memberSet[member] = true
	}
	last := members[len(members)-1]

	// Build the new base resource list: the migrated states take the position of the last member so that they appear
	// after anything the prior states could reference.
	candidates := make([]*pkgresource.State, 0, len(d.prev.Resources)-len(members)+len(migrated))
	migratedIndices := make([]int, 0, len(migrated))
	retainedIndices := make(map[*pkgresource.State]int, len(d.prev.Resources)-len(members))
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
		retainedIndices[res] = len(candidates)
		candidates = append(candidates, res)
	}

	// Derive reference target identity only from the migration result. The base may legally also contain a
	// pending-deletion state with the same URN as a returned successor; it must not override the live successor's ID.
	rewritten, err := rewriteStateMigrationReferencesWithTargets(
		candidates, stateMigrationTargets(migrated), successors)
	if err != nil {
		return fmt.Errorf("state migration for %s: rewriting successor references: %w", urn, err)
	}
	rewrittenMigrated := make([]*pkgresource.State, len(migratedIndices))
	for i, index := range migratedIndices {
		rewrittenMigrated[i] = rewritten[index]
	}
	retainedResources := make(map[*pkgresource.State]*pkgresource.State, len(retainedIndices))
	for old, index := range retainedIndices {
		if sdkproviders.IsProviderType(old.Type) && rewritten[index] != old {
			return fmt.Errorf("state migration for %s rewrites references in provider state %s; "+
				"provider states must remain unchanged", urn, old.URN)
		}
		retainedResources[old] = rewritten[index]
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

	plan := &StateMigrationPlan{
		RootURN:           urn,
		RemovedResources:  members,
		MigratedResources: rewrittenMigrated,
		SuccessorURNs:     successors,
		BaseResources:     rewritten,
		RetainedResources: retainedResources,
	}

	// Prepare rewrites for resources produced earlier in this update. These pointers also back getResource and the
	// legacy snapshot manager, so committing them centrally keeps runtime behavior and every persistence path aligned.
	currentSet := make(map[*pkgresource.State]struct{})
	if d.news != nil {
		d.news.Range(func(_ resource.URN, state *pkgresource.State) bool {
			currentSet[state] = struct{}{}
			return true
		})
	}
	if d.reads != nil {
		d.reads.Range(func(_ resource.URN, state *pkgresource.State) bool {
			currentSet[state] = struct{}{}
			return true
		})
	}
	current := make([]*pkgresource.State, 0, len(currentSet))
	for state := range currentSet {
		current = append(current, state)
	}
	rewrittenCurrent, err := plan.RewriteResources(current)
	if err != nil {
		return fmt.Errorf("state migration for %s: rewriting current resources: %w", urn, err)
	}
	currentResources := make(map[*pkgresource.State]*pkgresource.State)
	for i, state := range current {
		if rewrittenCurrent[i] != state {
			if sdkproviders.IsProviderType(state.Type) {
				return fmt.Errorf("state migration for %s rewrites references in provider state %s "+
					"created or updated earlier in this deployment; provider states must remain unchanged",
					urn, state.URN)
			}
			currentResources[state] = rewrittenCurrent[i]
		}
	}

	// Notify the snapshot manager before the in-memory base state is mutated: it resolves the removed states
	// against the current (pre-splice) base snapshot.
	migrationEvents, ok := d.events.(StateMigrationEvents)
	if !ok {
		return fmt.Errorf("state migration for %s: %w", urn, ErrStateMigrationsUnsupported)
	}
	if err := migrationEvents.OnStateMigration(plan); err != nil {
		return fmt.Errorf("state migration for %s: %w", urn, err)
	}

	for before, after := range currentResources {
		before.Lock.Lock()
		applyStateMigrationReferenceRewrite(before, after)
		before.Lock.Unlock()
	}

	// Point of no return: install the verified state while preserving retained-resource pointers. Steps may have
	// been generated before this migration and still identify their old state by pointer; replacing retained states
	// with rewrite copies would make snapshot-manager bookkeeping unable to find them.
	committedResources := make([]*pkgresource.State, len(plan.BaseResources))
	copy(committedResources, plan.BaseResources)
	for before, after := range plan.RetainedResources {
		if after != before {
			before.Lock.Lock()
			applyStateMigrationReferenceRewrite(before, after)
			before.Lock.Unlock()
		}
		committedResources[retainedIndices[before]] = before
	}
	d.prev.Resources = committedResources

	oldResources, hasRefreshBeforeUpdateResources, olds, allOlds, oldViews, err := buildResourceMaps(d.prev)
	contract.AssertNoErrorf(err, "state migration for %s produced duplicate resources after verification", urn)
	d.hasRefreshBeforeUpdateResources = hasRefreshBeforeUpdateResources
	d.depGraph = graph.NewDependencyGraph(oldResources)
	d.olds = olds
	d.allOlds = allOlds
	d.oldViews = oldViews

	// Publish the rewrite rule only after the prepared transaction is fully installed. Step execution and outputs
	// registration are paused for the duration of applyStateMigrations, so later producers either precede this
	// transaction and were rewritten above, or observe the completed rule and normalize through it.
	d.stateMigrationsM.Lock()
	d.stateMigrations = append(d.stateMigrations,
		newStateMigrationRewrite(plan.RootURN, plan.SuccessorURNs, plan.MigratedResources))
	d.stateMigrationsM.Unlock()

	logging.V(5).Infof("StateMigration: %s: %d prior resources migrated to %d resources",
		urn, len(members), len(rewrittenMigrated))
	return nil
}
