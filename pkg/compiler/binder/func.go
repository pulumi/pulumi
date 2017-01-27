// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

// bindFunctionBody binds a function body, including a scope, its parameters, and its expressions and statements.
func (b *binder) bindFunctionBody(node ast.Function) {
	contract.Require(node != nil, "node")
	contract.Assertf(b.ctx.Scope.Frame, "Expected an activation frame at the top of the scope")

	// Enter a new scope, bind the parameters, and then bind the body using a visitor.
	params := node.GetParameters()
	if params != nil {
		for _, param := range *params {
			// Register this variable's type and associate its name with the identifier.
			ty := b.bindType(param.Type)
			sym := symbols.NewLocalVariableSym(param, ty)
			b.ctx.RegisterSymbol(param, sym)
			b.ctx.Scope.TryRegister(param, sym)
		}
	}

	body := node.GetBody()
	if body != nil {
		v := newASTBinder(b, node)
		ast.Walk(v, body)
	}
}
