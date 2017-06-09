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
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// FreeVars computes the free variables referenced inside a function body.
// The free variables for a function will be either simple identifier tokens or tokens
// referencing module-scope variables.
func FreeVars(fnc ast.Function) []tokens.Token {
	visitor := &freeVarsVisitor{
		freeVars: map[tokens.Token]bool{},
	}

	ast.Walk(visitor, fnc)

	params := fnc.GetParameters()
	if params != nil {
		for _, lv := range *params {
			visitor.removeLocalVariable(lv)
		}
	}

	var vars []tokens.Token
	for k := range visitor.freeVars {
		vars = append(vars, k)
	}
	return vars
}

type freeVarsVisitor struct {
	freeVars map[tokens.Token]bool
}

var _ ast.Visitor = (*freeVarsVisitor)(nil)

func (visitor *freeVarsVisitor) Visit(node ast.Node) ast.Visitor {
	return visitor
}

// We walk the AST and process each node after visiting it in depth first order. There are two cases we care about:
// 1) After visiting a leaf node which is a reference to a local variable (`n.Object == nil``), we add it to our set.
// 2) After visiting a LocalVariableDeclaration or a Lambda, we remove the declared variables from our set.
func (visitor *freeVarsVisitor) After(node ast.Node) {
	switch n := node.(type) {
	case *ast.LoadLocationExpression:
		if n.Object == nil {
			visitor.addToken(n.Name.Tok)
		}
	case *ast.LoadDynamicExpression:
		if n.Object == nil {
			switch e := n.Name.(type) {
			case *ast.StringLiteral:
				visitor.addToken(tokens.Token(e.Value))
			default:
				contract.Failf("expected LoadDynamicExpression with Object == nil to have a StringLiteral expression")
			}
		}
	case *ast.TryLoadDynamicExpression:
		if n.Object == nil {
			switch e := n.Name.(type) {
			case *ast.StringLiteral:
				visitor.addToken(tokens.Token(e.Value))
			default:
				contract.Failf("expected LoadDynamicExpression with Object == nil to have a StringLiteral expression")
			}
		}
	case *ast.LambdaExpression:
		if n.Parameters != nil {
			for _, param := range *n.Parameters {
				visitor.removeLocalVariable(param)
			}
		}
	case *ast.Block:
		for _, stmt := range n.Statements {
			switch s := stmt.(type) {
			case *ast.LocalVariableDeclaration:
				visitor.removeLocalVariable(s.Local)
			}
		}
	}
}

func (visitor *freeVarsVisitor) addToken(tok tokens.Token) {
	visitor.freeVars[tok] = true
}

func (visitor *freeVarsVisitor) removeLocalVariable(lv *ast.LocalVariable) {
	delete(visitor.freeVars, tokens.Token(lv.Name.Ident))
}
