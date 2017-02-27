// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"github.com/pulumi/coconut/pkg/graph"
	"github.com/pulumi/coconut/pkg/util/contract"
)

type resourceGraph struct {
	objs  []*resourceEdge
	roots []graph.Edge
}

var _ graph.Graph = (*resourceGraph)(nil)

// newResourceGraph produces a DAG using the resources' properties embedded moniker information.
func newResourceGraph(resources []Resource) *resourceGraph {
	// First make two maps: one with monikers to resources, the other with resources to vertices.
	mks := make(map[Moniker]Resource)
	verts := make(map[Resource]*resourceVertex)
	for _, res := range resources {
		contract.Assert(res != nil)
		m := res.Moniker()
		contract.Assertf(mks[m] == nil, "Unexpected duplicate entry '%v' in resource list", m)
		mks[m] = res
		verts[res] = newResourceVertex(res)
	}

	// Now walk the list of resources and connect them to their dependencies.
	for _, res := range resources {
		m := res.Moniker()
		fromv := verts[res]
		for ref := range res.Properties().AllResources() {
			to := mks[ref]
			contract.Assertf(to != nil, "Missing resource for target; from=%v to=%v", m, ref)
			tov := verts[to]
			contract.Assertf(tov != nil, "Missing vertex entry for target; from=%v to=%v", m, ref)
			fromv.connectTo(tov)
		}
	}

	// For all vertices with no ins, make them root nodes.
	var objs []*resourceEdge
	for _, v := range verts {
		if len(v.Ins()) == 0 {
			objs = append(objs, &resourceEdge{to: v})
		}
	}

	roots := make([]graph.Edge, len(objs))
	for i, root := range objs {
		roots[i] = root
	}
	return &resourceGraph{
		objs:  objs,
		roots: roots,
	}
}

func (v *resourceGraph) Objs() []*resourceEdge { return v.objs }
func (v *resourceGraph) Roots() []graph.Edge   { return v.roots }

type resourceVertex struct {
	res    Resource     // this vertex's resource.
	ins    []graph.Edge // edges connecting from other vertices into this vertex.
	inres  []*resourceEdge
	outs   []graph.Edge // edges connecting this vertex to other vertices.
	outres []*resourceEdge
}

var _ graph.Vertex = (*resourceVertex)(nil)

func newResourceVertex(res Resource) *resourceVertex {
	return &resourceVertex{res: res}
}

func (v *resourceVertex) Resource() Resource      { return v.res }
func (v *resourceVertex) Data() interface{}       { return v.res }
func (v *resourceVertex) Label() string           { return "" }
func (v *resourceVertex) Ins() []graph.Edge       { return v.ins }
func (v *resourceVertex) InRes() []*resourceEdge  { return v.inres }
func (v *resourceVertex) Outs() []graph.Edge      { return v.outs }
func (v *resourceVertex) OutRes() []*resourceEdge { return v.outres }

// connectTo creates an edge connecting the receiver vertex to the argument vertex.
func (v *resourceVertex) connectTo(to *resourceVertex) {
	e := newResourceEdge(v, to)
	v.outs = append(v.outs, e) // outgoing from this vertex to the other.
	v.outres = append(v.outres, e)
	to.ins = append(to.ins, e) // incoming from this vertex to the other.
	to.inres = append(to.inres, e)
}

type resourceEdge struct {
	to   *resourceVertex // the vertex this edge connects to.
	from *resourceVertex // the vertex this edge connects from.
}

var _ graph.Edge = (*resourceEdge)(nil)

func newResourceEdge(from *resourceVertex, to *resourceVertex) *resourceEdge {
	return &resourceEdge{from: from, to: to}
}

func (e *resourceEdge) Data() interface{}       { return nil }
func (e *resourceEdge) Label() string           { return "" }
func (e *resourceEdge) To() graph.Vertex        { return e.to }
func (e *resourceEdge) ToStep() *resourceVertex { return e.to }
func (e *resourceEdge) From() graph.Vertex {
	if e.from == nil {
		return nil
	}
	return e.from
}
func (e *resourceEdge) FromObj() *resourceVertex { return e.from }
