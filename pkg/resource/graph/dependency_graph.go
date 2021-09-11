// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package graph

import (
	"sort"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type edgeKind int

const (
	edgeKindDeclared edgeKind = iota
	edgeKindParent
	edgeKindProvider
	edgeKindComponent
)

func (k edgeKind) String() string {
	switch k {
	case edgeKindDeclared:
		return "declared"
	case edgeKindParent:
		return "parent"
	case edgeKindProvider:
		return "provider"
	case edgeKindComponent:
		return "component"
	default:
		return "<unknown>"
	}
}

type edge struct {
	kind   edgeKind
	target *node
}

type node struct {
	index         int
	resource      *resource.State
	outgoingEdges []edge
	incomingEdges []edge
}

func isValidParentURN(urn resource.URN) bool {
	return urn != "" && urn.Type() != resource.RootStackType
}

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph struct {
	nodes map[resource.URN]*node // A mapping of resource states to nodes.
}

func (dg *DependencyGraph) hasPath(source, sink *node) bool {
	for _, edge := range source.outgoingEdges {
		if edge.target == sink || dg.hasPath(edge.target, sink) {
			return true
		}
	}
	return false
}

func (dg *DependencyGraph) dependingOn(set ResourceSet, list *[]*resource.State, res *resource.State,
	ignore map[resource.URN]bool) {

	resourceNode := dg.nodes[res.URN]
	contract.Assert(resourceNode != nil)

	for _, dependentEdge := range resourceNode.incomingEdges {
		dependent := dependentEdge.target.resource
		if !ignore[dependent.URN] && !set[dependent] {
			set[dependent] = true
			*list = append(*list, dependent)
			dg.dependingOn(set, list, dependent, ignore)
		}
	}
}

// DependingOn returns a slice containing all resources that directly or indirectly
// depend upon the given resource. The returned slice is guaranteed to be in topological
// order with respect to the snapshot dependency graph.
//
// The time complexity of DependingOn is linear with respect to the number of resources.
func (dg *DependencyGraph) DependingOn(res *resource.State, ignore map[resource.URN]bool) []*resource.State {
	var list []*resource.State
	dg.dependingOn(ResourceSet{}, &list, res, ignore)

	sort.Slice(list, func(i, j int) bool {
		resI, resJ := list[i], list[j]
		return dg.nodes[resI.URN].index < dg.nodes[resJ.URN].index
	})

	return list
}

// DependenciesOf returns a ResourceSet of resources upon which the given resource depends.
//
// This set only includes the immediate dependencies for a resource--not the transitiec dependencies. This includes:
//
// - Resources listed in resource.Dependencies, including the descendents of any component resources listed therein
// - The parent and provider for the resource, if any
//
func (dg *DependencyGraph) DependenciesOf(res *resource.State) ResourceSet {
	set := make(ResourceSet)

	resourceNode := dg.nodes[res.URN]
	contract.Assert(resourceNode != nil)

	for _, dependencyEdge := range resourceNode.outgoingEdges {
		set[dependencyEdge.target.resource] = true
	}
	return set
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
//
// The resources must be in topological order with respect to their declared dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	nodes := map[resource.URN]*node{}

	addEdge := func(source, sink *node, kind edgeKind) {
		source.outgoingEdges = append(source.outgoingEdges, edge{kind: kind, target: sink})
		sink.incomingEdges = append(sink.incomingEdges, edge{kind: kind, target: source})
	}

	addEdgeToURN := func(source *node, sinkURN resource.URN, kind edgeKind) {
		sinkNode := nodes[sinkURN]
		contract.Assert(sinkNode != nil)

		addEdge(source, sinkNode, kind)
	}

	// Populate the nodes and add direct dependency edges. Parent edges are added after component expansion to avoid
	// false dependencies.
	for i, res := range resources {
		resourceNode := &node{
			index:         i,
			resource:      res,
			outgoingEdges: make([]edge, 0, len(res.Dependencies)),
		}
		nodes[res.URN] = resourceNode

		for _, dependencyURN := range res.Dependencies {
			addEdgeToURN(resourceNode, dependencyURN, edgeKindDeclared)
		}

		if res.Parent != "" {
			addEdgeToURN(resourceNode, res.Parent, edgeKindParent)
		}

		if res.Provider != "" {
			ref, err := providers.ParseReference(res.Provider)
			contract.Assert(err == nil)

			addEdgeToURN(resourceNode, ref.URN(), edgeKindProvider)
		}
	}

	dg := &DependencyGraph{nodes: nodes}

	// Expand dependencies on components into dependencies on the component's descendents. Only add edges that would not
	// create cycles in the dependency graph.
	//
	// This expansion works by looping over all of the resources in the graph and adding edges between each resource
	// and the resources that have a declared dependency on its ancestors.
	for _, res := range resources {
		descendentNode := nodes[res.URN]
		contract.Assert(descendentNode != nil)

		parent := res.Parent
		for isValidParentURN(parent) {
			parentNode := nodes[parent]
			contract.Assert(parentNode != nil)

			if parentNode.resource.Custom {
				break
			}

			for _, dependentEdge := range parentNode.incomingEdges {
				if dependentEdge.kind == edgeKindParent {
					continue
				}

				dependentNode := dependentEdge.target
				if dependentNode != descendentNode && !dg.hasPath(descendentNode, dependentNode) {
					addEdge(dependentNode, descendentNode, edgeKindComponent)
				}
			}

			parent = parentNode.resource.Parent
		}
	}

	return dg
}
