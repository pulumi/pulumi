// Copyright 2016, Pulumi Corporation.
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

package graph

import (
	"slices"

	mapset "github.com/deckarep/golang-set/v2"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph struct {
	index      map[*pkgresource.State]int // A mapping of resource pointers to indexes within the snapshot
	resources  []*pkgresource.State       // The list of resources, obtained from the snapshot
	childrenOf map[resource.URN][]int     // Pre-computed map of transitive children for each resource
}

// Alias maps a new *pkgresource.State to an existing resource in the dependency graph.
func (dg *DependencyGraph) Alias(newRes *pkgresource.State, existingRes *pkgresource.State) {
	// Ensure that the new resource is not already in the dependency graph.
	_, exists := dg.index[newRes]
	contract.Assertf(!exists, "new resource %s already exists in the dependency graph", newRes.URN)
	// Ensure that the existing resource is in the dependency graph.
	index, exists := dg.index[existingRes]
	contract.Assertf(exists, "existing resource %s does not exist in the dependency graph", existingRes.URN)
	// Map the new resource to the index of the existing resource.
	dg.index[newRes] = index
}

// DependingOn returns a slice containing all resources that directly or indirectly
// depend upon the given resource. The returned slice is guaranteed to be in topological
// order with respect to the snapshot dependency graph.
//
// The time complexity of DependingOn is linear with respect to the number of resources.
//
// includeChildren adds children as another type of (transitive) dependency.
func (dg *DependencyGraph) DependingOn(res *pkgresource.State,
	ignore map[resource.URN]bool, includeChildren bool,
) []*pkgresource.State {
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*pkgresource.State
	dependentSet := make(map[resource.URN]bool)

	cursorIndex, ok := dg.index[res]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)
	dependentSet[res.URN] = true

	isDependent := func(candidate *pkgresource.State) bool {
		if ignore[candidate.URN] {
			return false
		}

		provider, allDeps := candidate.GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case pkgresource.ResourceParent:
				if includeChildren && dependentSet[dep.URN] {
					return true
				}
			case pkgresource.ResourceDependency, pkgresource.ResourcePropertyDependency,
				pkgresource.ResourceDeletedWith, pkgresource.ResourceReplaceWith:
				if dependentSet[dep.URN] {
					return true
				}
			}
		}

		if provider != "" {
			ref, err := providers.ParseReference(provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)
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

// OnlyDependsOn returns a slice containing all resources that directly or indirectly depend upon *only* the specific ID
// for the given resources URN. Resources that also depend on another resource with the same URN, but a different ID,
// will not be included in the returned slice. The returned slice is guaranteed to be in topological order with respect
// to the snapshot dependency graph.
//
// The time complexity of OnlyDependsOn is linear with respect to the number of resources.
func (dg *DependencyGraph) OnlyDependsOn(res *pkgresource.State) []*pkgresource.State {
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*pkgresource.State
	dependentSet := make(map[resource.URN][]resource.ID)
	nonDependentSet := make(map[resource.URN][]resource.ID)

	cursorIndex, ok := dg.index[res]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)
	dependentSet[res.URN] = []resource.ID{res.ID}
	isDependent := func(candidate *pkgresource.State) bool {
		if res.URN == candidate.URN && res.ID == candidate.ID {
			return false
		}

		provider, allDeps := candidate.GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case pkgresource.ResourceParent:
				if len(dependentSet[dep.URN]) > 0 && len(nonDependentSet[dep.URN]) == 0 {
					return true
				}
			case pkgresource.ResourceDependency, pkgresource.ResourcePropertyDependency, pkgresource.ResourceDeletedWith,
				pkgresource.ResourceReplaceWith:
				if len(dependentSet[dep.URN]) == 1 && len(nonDependentSet[dep.URN]) == 0 {
					return true
				}
			}
		}

		if provider != "" {
			ref, err := providers.ParseReference(provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)
			if slices.Contains(dependentSet[ref.URN()], ref.ID()) {
				return true
			}
		}

		return false
	}

	// The dependency graph encoded directly within the snapshot is the reverse of
	// the graph that we actually want to operate upon. Edges in the snapshot graph
	// originate in a resource and go to that resource's dependencies.
	//
	// The `OnlyDependsOn` is simpler when operating on the reverse of the snapshot graph,
	// where edges originate in a resource and go to resources that depend on that resource.
	// In this graph, `OnlyDependsOn` for a resource is the set of resources that are reachable from the
	// given resource, and only from the given resource.
	//
	// To accomplish this without building up an entire graph data structure, we'll do a linear
	// scan of the resource list starting at the requested resource and ending at the end of
	// the list. All resources that depend directly or indirectly on `res` are prepended
	// onto `dependents`.
	//
	// We also walk through the list of resources before the requested resource, as resources
	// sorted later could still be dependent on the requested resource.
	for i := range cursorIndex {
		candidate := dg.resources[i]
		nonDependentSet[candidate.URN] = append(nonDependentSet[candidate.URN], candidate.ID)
	}
	for i := cursorIndex + 1; i < len(dg.resources); i++ {
		candidate := dg.resources[i]
		if isDependent(candidate) {
			dependents = append(dependents, candidate)
			dependentSet[candidate.URN] = append(dependentSet[candidate.URN], candidate.ID)
		} else {
			nonDependentSet[candidate.URN] = append(nonDependentSet[candidate.URN], candidate.ID)
		}
	}

	return dependents
}

// DependenciesOf returns a set of resources upon which the given resource
// depends directly. This includes the resource's provider, parent, any
// resources in the `Dependencies` list, any resources in the
// `PropertyDependencies` map, and any resource referenced by the `DeletedWith`
// or `ReplaceWith` fields.
func (dg *DependencyGraph) DependenciesOf(res *pkgresource.State) mapset.Set[*pkgresource.State] {
	set := mapset.NewSet[*pkgresource.State]()

	dependentUrns := make(map[resource.URN]bool)
	provider, allDeps := res.GetAllDependencies()
	for _, dep := range allDeps {
		if dep.Type == pkgresource.ResourceParent {
			// We handle parents later on, so we won't include them here.
			continue
		}

		dependentUrns[dep.URN] = true
	}

	if provider != "" {
		ref, err := providers.ParseReference(provider)
		contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)
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

// Contains returns whether the given resource is in the dependency graph.
func (dg *DependencyGraph) Contains(res *pkgresource.State) bool {
	_, ok := dg.index[res]
	return ok
}

// `TransitiveDependenciesOf` calculates the set of resources upon which the given resource depends, directly or
// indirectly. This includes the resource's provider, parent, any resources in the `Dependencies` list, any resources in
// the `PropertyDependencies` map, and any resource referenced by the `DeletedWith` or `ReplaceWith` fields. Only a
// resource that precedes its dependent in the graph can be a dependency.
//
// This function takes O(N + E*k) time, where N is the number of resources, E is the number of dependency edges, and k
// is the largest number of copies of one URN.
func (dg *DependencyGraph) TransitiveDependenciesOf(r *pkgresource.State) mapset.Set[*pkgresource.State] {
	dependencies := mapset.NewSet[*pkgresource.State]()
	index, ok := dg.index[r]
	if !ok {
		return dependencies
	}

	// When traversing dependencies, we'll need to look them up by URN. It is possible that the same URN exists multiple
	// times in a dependency graph: in the case that the graph represents the state mid-way through one or more
	// replacements, both the old and new resources could appear. Dependencies between old and new resources are
	// permitted, so it's important that we know which is which and don't disambiguate by URN alone. To this end we keep
	// track of two lookup tables -- old resources (identifiable by their Delete flag being set) and new resources.
	//
	// We cannot tell multiple old resources with the same URN apart, so we keep all of them that precede the resource
	// that refers to them.
	//
	// We expect at most one new resource for each URN. A malformed snapshot can contain more than one, so we keep all of
	// them.
	oldsByURN := map[resource.URN][]*node{}
	newsByURN := map[resource.URN][]*node{}
	var rn *node
	for i, gr := range dg.resources {
		n := &node{resource: gr, index: i}
		if i == index {
			rn = n
			// Resources are topologically sorted, so we know that r doesn't depend on anything past itself.
			break
		}
		if gr.Delete {
			oldsByURN[gr.URN] = append(oldsByURN[gr.URN], n)
		} else {
			newsByURN[gr.URN] = append(newsByURN[gr.URN], n)
		}
	}

	// Both tables are in index order, so the resources that precede `from` form a prefix.
	precede := func(nodes []*node, from *node) []*node {
		end := 0
		for end < len(nodes) && nodes[end].index < from.index {
			end++
		}
		return nodes[:end]
	}

	// A helper function for looking up a dependency of a resource by URN. Resources are topologically sorted, so we first
	// drop every resource with the given URN that does not precede us. As mentioned above, URN alone is not a unique key
	// as a resource may exist in both old and new forms. For the resources that precede us, we proceed as follows:
	//
	// * If there are both old and new resources with the same URN, and we are old, we take the old ones. Since we are old,
	//   there is no way we could refer to a new state (since that state didn't exist when we were last updated).
	// * If there are both old and new resources with the same URN, and we are new, we take the new ones; it would be
	//   invalid for us to refer to the old state since it is going to be deleted.
	// * If there are only old or only new resources with the given URN, we take all of them.
	//
	// Snapshot integrity requires only that some resource with the URN precedes us. We filter before we choose, so we
	// always keep that resource.
	lookup := func(from *node, urn resource.URN) []*node {
		old := precede(oldsByURN[urn], from)
		new := precede(newsByURN[urn], from)
		if len(old) > 0 && (from.resource.Delete || len(new) == 0) {
			return old
		}
		return new
	}

	markAsDependency(rn, lookup)
	for _, nodes := range oldsByURN {
		for _, n := range nodes {
			if n.marked {
				dependencies.Add(n.resource)
			}
		}
	}
	for _, nodes := range newsByURN {
		for _, n := range nodes {
			if n.marked {
				dependencies.Add(n.resource)
			}
		}
	}

	return dependencies
}

// ChildrenOf returns a slice containing all resources that are children of the given resource.
func (dg *DependencyGraph) ChildrenOf(res *pkgresource.State) []*pkgresource.State {
	children := slice.Prealloc[*pkgresource.State](len(dg.childrenOf[res.URN]))
	for _, childIndex := range dg.childrenOf[res.URN] {
		children = append(children, dg.resources[childIndex])
	}
	return children
}

// ParentsOf returns a slice containing all resources that are parents of the given resource.
func (dg *DependencyGraph) ParentsOf(res *pkgresource.State) []*pkgresource.State {
	parents := make([]*pkgresource.State, 0)
	// The resources in dg.resources are topologically sorted, so when we walk backwards and we match a parent,
	// we know we have yet to see that parent's parent (if it exists).  We know it's safe to terminate when we've
	// traversed the full set in reverse.
	for i := len(dg.resources) - 1; i >= 0; i-- {
		if dg.resources[i].URN == res.Parent {
			parents = append(parents, dg.resources[i])
			res = dg.resources[i]
		}
	}
	return parents
}

// Mark a resource and its transitive dependencies.
func markAsDependency(r *node, lookup func(*node, resource.URN) []*node) {
	if r.marked {
		return
	}
	r.marked = true
	provider, allDeps := r.resource.GetAllDependencies()
	if provider != "" {
		ref, err := providers.ParseReference(provider)
		contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)

		for _, p := range lookup(r, ref.URN()) {
			markAsDependency(p, lookup)
		}
	}

	for _, dep := range allDeps {
		for _, d := range lookup(r, dep.URN) {
			markAsDependency(d, lookup)
		}
	}
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
// The resources should be in topological order with respect to their dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*pkgresource.State) *DependencyGraph {
	index := make(map[*pkgresource.State]int)
	childrenOf := make(map[resource.URN][]int)

	urnIndex := make(map[resource.URN]int)
	for idx, res := range resources {
		index[res] = idx
		urnIndex[res.URN] = idx
		parent := res.Parent
		for parent != "" {
			childrenOf[parent] = append(childrenOf[parent], idx)
			idx, has := urnIndex[parent]
			contract.Assertf(has, "parent %q not found in urnIndex", parent)
			parent = resources[idx].Parent
		}
	}

	return &DependencyGraph{index, resources, childrenOf}
}

// A node in a graph.
type node struct {
	marked   bool
	resource *pkgresource.State
	index    int // The index of the resource in the graph.
}
