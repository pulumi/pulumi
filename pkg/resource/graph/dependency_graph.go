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
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph struct {
	// A mapping of resource pointers to indexes within the snapshot
	startIndices map[*resource.State]int

	endIndices map[*resource.State]int

	// The list of resources, obtained from the snapshot
	resources []*resource.State

	// Pre-computed map of transitive children for each resource
	childrenOf map[*resource.State][]*resource.State

	parentsOf map[*resource.State][]*resource.State

	dependenciesOf map[*resource.State]mapset.Set[*resource.State]
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

	startIndex, ok := dg.startIndices[res]
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
	for i := startIndex + 1; i < len(dg.resources); i++ {
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
	var dependents []*resource.State

	return dependents
}

// DependenciesOf returns a set of resources upon which the given resource
// depends directly. This includes the resource's provider, parent, any
// resources in the `Dependencies` list, any resources in the
// `PropertyDependencies` map, and any resource referenced by the `DeletedWith`
// field.
func (dg *DependencyGraph) DependenciesOf(res *resource.State) mapset.Set[*resource.State] {
	return dg.dependenciesOf[res]
}

// Contains returns whether the given resource is in the dependency graph.
func (dg *DependencyGraph) Contains(res *resource.State) bool {
	_, ok := dg.startIndices[res]
	return ok
}

// `TransitiveDependenciesOf` calculates the set of resources upon which the given resource depends, directly or
// indirectly. This includes the resource's provider, parent, any resources in the `Dependencies` list, any resources in
// the `PropertyDependencies` map, and any resource referenced by the `DeletedWith` field.
//
// This function is linear in the number of resources in the `DependencyGraph`.
func (dg *DependencyGraph) TransitiveDependenciesOf(r *resource.State) mapset.Set[*resource.State] {
	s := mapset.NewSet[*resource.State]()

	queue := []*resource.State{r}
	for len(queue) > 0 {
		res := queue[0]
		queue = queue[1:]

		s.Add(res)

		deps := dg.dependenciesOf[res]
		for dep := range deps.Iter() {
			queue = append(queue, dep)
		}
	}

	return s
}

// ChildrenOf returns a slice containing all resources that are children of the given resource.
func (dg *DependencyGraph) ChildrenOf(res *resource.State) []*resource.State {
	_, ok := dg.startIndices[res]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)

	return dg.childrenOf[res]
}

// ParentsOf returns a slice containing all resources that are parents of the given resource.
func (dg *DependencyGraph) ParentsOf(res *resource.State) []*resource.State {
	_, ok := dg.startIndices[res]
	contract.Assertf(ok, "could not determine index for resource %s", res.URN)

	return dg.parentsOf[res]
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
// The resources should be in topological order with respect to their dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	startIndices := map[*resource.State]int{}
	endIndices := map[*resource.State]int{}

	lastURNIndices := map[resource.URN]int{}
	lastRefIndices := map[providers.Reference]int{}

	childrenOf := map[*resource.State][]*resource.State{}
	parentsOf := map[*resource.State][]*resource.State{}

	dependenciesOf := map[*resource.State]mapset.Set[*resource.State]{}

	defaultEndIndex := len(resources) - 1
	for idx, res := range resources {
		startIndices[res] = idx
		endIndices[res] = defaultEndIndex

		previousURNIndex, seenURN := lastURNIndices[res.URN]
		if seenURN {
			endIndices[resources[previousURNIndex]] = idx
		}

		lastURNIndices[res.URN] = idx

		if providers.IsProviderType(res.Type) {
			ref, err := providers.NewReference(res.URN, res.ID)
			contract.AssertNoErrorf(err, "cannot create provider reference %s::%s", res.URN, res.ID)

			lastRefIndices[ref] = idx
		}

		deps := mapset.NewSet[*resource.State]()
		dependenciesOf[res] = deps

		provider, allDeps := res.GetAllDependencies()
		if provider != "" {
			ref, err := providers.ParseReference(provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)

			provIdx, hasProv := lastRefIndices[ref]
			contract.Assertf(hasProv, "could not determine index for provider %s", ref.URN())

			prov := resources[provIdx]

			deps.Add(prov)
		}

		for _, dep := range allDeps {
			depIdx := lastURNIndices[dep.URN]
			depRes := resources[depIdx]

			deps.Add(depRes)
			if !depRes.Custom {
				// TODO: Comment about component children
				// If the dependency is a component, all transitive children of the dependency that are before this
				// resource in the topological sort are also implicitly dependencies. This is necessary because for remote
				// components, the dependencies will not include the transitive set of children directly, but will include
				// the parent component. We must walk that component's children here to ensure they are treated as
				// dependencies. Transitive children of the dependency that are after the resource in the topological sort
				// are not included as this could lead to cycles in the dependency order.
				for _, depChild := range childrenOf[depRes] {
					deps.Add(depChild)
				}
			}

			if dep.Type == resource.ResourceParent {
				parent := resources[depIdx]
				parentsOf[res] = append([]*resource.State{depRes}, parentsOf[parent]...)
			}
		}

		//parentURN := res.Parent
		//for parentURN != "" {
		//  parent := resources[lastURNIndices[parentURN]]
		//	childrenOf[parent] = append(childrenOf[parent], res)
		//	parentURN = parent.Parent
		//}
	}

	return &DependencyGraph{
		startIndices: startIndices,
		endIndices:   endIndices,
		resources:    resources,

		childrenOf: childrenOf,
		parentsOf:  parentsOf,

		dependenciesOf: dependenciesOf,
	}
}

// A node in a graph.
type node struct {
	marked   bool
	resource *resource.State
}
