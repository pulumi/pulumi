// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package binder

import (
	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// bindFunctionBody binds a function body, including a scope, its parameters, and its expressions and statements.
func (b *binder) bindFunctionBody(node ast.Function) {
	contract.Assertf(b.ctx.Scope.Activation, "Expected an activation frame at the top of the scope")
	fsym := b.ctx.RequireFunction(node)
	b.bindFunctionCommon(node, fsym.Signature())
}

func (b *binder) bindLambdaExpression(node *ast.LambdaExpression) symbols.Type {
	// Push a new scope, but keep the parent's variables visible (so, a non-frame).
	scope := b.ctx.Scope.Push(false)
	defer scope.Pop()

	// Make a signature type.
	var params []symbols.Type
	if pparams := node.GetParameters(); pparams != nil {
		for _, param := range *pparams {
			params = append(params, b.ctx.RequireVariable(param).Type())
		}
	}
	var ret symbols.Type
	if pret := node.GetReturnType(); pret != nil {
		ret = b.ctx.LookupType(pret)
	}

	// Now bind the body and return the type.
	sig := symbols.NewFunctionType(params, ret)
	b.bindFunctionCommon(node, sig)
	return sig
}

func (b *binder) bindFunctionCommon(node ast.Function, sig *symbols.FunctionType) {
	contract.Require(node != nil, "node")

	// Enter a new scope, bind the parameters, and then bind the body using a visitor.
	params := node.GetParameters()
	if params != nil {
		for _, param := range *params {
			// Register this variable's type and associate its name with the identifier.
			ty := b.ctx.LookupType(param.Type)
			sym := symbols.NewLocalVariableSym(param, ty)
			b.ctx.RegisterSymbol(param, sym)
			b.ctx.Scope.TryRegister(param, sym)
		}
	}

	body := node.GetBody()
	if body != nil {
		v := newASTBinder(b, node, sig)
		ast.Walk(v, body)
	}
}
