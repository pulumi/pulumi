// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
)

// bindFunctionBody binds a function body, including a scope, its parameters, and its expressions and statements.
func (b *binder) bindFunctionBody(node ast.Function) {
	// Enter a new scope, bind the parameters, and then bind the body using a visitor.
	scope := b.ctx.Scope.Push()
	defer scope.Pop()
	params := node.GetParameters()
	if params != nil {
		for _, param := range *params {
			// Register this variable's type and associate its name with the identifier.
			// TODO: stick this into the scope.
			ty := b.bindType(param.Type)
			b.ctx.Scope.TryRegister(param, symbols.NewLocalVariableSym(param, ty))
		}
	}

	body := node.GetBody()
	if body != nil {
		v := newASTBinder(b, node)
		ast.Walk(v, body)
	}
}
