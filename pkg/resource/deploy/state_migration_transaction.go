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

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// prepareStateMigrationTransaction validates a migration result and prepares every rewrite that must be persisted.
func (sg *stepGenerator) prepareStateMigrationTransaction(
	registrationURN resource.URN, priorRoot *pkgresource.State, priorSubtree, resultSubtree []*pkgresource.State,
	successors map[resource.URN]resource.URN,
) (*StateMigrationTransaction, error) {
	if err := sg.validateStateMigrationResult(
		registrationURN, priorRoot, priorSubtree, resultSubtree,
	); err != nil {
		return nil, err
	}
	d := sg.deployment
	priorSubtreeSet := make(map[*pkgresource.State]bool, len(priorSubtree))
	for _, state := range priorSubtree {
		priorSubtreeSet[state] = true
	}
	lastPriorResource := priorSubtree[len(priorSubtree)-1]

	retainedResources := make([]*pkgresource.State, 0, len(d.prev.Resources)-len(priorSubtree))
	for _, state := range d.prev.Resources {
		if !priorSubtreeSet[state] {
			retainedResources = append(retainedResources, state)
		}
	}

	// Resources outside PriorSubtree remain in the snapshot, but their references to removed subtree resources must
	// follow those resources to their final successors.
	rewrittenRetainedResources, err := rewriteStateMigrationReferences(
		retainedResources, successors, stateMigrationSuccessorIdentities(resultSubtree))
	if err != nil {
		return nil, fmt.Errorf("state migration for %s: rewriting successor references: %w", registrationURN, err)
	}
	retainedRewrites, err := mapResourcesToPreparedRewrites(
		registrationURN, retainedResources, rewrittenRetainedResources, "retained from prior state")
	if err != nil {
		return nil, err
	}

	preparedRetained := make(map[*pkgresource.State]*pkgresource.State, len(retainedResources))
	for i, state := range retainedResources {
		preparedRetained[state] = rewrittenRetainedResources[i]
	}

	// Start with the replacement at the last prior resource, which is after everything the prior subtree depended on.
	// We toposort below to move successors ahead of interleaved resources that now depend on them.
	preparedPriorResources := make(
		[]*pkgresource.State, 0, len(d.prev.Resources)-len(priorSubtree)+len(resultSubtree))
	for _, state := range d.prev.Resources {
		if priorSubtreeSet[state] {
			if state == lastPriorResource {
				preparedPriorResources = append(preparedPriorResources, resultSubtree...)
			}
			continue
		}
		preparedPriorResources = append(preparedPriorResources, preparedRetained[state])
	}

	verifySnap := &Snapshot{Manifest: d.prev.Manifest, Resources: preparedPriorResources}
	if err := verifySnap.Toposort(); err != nil {
		return nil, fmt.Errorf(
			"state migration for %s produced an invalid state; no changes were made: %w", registrationURN, err)
	}
	if err := verifySnap.VerifyIntegrity(); err != nil {
		return nil, fmt.Errorf(
			"state migration for %s produced an invalid state; no changes were made: %w", registrationURN, err)
	}
	preparedPriorResources = verifySnap.Resources

	// Remember where each rewritten retained resource landed so commit can restore its live pointer.
	preparedIndices := make(map[*pkgresource.State]int, len(preparedPriorResources))
	for i, state := range preparedPriorResources {
		preparedIndices[state] = i
	}
	retainedRewriteIndices := make(map[*pkgresource.State]int, len(retainedRewrites))
	for original, prepared := range retainedRewrites {
		index, ok := preparedIndices[prepared]
		contract.Assertf(ok, "state migration for %s lost a rewritten retained resource", registrationURN)
		retainedRewriteIndices[original] = index
	}

	transaction := &StateMigrationTransaction{
		RootURN:                  registrationURN,
		PriorSubtree:             priorSubtree,
		ResultSubtree:            resultSubtree,
		SuccessorURNs:            successors,
		PreparedPriorResources:   preparedPriorResources,
		RetainedResourceRewrites: retainedRewrites,
		retainedRewriteIndices:   retainedRewriteIndices,
	}

	// Deployment.news and Deployment.reads may already contain live states that reference migrated resources.
	currentResourceSet := make(map[*pkgresource.State]struct{})
	if d.news != nil {
		d.news.Range(func(_ resource.URN, state *pkgresource.State) bool {
			currentResourceSet[state] = struct{}{}
			return true
		})
	}
	if d.reads != nil {
		d.reads.Range(func(_ resource.URN, state *pkgresource.State) bool {
			currentResourceSet[state] = struct{}{}
			return true
		})
	}
	currentResources := make([]*pkgresource.State, 0, len(currentResourceSet))
	for state := range currentResourceSet {
		currentResources = append(currentResources, state)
	}
	rewrittenCurrentResources, err := transaction.RewriteResources(currentResources)
	if err != nil {
		return nil, fmt.Errorf("state migration for %s: rewriting current resources: %w", registrationURN, err)
	}
	transaction.currentResourceRewrites, err = mapResourcesToPreparedRewrites(
		registrationURN, currentResources, rewrittenCurrentResources, "created or updated earlier in this deployment")
	if err != nil {
		return nil, err
	}
	return transaction, nil
}

// mapResourcesToPreparedRewrites maps each live resource whose references changed to its prepared rewritten state.
// Provider states must remain unchanged and are rejected if normalization produced a rewrite for one.
func mapResourcesToPreparedRewrites(
	urn resource.URN, before, after []*pkgresource.State, origin string,
) (map[*pkgresource.State]*pkgresource.State, error) {
	contract.Assertf(len(before) == len(after), "state migration reference rewrite changed resource count")
	rewrites := make(map[*pkgresource.State]*pkgresource.State)
	for i, state := range before {
		if after[i] == state {
			continue
		}
		if sdkproviders.IsProviderType(state.Type) {
			return nil, fmt.Errorf("state migration for %s rewrites references in provider state %s %s; "+
				"provider states must remain unchanged", urn, state.URN, origin)
		}
		rewrites[state] = after[i]
	}
	return rewrites, nil
}
