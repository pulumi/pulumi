// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"github.com/marapongo/mu/pkg/graph"
)

type planGraph struct {
	plan  *plan
	objs  []*planEdge
	roots []graph.Edge
}

var _ graph.Graph = (*planGraph)(nil)

func newPlanGraph(plan *plan, objs []*planEdge) *planGraph {
	roots := make([]graph.Edge, len(objs))
	for i, root := range objs {
		roots[i] = root
	}
	return &planGraph{
		objs:  objs,
		roots: roots,
	}
}

func (v *planGraph) Plan() *plan         { return v.plan }
func (v *planGraph) Objs() []*planEdge   { return v.objs }
func (v *planGraph) Roots() []graph.Edge { return v.roots }

type planVertex struct {
	step     *step        // this vertex's step.
	ins      []graph.Edge // edges connecting from other vertices into this vertex.
	insteps  []*planEdge
	outs     []graph.Edge // edges connecting this vertex to other vertices.
	outsteps []*planEdge
}

var _ graph.Vertex = (*planVertex)(nil)

func newPlanVertex(step *step) *planVertex {
	return &planVertex{step: step}
}

func (v *planVertex) Step() *step           { return v.step }
func (v *planVertex) Data() interface{}     { return v.step }
func (v *planVertex) Label() string         { return "" }
func (v *planVertex) Ins() []graph.Edge     { return v.ins }
func (v *planVertex) InSteps() []*planEdge  { return v.insteps }
func (v *planVertex) Outs() []graph.Edge    { return v.outs }
func (v *planVertex) OutSteps() []*planEdge { return v.outsteps }

// connectTo creates an edge connecting the receiver vertex to the argument vertex.
func (v *planVertex) connectTo(to *planVertex) {
	e := newPlanEdge(v, to)
	v.outs = append(v.outs, e) // outgoing from this vertex to the other.
	v.outsteps = append(v.outsteps, e)
	to.ins = append(to.ins, e) // incoming from this vertex to the other.
	to.insteps = append(to.insteps, e)
}

type planEdge struct {
	to   *planVertex // the vertex this edge connects to.
	from *planVertex // the vertex this edge connects from.
}

var _ graph.Edge = (*planEdge)(nil)

func newPlanEdge(from *planVertex, to *planVertex) *planEdge {
	return &planEdge{from: from, to: to}
}

func (e *planEdge) Data() interface{}   { return nil }
func (e *planEdge) Label() string       { return "" }
func (e *planEdge) To() graph.Vertex    { return e.to }
func (e *planEdge) ToStep() *planVertex { return e.to }
func (e *planEdge) From() graph.Vertex {
	if e.from == nil {
		return nil
	}
	return e.from
}
func (e *planEdge) FromObj() *planVertex { return e.from }
