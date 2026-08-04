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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// rewriteStateMigrationState returns state unchanged if no migration rewrite applies, or a prepared rewritten copy.
func (d *Deployment) rewriteStateMigrationState(state *pkgresource.State) (*pkgresource.State, error) {
	if state == nil {
		return nil, nil
	}

	d.stateMigrationRewritesM.RLock()
	defer d.stateMigrationRewritesM.RUnlock()

	rewritten := state
	var err error
	for _, rewrite := range d.stateMigrationRewrites {
		rewritten, err = rewrite.applyToResource(rewritten)
		if err != nil {
			return nil, fmt.Errorf("rewriting state for migration of %s: %w", rewrite.rootURN, err)
		}
	}
	return rewritten, nil
}

func (d *Deployment) rewriteStateMigrationURN(urn resource.URN) resource.URN {
	if urn == "" {
		return ""
	}

	d.stateMigrationRewritesM.RLock()
	defer d.stateMigrationRewritesM.RUnlock()

	rewritten := urn
	for _, rewrite := range d.stateMigrationRewrites {
		rewritten = rewriteStateMigrationSuccessor(rewritten, rewrite.successorURNs)
	}
	return rewritten
}

// rejectStateMigrationPredecessorURN rejects a primary resource URN that was removed by a state migration committed
// earlier in this deployment. References to such a URN are normalized to its successor, so allowing a later
// registration or read to reuse it would make those references ambiguous.
//
// Aliases are intentionally not checked here. Naming a predecessor as an alias is how a program claims its migrated
// successor under a new primary URN.
func (d *Deployment) rejectStateMigrationPredecessorURN(urn resource.URN) error {
	if urn == "" {
		return nil
	}

	d.stateMigrationRewritesM.RLock()
	defer d.stateMigrationRewritesM.RUnlock()

	successor := urn
	var migrationRoot resource.URN
	for _, rewrite := range d.stateMigrationRewrites {
		if next, ok := rewrite.successorURNs[successor]; ok {
			if migrationRoot == "" {
				migrationRoot = rewrite.rootURN
			}
			successor = next
		}
	}
	if migrationRoot == "" {
		return nil
	}

	return fmt.Errorf("resource %s cannot be registered or read because state migration for %s replaced it with %s "+
		"earlier in this deployment", urn, migrationRoot, successor)
}

// rewriteStateMigrationStateInPlace preserves state's pointer and lock while applying migration rewrites.
func (d *Deployment) rewriteStateMigrationStateInPlace(state *pkgresource.State) error {
	if state == nil {
		return nil
	}
	state.Lock.Lock()
	defer state.Lock.Unlock()
	return d.rewriteStateMigrationStateInPlaceLocked(state)
}

func (d *Deployment) rewriteStateMigrationStateInPlaceLocked(state *pkgresource.State) error {
	rewritten, err := d.rewriteStateMigrationState(state)
	if err != nil {
		return err
	}
	if rewritten == state {
		return nil
	}

	applyStateMigrationReferenceRewrite(state, rewritten)
	return nil
}

func (d *Deployment) rewriteStateMigrationStep(step Step) error {
	if old := step.Old(); old != nil {
		if err := d.rewriteStateMigrationStateInPlace(old); err != nil {
			return fmt.Errorf("normalizing old state for %s after state migration: %w", step.URN(), err)
		}
	}
	if newState := step.New(); newState != nil && newState != step.Old() {
		if err := d.rewriteStateMigrationStateInPlace(newState); err != nil {
			return fmt.Errorf("normalizing new state for %s after state migration: %w", step.URN(), err)
		}
	}
	return nil
}
