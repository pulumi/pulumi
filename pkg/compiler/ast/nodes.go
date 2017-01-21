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
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Node is a discriminated type for all serialized blocks and instructions.
type Node interface {
	nd()
	GetKind() NodeKind                       // the node kind.
	GetLoc() *Location                       // an optional location associated with this node.
	Where() (*diag.Document, *diag.Location) // source location information for this node.
}

var _ diag.Diagable = (Node)(nil)

// NodeKind is a type discriminator, indicating what sort of kind a node instance represents.  Note that RTTI frequently
// takes its place, however (a) the kind is part of the serialized form, and (b) can be useful for debugging.
type NodeKind string

type node struct {
	Kind NodeKind  `json:"kind"`
	Loc  *Location `json:"loc,omitempty"`
}

var _ diag.Diagable = (*node)(nil)

func (node *node) nd()               {}
func (node *node) GetKind() NodeKind { return node.Kind }
func (node *node) GetLoc() *Location { return node.Loc }

func (node *node) Where() (*diag.Document, *diag.Location) {
	// TODO: consider caching Document objects; allocating one per Node is wasteful.
	// TODO: for development scenarios, it would be really great to recover the original source file text for purposes
	//     of the diag.Document part.  Doing so would give nice error messages tied back to the original source code
	//     for any errors associated with the AST.  Absent that, we will simply return nil.
	if node.Loc == nil {
		return nil, nil
	} else {
		var doc *diag.Document
		if node.Loc.File != nil {
			doc = diag.NewDocument(*node.Loc.File)
		}
		var end *diag.Pos
		if node.Loc.End != nil {
			end = &diag.Pos{int(node.Loc.End.Line), int(node.Loc.End.Column)}
		}
		return doc, &diag.Location{
			diag.Pos{int(node.Loc.Start.Line), int(node.Loc.Start.Column)}, end,
		}
	}
}

// Identifier represents a simple string token associated with its source location context.
type Identifier struct {
	node
	Ident tokens.Name `json:"ident"` // a valid identifier: (letter | "_") (letter | digit | "_")*
}

const IdentifierKind NodeKind = "Identifier"
