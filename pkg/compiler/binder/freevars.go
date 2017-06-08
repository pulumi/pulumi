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
)

// FreeVars computes the free variables referenced inside a function body.
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

	vars := []tokens.Token{}
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

func (visitor *freeVarsVisitor) After(node ast.Node) {
	switch n := node.(type) {
	case *ast.LoadLocationExpression:
		if n.Object == nil {
			visitor.addToken(n.Name)
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

func (visitor *freeVarsVisitor) addToken(tok *ast.Token) {
	visitor.freeVars[tok.Tok] = true
}

func (visitor *freeVarsVisitor) removeLocalVariable(lv *ast.LocalVariable) {
	delete(visitor.freeVars, tokens.Token(lv.Name.Ident))
}
