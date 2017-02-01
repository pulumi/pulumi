// Copyright 2016 Marapongo, Inc. All rights reserved.

package graphgen

import (
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/graph"
)

type objectGraph struct {
	roots []graph.Vertex
}

var _ graph.Graph = (*objectGraph)(nil)

func newObjectGraph(roots []graph.Vertex) *objectGraph {
	return &objectGraph{roots: roots}
}

func (v *objectGraph) Roots() []graph.Vertex { return v.roots }

type objectVertex struct {
	obj  *rt.Object     // this vertex's object.
	ins  []graph.Vertex // edges connecting from other vertices into this vertex.
	outs []graph.Vertex // edges connecting this vertex to other vertices.
}

var _ graph.Vertex = (*objectVertex)(nil)

func newObjectVertex(obj *rt.Object) *objectVertex {
	return &objectVertex{obj: obj}
}

func (v *objectVertex) Obj() *rt.Object      { return v.obj }
func (v *objectVertex) Ins() []graph.Vertex  { return v.ins }
func (v *objectVertex) Outs() []graph.Vertex { return v.outs }

// AddEdge creates an edge connecting the receiver vertex to the argument vertex.
func (v *objectVertex) AddEdge(to *objectVertex) {
	v.outs = append(v.outs, to) // outgoing from this vertex to the other.
	to.ins = append(to.ins, v)  // incoming from this vertex to the other.
}
