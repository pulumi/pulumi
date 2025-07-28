// Copyright 2016-2025, Pulumi Corporation.
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
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph struct {
	index      map[string]int         // A mapping of resource pointers to indexes within the snapshot
	resources  []*resource.State      // The list of resources, obtained from the snapshot
	childrenOf map[resource.URN][]int // Pre-computed map of transitive children for each resource
}

// indexKey returns a string key for the given resource. This is used to index into the dependency graph, it is made up
// of the resource's ID and URN.
func indexKey(res *resource.State) string {
	return fmt.Sprintf("%t::%s::%s", res.Delete, res.ID, res.URN)
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

	cursorIndex, ok := dg.index[indexKey(res)]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)
	dependentSet[res.URN] = true

	isDependent := func(candidate *resource.State) bool {
		if ignore[candidate.URN] {
			return false
		}

		provider, allDeps := candidate.GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case resource.ResourceParent:
				if includeChildren && dependentSet[dep.URN] {
					return true
				}
			case resource.ResourceDependency, resource.ResourcePropertyDependency, resource.ResourceDeletedWith:
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
func (dg *DependencyGraph) OnlyDependsOn(res *resource.State) []*resource.State {
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*resource.State
	dependentSet := make(map[resource.URN][]resource.ID)
	nonDependentSet := make(map[resource.URN][]resource.ID)

	cursorIndex, ok := dg.index[indexKey(res)]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)
	dependentSet[res.URN] = []resource.ID{res.ID}
	isDependent := func(candidate *resource.State) bool {
		if res.URN == candidate.URN && res.ID == candidate.ID {
			return false
		}

		provider, allDeps := candidate.GetAllDependencies()
		for _, dep := range allDeps {
			switch dep.Type {
			case resource.ResourceParent:
				if len(dependentSet[dep.URN]) > 0 && len(nonDependentSet[dep.URN]) == 0 {
					return true
				}
			case resource.ResourceDependency, resource.ResourcePropertyDependency, resource.ResourceDeletedWith:
				if len(dependentSet[dep.URN]) == 1 && len(nonDependentSet[dep.URN]) == 0 {
					return true
				}
			}
		}

		if provider != "" {
			ref, err := providers.ParseReference(provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)
			for _, id := range dependentSet[ref.URN()] {
				if id == ref.ID() {
					return true
				}
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
	// We also walk through the the list of resources before the requested resource, as resources
	// sorted later could still be dependent on the requested resource.
	for i := 0; i < cursorIndex; i++ {
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
// field.
func (dg *DependencyGraph) DependenciesOf(res *resource.State) mapset.Set[*resource.State] {
	set := mapset.NewSet[*resource.State]()

	dependentUrns := make(map[resource.URN]bool)
	provider, allDeps := res.GetAllDependencies()
	for _, dep := range allDeps {
		if dep.Type == resource.ResourceParent {
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

	cursorIndex, ok := dg.index[indexKey(res)]
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
func (dg *DependencyGraph) Contains(res *resource.State) bool {
	_, ok := dg.index[indexKey(res)]
	return ok
}

// `TransitiveDependenciesOf` calculates the set of resources upon which the given resource depends, directly or
// indirectly. This includes the resource's provider, parent, any resources in the `Dependencies` list, any resources in
// the `PropertyDependencies` map, and any resource referenced by the `DeletedWith` field.
//
// This function is linear in the number of resources in the `DependencyGraph`.
func (dg *DependencyGraph) TransitiveDependenciesOf(r *resource.State) mapset.Set[*resource.State] {
	// When traversing dependencies, we'll need to look them up by URN. It is possible that the same URN exists multiple
	// times in a dependency graph: in the case that the graph represents the state mid-way through one or more
	// replacements, both the old and new resources could appear. Dependencies between old and new resources are
	// permitted, so it's important that we know which is which and don't disambiguate by URN alone. To this end we keep
	// track of two lookup tables -- old resources (identifiable by their Delete flag being set) and new resources.
	//
	// NOTE: In the event of multiple old resources with the same URN, we can only implement a best-effort approach to
	// sorting, since there is technically no way to disambiguate.
	oldsByURN := map[resource.URN]*node{}
	newsByURN := map[resource.URN]*node{}
	for _, gr := range dg.resources {
		if gr.Delete {
			oldsByURN[gr.URN] = &node{resource: gr}
		} else {
			newsByURN[gr.URN] = &node{resource: gr}
		}
	}

	// A helper function for looking up a dependency of a resource by URN. As mentioned above, URN alone is not a unique
	// key as a resource may exist in both old and new forms. We proceed as follows:
	//
	// * If there are both old and new resources with the same URN, and we are old, we take the old one. Since we are old,
	//   there is no way we could refer to a new state (since that state didn't exist when we were last updated).
	// * If there are both old and new resources with the same URN, and we are new, we take the new one; it would be
	//   invalid for us to refer to the old state since it is going to be deleted.
	// * If there is only one resource with the given URN, we take it.
	lookup := func(from *resource.State, urn resource.URN) *node {
		old, hasOld := oldsByURN[urn]
		new, hasNew := newsByURN[urn]
		if hasOld && hasNew {
			if from.Delete {
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

	dependencies := mapset.NewSet[*resource.State]()

	rn := lookup(r, r.URN)
	if rn == nil {
		return dependencies
	}

	markAsDependency(rn, lookup)
	for _, r := range oldsByURN {
		if r.marked {
			dependencies.Add(r.resource)
		}
	}
	for _, r := range newsByURN {
		if r.marked {
			dependencies.Add(r.resource)
		}
	}

	// We don't want to include `r` as its own dependency.
	dependencies.Remove(r)
	return dependencies
}

// ChildrenOf returns a slice containing all resources that are children of the given resource.
func (dg *DependencyGraph) ChildrenOf(res *resource.State) []*resource.State {
	children := make([]*resource.State, 0)
	for _, childIndex := range dg.childrenOf[res.URN] {
		children = append(children, dg.resources[childIndex])
	}
	return children
}

// ParentsOf returns a slice containing all resources that are parents of the given resource.
func (dg *DependencyGraph) ParentsOf(res *resource.State) []*resource.State {
	parents := make([]*resource.State, 0)
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

// Mark a resource and its provider, parent, dependencies, property dependencies, and deletion dependencies, as a
// dependency. This is a helper function for `TransitiveDependenciesOf`.
func markAsDependency(r *node, lookup func(*resource.State, resource.URN) *node) {
	for {
		r.marked = true
		provider, allDeps := r.resource.GetAllDependencies()
		if provider != "" {
			ref, err := providers.ParseReference(provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)

			p := lookup(r.resource, ref.URN())
			markAsDependency(p, lookup)
		}

		for _, dep := range allDeps {
			if dep.Type == resource.ResourceParent {
				// We handle parents later on, so we won't include them here.
				continue
			}

			d := lookup(r.resource, dep.URN)
			markAsDependency(d, lookup)
		}

		// If the resource's parent is already marked, we don't need to continue to traverse. All nodes above its parent
		// will have already been marked. This is a property of the set of resources being topologically sorted.
		if p := lookup(r.resource, r.resource.Parent); p != nil && !p.marked {
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
	index := make(map[string]int)
	childrenOf := make(map[resource.URN][]int)

	urnIndex := make(map[resource.URN]int)
	for idx, res := range resources {
		index[indexKey(res)] = idx
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
