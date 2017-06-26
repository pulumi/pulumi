// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

// Package ast contains the core LumiIL AST types.  These contain fully qualified tokens ready to resolve to packages.
//
// All ASTs are fully serializable.  They require custom (de)serialization, however, due to the use of discriminated
// AST node types.  This is in contrast to the package metadata which is simple enough for trivial (de)serialization.
//
// During this binding process, we mutate nodes in place, rather than taking the performance hit of immutability.  This
// is a controversial decision and can introduce some subtleties for all the usual mutable state reasons, however, it is
// a simpler and more performant approach, and we can revisit it down the road if needed.
package ast

import (
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
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

type NodeValue struct {
	Kind NodeKind  `json:"kind"`
	Loc  *Location `json:"loc,omitempty"`
}

var _ diag.Diagable = (*NodeValue)(nil)

func (node *NodeValue) nd()               {}
func (node *NodeValue) GetKind() NodeKind { return node.Kind }
func (node *NodeValue) GetLoc() *Location { return node.Loc }

func (node *NodeValue) Where() (*diag.Document, *diag.Location) {
	// IDEA: consider caching Document objects; allocating one per Node is wasteful.
	// IDEA[pulumi/lumi#15]: for development scenarios, it would be really great to recover the original source file
	//     text for purposes of the diag.Document part.  Doing so would give nice error messages tied back to the
	//     original source code for any errors associated with the AST.  Absent that, we will simply return nil.
	if node.Loc == nil {
		return nil, nil
	}

	var doc *diag.Document
	if node.Loc.File != nil {
		doc = diag.NewDocument(*node.Loc.File)
	}
	var end *diag.Pos
	if node.Loc.End != nil {
		end = &diag.Pos{Line: int(node.Loc.End.Line), Column: int(node.Loc.End.Column)}
	}
	return doc, &diag.Location{
		Start: diag.Pos{
			Line:   int(node.Loc.Start.Line),
			Column: int(node.Loc.Start.Column),
		},
		End: end,
	}
}

// Identifier represents a simple string name associated with its source location context.
type Identifier struct {
	NodeValue
	Ident tokens.Name `json:"ident"` // a valid identifier: (letter | "_") (letter | digit | "_")*
}

var _ Node = (*Identifier)(nil)

const IdentifierKind NodeKind = "Identifier"

// Token represents a real string type token associated with its source location context.
type Token struct {
	NodeValue
	Tok tokens.Token `json:"tok"`
}

var _ Node = (*Token)(nil)

const TokenKind NodeKind = "Token"

// ClassMemberToken represents a real string class member token associated with its source location context.
type ClassMemberToken struct {
	NodeValue
	Tok tokens.ClassMember `json:"tok"`
}

var _ Node = (*ClassMemberToken)(nil)

const ClassMemberTokenKind NodeKind = "ClassMemberToken"

// ModuleToken represents a real string type token associated with its source location context.
type ModuleToken struct {
	NodeValue
	Tok tokens.Module `json:"tok"`
}

var _ Node = (*ModuleToken)(nil)

const ModuleTokenKind NodeKind = "ModuleToken"

// TypeToken represents a real string module token associated with its source location context.
type TypeToken struct {
	NodeValue
	Tok tokens.Type `json:"tok"`
}

var _ Node = (*TypeToken)(nil)

const TypeTokenKind NodeKind = "TypeToken"
