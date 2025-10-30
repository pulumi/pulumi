package graph

import graph "github.com/pulumi/pulumi/sdk/v3/pkg/graph"

// Toposort topologically sorts the graph, yielding an array of nodes that are in dependency order, using a simple
// DFS-based algorithm.  The graph must be acyclic, otherwise this function will return an error.
func Toposort(g Graph) ([]Vertex, error) {
	return graph.Toposort(g)
}

