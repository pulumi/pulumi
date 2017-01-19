// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Module is a fully bound module symbol.
type Module struct {
	Tree    *ast.Module
	Members map[tokens.Module]ModuleMember
}

var _ Symbol = (*Module)(nil)

func (node *Module) symbol()                {}
func (node *Module) GetName() tokens.Token  { return node.Tree.Name.Ident }
func (node *Module) GetTree() diag.Diagable { return node.Tree }

// ModuleMember is a marker interface for things that can be module members.
type ModuleMember interface {
	Symbol
	moduleMember()
}

// ModuleProperty is a fully bound module property symbol.
type ModuleProperty struct {
	Tree *ast.ModuleProperty
}

var _ Symbol = (*ModuleProperty)(nil)
var _ ModuleMember = (*ModuleProperty)(nil)

func (node *ModuleProperty) symbol()                {}
func (node *ModuleProperty) moduleMember()          {}
func (node *ModuleProperty) GetName() tokens.Token  { return node.Tree.Name.Ident }
func (node *ModuleProperty) GetTree() diag.Diagable { return node.Tree }

// ModuleMethod is a fully bound module method symbol.
type ModuleMethod struct {
	Tree *ast.ModuleMethod
}

var _ Symbol = (*ModuleMethod)(nil)
var _ ModuleMember = (*ModuleMethod)(nil)

func (node *ModuleMethod) symbol()                {}
func (node *ModuleMethod) moduleMember()          {}
func (node *ModuleMethod) GetName() tokens.Token  { return node.Tree.Name.Ident }
func (node *ModuleMethod) GetTree() diag.Diagable { return node.Tree }
