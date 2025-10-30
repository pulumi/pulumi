package graph

import graph "github.com/pulumi/pulumi/sdk/v3/pkg/graph"

// Graph is an instance of a resource digraph.  Each is associated with a single program input, along
// with a set of optional arguments used to evaluate it, along with the output DAG with node types and properties.
type Graph = graph.Graph

// Vertex is a single vertex within an overall resource graph.
type Vertex = graph.Vertex

// Edge is a directed edge from one vertex to another.
type Edge = graph.Edge

