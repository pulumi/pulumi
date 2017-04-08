// Copyright 2017 Pulumi, Inc. All rights reserved.

package symbols

import (
	"github.com/pulumi/coconut/pkg/compiler/ast"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
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

func (node *Class) Name() tokens.Name                   { return tokens.Name(node.Nm) }
func (node *Class) Token() tokens.Token                 { return tokens.Token(node.Tok) }
func (node *Class) Special() bool                       { return false }
func (node *Class) Tree() diag.Diagable                 { return node.Node }
func (node *Class) moduleMember()                       {}
func (node *Class) MemberNode() ast.ModuleMember        { return node.Node }
func (node *Class) MemberName() tokens.ModuleMemberName { return tokens.ModuleMemberName(node.Name()) }
func (node *Class) MemberParent() *Module               { return node.Parent }
func (node *Class) typesym()                            {}
func (node *Class) Base() Type                          { return node.Extends }
func (node *Class) TypeName() tokens.TypeName           { return node.Nm }
func (node *Class) TypeToken() tokens.Type              { return node.Tok }
func (node *Class) TypeMembers() ClassMemberMap         { return node.Members }
func (node *Class) Sealed() bool                        { return node.Node.Sealed != nil && *node.Node.Sealed }
func (node *Class) Abstract() bool                      { return node.Node.Abstract != nil && *node.Node.Abstract }
func (node *Class) Record() bool                        { return node.Node.Record != nil && *node.Node.Record }
func (node *Class) Interface() bool                     { return node.Node.Interface != nil && *node.Node.Interface }
func (node *Class) String() string                      { return string(node.Token()) }

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
	Optional() bool
	Static() bool
	Default() *interface{}
	Type() Type
	MemberNode() ast.ClassMember
	MemberName() tokens.ClassMemberName
	MemberParent() *Class
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

func (node *ClassProperty) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ClassProperty) Token() tokens.Token {
	return tokens.Token(tokens.NewClassMemberToken(node.Parent.Tok, node.MemberName()))
}
func (node *ClassProperty) Special() bool               { return false }
func (node *ClassProperty) Tree() diag.Diagable         { return node.Node }
func (node *ClassProperty) Optional() bool              { return node.Node.Optional != nil && *node.Node.Optional }
func (node *ClassProperty) Readonly() bool              { return node.Node.Readonly != nil && *node.Node.Readonly }
func (node *ClassProperty) Static() bool                { return node.Node.Static != nil && *node.Node.Static }
func (node *ClassProperty) Default() *interface{}       { return node.Node.Default }
func (node *ClassProperty) Type() Type                  { return node.Ty }
func (node *ClassProperty) MemberNode() ast.ClassMember { return node.Node }
func (node *ClassProperty) MemberName() tokens.ClassMemberName {
	return tokens.ClassMemberName(node.Name())
}
func (node *ClassProperty) MemberParent() *Class  { return node.Parent }
func (node *ClassProperty) VarNode() ast.Variable { return node.Node }
func (node *ClassProperty) String() string        { return string(node.Token()) }

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
	Sig    *FunctionType
}

var _ Symbol = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)
var _ Function = (*ClassMethod)(nil)

func (node *ClassMethod) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ClassMethod) Token() tokens.Token {
	return tokens.Token(tokens.NewClassMemberToken(node.Parent.Tok, node.MemberName()))
}
func (node *ClassMethod) Special() bool {
	nm := node.MemberName()
	return nm == tokens.ClassInitFunction || nm == tokens.ClassConstructorFunction
}
func (node *ClassMethod) SpecialModInit() bool        { return false }
func (node *ClassMethod) Tree() diag.Diagable         { return node.Node }
func (node *ClassMethod) Optional() bool              { return false }
func (node *ClassMethod) Static() bool                { return node.Node.Static != nil && *node.Node.Static }
func (node *ClassMethod) Default() *interface{}       { return nil }
func (node *ClassMethod) Type() Type                  { return node.Sig }
func (node *ClassMethod) MemberNode() ast.ClassMember { return node.Node }
func (node *ClassMethod) MemberName() tokens.ClassMemberName {
	return tokens.ClassMemberName(node.Name())
}
func (node *ClassMethod) MemberParent() *Class { return node.Parent }
func (node *ClassMethod) Constructor() bool {
	return node.MemberName() == tokens.ClassConstructorFunction
}
func (node *ClassMethod) Function() ast.Function   { return node.Node }
func (node *ClassMethod) Signature() *FunctionType { return node.Sig }
func (node *ClassMethod) String() string           { return string(node.Token()) }

// NewClassMethodSym returns a new ClassMethod symbol with the given node and parent.
func NewClassMethodSym(node *ast.ClassMethod, parent *Class, sig *FunctionType) *ClassMethod {
	return &ClassMethod{
		Node:   node,
		Parent: parent,
		Sig:    sig,
	}
}
