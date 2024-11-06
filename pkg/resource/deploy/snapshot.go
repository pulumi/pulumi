// Copyright 2016-2024, Pulumi Corporation.
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
	"runtime/debug"

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
	Metadata          SnapshotMetadata     // metadata associated with the snapshot.
}

// SnapshotMetadata contains metadata about a snapshot.
type SnapshotMetadata struct {
	// Metadata associated with any integrity error affecting the snapshot.
	IntegrityErrorMetadata *SnapshotIntegrityErrorMetadata
}

// SnapshotIntegrityErrorMetadata contains metadata about a snapshot integrity error, such as the version
// and invocation of the Pulumi engine that caused it.
type SnapshotIntegrityErrorMetadata struct {
	// The version of the Pulumi engine that caused the integrity error.
	Version string
	// The command/invocation of the Pulumi engine that caused the integrity error.
	Command string
	// The error message associated with the integrity error.
	Error string
}

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
// This property is not checked; for verification, please refer to the VerifyIntegrity function below.
func NewSnapshot(manifest Manifest, secretsManager secrets.Manager,
	resources []*resource.State, ops []resource.Operation,
	metadata SnapshotMetadata,
) *Snapshot {
	return &Snapshot{
		Manifest:          manifest,
		SecretsManager:    secretsManager,
		Resources:         resources,
		PendingOperations: ops,
		Metadata:          metadata,
	}
}

// Prune removes all dangling dependencies from this snapshot, *which is assumed to be topologically sorted with respect
// to dependencies*. A dangling dependency is one which points a resource which is not present in the snapshot. An
// absence of dangling resources is a necessary but not sufficient condition for a snapshot to be valid; the
// VerifyIntegrity method should be used to ensure that a snapshot is well-formed.
//
// Prune returns a list of PruneResults, each of which describes the changes made to a resource in the snapshot. These
// changes include any URN rewriting that was necessary to remove dangling parent dependencies, as well as the set of
// dependencies that were removed.
func (snap *Snapshot) Prune() []PruneResult {
	results := []PruneResult{}

	// As we go through the set of resources, we'll maintain a mapping from old URNs to new URNs. This lets us use one map
	// to keep track of both resources we've seen and resources we've rewritten (e.g. if parent-child relationships
	// changed).
	//
	// NOTE: We shouldn't have to worry about duplicate URNs here. These can occur when a resource is being replaced and
	// both its old and new state are present in the snapshot. In these cases, the old states will be at the end of the
	// snapshot with their Delete flag set. Old states can only depend on old states (since the new states didn't exist
	// when they were created). Thus, by the time we "overwrite" entries in the map, we will only be dealing with old
	// states, and so will have no need to refer to the clobbered entries.
	seen := map[resource.URN]resource.URN{}

	for _, state := range snap.Resources {
		var removedDeps []resource.StateDependency

		func() {
			// Since we're potentially modifying the state, we'll need to lock it.
			state.Lock.Lock()
			defer state.Lock.Unlock()

			newURN := state.URN

			newDeps := []resource.URN{}
			newPropDeps := map[resource.PropertyKey][]resource.URN{}

			// If a provider reference is dangling, there's not much we can do -- resource states *must* have providers, so we
			// can't simply remove it. Better to leave it so that VerifyIntegrity can spot it and present an appropriate error
			// message.
			_, allDeps := state.GetAllDependencies()
			for _, dep := range allDeps {
				switch dep.Type {
				case resource.ResourceParent:
					// Since parent-child relationships affect URNs, we have more work to do for a parent dependency. If our parent
					// is missing, we'll clear the reference and update our URN to remove the parent type. Moreover, we'll record
					// the fact that we rewrote our URN so that any of our children can update their URNs appropriately.
					//
					// If our parent is present, but was rewritten, we'll need to rewrite our URN and record that it was rewritten
					// for our children, and so on.
					//
					// Note: the precondition that the snapshot is topologically sorted allows us to assume that our parent's
					// presence/rewriting has already been determined.
					newParentURN, has := seen[dep.URN]
					if !has {
						newURN = resource.NewURN(state.URN.Stack(), state.URN.Project(), "", state.URN.Type(), state.URN.Name())
						state.Parent = ""
						removedDeps = append(removedDeps, dep)
					} else {
						newURN = resource.NewURN(
							state.URN.Stack(),
							state.URN.Project(),
							newParentURN.QualifiedType(),
							state.URN.Type(),
							state.URN.Name(),
						)
						state.Parent = newParentURN
					}
				case resource.ResourceDependency:
					// For dependencies, only preserve those that aren't dangling, taking into account any rewrites that may have
					// occurred.
					if newDepURN, has := seen[dep.URN]; has {
						newDeps = append(newDeps, newDepURN)
					} else {
						removedDeps = append(removedDeps, dep)
					}
				case resource.ResourcePropertyDependency:
					// For property dependencies, only preserve those that aren't dangling, taking into account any rewrites that
					// may have occurred.
					if newPropDepURN, has := seen[dep.URN]; has {
						newPropDeps[dep.Key] = append(newPropDeps[dep.Key], newPropDepURN)
					} else {
						removedDeps = append(removedDeps, dep)
					}
				case resource.ResourceDeletedWith:
					// Only preseve a deleted-with relationship if it isn't dangling, taking into account any rewrites that may have
					// occurred.
					if newDeletedWithURN, has := seen[dep.URN]; has {
						state.DeletedWith = newDeletedWithURN
					} else {
						state.DeletedWith = ""
						removedDeps = append(removedDeps, dep)
					}
				}
			}

			// If we rewrote the URN or removed any dependencies, add a PruneResult.
			if state.URN != newURN || len(removedDeps) > 0 {
				results = append(results, PruneResult{
					OldURN:              state.URN,
					NewURN:              newURN,
					Delete:              state.Delete,
					RemovedDependencies: removedDeps,
				})
			}

			// Since we can only have shrunk the sets of dependencies and property dependencies, we'll only update them if they
			// were non-empty to begin with. This is to avoid e.g. replacing a nil input with an non-nil but empty output, which
			// while equivalent in many cases is not the same and could result in subtly different behaviour in some parts of
			// the engine.
			if len(state.Dependencies) > 0 {
				state.Dependencies = newDeps
			}
			if len(state.PropertyDependencies) > 0 {
				state.PropertyDependencies = newPropDeps
			}

			seen[state.URN] = newURN
			state.URN = newURN
		}()
	}

	return results
}

// A PruneResult describes the changes made to a resource in a snapshot as a result of pruning dangling dependencies.
type PruneResult struct {
	// The URN of the resource before it was pruned.
	OldURN resource.URN
	// The URN of the resource after it was pruned. This will differ from the OldURN if the resource URN was changed as a
	// result of pruning (e.g. because a missing parent dependency was removed).
	NewURN resource.URN
	// True if and only if the resource was pending deletion.
	Delete bool
	// A list of dependencies that were removed as a result of pruning.
	RemovedDependencies []resource.StateDependency
}

// Toposort attempts sorts this snapshot so that it is topologically sorted with respect to dependencies (where a
// dependency could be a provider, parent-child relationship, dependency, and so on). Resources in the resulting
// snapshot will appear in an order such that all dependencies of a resource will appear before the resource itself.
// Sorting may fail if there are cycles in the snapshot, or in cases where references between resources are genuinely
// ambiguous (e.g. if there are multiple deleted versions of a resource with the same URN that cannot be meaningfully
// differentiated). As a result of this, callers should be mindful that the snapshot could be left in an invalid state
// if sorting terminates mid-way through due to an error.
//
// This method is generally only used for repairing invalid snapshots, since most snapshots are built in response to
// resource registrations from a program, and programs are required to submit such registrations in a
// dependency-respecting order. Note that sortedness is a necessary but not sufficient condition for a snapshot to be
// valid; the VerifyIntegrity method should be used to ensure that a snapshot is well-formed.
func (snap *Snapshot) Toposort() error {
	sorted := []*resource.State{}

	// We implement the sort using a post-order depth-first search, keeping track of nodes we have visited and terminating
	// when we have seen them all. It is not possible to sort a snapshot with cycles (and indeed, such snapshots will
	// never be valid Pulumi states). To this end we also keep track of the path we are currently visiting so that we can
	// spot if we are in a cycle.
	visiting := map[*resource.State]bool{}
	visited := map[*resource.State]bool{}

	// When traversing dependencies, we'll need to look them up by URN. It is possible that the same URN exists multiple
	// times in a snapshot: in the case that the snapshot represents the state mid-way through one or more replacements,
	// both the old and new resources could appear in the snapshot. Dependencies between old and new resources are
	// permitted, so it's important that we know which is which and don't disambiguate by URN alone. To this end we keep
	// track of two lookup tables -- old resources (identifiable by their Delete flag being set) and new resources.
	//
	// NOTE: In the event of multiple old resources with the same URN, we can only implement a best-effort approach to
	// sorting, since there is technically no way to disambiguate.
	oldsByURN := map[resource.URN]*resource.State{}
	newsByURN := map[resource.URN]*resource.State{}
	for _, state := range snap.Resources {
		if state.Delete {
			oldsByURN[state.URN] = state
		} else {
			newsByURN[state.URN] = state
		}
	}

	for _, state := range snap.Resources {
		err := topoVisit(state, &sorted, oldsByURN, newsByURN, visiting, visited)
		if err != nil {
			return err
		}
	}

	snap.Resources = sorted
	return nil
}

// topoVisit is a helper function for Toposort that visits a resource and its dependencies recursively.
func topoVisit(
	state *resource.State,
	sorted *[]*resource.State,
	oldsByURN map[resource.URN]*resource.State,
	newsByURN map[resource.URN]*resource.State,
	visiting map[*resource.State]bool,
	visited map[*resource.State]bool,
) error {
	if visiting[state] {
		return errors.New("snapshot has cyclic dependencies")
	}

	// A helper function for looking up a dependency of this resource. As mentioned above, URN alone is not a unique key
	// as a resource may exist in both old and new forms. We proceed as follows:
	//
	// * If there are both old and new resources with the same URN, and we are old, we take the old one. Since we are old,
	//   there is no way we could refer to a new state (since that state didn't exist when we were last updated).
	// * If there are both old and new resources with the same URN, and we are new, we take the new one; it would be
	//   invalid for us to refer to the old state since it is going to be deleted.
	// * If there is only one resource with the given URN, we take it.
	lookup := func(urn resource.URN) *resource.State {
		old, hasOld := oldsByURN[urn]
		new, hasNew := newsByURN[urn]
		if hasOld && hasNew {
			if state.Delete {
				return old
			}

			return new
		} else if hasOld {
			return old
		} else if hasNew {
			return new
		}

		return nil
	}

	if !visited[state] {
		visiting[state] = true

		provider, allDeps := state.GetAllDependencies()
		nexts := map[*resource.State]bool{}
		for _, dep := range allDeps {
			next := lookup(dep.URN)
			if next != nil {
				nexts[next] = true
			}
		}

		if provider != "" {
			ref, err := providers.ParseReference(provider)
			if err != nil {
				return fmt.Errorf("failed to parse provider reference for resource %s: %w", state.URN, err)
			}

			next := lookup(ref.URN())
			if next != nil {
				nexts[next] = true
			}
		}

		for next := range nexts {
			if err := topoVisit(next, sorted, oldsByURN, newsByURN, visiting, visited); err != nil {
				return err
			}
		}

		visited[state] = true
		visiting[state] = false

		// Append this node after all the dependencies have been visited (and thus appended before it, ensuring topological
		// order).
		*sorted = append(*sorted, state)
	}

	return nil
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
			withAllUpdatedDependencies(
				fixProvider,
				fixUrn,

				// We want to fix up all dependency types, so we pass a nil include function.
				nil,
			).
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
			return SnapshotIntegrityErrorf("magic cookie mismatch; possible tampering/corruption detected")
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
					return SnapshotIntegrityErrorf("provider %s is not referenceable: %w", urn, err)
				}
				provs[ref] = struct{}{}
			}

			provider, allDeps := state.GetAllDependencies()
			if provider != "" {
				ref, err := providers.ParseReference(provider)
				if err != nil {
					return SnapshotIntegrityErrorf("failed to parse provider reference for resource %s: %w", urn, err)
				}
				if _, has := provs[ref]; !has && !state.PendingReplacement {
					return SnapshotIntegrityErrorf("resource %s refers to unknown provider %s", urn, ref)
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

			for _, dep := range allDeps {
				switch dep.Type {
				case resource.ResourceParent:
					if _, has := urns[dep.URN]; !has {
						for _, other := range snap.Resources[i+1:] {
							if other.URN == dep.URN {
								return SnapshotIntegrityErrorf("child resource %s's parent %s comes after it", urn, dep.URN)
							}
						}
						return SnapshotIntegrityErrorf("child resource %s refers to missing parent %s", urn, dep.URN)
					}

					// Ensure that our URN is a child of the parent's URN.
					expectedType := urn.Type()
					if dep.URN.QualifiedType() != resource.RootStackType {
						expectedType = dep.URN.QualifiedType() + "$" + expectedType
					}

					if urn.QualifiedType() != expectedType {
						logging.Warningf("child resource %s has parent %s but its URN doesn't match", urn, dep.URN)
						// TODO: Change this to an error once we're sure users won't hit this in the wild.
						// return fmt.Errorf("child resource %s has parent %s but its URN doesn't match", urn, dep.URN)
					}
				case resource.ResourceDependency:
					if _, has := urns[dep.URN]; !has {
						for _, other := range snap.Resources[i+1:] {
							if other.URN == dep.URN {
								return SnapshotIntegrityErrorf(
									"resource %s's dependency %s comes after it",
									urn, other.URN,
								)
							}
						}

						return SnapshotIntegrityErrorf(
							"resource %s's dependency %s refers to missing resource",
							urn, dep.URN,
						)
					}
				case resource.ResourcePropertyDependency:
					if _, has := urns[dep.URN]; !has {
						for _, other := range snap.Resources[i+1:] {
							if other.URN == dep.URN {
								return SnapshotIntegrityErrorf(
									"resource %s's property dependency %s (from property %s) comes after it",
									urn, other.URN, dep.Key,
								)
							}
						}

						return SnapshotIntegrityErrorf(
							"resource %s's property dependency %s (from property %s) refers to missing resource",
							urn, dep.URN, dep.Key,
						)
					}
				case resource.ResourceDeletedWith:
					if _, has := urns[dep.URN]; !has {
						for _, other := range snap.Resources[i+1:] {
							if other.URN == dep.URN {
								return SnapshotIntegrityErrorf(
									"resource %s is specified as being deleted with %s, which comes after it",
									urn, dep.URN,
								)
							}
						}

						return SnapshotIntegrityErrorf(
							"resource %s is specified as being deleted with %s, which is missing",
							urn, dep.URN,
						)
					}
				}
			}

			if _, has := urns[urn]; has && !state.Delete {
				// The only time we should have duplicate URNs is when all but one of them are marked for deletion.
				return SnapshotIntegrityErrorf("duplicate resource %s (not marked for deletion)", urn)
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

// A snapshot integrity error is raised when a snapshot is found to be malformed
// or invalid in some way (e.g. missing or out-of-order dependencies, or
// unparseable data).
type SnapshotIntegrityError struct {
	// The underlying error that caused this integrity error, if applicable.
	Err error

	// The operation which caused the error. Defaults to SnapshotIntegrityWrite.
	Op SnapshotIntegrityOperation

	// The stack trace at the point the error was raised.
	Stack []byte

	// Metadata about the operation that caused the error, if available.
	Metadata *SnapshotIntegrityErrorMetadata
}

// The set of operations alongside which snapshot integrity checks can be
// performed.
type SnapshotIntegrityOperation int

const (
	// Snapshot integrity checks were performed at write time.
	SnapshotIntegrityWrite SnapshotIntegrityOperation = 0
	// Snapshot integrity checks were performed at read time.
	SnapshotIntegrityRead SnapshotIntegrityOperation = 1
)

// Creates a new snapshot integrity error with a message produced by the given
// format string and arguments. Supports wrapping errors with %w. Snapshot
// integrity errors are raised by Snapshot.VerifyIntegrity when a problem is
// detected with a snapshot (e.g. missing or out-of-order dependencies, or
// unparseable data).
func SnapshotIntegrityErrorf(format string, args ...interface{}) error {
	return &SnapshotIntegrityError{
		Err:   fmt.Errorf(format, args...),
		Op:    SnapshotIntegrityWrite,
		Stack: debug.Stack(),
	}
}

func (s *SnapshotIntegrityError) Error() string {
	if s.Err == nil {
		return "snapshot integrity error"
	}

	return s.Err.Error()
}

func (s *SnapshotIntegrityError) Unwrap() error {
	return s.Err
}

// Returns a copy of the given snapshot integrity error with the operation set to
// SnapshotIntegrityRead and metadata set to the given snapshot's integrity error
// metadata.
func (s *SnapshotIntegrityError) ForRead(snap *Snapshot) *SnapshotIntegrityError {
	return &SnapshotIntegrityError{
		Err:      s.Err,
		Op:       SnapshotIntegrityRead,
		Stack:    s.Stack,
		Metadata: snap.Metadata.IntegrityErrorMetadata,
	}
}

// Returns a tuple in which the second element is true if and only if any error
// in the given error's tree is a SnapshotIntegrityError. In that case, the
// first element will be the first SnapshotIntegrityError in the tree. In the
// event that there is no such SnapshotIntegrityError, the first element will be
// nil.
func AsSnapshotIntegrityError(err error) (*SnapshotIntegrityError, bool) {
	var sie *SnapshotIntegrityError
	ok := errors.As(err, &sie)
	return sie, ok
}
