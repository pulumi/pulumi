// Copyright 2016 Marapongo, Inc. All rights reserved.

package graph

import (
	"github.com/marapongo/mu/pkg/eval/rt"
)

// Vertex is a single vertex within an overall MuGL graph.
type Vertex interface {
	Obj() *rt.Object // the vertex's object.
	Ins() []Vertex   // incoming edges from other vertices within the graph to this vertex.
	Outs() []Vertex  // outgoing edges from this vertex to other vertices within the graph.
}
