// Copyright 2017 Pulumi, Inc. All rights reserved.

package symbols

import (
	"github.com/pulumi/coconut/pkg/compiler/ast"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/tokens"
)

// Module is a fully bound module symbol.
type Module struct {
	Node    *ast.Module
	Parent  *Package
	Tok     tokens.Module
	Exports ModuleExportMap
	Members ModuleMemberMap
}

var _ Symbol = (*Module)(nil)

func (node *Module) Name() tokens.Name   { return node.Node.Name.Ident }
func (node *Module) Token() tokens.Token { return tokens.Token(node.Tok) }
func (node *Module) Tree() diag.Diagable { return node.Node }
func (node *Module) Special() bool       { return false }
func (node *Module) String() string      { return string(node.Token()) }

// HasInit returns true if this module has an initialzer associated with it.
func (node *Module) HasInit() bool { return node.GetInit() != nil }

// GetInit returns the initializer for this module, if one exists, or nil otherwise.
func (node *Module) GetInit() *ModuleMethod {
	if meth, has := node.Members[tokens.ModuleInitFunction]; has {
		return meth.(*ModuleMethod)
	}
	return nil
}

// NewModuleSym returns a new Module symbol with the given node and parent, and empty imports and members.
func NewModuleSym(node *ast.Module, parent *Package) *Module {
	return &Module{
		Node:    node,
		Parent:  parent,
		Tok:     tokens.NewModuleToken(parent.Tok, tokens.ModuleName(node.Name.Ident)),
		Exports: make(ModuleExportMap),
		Members: make(ModuleMemberMap),
	}
}

// Modules is an array of module pointers.
type Modules []*Module

// ModuleExportMap is a map from a module's export name to the actual export symbol.
type ModuleExportMap map[tokens.ModuleMemberName]*Export

// Export is a fully bound module property symbol that associates a name with some other symbol.
type Export struct {
	Node     *ast.Export
	Parent   *Module
	Referent Symbol
}

var _ Symbol = (*Export)(nil)

func (node *Export) Name() tokens.Name { return node.Node.Name.Ident }
func (node *Export) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			tokens.ModuleMemberName(node.Name()),
		),
	)
}
func (node *Export) Special() bool       { return false }
func (node *Export) Tree() diag.Diagable { return node.Node }
func (node *Export) String() string      { return string(node.Token()) }

// NewExportSym returns a new Export symbol with the given node, parent, and referent.  The referent may be a fully
// resolved module member or it might just point to yet another export symbol, in the case of chaining.
func NewExportSym(node *ast.Export, parent *Module, referent Symbol) *Export {
	return &Export{
		Node:     node,
		Parent:   parent,
		Referent: referent,
	}
}

// ModuleMember is a marker interface for things that can be module members.
type ModuleMember interface {
	Symbol
	MemberNode() ast.ModuleMember
	MemberName() tokens.ModuleMemberName
	MemberParent() *Module
}

// ModuleMemberMap is a map from a module member's name to its associated symbol.
type ModuleMemberMap map[tokens.ModuleMemberName]ModuleMember

// ModuleMemberProperty is an interface that gives a module member a type, such that it can be used as a property.
type ModuleMemberProperty interface {
	ModuleMember
	ModuleMemberProperty()
	MemberType() Type
}

// ModuleProperty is a fully bound module property symbol.
type ModuleProperty struct {
	Node   *ast.ModuleProperty
	Parent *Module
	Ty     Type
}

var _ Symbol = (*ModuleProperty)(nil)
var _ ModuleMember = (*ModuleProperty)(nil)
var _ ModuleMemberProperty = (*ModuleProperty)(nil)
var _ Variable = (*ClassProperty)(nil)

func (node *ModuleProperty) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ModuleProperty) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			node.MemberName(),
		),
	)
}
func (node *ModuleProperty) Special() bool                { return false }
func (node *ModuleProperty) Tree() diag.Diagable          { return node.Node }
func (node *ModuleProperty) MemberNode() ast.ModuleMember { return node.Node }
func (node *ModuleProperty) MemberName() tokens.ModuleMemberName {
	return tokens.ModuleMemberName(node.Name())
}
func (node *ModuleProperty) MemberParent() *Module { return node.Parent }
func (node *ModuleProperty) ModuleMemberProperty() {}
func (node *ModuleProperty) MemberType() Type      { return node.Ty }
func (node *ModuleProperty) Default() *interface{} { return node.Node.Default }
func (node *ModuleProperty) Readonly() bool        { return node.Node.Readonly != nil && *node.Node.Readonly }
func (node *ModuleProperty) Type() Type            { return node.Ty }
func (node *ModuleProperty) VarNode() ast.Variable { return node.Node }
func (node *ModuleProperty) String() string        { return string(node.Token()) }

// NewModulePropertySym returns a new ModuleProperty symbol with the given node and parent.
func NewModulePropertySym(node *ast.ModuleProperty, parent *Module, ty Type) *ModuleProperty {
	return &ModuleProperty{
		Node:   node,
		Parent: parent,
		Ty:     ty,
	}
}

// ModuleMethod is a fully bound module method symbol.
type ModuleMethod struct {
	Node   *ast.ModuleMethod
	Parent *Module
	Sig    *FunctionType
}

var _ Symbol = (*ModuleMethod)(nil)
var _ ModuleMember = (*ModuleMethod)(nil)
var _ ModuleMemberProperty = (*ModuleMethod)(nil)
var _ Function = (*ClassMethod)(nil)

func (node *ModuleMethod) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ModuleMethod) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			node.MemberName(),
		),
	)
}
func (node *ModuleMethod) Special() bool                { return node.SpecialModInit() }
func (node *ModuleMethod) SpecialModInit() bool         { return node.MemberName() == tokens.ModuleInitFunction }
func (node *ModuleMethod) Tree() diag.Diagable          { return node.Node }
func (node *ModuleMethod) MemberNode() ast.ModuleMember { return node.Node }
func (node *ModuleMethod) MemberName() tokens.ModuleMemberName {
	return tokens.ModuleMemberName(node.Name())
}
func (node *ModuleMethod) MemberParent() *Module    { return node.Parent }
func (node *ModuleMethod) ModuleMemberProperty()    {}
func (node *ModuleMethod) MemberType() Type         { return node.Sig }
func (node *ModuleMethod) Function() ast.Function   { return node.Node }
func (node *ModuleMethod) Signature() *FunctionType { return node.Sig }
func (node *ModuleMethod) String() string           { return string(node.Token()) }

// NewModuleMethodSym returns a new ModuleMethod symbol with the given node and parent.
func NewModuleMethodSym(node *ast.ModuleMethod, parent *Module, sig *FunctionType) *ModuleMethod {
	return &ModuleMethod{
		Node:   node,
		Parent: parent,
		Sig:    sig,
	}
}
