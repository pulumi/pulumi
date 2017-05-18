// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package heapstate

import (
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/graph"
)

type ObjectGraph struct {
	objs  []*ObjectEdge
	roots []graph.Edge
}

var _ graph.Graph = (*ObjectGraph)(nil)

func NewObjectGraph(objs []*ObjectEdge) *ObjectGraph {
	roots := make([]graph.Edge, len(objs))
	for i, root := range objs {
		roots[i] = root
	}
	return &ObjectGraph{
		objs:  objs,
		roots: roots,
	}
}

func (v *ObjectGraph) Objs() []*ObjectEdge { return v.objs }
func (v *ObjectGraph) Roots() []graph.Edge { return v.roots }

type ObjectVertex struct {
	obj     *rt.Object   // this vertex's object.
	ins     []graph.Edge // edges connecting from other vertices into this vertex.
	inobjs  []*ObjectEdge
	outs    []graph.Edge // edges connecting this vertex to other vertices.
	outobjs []*ObjectEdge
}

var _ graph.Vertex = (*ObjectVertex)(nil)

func NewObjectVertex(obj *rt.Object) *ObjectVertex {
	return &ObjectVertex{obj: obj}
}

func (v *ObjectVertex) Obj() *rt.Object        { return v.obj }
func (v *ObjectVertex) Data() interface{}      { return v.obj }
func (v *ObjectVertex) Label() string          { return string(v.obj.Type().Token()) }
func (v *ObjectVertex) Ins() []graph.Edge      { return v.ins }
func (v *ObjectVertex) InObjs() []*ObjectEdge  { return v.inobjs }
func (v *ObjectVertex) Outs() []graph.Edge     { return v.outs }
func (v *ObjectVertex) OutObjs() []*ObjectEdge { return v.outobjs }

// ConnectTo creates an edge connecting the receiver vertex to the argument vertex.
func (v *ObjectVertex) ConnectTo(to *ObjectVertex) {
	e := NewObjectEdge(v, to)
	v.outs = append(v.outs, e) // outgoing from this vertex to the other.
	v.outobjs = append(v.outobjs, e)
	to.ins = append(to.ins, e) // incoming from this vertex to the other.
	to.inobjs = append(to.inobjs, e)
}

type ObjectEdge struct {
	to   *ObjectVertex // the vertex this edge connects to.
	from *ObjectVertex // the vertex this edge connects from.
}

var _ graph.Edge = (*ObjectEdge)(nil)

func NewObjectEdge(from *ObjectVertex, to *ObjectVertex) *ObjectEdge {
	return &ObjectEdge{from: from, to: to}
}

func (e *ObjectEdge) Data() interface{}    { return nil }
func (e *ObjectEdge) Label() string        { return "" }
func (e *ObjectEdge) To() graph.Vertex     { return e.to }
func (e *ObjectEdge) ToObj() *ObjectVertex { return e.to }
func (e *ObjectEdge) From() graph.Vertex {
	if e.from == nil {
		return nil
	}
	return e.from
}
func (e *ObjectEdge) FromObj() *ObjectVertex { return e.from }
