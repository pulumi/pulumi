// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
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
	This       *LocalVariable // the special "this" local for instance members.
	Super      *LocalVariable // the special "super" local for instance members.
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

// HasInit returns true if this module has an initialzer associated with it.
func (node *Class) HasInit() bool { return node.GetInit() != nil }

// GetInit returns the initializer for this module, if one exists, or nil otherwise.
func (node *Class) GetInit() *ClassMethod {
	if meth, has := node.Members[tokens.ClassInitFunction]; has {
		return meth.(*ClassMethod)
	}
	return nil
}

// SetBase mutates the base class, in cases where it wasn't available at initialization time.
func (node *Class) SetBase(extends Type) {
	contract.Assert(node.Extends == nil)
	node.Extends = extends

	// If this class extends something else, wire up the "super" variable too.
	if extends != nil {
		node.Super = NewSpecialVariableSym(tokens.SuperVariable, extends)
	}
}

// NewClassSym returns a new Class symbol with the given node, parent, extends, and implements, and empty members.
func NewClassSym(node *ast.Class, parent *Module, extends Type, implements Types) *Class {
	nm := tokens.TypeName(node.Name.Ident)
	class := &Class{
		Node: node,
		Nm:   nm,
		Tok: tokens.Type(
			tokens.NewModuleMemberToken(
				tokens.Module(parent.Token()),
				tokens.ModuleMemberName(nm),
			),
		),
		Parent:     parent,
		Implements: implements,
		Members:    make(ClassMemberMap),
	}

	// Populate the "this" variable for instance methods.
	class.This = NewSpecialVariableSym(tokens.ThisVariable, class)

	// Set the base class, possibly initializing "super" if appropriate.
	class.SetBase(extends)

	return class
}

// ClassMember is a marker interface for things that can be module members.
type ClassMember interface {
	Symbol
	classMember()
	Optional() bool
	Static() bool
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
	Ty     Type
}

var _ Symbol = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)
var _ Variable = (*ClassProperty)(nil)

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
func (node *ClassProperty) Readonly() bool              { return node.Node.Readonly != nil && *node.Node.Readonly }
func (node *ClassProperty) Static() bool                { return node.Node.Static != nil && *node.Node.Static }
func (node *ClassProperty) Default() *interface{}       { return node.Node.Default }
func (node *ClassProperty) Type() Type                  { return node.Ty }
func (node *ClassProperty) MemberNode() ast.ClassMember { return node.Node }
func (node *ClassProperty) VarNode() ast.Variable       { return node.Node }
func (node *ClassProperty) String() string              { return string(node.Name()) }

// NewClassPropertySym returns a new ClassProperty symbol with the given node and parent.
func NewClassPropertySym(node *ast.ClassProperty, parent *Class, ty Type) *ClassProperty {
	return &ClassProperty{
		Node:   node,
		Parent: parent,
		Ty:     ty,
	}
}

// ClassMethod is a fully bound module method symbol.
type ClassMethod struct {
	Node   *ast.ClassMethod
	Parent *Class
	Ty     *FunctionType
}

var _ Symbol = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)
var _ Function = (*ClassMethod)(nil)

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
func (node *ClassMethod) Optional() bool              { return false }
func (node *ClassMethod) Static() bool                { return node.Node.Static != nil && *node.Node.Static }
func (node *ClassMethod) Default() *interface{}       { return nil }
func (node *ClassMethod) Type() Type                  { return node.Ty }
func (node *ClassMethod) MemberNode() ast.ClassMember { return node.Node }
func (node *ClassMethod) FuncNode() ast.Function      { return node.Node }
func (node *ClassMethod) FuncType() *FunctionType     { return node.Ty }
func (node *ClassMethod) String() string              { return string(node.Name()) }

// NewClassMethodSym returns a new ClassMethod symbol with the given node and parent.
func NewClassMethodSym(node *ast.ClassMethod, parent *Class, ty *FunctionType) *ClassMethod {
	return &ClassMethod{
		Node:   node,
		Parent: parent,
		Ty:     ty,
	}
}
