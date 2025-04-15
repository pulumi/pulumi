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
	startIndices      map[*resource.State]int

  endIndices map[*resource.State]int

  // The list of resources, obtained from the snapshot
	resources  []*resource.State

  // Pre-computed map of transitive children for each resource
	childrenOf map[*resource.State][]*resource.State

	parentsOf map[*resource.State][]*resource.State

  dependenciesOf map[*resource.State]mapset.Set[*resource.State]
  transitiveDependenciesOf map[*resource.State]mapset.Set[*resource.State]
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
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*resource.State
	dependentSet := make(map[resource.URN][]resource.ID)
	nonDependentSet := make(map[resource.URN][]resource.ID)

	cursorIndex, ok := dg.startIndices[res]
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
  return dg.transitiveDependenciesOf[r]
}

// ChildrenOf returns a slice containing all resources that are children of the given resource.
func (dg *DependencyGraph) ChildrenOf(res *resource.State) []*resource.State {
  return dg.childrenOf[res]
}

// ParentsOf returns a slice containing all resources that are parents of the given resource.
func (dg *DependencyGraph) ParentsOf(res *resource.State) []*resource.State {
  return dg.parentsOf[res]
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
// The resources should be in topological order with respect to their dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	startIndices := map[*resource.State]int{}
  endIndices := map[*resource.State]int{}

	lastURNIndices := make(map[resource.URN]int)

	childrenOf := map[*resource.State][]*resource.State{}
  parentsOf := map[*resource.State][]*resource.State{}

  dependenciesOf := map[*resource.State]mapset.Set[*resource.State]{}
  transitiveDependenciesOf := map[*resource.State]mapset.Set[*resource.State]{}

  defaultEndIndex := len(resources) - 1
	for idx, res := range resources {
		startIndices[res] = idx
    endIndices[res] = defaultEndIndex

    previousURNIndex, seenURN := lastURNIndices[res.URN]
    if seenURN {
      endIndices[resources[previousURNIndex]] = idx
    }

		lastURNIndices[res.URN] = idx

    deps := mapset.NewSet[*resource.State]()
    dependenciesOf[res] = deps

    transDeps := mapset.NewSet[*resource.State]()
    transitiveDependenciesOf[res] = transDeps

    provider, allDeps := res.GetAllDependencies()
    if provider != "" {
      ref, err := providers.ParseReference(provider)
      contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)

      provIdx := lastURNIndices[ref.URN()]
      prov := resources[provIdx]

      deps.Add(prov)
      transDeps.Add(prov)
      for provDep := range transitiveDependenciesOf[prov].Iter() {
        transDeps.Add(provDep)
      }
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

      transDeps.Add(depRes)
      for depDep := range transitiveDependenciesOf[depRes].Iter() {
        transDeps.Add(depDep)
      }

      if dep.Type == resource.ResourceParent {
        parent := resources[depIdx]
        parentsOf[res] = append([]*resource.State{depRes}, parentsOf[parent]...)
      }
    }

    parentURN := res.Parent
		for parentURN != "" {
      parent := resources[lastURNIndices[parentURN]]
			childrenOf[parent] = append(childrenOf[parent], res)
			parentURN = parent.Parent
		}
	}

	return &DependencyGraph{
    startIndices: startIndices,
    endIndices: endIndices,
    resources: resources,

    childrenOf: childrenOf,
    parentsOf: parentsOf,

    dependenciesOf: dependenciesOf,
    transitiveDependenciesOf: transitiveDependenciesOf,
  }
}

// A node in a graph.
type node struct {
	marked   bool
	resource *resource.State
}
