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
	"log/slog"
	"strings"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// applyStateMigrations runs the state migrations attached to a resource registration against the prior state of
// the resource and all prior resources transitively parented to it, and splices the transformed states back into
// the deployment's view of the prior state before any diffing takes place for those resources.
//
// Every resource removed from state must name a returned successor, so a migration cannot silently leave a physical
// resource unmanaged.
//
// Migrations run against the engine's in-memory view of the prior state only and issue no provider calls. A callback
// may be evaluated while generating an update plan, but a state-changing result is rejected because plans cannot
// describe checkpoint rewrites.
func (sg *stepGenerator) applyStateMigrations(
	ctx context.Context, event RegisterResourceEvent, urn resource.URN,
) error {
	migrations := event.StateMigrations()
	if len(migrations) == 0 {
		return nil
	}

	// State migrations only run during normal updates. Destroy with `--run-program` evaluates the program for current
	// provider configuration and hooks, but still deletes resources from the recorded prior state. Refresh similarly
	// owns reconciliation of prior state and must not have it rewritten underneath it.
	if sg.mode != updateMode {
		slog.DebugContext(ctx, "skipping state migrations outside update mode", "urn", urn)
		return nil
	}

	opts := sg.deployment.opts

	// Resolve the prior state of the registering resource: by URN first and then by any aliases. If there is no prior
	// state this is a new resource and there is nothing to migrate. Do this before pausing the executor so attaching a
	// migration callback to a newly created resource does not wait for unrelated work.
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
		slog.DebugContext(ctx, "skipping state migrations without prior state", "urn", urn)
		return nil
	}

	contract.Assertf(opts.StateMigrationSerializer != nil,
		"state migration serializer must be configured for %s", urn)

	// The deployment executor waits for earlier planning to finish before handling this registration, so all earlier
	// step chains have been submitted. Pause execution until those steps finish, and keep it paused while the migration
	// updates deployment state.
	contract.Assertf(sg.stepExecutionBarrier != nil,
		"step execution barrier must be configured for state migration of %s", urn)
	sg.stepExecutionBarrier.Lock()
	defer sg.stepExecutionBarrier.Unlock()

	// Collect the prior state of the resource and all resources transitively parented to it, in snapshot order.
	//
	// Do not pass resources that are already pending deletion to the callback. The update still needs to finish deleting
	// them, so we must not combine that unfinished work with a state rewrite. Keep them separately and reject the
	// migration below only if it changes state. A no-op migration can continue so the update can finish deleting them.
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

	// Reject migrating prior state that an earlier registration in this deployment already claimed.
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
		if state == root && state.URN == urn {
			// generateURN recorded the current registration before applyStateMigrations was called
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
		res, err := opts.StateMigrationSerializer.Serialize(ctx, state)
		if err != nil {
			return fmt.Errorf("state migration for %s: serializing state of %s: %w", urn, state.URN, err)
		}
		serialized[i] = res
	}
	for _, state := range priorSubtree {
		logStateMigrationResource(ctx, "state migration prior resource", urn, state)
	}

	callbackResult, err := runStateMigrationCallbacks(ctx, urn, migrations, serialized)
	if err != nil {
		return err
	}
	if callbackResult == nil {
		slog.DebugContext(ctx, "state migrations made no changes", "urn", urn)
		return nil
	}
	successors := callbackResult.originalToFinal

	// Update plans constrain resource operations, they do not describe or authorize checkpoint rewrites. Reject
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

	// The migrations changed the state, so reject the change if the subtree contains resources that are pending
	// deletion. Migrations that make no changes already returned above, allowing the update to reap pending deletions
	// normally.
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
		state, err := opts.StateMigrationSerializer.Deserialize(res)
		if err != nil {
			return fmt.Errorf("state migration for %s: deserializing returned state of %s: %w", urn, res.URN, err)
		}
		resultSubtree[i] = state
	}

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
	for _, state := range resultSubtree {
		logStateMigrationResource(ctx, "state migration result resource", urn, state)
	}

	transaction, err := sg.prepareStateMigrationTransaction(urn, root, priorSubtree, resultSubtree, successors)
	if err != nil {
		return err
	}
	return sg.commitStateMigration(ctx, transaction)
}

func logStateMigrationResource(
	ctx context.Context, message string, rootURN resource.URN, state *pkgresource.State,
) {
	// PropertyMap's logging integration redacts secret values from plaintext logs while retaining structured values
	// for encrypted logs and OTLP export.
	slog.DebugContext(ctx, message,
		"rootURN", rootURN,
		"resourceURN", state.URN,
		"resourceType", state.Type,
		"resourceID", state.ID,
		"parentURN", state.Parent,
		"inputs", state.Inputs,
		"outputs", state.Outputs)
}
