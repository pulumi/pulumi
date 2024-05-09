// Copyright 2016-2022, Pulumi Corporation.
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
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Snapshot is a view of a collection of resources in an stack at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot struct {
	Manifest          Manifest             // a deployment manifest of versions, checksums, and so on.
	SecretsManager    secrets.Manager      // the manager to use use when serializing this snapshot.
	Resources         []*resource.State    // fetches all resources and their associated states.
	PendingOperations []resource.Operation // all currently pending resource operations.
}

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
// This property is not checked; for verification, please refer to the VerifyIntegrity function below.
func NewSnapshot(manifest Manifest, secretsManager secrets.Manager,
	resources []*resource.State, ops []resource.Operation,
) *Snapshot {
	return &Snapshot{
		Manifest:          manifest,
		SecretsManager:    secretsManager,
		Resources:         resources,
		PendingOperations: ops,
	}
}

// NormalizeURNReferences fixes up all URN references in a snapshot to use the new URNs instead of potentially-aliased
// URNs.  This will affect resources that are "old", and which would be expected to be updated to refer to the new names
// later in the deployment.  But until they are, we still want to ensure that any serialization of the snapshot uses URN
// references which do not need to be indirected through any alias lookups, and which instead refer directly to the URN
// of a resource in the resources map.
//
// Note: This method does not modify the snapshot (and resource.States
// in the snapshot) in-place, but returns an independent structure,
// with minimal copying necessary.
func (snap *Snapshot) NormalizeURNReferences() (*Snapshot, error) {
	if snap == nil {
		return nil, nil
	}

	aliased := make(map[resource.URN]resource.URN)
	for _, state := range snap.Resources {
		// Add to aliased maps
		for _, alias := range state.Aliases {
			// For ease of implementation, some SDKs may end up creating the same alias to the
			// same resource multiple times.  That's fine, only error if we see the same alias,
			// but it maps to *different* resources.
			if otherUrn, has := aliased[alias]; has && otherUrn != state.URN {
				return nil, fmt.Errorf("Two resources ('%s' and '%s') aliased to the same: '%s'", otherUrn, state.URN, alias)
			}
			aliased[alias] = state.URN
		}
		// If our parent has changed URN, then we need to update our URN as well.
		if parent, has := aliased[state.Parent]; has {
			if parent != "" && parent.QualifiedType() != resource.RootStackType {
				aliased[state.URN] = resource.NewURN(
					state.URN.Stack(), state.URN.Project(),
					parent.QualifiedType(), state.URN.Type(),
					state.URN.Name())
			}
		}
	}

	fixUrn := func(urn resource.URN) resource.URN {
		if newUrn, has := aliased[urn]; has {
			// TODO should this recur to see if newUrn is similarly aliased?
			return newUrn
		}
		return urn
	}

	fixProvider := func(provider string) string {
		ref, err := providers.ParseReference(provider)
		contract.AssertNoErrorf(err, "malformed provider reference: %s", provider)
		newURN := fixUrn(ref.URN())
		ref, err = providers.NewReference(newURN, ref.ID())
		contract.AssertNoErrorf(err, "could not create provider reference with URN %s and ID %s", newURN, ref.ID())
		return ref.String()
	}

	fixResource := func(old *resource.State) *resource.State {
		old.Lock.Lock()
		defer old.Lock.Unlock()

		return newStateBuilder(old).
			withUpdatedURN(fixUrn).
			withUpdatedParent(fixUrn).
			withUpdatedDependencies(fixUrn).
			withUpdatedPropertyDependencies(fixUrn).
			withUpdatedProvider(fixProvider).
			withUpdatedAliases().
			build()
	}

	return snap.withUpdatedResources(fixResource), nil
}

// VerifyIntegrity checks a snapshot to ensure it is well-formed.  Because of the cost of this operation,
// integrity verification is only performed on demand, and not automatically during snapshot construction.
//
// This function verifies a number of invariants:
//  1. Provider resources must be referenceable (i.e. they must have a valid URN and ID)
//  2. A resource's provider must precede the resource in the resource list
//  3. Parents must precede children in the resource list
//  4. Dependents must precede their dependencies in the resource list
//  5. For every URN in the snapshot, there must be at most one resource with that URN that is not pending deletion
//  6. The magic manifest number should change every time the snapshot is mutated
//
// N.B. Constraints 2 does NOT apply for resources that are pending deletion. This is because they may have
// had their provider replaced but not yet be replaced themselves yet (due to a partial update). Pending
// replacement resources also can't just be wholly removed from the snapshot because they may have dependents
// that are not being replaced and thus would fail validation if the pending replacement resource was removed
// and not re-created (again due to partial updates).
func (snap *Snapshot) VerifyIntegrity() error {
	if snap != nil {
		// Ensure the magic cookie checks out.
		if snap.Manifest.Magic != snap.Manifest.NewMagic() {
			return errors.New("magic cookie mismatch; possible tampering/corruption detected")
		}

		// Now check the resources.  For now, we just verify that parents come before children, and that there aren't
		// any duplicate URNs.
		urns := make(map[resource.URN]*resource.State)
		provs := make(map[providers.Reference]struct{})
		for i, state := range snap.Resources {
			urn := state.URN

			if providers.IsProviderType(state.Type) {
				ref, err := providers.NewReference(urn, state.ID)
				if err != nil {
					return fmt.Errorf("provider %s is not referenceable: %w", urn, err)
				}
				provs[ref] = struct{}{}
			}
			if provider := state.Provider; provider != "" {
				ref, err := providers.ParseReference(provider)
				if err != nil {
					return fmt.Errorf("failed to parse provider reference for resource %s: %w", urn, err)
				}
				if _, has := provs[ref]; !has && !state.PendingReplacement {
					return fmt.Errorf("resource %s refers to unknown provider %s", urn, ref)
				}
			}

			// For each resource, we'll ensure that all its dependencies are declared
			// before it in the snapshot. In this case, "dependencies" includes the
			// Dependencies field, as well as the resource's Parent (if it has one),
			// any PropertyDependencies, and the DeletedWith field.
			//
			// If a dependency is missing, we'll return an error. In such cases, we'll
			// walk through the remaining resources in the snapshot to see if the
			// missing dependency is declared later in the snapshot or whether it is
			// missing entirely, producing a specific error message depending on the
			// outcome.

			// Parent dependencies
			if par := state.Parent; par != "" {
				if _, has := urns[par]; !has {
					for _, other := range snap.Resources[i+1:] {
						if other.URN == par {
							return fmt.Errorf("child resource %s's parent %s comes after it", urn, par)
						}
					}
					return fmt.Errorf("child resource %s refers to missing parent %s", urn, par)
				}

				// Ensure that our URN is a child of the parent's URN.
				expectedType := urn.Type()
				if par.QualifiedType() != resource.RootStackType {
					expectedType = par.QualifiedType() + "$" + expectedType
				}

				if urn.QualifiedType() != expectedType {
					logging.Warningf("child resource %s has parent %s but it's URN doesn't match", urn, par)
					// TODO: Change this to an error once we're sure users won't hit this in the wild.
					// return fmt.Errorf("child resource %s has parent %s but it's URN doesn't match", urn, par)
				}
			}

			// Dependencies
			for _, dep := range state.Dependencies {
				if _, has := urns[dep]; !has {
					for _, other := range snap.Resources[i+1:] {
						if other.URN == dep {
							return fmt.Errorf("resource %s's dependency %s comes after it", urn, other.URN)
						}
					}

					return fmt.Errorf("resource %s's dependency %s refers to missing resource", urn, dep)
				}
			}

			// Property dependencies
			for prop, deps := range state.PropertyDependencies {
				for _, dep := range deps {
					if _, has := urns[dep]; !has {
						for _, other := range snap.Resources[i+1:] {
							if other.URN == dep {
								return fmt.Errorf(
									"resource %s's property dependency %s (from property %s) comes after it",
									urn, other.URN, prop,
								)
							}
						}

						return fmt.Errorf(
							"resource %s's property dependency %s (from property %s) refers to missing resource",
							urn, dep, prop,
						)
					}
				}
			}

			// DeletedWith
			if dw := state.DeletedWith; dw != "" {
				if _, has := urns[dw]; !has {
					for _, other := range snap.Resources[i+1:] {
						if other.URN == dw {
							return fmt.Errorf("resource %s is specified as being deleted with %s, which comes after it", urn, other.URN)
						}
					}

					return fmt.Errorf("resource %s is specified as being deleted with %s, which is missing", urn, dw)
				}
			}

			if _, has := urns[urn]; has && !state.Delete {
				// The only time we should have duplicate URNs is when all but one of them are marked for deletion.
				return fmt.Errorf("duplicate resource %s (not marked for deletion)", urn)
			}

			urns[urn] = state
		}
	}

	return nil
}

// Applies a non-mutating modification for every resource.State in the
// Snapshot, returns the edited Snapshot.
func (snap *Snapshot) withUpdatedResources(update func(*resource.State) *resource.State) *Snapshot {
	old := snap.Resources
	new := []*resource.State{}
	edited := false
	for _, s := range old {
		n := update(s)
		if n != s {
			edited = true
		}
		new = append(new, n)
	}
	if !edited {
		return snap
	}
	newSnap := *snap // shallow copy
	newSnap.Resources = new
	return &newSnap
}
