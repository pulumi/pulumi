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
	"fmt"
	"strings"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// applyStateMigrations runs the state migrations attached to a resource registration against the prior state of
// the resource and all prior resources transitively parented to it, and splices the transformed states back into
// the deployment's view of the prior state before any diffing takes place for those resources.
//
// Migrations run against the engine's in-memory view of the prior state only and issue no provider calls. A callback
// may be evaluated while generating an update plan, but a state-changing result is rejected because plans cannot
// describe checkpoint rewrites. Resources returned by the migration are diffed as usual against the program's
// registrations. Every resource removed from state must name a returned successor, so a migration cannot silently
// leave a physical resource unmanaged.
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
	// order used by getOldResource. If there is no prior state this is a new resource and there is nothing to
	// migrate. Do this before pausing the executor so attaching a migration callback to a newly created resource
	// does not wait for unrelated work.
	goal := event.Goal()
	olds := sg.deployment.Olds()
	root, hasOld := olds[urn]
	if !hasOld {
		aliases := sg.generateRewrittenAliases(goal.Name, goal.Type, goal.Parent, goal.Aliases)
		for _, alias := range aliases {
			if root, hasOld = olds[alias]; hasOld {
				break
			}
		}
	}
	if !hasOld {
		logging.V(5).Infof("StateMigration: no prior state for %s, skipping migrations", urn)
		return nil
	}

	// Earlier asynchronous planning continuations have already been handled before this registration, so ExecuteSerial
	// has submitted each of their step chains. Each chain holds the step executor's read lock until executeChain
	// finishes; taking the write lock here waits for those chains and prevents workers from mutating deployment state
	// while the callback runs. Hold it through persistence and the in-memory splice so the callback input and committed
	// result are based on the same deployment state.
	if sg.stepExecLock != nil {
		sg.stepExecLock.Lock()
		defer sg.stepExecLock.Unlock()
	}

	if opts.StateSerializer == nil || opts.StateDeserializer == nil {
		return fmt.Errorf("state migration for %s: the deployment is not configured for state migrations", urn)
	}

	// Collect the prior state of the resource and all resources transitively parented to it, in snapshot order.
	// Resources that are pending deletion (mid-replacement leftovers of an earlier, failed update) are not handed
	// to migrations: the callbacks could not meaningfully account for them. They are collected separately so
	// that a migration which changes the state can be rejected explicitly below — splicing the subtree out from
	// around them could leave them referencing states that no longer exist.
	priorSubtree := []*pkgresource.State{root}
	var pendingDeletes []*pkgresource.State
	for _, child := range sg.deployment.depGraph.ChildrenOf(root) {
		if child.Delete {
			pendingDeletes = append(pendingDeletes, child)
			continue
		}
		priorSubtree = append(priorSubtree, child)
	}
	for _, state := range priorSubtree {
		if state.ViewOf != "" || len(sg.deployment.oldViews[state.URN]) > 0 {
			return fmt.Errorf("state migration for %s: resource %s is or has a view; "+
				"state migrations do not support views", urn, state.URN)
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
	for _, state := range priorSubtree {
		if claimant, ok := sg.aliased[state.URN]; ok {
			return fmt.Errorf("state migration for %s: the prior state of %s was already claimed by %s earlier "+
				"in this deployment (via an alias); it cannot also be migrated as part of %s in the same "+
				"operation", urn, state.URN, claimant, urn)
		}
		// generateURN recorded the current registration before applyStateMigrations was called. Exempt only that
		// exact marker; an aliased old root has a different URN, so a seen marker for it belongs to earlier work.
		if state == root && state.URN == urn {
			continue
		}
		if sg.urns[state.URN] {
			return fmt.Errorf("state migration for %s: the prior state of %s was already registered earlier in "+
				"this deployment; it cannot also be migrated as part of %s", urn, state.URN, urn)
		}
	}

	// Serialize PriorSubtree to its checkpoint representation and hand it to each migration in turn.
	serialized := make([]apitype.ResourceV3, len(priorSubtree))
	for i, state := range priorSubtree {
		res, err := opts.StateSerializer(ctx, state)
		if err != nil {
			return fmt.Errorf("state migration for %s: serializing state of %s: %w", urn, state.URN, err)
		}
		serialized[i] = res
	}
	// Callback state contains plaintext secrets, so it must never be logged. Log a redacted rendering instead.
	if logging.V(9).Enabled() {
		logging.V(9).Infof("StateMigration: prior state for %s:%s", urn, redactStatesForLog(priorSubtree))
	}

	callbackResult, err := runStateMigrationCallbacks(ctx, urn, migrations, serialized)
	if err != nil {
		return err
	}
	if callbackResult == nil {
		logging.V(5).Infof("StateMigration: migrations for %s made no changes", urn)
		return nil
	}
	successors := callbackResult.originalToFinal
	// Update plans constrain resource operations; they do not describe or authorize checkpoint rewrites. Reject
	// state-changing migrations both when generating a plan and when applying one. No-op migrations returned above,
	// so permanently attached migrations still work with plans.
	if sg.deployment.plan != nil {
		return fmt.Errorf("state migration for %s cannot change state while applying an update plan; "+
			"run the update without the plan", urn)
	}
	if opts.DryRun && opts.GeneratePlan {
		return fmt.Errorf("state migration for %s cannot change state while generating an update plan; "+
			"run the update without a plan", urn)
	}
	if err := validateStateMigrationContext(urn, opts, sg.deployment.prev, successors); err != nil {
		return err
	}
	if err := validateStateMigrationManagedIdentity(
		urn, serialized, callbackResult.resultResources, successors); err != nil {
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

	resultSubtree := make([]*pkgresource.State, len(callbackResult.resultResources))
	for i, res := range callbackResult.resultResources {
		state, err := opts.StateDeserializer(res)
		if err != nil {
			return fmt.Errorf("state migration for %s: deserializing returned state of %s: %w", urn, res.URN, err)
		}
		resultSubtree[i] = state
	}
	// Every successor is present in the callback result, so its physical identity can be derived from resultSubtree.
	rewrittenResultSubtree, err := rewriteStateMigrationReferences(
		resultSubtree, callbackResult.allToFinal, stateMigrationSuccessorIdentities(resultSubtree))
	if err != nil {
		return fmt.Errorf("state migration for %s: rewriting successor references: %w", urn, err)
	}
	if _, err := mapResourcesToPreparedRewrites(
		urn, resultSubtree, rewrittenResultSubtree, "returned by the migration"); err != nil {
		return err
	}
	resultSubtree = rewrittenResultSubtree
	if logging.V(9).Enabled() {
		logging.V(9).Infof("StateMigration: result state for %s:%s", urn, redactStatesForLog(resultSubtree))
	}

	transaction, err := sg.prepareStateMigrationTransaction(urn, root, priorSubtree, resultSubtree, successors)
	if err != nil {
		return err
	}
	return sg.commitStateMigration(transaction)
}

// redactStatesForLog renders resource states for debug logging with secret values masked. State migrations
// serialize prior and result state with plaintext secrets for the callback exchange, so that JSON must never
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
