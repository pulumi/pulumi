// Copyright 2016 Marapongo, Inc. All rights reserved.

package graph

import (
	"github.com/marapongo/mu/pkg/tokens"
)

// Vertex is a single vertex within an overall MuGL graph.
type Vertex interface {
	Type() tokens.Type                          // the type of the node.
	Properties() map[tokens.Name]VertexProperty // a complete set of properties, known and unknown.
	Edges() []Vertex                            // other nodes that this node depends upon.
}

// VertexProperty represents a single property associated with a node.
type VertexProperty interface {
	Name() tokens.Variable // the property's name.
	Type() tokens.Type     // the type of this property's value.
	Value() *interface{}   // the value of this property, or nil if unknown.
	Computed() bool        // true if this property's value is unknown because it will be computed.
}
