// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package graph

import (
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph struct {
	index      map[*resource.State]int // A mapping of resource pointers to indexes within the snapshot
	resources  []*resource.State       // The list of resources, obtained from the snapshot
	childrenOf map[resource.URN][]int  // Pre-computed map of transitive children for each resource
}

// DependingOn returns a slice containing all resources that directly or indirectly
// depend upon the given resource. The returned slice is guaranteed to be in topological
// order with respect to the snapshot dependency graph.
//
// The time complexity of DependingOn is linear with respect to the number of resources.
//
// includeChildren adds children as another type of (transitive) dependency.
func (dg *DependencyGraph) DependingOn(res *resource.State,
	ignore map[resource.URN]bool, includeChildren bool,
) []*resource.State {
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*resource.State
	dependentSet := make(map[resource.URN]bool)

	cursorIndex, ok := dg.index[res]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)
	dependentSet[res.URN] = true

	isDependent := func(candidate *resource.State) bool {
		if ignore[candidate.URN] {
			return false
		}
		if includeChildren && dependentSet[candidate.Parent] {
			return true
		}
		for _, dependency := range candidate.Dependencies {
			if dependentSet[dependency] {
				return true
			}
		}
		if candidate.Provider != "" {
			ref, err := providers.ParseReference(candidate.Provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", candidate.Provider)
			if dependentSet[ref.URN()] {
				return true
			}
		}
		return false
	}

	// The dependency graph encoded directly within the snapshot is the reverse of
	// the graph that we actually want to operate upon. Edges in the snapshot graph
	// originate in a resource and go to that resource's dependencies.
	//
	// The `DependingOn` is simpler when operating on the reverse of the snapshot graph,
	// where edges originate in a resource and go to resources that depend on that resource.
	// In this graph, `DependingOn` for a resource is the set of resources that are reachable from the
	// given resource.
	//
	// To accomplish this without building up an entire graph data structure, we'll do a linear
	// scan of the resource list starting at the requested resource and ending at the end of
	// the list. All resources that depend directly or indirectly on `res` are prepended
	// onto `dependents`.
	for i := cursorIndex + 1; i < len(dg.resources); i++ {
		candidate := dg.resources[i]
		if isDependent(candidate) {
			dependents = append(dependents, candidate)
			dependentSet[candidate.URN] = true
		}
	}

	return dependents
}

// DependenciesOf returns a set of resources upon which the given resource depends. The resource's parent is
// included in the returned set.
func (dg *DependencyGraph) DependenciesOf(res *resource.State) mapset.Set[*resource.State] {
	set := mapset.NewSet[*resource.State]()

	dependentUrns := make(map[resource.URN]bool)
	for _, dep := range res.Dependencies {
		dependentUrns[dep] = true
	}

	if res.Provider != "" {
		ref, err := providers.ParseReference(res.Provider)
		contract.AssertNoErrorf(err, "cannot parse provider reference %q", res.Provider)
		dependentUrns[ref.URN()] = true
	}

	cursorIndex, ok := dg.index[res]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)
	for i := cursorIndex - 1; i >= 0; i-- {
		candidate := dg.resources[i]
		// Include all resources that are dependencies of the resource
		if dependentUrns[candidate.URN] {
			set.Add(candidate)
			// If the dependency is a component, all transitive children of the dependency that are before this
			// resource in the topological sort are also implicitly dependencies. This is necessary because for remote
			// components, the dependencies will not include the transitive set of children directly, but will include
			// the parent component. We must walk that component's children here to ensure they are treated as
			// dependencies. Transitive children of the dependency that are after the resource in the topological sort
			// are not included as this could lead to cycles in the dependency order.
			if !candidate.Custom {
				for _, transitiveCandidateIndex := range dg.childrenOf[candidate.URN] {
					if transitiveCandidateIndex < cursorIndex {
						set.Add(dg.resources[transitiveCandidateIndex])
					}
				}
			}
		}
		// Include the resource's parent, as the resource depends on it's parent existing.
		if candidate.URN == res.Parent {
			set.Add(candidate)
		}
	}

	return set
}

// `TransitiveDependenciesOf` calculates the set of resources that `r` depends
// on, directly or indirectly. This includes as a `Parent`, a member of r's
// `Dependencies` list or as a provider.
//
// This function is linear in the number of resources in the `DependencyGraph`.
func (dg *DependencyGraph) TransitiveDependenciesOf(r *resource.State) mapset.Set[*resource.State] {
	dependentProviders := make(map[resource.URN]struct{})

	urns := make(map[resource.URN]*node, len(dg.resources))
	for _, r := range dg.resources {
		urns[r.URN] = &node{resource: r}
	}

	// Linearity is due to short circuiting in the traversal.
	markAsDependency(r.URN, urns, dependentProviders)

	// This will only trigger if (urn, node) is a provider. The check is implicit
	// in the set lookup.
	for urn := range urns {
		if _, ok := dependentProviders[urn]; ok {
			markAsDependency(urn, urns, dependentProviders)
		}
	}

	dependencies := mapset.NewSet[*resource.State]()
	for _, r := range urns {
		if r.marked {
			dependencies.Add(r.resource)
		}
	}
	// We don't want to include `r` as it's own dependency.
	dependencies.Remove(r)
	return dependencies
}

// Mark a resource and its parents as a dependency. This is a helper function for `TransitiveDependenciesOf`.
func markAsDependency(urn resource.URN, urns map[resource.URN]*node, dependedProviders map[resource.URN]struct{}) {
	r := urns[urn]
	for {
		r.marked = true
		if r.resource.Provider != "" {
			ref, err := providers.ParseReference(r.resource.Provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", r.resource.Provider)
			dependedProviders[ref.URN()] = struct{}{}
		}
		for _, dep := range r.resource.Dependencies {
			markAsDependency(dep, urns, dependedProviders)
		}

		// If p is already marked, we don't need to continue to traverse. All
		// nodes above p will have already been marked. This is a property of
		// `resources` being topologically sorted.
		if p, ok := urns[r.resource.Parent]; ok && !p.marked {
			r = p
		} else {
			break
		}
	}
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
// The resources should be in topological order with respect to their dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	index := make(map[*resource.State]int)
	childrenOf := make(map[resource.URN][]int)

	urnIndex := make(map[resource.URN]int)
	for idx, res := range resources {
		index[res] = idx
		urnIndex[res.URN] = idx
		parent := res.Parent
		for parent != "" {
			childrenOf[parent] = append(childrenOf[parent], idx)
			parent = resources[urnIndex[parent]].Parent
		}
	}

	return &DependencyGraph{index, resources, childrenOf}
}

// A node in a graph.
type node struct {
	marked   bool
	resource *resource.State
}
