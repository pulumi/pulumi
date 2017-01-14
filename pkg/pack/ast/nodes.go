// Copyright 2016 Marapongo, Inc. All rights reserved.

// This package contains the core MuIL AST types.  These contain fully qualified tokens ready to resolve to packages.
//
// All ASTs are fully serializable.  They require custom (de)serialization, however, due to the use of discriminated
// AST node types.  This is in contrast to the package metadata which is simple enough for trivial (de)serialization.
//
// During this binding process, we mutate nodes in place, rather than taking the performance hit of immutability.  This
// is a controversial decision and can introduce some subtleties for all the usual mutable state reasons, however, it is
// a simpler and more performant approach, and we can revisit it down the road if needed.
package ast

import (
	"github.com/marapongo/mu/pkg/pack/symbols"
)

// Node is a discriminated type for all serialized blocks and instructions.
type Node interface {
	nd()
	GetKind() string   // the node kind.
	GetLoc() *Location // an optional location associated with this node.
}

type node struct {
	Kind string    `json:"kind"`
	Loc  *Location `json:"loc,omitempty"`
}

func (node *node) nd()               {}
func (node *node) GetKind() string   { return node.Kind }
func (node *node) GetLoc() *Location { return node.Loc }

// Identifier represents a simple string token associated with its source location context.
type Identifier struct {
	node
	Ident symbols.Token `json:"ident"` // a valid identifier: (letter | "_") (letter | digit | "_")*
}
