// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"github.com/pkg/errors"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/graph"
	"github.com/pulumi/pulumi/pkg/v3/graph/dotconv"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

// Whether or not we should ignore parent edges when building up our graph.
var ignoreParentEdges bool

// Whether or not we should ignore dependency edges when building up our graph.
var ignoreDependencyEdges bool

// The color of dependency edges in the graph. Defaults to #246C60, a blush-green.
var dependencyEdgeColor string

// The color of parent edges in the graph. Defaults to #AA6639, an orange.
var parentEdgeColor string

func newStackGraphCmd() *cobra.Command {
	var stackName string

	cmd := &cobra.Command{
		Use:   "graph [filename]",
		Args:  cmdutil.ExactArgs(1),
		Short: "Export a stack's dependency graph to a file",
		Long: "Export a stack's dependency graph to a file.\n" +
			"\n" +
			"This command can be used to view the dependency graph that a Pulumi program\n" +
			"admitted when it was ran. This graph is output in the DOT format. This command operates\n" +
			"on your stack's most recent deployment.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			s, err := requireStack(stackName, false, opts, false /*setCurrent*/)
			if err != nil {
				return err
			}
			snap, err := s.Snapshot(commandContext())
			if err != nil {
				return err
			}

			// This will prevent a panic when trying to assemble a dependencyGraph when no snapshot is found
			if snap == nil {
				return errors.Errorf("unable to find snapshot for stack %q", stackName)
			}

			dg := makeDependencyGraph(snap)
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
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVar(&ignoreParentEdges, "ignore-parent-edges", false,
		"Ignores edges introduced by parent/child resource relationships")
	cmd.PersistentFlags().BoolVar(&ignoreDependencyEdges, "ignore-dependency-edges", false,
		"Ignores edges introduced by dependency resource relationships")
	cmd.PersistentFlags().StringVar(&dependencyEdgeColor, "dependency-edge-color", "#246C60",
		"Sets the color of dependency edges in the graph")
	cmd.PersistentFlags().StringVar(&parentEdgeColor, "parent-edge-color", "#AA6639",
		"Sets the color of parent edges in the graph")
	return cmd
}

// All of the types and code within this file are to provide implementations of the interfaces
// in the `graph` package, so that we can use the `dotconv` package to output our graph in the
// DOT format.
//
// `dependencyEdge` implements graph.Edge, `dependencyVertex` implements graph.Vertex, and
// `dependencyGraph` implements `graph.Graph`.
type dependencyEdge struct {
	to     *dependencyVertex
	from   *dependencyVertex
	labels []string
}

// In this simple case, edges have no data.
func (edge *dependencyEdge) Data() interface{} {
	return nil
}

func (edge *dependencyEdge) Label() string {
	return strings.Join(edge.labels, ", ")
}

func (edge *dependencyEdge) To() graph.Vertex {
	return edge.to
}

func (edge *dependencyEdge) From() graph.Vertex {
	return edge.from
}

func (edge *dependencyEdge) Color() string {
	return dependencyEdgeColor
}

// parentEdges represent edges in the parent-child graph, which
// exists alongside the dependency graph. An edge exists from node
// A to node B if node B is considered to be a parent of node A.
type parentEdge struct {
	to   *dependencyVertex
	from *dependencyVertex
}

func (edge *parentEdge) Data() interface{} {
	return nil
}

// In this simple case, edges have no label.
func (edge *parentEdge) Label() string {
	return ""
}

func (edge *parentEdge) To() graph.Vertex {
	return edge.to
}

func (edge *parentEdge) From() graph.Vertex {
	return edge.from
}

func (edge *parentEdge) Color() string {
	return parentEdgeColor
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
// for simplicity, we define the root set of our dependency graph to be everything.
func (dg *dependencyGraph) Roots() []graph.Edge {
	rootEdges := []graph.Edge{}
	for _, vertex := range dg.vertices {
		edge := &dependencyEdge{
			to:   vertex,
			from: nil,
		}

		rootEdges = append(rootEdges, edge)
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
		if !ignoreDependencyEdges {
			// If we have per-property dependency information, annotate the dependency edges
			// we generate with the names of the properties associated with each dependency.
			depBlame := make(map[resource.URN][]string)
			for k, deps := range vertex.resource.PropertyDependencies {
				for _, dep := range deps {
					depBlame[dep] = append(depBlame[dep], string(k))
				}
			}

			// Incoming edges are directly stored within the checkpoint file; they represent
			// resources on which this vertex immediately depends upon.
			for _, dep := range vertex.resource.Dependencies {
				vertexWeDependOn := vertex.graph.vertices[dep]
				edge := &dependencyEdge{to: vertex, from: vertexWeDependOn, labels: depBlame[dep]}
				vertex.incomingEdges = append(vertex.incomingEdges, edge)
				vertexWeDependOn.outgoingEdges = append(vertexWeDependOn.outgoingEdges, edge)
			}
		}

		// alongside the dependency graph sits the resource parentage graph, which
		// is also displayed as part of this graph, although with different colored
		// edges.
		if !ignoreParentEdges {
			if parent := vertex.resource.Parent; parent != resource.URN("") {
				parentVertex := dg.vertices[parent]
				vertex.outgoingEdges = append(vertex.outgoingEdges, &parentEdge{
					to:   parentVertex,
					from: vertex,
				})
			}
		}
	}

	return dg
}
