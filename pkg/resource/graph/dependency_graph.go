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
	// A mapping of resource pointers to the indices at which they appear in the snapshot.
	startIndices map[*resource.State]int

	// A mapping of resource pointers to the indices at which they are "no longer visible" in the snapshot. Visibility is
	// defined according to the ways in which dependencies can be encoded between resources. Presently there are two
	// methods:
	//
	// * By URN, as used for parents, dependencies, property dependencies and deleted-with relationships. In this case, a
	//   resource ceases to be visible if another resource with the same URN appears after it in the snapshot.
	// * By URN and ID, as used for provider references. In this case, a resource ceases to be visible if another provider
	//   resource with the same URN and ID appears after it in the snapshot.
	endIndices map[*resource.State]int

	// The list of resources, obtained from the snapshot.
	resources []*resource.State

	// A mapping from resource pointers to the set of all children of that resource. The children of a resource are
	// structured as a forest of trees. The trees of the forest are rooted by the direct children of the resource, with
	// their (transitive) children represented as subtrees.
	childrenOf map[*resource.State]*childForest

	// A mapping from resource pointers to the set of all parents of that resource. The parents of a resource are
	// structured as a linked list, with the head of the list being the resource's immediate parent, the head's next being
	// the resource's grandparent, and so on.
	parentsOf map[*resource.State]*parentList

	// A mapping from resource pointers to the set of all transitive dependencies of that resource. The transitive
	// dependencies of a resource are structured as a forest of trees. The trees of the forest are rooted by the direct
	// dependencies of the resource, with their (transitive) dependencies represented as subtrees.
	transitiveDependenciesOf map[*resource.State][]*dependencyTree
}

// DependingOn returns a slice containing all resources that directly or indirectly depend upon the given resource. The
// returned slice is guaranteed to be in topological order with respect to the snapshot dependency graph.
//
// Note:
//
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
//   - includeChildren adds children as another type of (transitive) dependency.
func (dg *DependencyGraph) DependingOn(r *resource.State,
	ignore map[resource.URN]bool, includeChildren bool,
) []*resource.State {
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*resource.State
	dependentSet := make(map[*resource.State]bool)

	rStart, ok := dg.startIndices[r]
	contract.Assertf(ok, "could not determine index for resource %s", r.URN)

	dependentSet[r] = true
	isDependent := func(candidate *resource.State) bool {
		if ignore[candidate.URN] {
			return false
		}

		for _, dep := range dg.transitiveDependenciesOf[candidate] {
			if dep.Type == resource.ResourceParent && !includeChildren {
				continue
			}

			if dependentSet[dep.resource] {
				return true
			}
		}

		return false
	}

	for i := rStart + 1; i < len(dg.resources); i++ {
		candidate := dg.resources[i]
		if isDependent(candidate) {
			dependents = append(dependents, candidate)
			dependentSet[candidate] = true
		}
	}

	return dependents
}

// OnlyDependsOn returns a slice containing all resources that directly or indirectly depend upon *only* the specific ID
// for the given resource's URN. Resources that also depend on another resource with the same URN, but a different ID,
// will not be included in the returned slice. The returned slice is guaranteed to be in topological order with respect
// to the snapshot dependency graph.
//
// Note:
//
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
func (dg *DependencyGraph) OnlyDependsOn(r *resource.State) []*resource.State {
	var dependents []*resource.State
	dependentSet := make(map[*resource.State]bool)

	rStart, ok := dg.startIndices[r]
	contract.Assertf(ok, "could not determine index for resource %s", r.URN)

	// The idea behind OnlyDependsOn is as follows:
	//
	// * Resources can only depend on resources before them in the topological sort.
	// * Thus, we want to start at the target resource and walk to the end of the snapshot, since only those resources
	//   could depend on it.
	// * However, when we _do_ find a dependency, we want to ask "are there _also_ dependencies from this resource to
	//   _other_ resources with the same URN, but a different ID?"
	// * To accomplish this, we'll accumulate a set of "poisoned" resources -- those that either have the same URN but a
	//   different ID, or which depend directly or transitively on one that does.
	// * When we find a dependency on our target, we check the other dependencies to see if any are poisoned. If not, we
	//   have a candidate for our result set.
	//
	// The trick then is initialising the poisoned set. We do this by walking from the start of the snapshot up to our
	// target resource. Any that we spot that have a matching URN and a different ID are added to the poisoned set. Any
	// that depend on a poisoned resource are also poisoned. When we get to our target, we can start checking for
	// non-poisoned dependencies and from there build the desired result.
	poisonSet := map[*resource.State]bool{}

	for i := 0; i < rStart; i++ {
		gr := dg.resources[i]
		if gr.URN == r.URN && gr.ID != r.ID {
			// This resource is "directly" poisoned, since its URN matches but its ID doesn't. We don't need to check its
			// dependencies and can move on.
			poisonSet[gr] = true
			continue
		}

		for _, dep := range dg.transitiveDependenciesOf[gr] {
			if poisonSet[dep.resource] {
				// This resource depends on one that is poisoned. It is now also poisoned and we can move on.
				poisonSet[gr] = true
				break
			}
		}
	}

	for i := rStart + 1; i < len(dg.resources); i++ {
		candidate := dg.resources[i]

		if candidate.URN == r.URN && candidate.ID != r.ID {
			// This resource is "directly" poisoned, since its URN matches but its ID doesn't. We don't need to check its
			// dependencies and can move on.
			poisonSet[candidate] = true
			continue
		}

		saw := false
		poisoned := false

		for _, dep := range dg.transitiveDependenciesOf[candidate] {
			if poisonSet[dep.resource] {
				// This resource depends on one that is poisoned. It is now also poisoned.
				poisoned = true
				break
			} else if dep.resource == r || dependentSet[dep.resource] {
				saw = true
			}
		}

		// If we were poisoned, mark ourselves as such. If not, and we saw our target, add ourselves to the result set.
		if poisoned {
			poisonSet[candidate] = true
		} else if saw {
			dependentSet[candidate] = true
			dependents = append(dependents, candidate)
		}
	}

	return dependents
}

// DependenciesOf returns a set of resources upon which the given resource depends directly. This includes the
// resource's provider, parent, any resources in the `Dependencies` list, any resources in the `PropertyDependencies`
// map, and any resource referenced by the `DeletedWith` field.
//
// Note:
//
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
func (dg *DependencyGraph) DependenciesOf(r *resource.State) mapset.Set[*resource.State] {
	rStart := dg.startIndices[r]
	s := mapset.NewSet[*resource.State]()

	var addAllChildren func(ns []*childTree)
	addAllChildren = func(ns []*childTree) {
		if len(ns) == 0 {
			return
		}

		// Since we are adding to a set and not appending to a list, the order in which we traverse the childTrees doesn't
		// matter in terms of the result, but it does allow us to make a performance optimization. If at any point the index
		// of a potential child is greater than the start index of the resource we are computing dependencies for, we can
		// skip the rest of the forest.
		for i := len(ns) - 1; i >= 0; i-- {
			n := ns[i]
			if n.index >= rStart {
				break
			}

			if s.Contains(n.resource) {
				continue
			}

			s.Add(n.resource)
			addAllChildren(n.children)
		}
	}

	for _, n := range dg.transitiveDependenciesOf[r] {
		s.Add(n.resource)

		// If we depend on a component resource (that is, where Custom: false), we must count all of the children of that
		// component that appear before us topologically as potential dependencies. This is necessary because for remote
		// components, the dependencies will not include the transitive set of children directly, but will include the
		// parent component. We must walk that component's children here to ensure they are treated as dependencies.
		// Transitive children of the dependency that are after the resource in the topological sort are not included as
		// this could lead to cycles in the dependency order.
		if !n.resource.Custom {
			addAllChildren(n.children.trees)
		}
	}

	return s
}

// Contains returns whether the given resource is in the dependency graph.
//
// Note:
//
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
func (dg *DependencyGraph) Contains(res *resource.State) bool {
	_, ok := dg.startIndices[res]
	return ok
}

// `TransitiveDependenciesOf` calculates the set of resources upon which the given resource depends, directly or
// indirectly. This includes the resource's provider, parent, any resources in the `Dependencies` list, any resources in
// the `PropertyDependencies` map, and any resource referenced by the `DeletedWith` field.
//
// Note:
//
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
func (dg *DependencyGraph) TransitiveDependenciesOf(r *resource.State) mapset.Set[*resource.State] {
	rStart := dg.startIndices[r]
	s := mapset.NewSet[*resource.State]()

	var addAllChildren func(ns []*childTree)
	addAllChildren = func(ns []*childTree) {
		if len(ns) == 0 {
			return
		}

		// Since we are adding to a set and not appending to a list, the order in which we traverse the childTrees doesn't
		// matter in terms of the result, but it does allow us to make a performance optimization. If at any point the index
		// of a potential child is greater than the start index of the resource we are computing dependencies for, we can
		// skip the rest of the forest.
		for i := len(ns) - 1; i >= 0; i-- {
			n := ns[i]
			if n.index >= rStart {
				break
			}

			if s.Contains(n.resource) {
				continue
			}

			s.Add(n.resource)
			addAllChildren(n.children)
		}
	}

	var addAll func(ns []*dependencyTree)
	addAll = func(ns []*dependencyTree) {
		if len(ns) == 0 {
			return
		}

		for _, n := range ns {
			if s.Contains(n.resource) {
				continue
			}

			s.Add(n.resource)

			// If we depend on a component resource (that is, where Custom: false), we must count all of the children of that
			// component that appear before us topologically as potential dependencies. This is necessary because for remote
			// components, the dependencies will not include the transitive set of children directly, but will include the
			// parent component. We must walk that component's children here to ensure they are treated as dependencies.
			// Transitive children of the dependency that are after the resource in the topological sort are not included as
			// this could lead to cycles in the dependency order.
			if !n.resource.Custom {
				addAllChildren(n.children.trees)
			}

			addAll(n.next)
		}
	}

	addAll(dg.transitiveDependenciesOf[r])

	return s
}

// ChildrenOf returns a slice containing all resources that are children of the given resource.
//
// Note:
//
//   - While the resulting slice is guaranteed to be ordered such that children appear after their parents, it is *not*
//     guaranteed to be topologically sorted. That is, there may be dependencies between the elements of the resulting
//     slice which are not respected by their ordering.
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
func (dg *DependencyGraph) ChildrenOf(res *resource.State) []*resource.State {
	c := dg.childrenOf[res]
	if c == nil {
		return nil
	}

	result := []*resource.State{}

	var addAll func(ns []*childTree)
	addAll = func(ns []*childTree) {
		if len(ns) == 0 {
			return
		}

		// childTrees are built in reverse order, so we must traverse the forest in reverse order to construct the
		// correct result.
		for i := len(ns) - 1; i >= 0; i-- {
			n := ns[i]
			result = append(result, n.resource)
			addAll(n.children)
		}
	}

	addAll(c.trees)
	return result
}

// ParentsOf returns a slice containing all resources that are parents of the given resource.
//
// Note:
//
//   - The resulting slice is guaranteed to be ordered from the most immediate parent to the most distant ancestor.
//   - DependencyGraphs operate on pointer equality to ensure resource uniqueness, so the argument *must* be a pointer
//     which is contained within the DependencyGraph.
func (dg *DependencyGraph) ParentsOf(r *resource.State) []*resource.State {
	var result []*resource.State

	current := dg.parentsOf[r]
	for current != nil {
		result = append(result, current.resource)
		current = current.next
	}

	return result
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources. Resources must be in topological order,
// with resources that depend on others appearing _after_ those they depend on. O(N*D) where N is the number of
// resources and D is the number of dependencies per resource. In pathological cases therefore where D approaches N,
// this function can be O(N^2).
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	// For each resource, we need to track when it first appears (its start index) and when it is shadowed by a subsequent
	// resource (its end index). For provider resources, shadowing only occurs when another provider resource with the
	// same URN and ID appears (since provider references contain both URN and ID). For non-provider resources, shadowing
	// occurs when another resource with the same URN appears (since non-provider references such as dependencies,
	// property dependencies, and deleted-with relationships only contain the URN).
	startIndices := map[*resource.State]int{}
	endIndices := map[*resource.State]int{}

	lastURNIndices := map[resource.URN]int{}
	lastRefIndices := map[providers.Reference]int{}

	childrenOf := map[*resource.State]*childForest{}
	parentsOf := map[*resource.State]*parentList{}

	transitiveDependenciesOf := map[*resource.State][]*dependencyTree{}

	// The default end index for a resource is the last index in the list of resources (i.e. in the case no subsequent
	// resource shadows it).
	defaultEndIndex := len(resources) - 1

	// We have several jobs to do as part of graph construction:
	//
	// 1. Discover the start and end indices for each resource.
	// 2. Build the transitive parents list for each resource.
	// 3. Build the transitive children tree for each resource.
	// 4. Build the transitive dependency tree for each resource.
	//
	// We can do 1, 2, and 4 in a single pass over the resources, from the first to the last:
	//
	// * We capture start indices (1) as resources appear.
	// * We spot end indices (1) when we see a resource with the same URN (or URN and ID for providers) as one we've
	//   already seen.
	// * We can build the transitive dependencies for a resource in O(D) time where D is the number of immediate
	//   dependencies. First, we construct a new tree node for the resource we are visiting. Then, for each immediate
	//   dependency, we look up its transitive dependencies (which we must have already built since resources are
	//   topologically sorted) and add a pointer to that tree as a subtree of the one we're constructing. Node
	//   construction and lookup are O(1) so we end up with O(D). Doing this for O(N) resources results in the function's
	//   O(N*D) complexity.
	// * One gotcha is that if a resource depends on a component, the *children* of that component which appear *before*
	//   the resource are also implicitly considered its dependencies. To this end, we pre-allocate a child forest for each
	//   resource and then point to this when constructing transitive dependencies. These forests will be filled in
	//   shortly.
	//
	// As hinted at above, to compute children, we need a second pass of the resources. We do this in reverse order, so
	// that children appear before their parents and we can build everything in a single pass. The idea is much the same
	// as that of transitive dependencies, resulting in O(N) complexity.

	// Pass 1 -- in order, build start/end indices, transitive dependencies, and parents.
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

		// Pre-allocate an empty child forest for this resource. This will be filled in pass 1, but the container can be
		// pointed to now when computing transitive dependency trees.
		childrenOf[res] = &childForest{}

		var transDeps []*dependencyTree

		provider, allDeps := res.GetAllDependencies()
		if provider != "" {
			ref, err := providers.ParseReference(provider)
			contract.AssertNoErrorf(err, "cannot parse provider reference %q", provider)

			provIdx, hasProv := lastRefIndices[ref]
			contract.Assertf(hasProv, "could not determine index for provider %s", ref.URN())

			prov := resources[provIdx]

			transDeps = append(transDeps, &dependencyTree{
				resource: prov,
				next:     transitiveDependenciesOf[prov],
				children: childrenOf[prov],
			})
		}

		for _, dep := range allDeps {
			depIdx := lastURNIndices[dep.URN]
			depRes := resources[depIdx]

			transDeps = append(transDeps, &dependencyTree{
				Type:     dep.Type,
				resource: depRes,
				next:     transitiveDependenciesOf[depRes],
				children: childrenOf[depRes],
			})

			if dep.Type == resource.ResourceParent {
				parent := resources[depIdx]

				parentsOf[res] = &parentList{
					resource: parent,
					next:     parentsOf[parent],
				}
			}
		}

		transitiveDependenciesOf[res] = transDeps
	}

	// Pass 2 -- in reverse order, build children. Note that, as a result of the reverse ordering, the forests of children
	// that we build are reversed:
	//
	// * The set of trees in the forest are in reverse order; and
	// * The trees themselves are in reverse order.
	//
	// So, for example, if we have the following resource graph:
	//
	//   A
	//   ├── B
	//   │   ├── C
	//   │   └── D
	//   │       └── E
	//   └── F
	//
	// We will:
	//
	// * Visit F and declare that it is a child of A
	//     [A = {F}]
	// * Push F's children (none) onto the forest of A
	//     [A = {F}]
	// * Visit E and declare that it is a child of D
	//     [D = {E}]
	// * Push E's children (none) onto the forest of D
	//     [D = {E}]
	// * Visit D and declare that it is a child of B
	//     [B = {D}]
	// * Push D's children (E) onto the forest of B
	//     [B = {D {E}}]
	// * Visit C and declare that it is a child of B
	//     [B = {D {E}, C}]
	// * Push C's children (none) onto the forest of B
	//     [B = {D {E}, C}]
	// * Visit B and declare that it is a child of A
	//     [A = {F, B}]
	// * Push B's children (C, D) onto the forest of A
	//     [A = {F, B {C, D {E}}}]
	//
	// Thus, in order to recover the children of A in the order we'd "expect" (B, C, D, E, F), we need to reverse the
	// order of both the traversal of trees in each forest. This can be seen in the various methods of DependencyGraph.
	for i := len(resources) - 1; i >= 0; i-- {
		res := resources[i]
		if res.Parent == "" {
			continue
		}

		parent := resources[lastURNIndices[res.Parent]]
		resChildren := childrenOf[res]

		childrenOf[parent].trees = append(childrenOf[parent].trees, &childTree{
			index:    i,
			resource: res,
			children: resChildren.trees,
		})
	}

	return &DependencyGraph{
		startIndices: startIndices,
		endIndices:   endIndices,
		resources:    resources,

		childrenOf: childrenOf,
		parentsOf:  parentsOf,

		transitiveDependenciesOf: transitiveDependenciesOf,
	}
}

// dependencyTree encodes the transitive dependencies of a resource. For example, given the set of resources:
//
//	A
//	B {Parent: A}
//	C {Parent: C}
//	D
//	E {Dependencies: [C], DeletedWith: D}
//
// We could build a dependencyTree encoding the transitive dependencies of E as follows:
//
//	[]*dependencyTree{
//	    &dependencyTree{
//	        Type:     resource.ResourceDependency,
//	        resource: C,
//	        next: []*dependencyTree{
//	            &dependencyTree{
//	                Type:     resource.ResourceParent,
//	                resource: B,
//	                next: []*dependencyTree{
//	                    &dependencyTree{
//	                        Type:     resource.ResourceParent,
//	                        resource: A,
//	                    },
//	                },
//	            },
//	        },
//	    },
//	    &dependencyTree{
//	        Type:     resource.ResourceDeletedWith,
//	        resource: D,
//	    },
//	}
//
// It costs a bit more to traverse this tree than it would to flatten all transitive dependencies into a single
// slice/set at graph construction time, but such a construction would be at least O(N^2) in the number of resources,
// since for each of N resources we might have to walk and copy a slice with O(N) elements (consider a graph where each
// resource is the parent of the one that follows it, for instance).
type dependencyTree struct {
	// The type of dependency between the resource pointing to this tree and the resource at the root of this tree.
	Type resource.StateDependencyType

	// The resource at the root of this tree, whose (transitive) dependencies are captured as subtrees.
	resource *resource.State

	// The transitive dependencies of the resource at the root of this tree, represented as subtrees.
	next []*dependencyTree

	// A pointer to the forest of children for the resource at the root of the tree. This is used for tracking
	// dependencies caused by component resources which register children.
	children *childForest
}

// parentList encodes the parents of a resource. The parents of a resource are structured as a linked list, with the
// head of the list being the resource's immediate parent, the head's next being the resource's grandparent, and so on.
// For example, given the set of resources (where a line represents a parent/child relationship):
//
//	A
//	├── B
//	│   ├── C
//	│   └── D
//	│       └── E
//	└── F
//
// We could represent the parents of E as:
//
//	&parentList{
//	    resource: D,
//	    next: &parentList{
//	        resource: B,
//	        next: &parentList{
//	            resource: A,
//	        },
//	    },
//	}
//
// It costs a bit more to traverse this list than it would to flatten all parents into a single slice/set at graph
// construction time, but such a construction would be at least O(N^2) in the number of resources, since for each of N
// resources we might have to walk and copy a slice with O(N) elements (consider a graph where each resource is the
// child of the one that precedes it, for instance).
type parentList struct {
	// The resource at the head of this list, whose parents are represented as next, next.next, and so on.
	resource *resource.State

	// The next parent in the list.
	next *parentList
}

// childForest encodes the children of a resource. The children of a resource are structured as a forest of trees. The
// trees of the forest are rooted by the direct children of the resource, with their (transitive) children represented
// as subtrees. For example, given the set of resources (where a line represents a parent/child relationship):
//
//	A
//	├── B
//	│   ├── C
//	│   └── D
//	│       └── E
//	└── F
//
// We could represent the children of A as:
//
//	&childForest{
//	    trees: []*childTree{
//	        &childTree{
//	            resource: B,
//	            children: []*childTree{
//	                &childTree{
//	                    resource: C,
//	                },
//	                &childTree{
//	                    resource: D,
//	                    children: []*childTree{
//	                        &childTree{
//	                            resource: E,
//	                        },
//	                    },
//	                },
//	            },
//	        },
//	        &childTree{
//	            resource: F,
//	        },
//	    },
//	}
//
// It costs a bit more to traverse this tree than it would to flatten all children into a single slice/set at graph
// construction time, but such a construction would be at least O(N^2) in the number of resources, since for each of N
// resources we might have to walk and copy a slice with O(N) elements (consider a graph where each resource is the
// parent of the one that follows it, for instance).
type childForest struct {
	// The trees in this forest.
	trees []*childTree
}

// childTree is the type of trees of child dependencies. Each node in the tree points to a resource and subtrees
// representing the (transitive) children of that resource.
type childTree struct {
	// The index of the resource at the root of this tree in the snapshot.
	index int

	// The resource at the root of this tree, whose children are represented as subtrees.
	resource *resource.State

	// The children of the resource at the root of this tree, represented as subtrees.
	children []*childTree
}
