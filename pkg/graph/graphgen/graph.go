// Copyright 2016 Marapongo, Inc. All rights reserved.

package graphgen

import (
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/graph"
)

type objectGraph struct {
	roots []graph.Edge
}

var _ graph.Graph = (*objectGraph)(nil)

func newObjectGraph(roots []graph.Edge) *objectGraph {
	return &objectGraph{roots: roots}
}

func (v *objectGraph) Roots() []graph.Edge { return v.roots }

type objectVertex struct {
	obj  *rt.Object   // this vertex's object.
	ins  []graph.Edge // edges connecting from other vertices into this vertex.
	outs []graph.Edge // edges connecting this vertex to other vertices.
}

var _ graph.Vertex = (*objectVertex)(nil)

func newObjectVertex(obj *rt.Object) *objectVertex {
	return &objectVertex{obj: obj}
}

func (v *objectVertex) Obj() *rt.Object    { return v.obj }
func (v *objectVertex) Ins() []graph.Edge  { return v.ins }
func (v *objectVertex) Outs() []graph.Edge { return v.outs }

// AddEdge creates an edge connecting the receiver vertex to the argument vertex.
func (v *objectVertex) AddEdge(to *objectVertex) {
	e := newObjectEdge(v, to)
	v.outs = append(v.outs, e) // outgoing from this vertex to the other.
	to.ins = append(to.ins, e) // incoming from this vertex to the other.
}

type objectEdge struct {
	to   *objectVertex // the vertex this edge connects to.
	from *objectVertex // the vertex this edge connects from.
}

var _ graph.Edge = (*objectEdge)(nil)

func newObjectEdge(from *objectVertex, to *objectVertex) *objectEdge {
	return &objectEdge{from: from, to: to}
}

func (e *objectEdge) To() graph.Vertex { return e.to }
func (e *objectEdge) From() graph.Vertex {
	if e.from == nil {
		return nil
	}
	return e.from
}
