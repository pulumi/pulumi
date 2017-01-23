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
	Nm         tokens.TypeName
	Tok        tokens.Type
	Parent     *Module
	Extends    Type
	Implements Types
	Members    ClassMemberMap
}

var _ Symbol = (*Class)(nil)
var _ Type = (*Class)(nil)
var _ ModuleMember = (*Class)(nil)

func (node *Class) symbol()                      {}
func (node *Class) Name() tokens.Name            { return tokens.Name(node.Nm) }
func (node *Class) Token() tokens.Token          { return tokens.Token(node.Tok) }
func (node *Class) Tree() diag.Diagable          { return node.Node }
func (node *Class) moduleMember()                {}
func (node *Class) MemberNode() ast.ModuleMember { return node.Node }
func (node *Class) typesym()                     {}
func (node *Class) TypeName() tokens.TypeName    { return node.Nm }
func (node *Class) TypeToken() tokens.Type       { return node.Tok }
func (node *Class) TypeMembers() ClassMemberMap  { return node.Members }
func (node *Class) Sealed() bool                 { return node.Node.Sealed != nil && *node.Node.Sealed }
func (node *Class) Abstract() bool               { return node.Node.Abstract != nil && *node.Node.Abstract }
func (node *Class) Record() bool                 { return node.Node.Record != nil && *node.Node.Record }
func (node *Class) Interface() bool              { return node.Node.Interface != nil && *node.Node.Interface }
func (node *Class) String() string               { return string(node.Name()) }

// NewClassSym returns a new Class symbol with the given node, parent, extends, and implements, and empty members.
func NewClassSym(node *ast.Class, parent *Module, extends Type, implements Types) *Class {
	nm := tokens.TypeName(node.Name.Ident)
	return &Class{
		Node: node,
		Nm:   nm,
		Tok: tokens.Type(
			tokens.NewModuleMemberToken(
				tokens.Module(parent.Token()),
				tokens.ModuleMemberName(nm),
			),
		),
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
	Optional() bool
	Default() *interface{}
	Type() Type
	MemberNode() ast.ClassMember
}

// ClassMemberMap is a map from a class member's name to its associated symbol.
type ClassMemberMap map[tokens.ClassMemberName]ClassMember

// noClassMembers is a permanently empty class member map for efficient returning of empty ones.
var noClassMembers = make(ClassMemberMap)

// ClassProperty is a fully bound module property symbol.
type ClassProperty struct {
	Node   *ast.ClassProperty
	Parent *Class
	Typ    Type
}

var _ Symbol = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)

func (node *ClassProperty) symbol()           {}
func (node *ClassProperty) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ClassProperty) Token() tokens.Token {
	return tokens.Token(
		tokens.NewClassMemberToken(
			tokens.Type(node.Parent.Token()),
			tokens.ClassMemberName(node.Name()),
		),
	)
}
func (node *ClassProperty) Tree() diag.Diagable         { return node.Node }
func (node *ClassProperty) classMember()                {}
func (node *ClassProperty) Optional() bool              { return node.Node.Optional != nil && *node.Node.Optional }
func (node *ClassProperty) Default() *interface{}       { return node.Node.Default }
func (node *ClassProperty) Type() Type                  { return node.Typ }
func (node *ClassProperty) MemberNode() ast.ClassMember { return node.Node }
func (node *ClassProperty) String() string              { return string(node.Name()) }

// NewClassPropertySym returns a new ClassProperty symbol with the given node and parent.
func NewClassPropertySym(node *ast.ClassProperty, parent *Class, typ Type) *ClassProperty {
	return &ClassProperty{
		Node:   node,
		Parent: parent,
		Typ:    typ,
	}
}

// ClassMethod is a fully bound module method symbol.
type ClassMethod struct {
	Node   *ast.ClassMethod
	Parent *Class
	Typ    *FunctionType
}

var _ Symbol = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

func (node *ClassMethod) symbol()           {}
func (node *ClassMethod) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ClassMethod) Token() tokens.Token {
	return tokens.Token(
		tokens.NewClassMemberToken(
			tokens.Type(node.Parent.Token()),
			tokens.ClassMemberName(node.Name()),
		),
	)
}
func (node *ClassMethod) Tree() diag.Diagable         { return node.Node }
func (node *ClassMethod) classMember()                {}
func (node *ClassMethod) Optional() bool              { return true }
func (node *ClassMethod) Default() *interface{}       { return nil }
func (node *ClassMethod) Type() Type                  { return node.Typ }
func (node *ClassMethod) MemberNode() ast.ClassMember { return node.Node }
func (node *ClassMethod) String() string              { return string(node.Name()) }

// NewClassMethodSym returns a new ClassMethod symbol with the given node and parent.
func NewClassMethodSym(node *ast.ClassMethod, parent *Class, typ *FunctionType) *ClassMethod {
	return &ClassMethod{
		Node:   node,
		Parent: parent,
		Typ:    typ,
	}
}
