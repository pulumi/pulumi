// Copyright 2016 Marapongo, Inc. All rights reserved.

package graph

import (
	"github.com/marapongo/mu/pkg/tokens"
)

// Node is a single node within an overall MuGL graph.
type Node interface {
	Type() tokens.Type                            // the type of the node.
	Properties() map[tokens.Variable]NodeProperty // a complete set of properties, known and unknown.
	Conditional() bool                            // true if this node may or may not be part of the true graph.
	Edges() []Node                                // other nodes that this node depends upon.
}

// NodeProperty represents a single property associated with a node.
type NodeProperty interface {
	Name() tokens.Variable // the property's name.
	Type() tokens.Type     // the type of this property's value.
	Value() *interface{}   // the value of this property, or nil if unknown.
	Computed() bool        // true if this property's value is unknown because it will be computed.
	Conditional() bool     // true if this property's value is unknown because it requires conditional evaluation.
}
