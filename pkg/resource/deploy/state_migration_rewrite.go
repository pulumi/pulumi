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
	"reflect"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type stateMigrationTarget struct {
	custom bool
	id     resource.ID
}

// stateMigrationRewrite is the small, immutable part of a committed transaction needed to normalize resources that
// are produced later in the update. Keeping it separate avoids retaining a complete snapshot per migration.
type stateMigrationRewrite struct {
	rootURN       resource.URN
	successorURNs map[resource.URN]resource.URN
	targets       map[resource.URN]stateMigrationTarget
}

func stateMigrationTargets(states []*pkgresource.State) map[resource.URN]stateMigrationTarget {
	targets := make(map[resource.URN]stateMigrationTarget, len(states))
	for _, state := range states {
		targets[state.URN] = stateMigrationTarget{custom: state.Custom, id: state.ID}
	}
	return targets
}

func newStateMigrationRewrite(
	rootURN resource.URN,
	successors map[resource.URN]resource.URN,
	targets []*pkgresource.State,
) *stateMigrationRewrite {
	successorURNs := make(map[resource.URN]resource.URN, len(successors))
	for source, target := range successors {
		successorURNs[source] = target
	}
	return &stateMigrationRewrite{
		rootURN:       rootURN,
		successorURNs: successorURNs,
		targets:       stateMigrationTargets(targets),
	}
}

// RewriteResources rewrites references in states to the transaction's canonical successors. Snapshot managers use
// this while preparing exact patches for states produced earlier in the update.
func (plan *StateMigrationPlan) RewriteResources(states []*pkgresource.State) ([]*pkgresource.State, error) {
	return rewriteStateMigrationReferencesWithTargets(
		states, stateMigrationTargets(plan.MigratedResources), plan.SuccessorURNs)
}

// RewriteResourcesInPlace rewrites states while preserving their pointer and lock identities.
func (plan *StateMigrationPlan) RewriteResourcesInPlace(states []*pkgresource.State) error {
	rewritten, err := plan.RewriteResources(states)
	if err != nil {
		return err
	}
	for i, state := range states {
		if rewritten[i] == state {
			continue
		}
		state.Lock.Lock()
		applyStateMigrationReferenceRewrite(state, rewritten[i])
		state.Lock.Unlock()
	}
	return nil
}

// applyStateMigrationReferenceRewrite copies precisely the fields changed by reference rewriting. The caller must
// synchronize access to state.
func applyStateMigrationReferenceRewrite(state, fixed *pkgresource.State) {
	state.Parent = fixed.Parent
	state.Dependencies = fixed.Dependencies
	state.PropertyDependencies = fixed.PropertyDependencies
	state.DeletedWith = fixed.DeletedWith
	state.ReplaceWith = fixed.ReplaceWith
	state.ViewOf = fixed.ViewOf
	state.Provider = fixed.Provider
	state.Inputs = fixed.Inputs
	state.Outputs = fixed.Outputs
	state.ReplacementTrigger = fixed.ReplacementTrigger
}

// rewriteStateMigrationReferences returns independent copies of states with every reference to a removed URN
// rewritten to its final successor. This includes structural dependencies and resource references nested in property
// values. Multiple sources may resolve to the same target; dependency lists are deduplicated in that case.
func rewriteStateMigrationReferences(
	states []*pkgresource.State, successors map[resource.URN]resource.URN,
) ([]*pkgresource.State, error) {
	return rewriteStateMigrationReferencesWithTargets(states, stateMigrationTargets(states), successors)
}

func rewriteStateMigrationReferencesWithTargets(
	states []*pkgresource.State,
	targets map[resource.URN]stateMigrationTarget,
	successors map[resource.URN]resource.URN,
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
				ref.PackageVersion = ""
				if target, ok := targets[fixed]; ok {
					if target.custom {
						ref.ID = resource.NewProperty(string(target.id))
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
			originalProviderURN := ref.URN()
			providerURN, err := fixURN(originalProviderURN)
			if err != nil {
				return nil, err
			}
			providerID := ref.ID()
			if providerURN != originalProviderURN {
				if provider, ok := targets[providerURN]; ok {
					providerID = provider.id
				}
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
