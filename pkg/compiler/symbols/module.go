// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Module is a fully bound module symbol.
type Module struct {
	Node    *ast.Module
	Parent  *Package
	Imports Modules
	Members ModuleMemberMap
}

var _ Symbol = (*Module)(nil)

func (node *Module) symbol()           {}
func (node *Module) Name() tokens.Name { return node.Node.Name.Ident }
func (node *Module) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleToken(
			tokens.Package(node.Parent.Token()),
			tokens.ModuleName(node.Name()),
		),
	)
}
func (node *Module) Tree() diag.Diagable { return node.Node }

// NewModuleSym returns a new Module symbol with the given node and parent, and empty imports and members.
func NewModuleSym(node *ast.Module, parent *Package) *Module {
	return &Module{
		Node:    node,
		Parent:  parent,
		Imports: make(Modules, 0),
		Members: make(ModuleMemberMap),
	}
}

// Modules is an array of module pointers.
type Modules []*Module

// ModuleMember is a marker interface for things that can be module members.
type ModuleMember interface {
	Symbol
	moduleMember()
}

// ModuleMembers is a map from a module member's name to its associated symbol.
type ModuleMemberMap map[tokens.ModuleMemberName]ModuleMember

// Export is a fully bound module property symbol that associates a name with some other symbol.
type Export struct {
	Node     *ast.Export
	Parent   *Module
	Referent Symbol // the symbol that this export actually refers to.
}

var _ Symbol = (*Export)(nil)
var _ ModuleMember = (*Export)(nil)

func (node *Export) symbol()           {}
func (node *Export) moduleMember()     {}
func (node *Export) Name() tokens.Name { return node.Node.Name.Ident }
func (node *Export) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			tokens.ModuleMemberName(node.Name()),
		),
	)
}
func (node *Export) Tree() diag.Diagable { return node.Node }

// NewExportSym returns a new Export symbol with the given node, parent, and referent symbol.
func NewExportSym(node *ast.Export, parent *Module, referent Symbol) *Export {
	return &Export{
		Node:     node,
		Parent:   parent,
		Referent: referent,
	}
}

// ModuleProperty is a fully bound module property symbol.
type ModuleProperty struct {
	Node   *ast.ModuleProperty
	Parent *Module
}

var _ Symbol = (*ModuleProperty)(nil)
var _ ModuleMember = (*ModuleProperty)(nil)

func (node *ModuleProperty) symbol()           {}
func (node *ModuleProperty) moduleMember()     {}
func (node *ModuleProperty) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ModuleProperty) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			tokens.ModuleMemberName(node.Name()),
		),
	)
}
func (node *ModuleProperty) Tree() diag.Diagable { return node.Node }

// NewModulePropertySym returns a new ModuleProperty symbol with the given node and parent.
func NewModulePropertySym(node *ast.ModuleProperty, parent *Module) *ModuleProperty {
	return &ModuleProperty{
		Node:   node,
		Parent: parent,
	}
}

// ModuleMethod is a fully bound module method symbol.
type ModuleMethod struct {
	Node   *ast.ModuleMethod
	Parent *Module
}

var _ Symbol = (*ModuleMethod)(nil)
var _ ModuleMember = (*ModuleMethod)(nil)

func (node *ModuleMethod) symbol()           {}
func (node *ModuleMethod) moduleMember()     {}
func (node *ModuleMethod) Name() tokens.Name { return node.Node.Name.Ident }
func (node *ModuleMethod) Token() tokens.Token {
	return tokens.Token(
		tokens.NewModuleMemberToken(
			tokens.Module(node.Parent.Token()),
			tokens.ModuleMemberName(node.Name()),
		),
	)
}
func (node *ModuleMethod) Tree() diag.Diagable { return node.Node }

// NewModuleMethodSym returns a new ModuleMethod symbol with the given node and parent.
func NewModuleMethodSym(node *ast.ModuleMethod, parent *Module) *ModuleMethod {
	return &ModuleMethod{
		Node:   node,
		Parent: parent,
	}
}
