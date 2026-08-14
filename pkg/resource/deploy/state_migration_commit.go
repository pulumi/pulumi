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
	"slices"

	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// commitStateMigration persists a prepared transaction and then commits it to Deployment.prev
func (sg *stepGenerator) commitStateMigration(
	ctx context.Context, transaction *StateMigrationTransaction,
) error {
	d := sg.deployment
	urn := transaction.RootURN

	// Persist the prepared migration before committing it to Deployment.prev.
	if err := d.events.OnStateMigration(transaction); err != nil {
		return fmt.Errorf("state migration for %s: %w", urn, err)
	}

	// Rewrite references in states already recorded in Deployment.news or Deployment.reads, preserving their pointers.
	for before, after := range transaction.currentResourceRewrites {
		before.Lock.Lock()
		applyStateMigrationReferenceRewrite(before, after)
		before.Lock.Unlock()
	}

	// Replace Deployment.prev.Resources while preserving retained-resource pointers.
	committedResources := slices.Clone(transaction.PreparedPriorResources)
	for before, after := range transaction.RetainedResourceRewrites {
		before.Lock.Lock()
		applyStateMigrationReferenceRewrite(before, after)
		before.Lock.Unlock()
		committedResources[transaction.retainedRewriteIndices[before]] = before
	}
	d.prev.Resources = committedResources

	// Rebuild the deployment indexes derived from the now-migrated prior snapshot.
	oldResources, hasRefreshBeforeUpdateResources, olds, allOlds, oldViews, err := buildResourceMaps(d.prev)
	contract.AssertNoErrorf(err, "state migration for %s produced duplicate resources after verification", urn)
	d.hasRefreshBeforeUpdateResources = hasRefreshBeforeUpdateResources
	d.depGraph = graph.NewDependencyGraph(oldResources)
	d.olds = olds
	d.allOlds = allOlds
	d.oldViews = oldViews

	// Add a stateMigrationRewrite so resource states produced later in the update use successor URNs instead of
	// predecessor URNs.
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

	slog.InfoContext(ctx, "state migration applied",
		"rootURN", urn,
		"preview", d.opts != nil && d.opts.DryRun,
		"priorResourceCount", len(transaction.PriorSubtree),
		"resultResourceCount", len(transaction.ResultSubtree),
		"priorStateURNs", priorStateURNs,
		"resultStateURNs", resultStateURNs,
		"successors", transaction.SuccessorURNs)
	return nil
}
