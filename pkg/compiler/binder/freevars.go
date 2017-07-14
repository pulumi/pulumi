// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package binder

import (
	"sort"

	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type byName []tokens.Token

func (ts byName) Len() int               { return len(ts) }
func (ts byName) Less(i int, j int) bool { return ts[i] < ts[j] }
func (ts byName) Swap(i int, j int)      { ts[i], ts[j] = ts[j], ts[i] }

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
	sort.Sort(byName(vars))
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
		visitor.removeBlockVariables(n.Statements)
	}
}

func (visitor *freeVarsVisitor) removeBlockVariables(statements []ast.Statement) {
	for _, stmt := range statements {
		switch s := stmt.(type) {
		case *ast.LocalVariableDeclaration:
			visitor.removeLocalVariable(s.Local)
		case *ast.MultiStatement:
			visitor.removeBlockVariables(s.Statements)
		}
	}
}

func (visitor *freeVarsVisitor) addToken(tok tokens.Token) {
	visitor.freeVars[tok] = true
}

func (visitor *freeVarsVisitor) removeLocalVariable(lv *ast.LocalVariable) {
	delete(visitor.freeVars, tokens.Token(lv.Name.Ident))
}
