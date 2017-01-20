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
	Parent     *Module
	Extends    *Type
	Implements *Types
	Members    ClassMemberMap
}

var _ Symbol = (*Class)(nil)
var _ Type = (*Class)(nil)
var _ ModuleMember = (*Class)(nil)

func (node *Class) symbol()           {}
func (node *Class) typesym()          {}
func (node *Class) moduleMember()     {}
func (node *Class) Name() tokens.Name { return node.Node.Name.Ident }
func (node *Class) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			tokens.ModuleMemberName(node.Name()),
		),
	)
}
func (node *Class) Tree() diag.Diagable { return node.Node }

// NewClassSym returns a new Class symbol with the given node, parent, extends, and implements, and empty members.
func NewClassSym(node *ast.Class, parent *Module, extends *Type, implements *Types) *Class {
	return &Class{
		Node:       node,
		Parent:     parent,
		Extends:    extends,
		Implements: implements,
		Members:    make(ClassMemberMap),
	}
}

// ClassMember is a marker interface for things that can be module members.
type ClassMember interface {
	Symbol
	classMember()
}

// ClassMemberMap is a map from a class member's name to its associated symbol.
type ClassMemberMap map[tokens.ClassMemberName]ClassMember

// ClassProperty is a fully bound module property symbol.
type ClassProperty struct {
	Node   *ast.ClassProperty
	Parent *Class
}

var _ Symbol = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)

func (node *ClassProperty) symbol()           {}
func (node *ClassProperty) classMember()      {}
func (node *ClassProperty) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ClassProperty) Token() tokens.Token {
	return tokens.Token(
		tokens.NewClassMemberToken(
			tokens.ModuleMember(node.Parent.Token()),
			tokens.ClassMemberName(node.Name()),
		),
	)
}
func (node *ClassProperty) Tree() diag.Diagable { return node.Node }

// NewClassPropertySym returns a new ClassProperty symbol with the given node and parent.
func NewClassPropertySym(node *ast.ClassProperty, parent *Class) *ClassProperty {
	return &ClassProperty{
		Node:   node,
		Parent: parent,
	}
}

// ClassMethod is a fully bound module method symbol.
type ClassMethod struct {
	Node   *ast.ClassMethod
	Parent *Class
}

var _ Symbol = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

func (node *ClassMethod) symbol()           {}
func (node *ClassMethod) classMember()      {}
func (node *ClassMethod) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ClassMethod) Token() tokens.Token {
	return tokens.Token(
		tokens.NewClassMemberToken(
			tokens.ModuleMember(node.Parent.Token()),
			tokens.ClassMemberName(node.Name()),
		),
	)
}
func (node *ClassMethod) Tree() diag.Diagable { return node.Node }

// NewClassMethodSym returns a new ClassMethod symbol with the given node and parent.
func NewClassMethodSym(node *ast.ClassMethod, parent *Class) *ClassMethod {
	return &ClassMethod{
		Node:   node,
		Parent: parent,
	}
}
