// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"os"

	"github.com/pulumi/pulumi/pkg/graph"
	"github.com/pulumi/pulumi/pkg/graph/dotconv"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/spf13/cobra"
)

func newStackGraphCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "graph",
		Args:  cmdutil.ExactArgs(1),
		Short: "Export a stack's dependency graph to a file",
		Long: "Export a stack's dependency graph to a file.\n" +
			"\n" +
			"This command can be used to view the dependency graph that a Pulumi program\n" +
			"admitted when it was ran. This graph is output in the DOT format. This command operates\n" +
			"on your stack's most recent deployment.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			s, err := requireCurrentStack(false)
			if err != nil {
				return err
			}

			dg := makeDependencyGraph(s.Snapshot())
			file, err := os.Create(args[0])
			if err != nil {
				return err
			}

			if err := dotconv.Print(dg, file); err != nil {
				_ = file.Close()
				return err
			}

			cmd.Printf("%sWrote stack dependency graph to `%s`", cmdutil.EmojiOr("🔍 ", ""), args[0])
			cmd.Println()
			return file.Close()
		}),
	}
}

// All of the types and code within this file are to provide implementations of the interfaces
// in the `graph` package, so that we can use the `dotconv` package to output our graph in the
// DOT format.
//
// `dependencyEdge` implements graph.Edge, `dependencyVertex` implements graph.Vertex, and
// `dependencyGraph` implements `graph.Graph`.
type dependencyEdge struct {
	to   *dependencyVertex
	from *dependencyVertex
}

// In this simple case, edges have no data.
func (edge *dependencyEdge) Data() interface{} {
	return nil
}

// In this simple case, edges have no label.
func (edge *dependencyEdge) Label() string {
	return ""
}

func (edge *dependencyEdge) To() graph.Vertex {
	return edge.to
}

func (edge *dependencyEdge) From() graph.Vertex {
	return edge.from
}

// A dependencyVertex contains a reference to the graph to which it belongs
// and to the resource state that it represents. Incoming and outgoing edges
// are calculated on-demand using the combination of the graph and the state.
type dependencyVertex struct {
	graph         *dependencyGraph
	resource      *resource.State
	incomingEdges []graph.Edge
	outgoingEdges []graph.Edge
}

func (vertex *dependencyVertex) Data() interface{} {
	return vertex.resource
}

func (vertex *dependencyVertex) Label() string {
	return string(vertex.resource.URN)
}

func (vertex *dependencyVertex) Ins() []graph.Edge {
	return vertex.incomingEdges
}

// Outgoing edges are indirectly calculated by traversing the entire graph looking
// for edges that point to this vertex. This is slow, but our graphs aren't big enough
// for this to matter too much.
func (vertex *dependencyVertex) Outs() []graph.Edge {
	return vertex.outgoingEdges
}

// A dependencyGraph is a thin wrapper around a map of URNs to vertices in
// the graph. It is constructed directly from a snapshot.
type dependencyGraph struct {
	vertices map[resource.URN]*dependencyVertex
}

// Roots are edges that point to the root set of our graph. In our case,
// for simplicity, we define the root set of our dependency graph to be resources
// that have no incoming edges.
func (dg *dependencyGraph) Roots() []graph.Edge {
	rootEdges := make([]graph.Edge, 0)
	for _, vertex := range dg.vertices {
		if len(vertex.Ins()) == 0 {
			edge := &dependencyEdge{
				to:   vertex,
				from: nil,
			}

			rootEdges = append(rootEdges, edge)
		}
	}

	return rootEdges
}

// Makes a dependency graph from a deployment snapshot, allocating a vertex
// for every resource in the graph.
func makeDependencyGraph(snapshot *deploy.Snapshot) *dependencyGraph {
	dg := &dependencyGraph{
		vertices: make(map[resource.URN]*dependencyVertex),
	}

	for _, resource := range snapshot.Resources {
		vertex := &dependencyVertex{
			graph:    dg,
			resource: resource,
		}

		dg.vertices[resource.URN] = vertex
	}

	for _, vertex := range dg.vertices {
		// Incoming edges are directly stored within the checkpoint file; they represent
		// resources on which this vertex immediately depends upon.
		for _, dep := range vertex.resource.Dependencies {
			vertexWeDependOn := vertex.graph.vertices[dep]
			vertex.incomingEdges = append(vertex.incomingEdges, &dependencyEdge{
				to:   vertex,
				from: vertexWeDependOn,
			})

			vertexWeDependOn.outgoingEdges = append(vertexWeDependOn.outgoingEdges, &dependencyEdge{
				to:   vertex,
				from: vertexWeDependOn,
			})
		}
	}

	return dg
}
