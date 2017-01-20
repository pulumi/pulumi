// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
)

// bindFunctionBody binds a function body, including a scope, its parameters, and its expressions and statements.
func (b *binder) bindFunctionBody(node ast.Function) {
	// Enter a new scope, bind the parameters, and then bind the body using a visitor.
	scope := b.scope.Push()
	defer scope.Pop()
	params := node.GetParameters()
	if params != nil {
		for _, param := range *params {
			// Register this variable's type and associate its name with the identifier.
			b.registerVariableType(param)
			b.scope.MustRegister(symbols.NewLocalVariableSym(param))
		}
	}

	body := node.GetBody()
	if body != nil {
		v := &astBinder{b}
		ast.Walk(v, body)
	}
}

// astBinder is an AST visitor implementation that understands how to deal with all sorts of node types.  It
// does not visit children, however, as it relies on the depth-first order walk supplied by the AST package.  The
// overall purpose of this is to perform validation, and record types and symbols that're needed during evaluation.
type astBinder struct {
	b *binder
}

var _ ast.Visitor = (*astBinder)(nil)

func (a *astBinder) Visit(node ast.Node) ast.Visitor {
	switch node.(type) {
	case *ast.Block:
		// Entering a new block requires a fresh lexical scope.
		a.b.scope.Push()
	}
	// TODO: bind the other node kinds.
	return a
}

func (a *astBinder) After(node ast.Node) {
	switch node.(type) {
	case *ast.Block:
		// Exiting a block restores the prior lexical context.
		a.b.scope.Pop()
	}
	// TODO: bind the other node kinds.
}
