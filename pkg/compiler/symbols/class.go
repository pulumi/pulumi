// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// Class is a fully bound class symbol.
type Class struct {
	Tree    *ast.Class
	Members map[tokens.Module]ClassMember
}

var _ Symbol = (*Class)(nil)
var _ Type = (*Class)(nil)
var _ ModuleMember = (*Class)(nil)

func (node *Class) symbol()                {}
func (node *Class) typesym()               {}
func (node *Class) moduleMember()          {}
func (node *Class) GetName() tokens.Token  { return node.Tree.Name.Ident }
func (node *Class) GetTree() diag.Diagable { return node.Tree }

// ClassMember is a marker interface for things that can be module members.
type ClassMember interface {
	Symbol
	classMember()
}

// ClassProperty is a fully bound module property symbol.
type ClassProperty struct {
	Tree *ast.ClassProperty
}

var _ Symbol = (*ClassProperty)(nil)
var _ ClassMember = (*ClassProperty)(nil)

func (node *ClassProperty) symbol()                {}
func (node *ClassProperty) classMember()           {}
func (node *ClassProperty) GetName() tokens.Token  { return node.Tree.Name.Ident }
func (node *ClassProperty) GetTree() diag.Diagable { return node.Tree }

// ClassMethod is a fully bound module method symbol.
type ClassMethod struct {
	Tree *ast.ClassMethod
}

var _ Symbol = (*ClassMethod)(nil)
var _ ClassMember = (*ClassMethod)(nil)

func (node *ClassMethod) symbol()                {}
func (node *ClassMethod) classMember()           {}
func (node *ClassMethod) GetName() tokens.Token  { return node.Tree.Name.Ident }
func (node *ClassMethod) GetTree() diag.Diagable { return node.Tree }
