package graph

import graph "github.com/pulumi/pulumi/sdk/v3/pkg/resource/graph"

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph = graph.DependencyGraph

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
// The resources should be in topological order with respect to their dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	return graph.NewDependencyGraph(resources)
}

