// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Class is a fully bound class symbol.
type Class struct {
	Node       *ast.Class
	Extends    *Type
	Implements *Types
	Members    ClassMemberMap
}

var _ Symbol = (*Class)(nil)
var _ Type = (*Class)(nil)
var _ ModuleMember = (*Class)(nil)

func (node *Class) symbol()             {}
func (node *Class) typesym()            {}
func (node *Class) moduleMember()       {}
func (node *Class) Token() tokens.Token { return node.Node.Name.Ident }
func (node *Class) Tree() diag.Diagable { return node.Node }

// ClassMember is a marker interface for things that can be module members.
type ClassMember interface {
	Symbol
	classMember()
}

// ClassMemberMap is a map from a class member's token to its associated symbol.
type ClassMemberMap map[tokens.Token]ClassMember

// ClassProperty is a fully bound module property symbol.
type ClassProperty struct {
	Node *ast.ClassProperty
}

var _ Symbol = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)

func (node *ClassProperty) symbol()             {}
func (node *ClassProperty) classMember()        {}
func (node *ClassProperty) Token() tokens.Token { return node.Node.Name.Ident }
func (node *ClassProperty) Tree() diag.Diagable { return node.Node }

// ClassMethod is a fully bound module method symbol.
type ClassMethod struct {
	Node *ast.ClassMethod
}

var _ Symbol = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

func (node *ClassMethod) symbol()             {}
func (node *ClassMethod) classMember()        {}
func (node *ClassMethod) Token() tokens.Token { return node.Node.Name.Ident }
func (node *ClassMethod) Tree() diag.Diagable { return node.Node }
