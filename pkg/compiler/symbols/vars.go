// Copyright 2016 Marapongo, Inc. All rights reserved.

package symbols

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

// LocalVariable is a fully bound local variable symbol.
type LocalVariable struct {
	Node *ast.LocalVariable
}

var _ Symbol = (*LocalVariable)(nil)

func (node *LocalVariable) symbol()             {}
func (node *LocalVariable) Name() tokens.Name   { return node.Node.Name.Ident }
func (node *LocalVariable) Token() tokens.Token { return tokens.Token(node.Name()) }
func (node *LocalVariable) Tree() diag.Diagable { return node.Node }
func (node *LocalVariable) String() string      { return string(node.Name()) }

// NewLocalVariableSym returns a new LocalVariable symbol associated with the given AST node.
func NewLocalVariableSym(node *ast.LocalVariable) *LocalVariable {
	return &LocalVariable{Node: node}
}
