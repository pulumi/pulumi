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
	"log/slog"
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// commitStateMigration persists a prepared transaction and then commits it to Deployment.prev, which supplies old
// state for subsequent lookup and diffing.
func (sg *stepGenerator) commitStateMigration(transaction *StateMigrationTransaction) error {
	d := sg.deployment
	urn := transaction.RootURN

	// Notify the snapshot manager before Deployment.prev is mutated: it resolves the removed states against the
	// current prior snapshot.
	migrationEvents, ok := d.events.(StateMigrationEvents)
	if !ok {
		return fmt.Errorf("state migration for %s: %w", urn, ErrStateMigrationsUnsupported)
	}
	if err := migrationEvents.OnStateMigration(transaction); err != nil {
		return fmt.Errorf("state migration for %s: %w", urn, err)
	}

	for before, after := range transaction.currentResourceRewrites {
		before.Lock.Lock()
		applyStateMigrationReferenceRewrite(before, after)
		before.Lock.Unlock()
	}

	// Point of no return: replace Deployment.prev.Resources while preserving retained-resource pointers.
	// Steps may have been generated before this migration and still identify their old state by pointer; replacing
	// retained states with rewrite copies would make snapshot-manager bookkeeping unable to find them.
	committedResources := slices.Clone(transaction.PreparedPriorResources)
	for before, after := range transaction.RetainedResourceRewrites {
		before.Lock.Lock()
		applyStateMigrationReferenceRewrite(before, after)
		before.Lock.Unlock()
		committedResources[transaction.retainedRewriteIndices[before]] = before
	}
	d.prev.Resources = committedResources

	oldResources, hasRefreshBeforeUpdateResources, olds, allOlds, oldViews, err := buildResourceMaps(d.prev)
	contract.AssertNoErrorf(err, "state migration for %s produced duplicate resources after verification", urn)
	d.hasRefreshBeforeUpdateResources = hasRefreshBeforeUpdateResources
	d.depGraph = graph.NewDependencyGraph(oldResources)
	d.olds = olds
	d.allOlds = allOlds
	d.oldViews = oldViews

	// If the migration has successors, publish its rewrite rule only after the prepared transaction is fully
	// committed. Step execution and outputs registration are paused for the duration of applyStateMigrations, so
	// later producers either precede this transaction and were rewritten above, or observe the completed rule and
	// normalize through it. A transaction without successors has no references to rewrite in later state.
	if len(transaction.SuccessorURNs) != 0 {
		d.stateMigrationRewritesM.Lock()
		d.stateMigrationRewrites = append(d.stateMigrationRewrites,
			newStateMigrationRewrite(transaction.RootURN, transaction.SuccessorURNs, transaction.ResultSubtree))
		d.stateMigrationRewritesM.Unlock()
	}

	priorStateURNs := make([]resource.URN, len(transaction.PriorSubtree))
	for i, state := range transaction.PriorSubtree {
		priorStateURNs[i] = state.URN
	}
	resultStateURNs := make([]resource.URN, len(transaction.ResultSubtree))
	for i, state := range transaction.ResultSubtree {
		resultStateURNs[i] = state.URN
	}

	// Keep this always-on audit record structural. The callback JSON contains plaintext secrets, and even typed
	// property logging would retain unredacted values in the encrypted and OTLP sinks.
	slog.Info("state migration applied",
		"rootURN", urn,
		"preview", d.opts != nil && d.opts.DryRun,
		"priorResourceCount", len(transaction.PriorSubtree),
		"resultResourceCount", len(transaction.ResultSubtree),
		"priorStateURNs", priorStateURNs,
		"resultStateURNs", resultStateURNs,
		"successors", transaction.SuccessorURNs)
	return nil
}
