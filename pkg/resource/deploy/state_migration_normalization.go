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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

// normalizeStateMigrationGoal rewrites state references in a registration goal. Aliases are resolved and rewritten
// separately.
func (d *Deployment) normalizeStateMigrationGoal(goal *pkgresource.Goal) error {
	state := &pkgresource.State{
		Inputs:               resource.ToResourcePropertyMap(goal.Properties),
		Parent:               goal.Parent,
		Dependencies:         goal.Dependencies,
		Provider:             goal.Provider,
		PropertyDependencies: goal.PropertyDependencies,
		DeletedWith:          goal.DeletedWith,
		ReplaceWith:          goal.ReplaceWith,
		ReplacementTrigger:   goal.ReplacementTrigger,
	}
	rewritten, err := d.rewriteStateMigrationState(state)
	if err != nil {
		return err
	}
	if rewritten == state {
		return nil
	}

	if !rewritten.Inputs.DeepEquals(state.Inputs) {
		goal.Properties = resource.FromResourcePropertyMap(rewritten.Inputs)
	}
	goal.Parent = rewritten.Parent
	goal.Dependencies = rewritten.Dependencies
	goal.Provider = rewritten.Provider
	goal.PropertyDependencies = rewritten.PropertyDependencies
	goal.DeletedWith = rewritten.DeletedWith
	goal.ReplaceWith = rewritten.ReplaceWith
	goal.ReplacementTrigger = rewritten.ReplacementTrigger
	return nil
}

// normalizedReadResourceEvent wraps a read event with rewritten state migration references.
type normalizedReadResourceEvent struct {
	ReadResourceEvent
	parent       resource.URN
	provider     string
	properties   resource.PropertyMap
	dependencies []resource.URN
}

func (e *normalizedReadResourceEvent) Parent() resource.URN             { return e.parent }
func (e *normalizedReadResourceEvent) Provider() string                 { return e.provider }
func (e *normalizedReadResourceEvent) Properties() resource.PropertyMap { return e.properties }
func (e *normalizedReadResourceEvent) Dependencies() []resource.URN     { return e.dependencies }

// normalizedRegisterResourceOutputsEvent wraps an outputs event with rewritten state migration references.
type normalizedRegisterResourceOutputsEvent struct {
	RegisterResourceOutputsEvent
	outputs resource.PropertyMap
}

func (e *normalizedRegisterResourceOutputsEvent) Outputs() resource.PropertyMap { return e.outputs }

// normalizeStateMigrationSourceEvent applies saved state migration rewrites to program events.
func (d *Deployment) normalizeStateMigrationSourceEvent(event SourceEvent) (SourceEvent, error) {
	switch e := event.(type) {
	case ContinueResourceImportEvent,
		ContinueResourceRefreshEvent,
		ContinueExtensionEvent,
		ContinueResourceDiffEvent:
		// Continuations contain engine-held state that was already normalized.
		return event, nil
	case RegisterResourceEvent:
		// Registrations are normalized later so migrations they carry can be applied first.
		return event, nil
	case ReadResourceEvent:
		state := &pkgresource.State{
			Inputs:       e.Properties(),
			Parent:       e.Parent(),
			Dependencies: e.Dependencies(),
			Provider:     e.Provider(),
		}
		rewritten, err := d.rewriteStateMigrationState(state)
		if err != nil {
			return nil, fmt.Errorf("normalizing read %s after state migration: %w", e.Name(), err)
		}
		if rewritten == state {
			return event, nil
		}
		return &normalizedReadResourceEvent{
			ReadResourceEvent: e,
			parent:            rewritten.Parent,
			provider:          rewritten.Provider,
			properties:        rewritten.Inputs,
			dependencies:      rewritten.Dependencies,
		}, nil
	case RegisterResourceOutputsEvent:
		state := &pkgresource.State{Outputs: e.Outputs()}
		rewritten, err := d.rewriteStateMigrationState(state)
		if err != nil {
			return nil, fmt.Errorf("normalizing outputs for %s after state migration: %w", e.URN(), err)
		}
		if rewritten == state {
			return event, nil
		}
		return &normalizedRegisterResourceOutputsEvent{
			RegisterResourceOutputsEvent: e,
			outputs:                      rewritten.Outputs,
		}, nil
	default:
		contract.Failf("unrecognized source event type %T", event)
		return nil, nil
	}
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
