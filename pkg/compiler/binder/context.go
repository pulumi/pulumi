// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context holds binder-specific context information, like symbol and type binding information.
type Context struct {
	*core.Context           // inherits all of the other context info.
	Scope         *Scope    // the current (mutable) scope.
	Types         TypeMap   // the type-checked type symbols for expressions.
	Symbols       SymbolMap // the fully bound symbol information for all definitions.
}

// TypeMap maps AST expression nodes to their corresponding type.  This is used during binding, type checking, and
// evaluation, to perform type-sensitive operations.  This avoids needing to recreate scopes in subsequent passes of the
// compiler and/or storing type information on every single node in the AST.
type TypeMap map[ast.Expression]symbols.Type

// SymbolMap maps all known definition AST definition nodes to their corresponding symbols.
type SymbolMap map[ast.Definition]symbols.Symbol

// NewContextFrom allocates a fresh binding context linked to the shared context object.
func NewContextFrom(ctx *core.Context) *Context {
	return &Context{
		Context: ctx,
		Types:   make(TypeMap),
		Symbols: make(SymbolMap),
	}
}

// RequireType requires that a type exists for the given AST expression node.
func (ctx *Context) RequireType(node ast.Expression) symbols.Type {
	contract.Require(node != nil, "node")
	ty := ctx.Types[node]
	contract.Assertf(ty != nil, "Expected a typemap entry for %v node", node.GetKind())
	return ty
}

// RegisterType registers an expression's type.
func (ctx *Context) RegisterType(node ast.Expression, tysym symbols.Type) {
	contract.Require(node != nil, "node")
	contract.Require(tysym != nil, "tysym")
	contract.Assert(ctx.Types[node] == nil)
	if glog.V(7) {
		glog.V(7).Infof("Registered expression type: '%v' => %v", node.GetKind(), tysym.Name())
	}
	ctx.Types[node] = tysym
}

// RequireSymbol fetches the non-nil registered symbol for a given definition node.
func (ctx *Context) RequireSymbol(node ast.Definition) symbols.Symbol {
	contract.Require(node != nil, "node")
	sym := ctx.Symbols[node]
	contract.Assertf(sym != nil, "Expected a symbol entry for %v node", node.GetKind())
	return sym
}

// RegisterSymbol registers a definition's symbol.
func (ctx *Context) RegisterSymbol(node ast.Definition, sym symbols.Symbol) {
	contract.Require(node != nil, "node")
	contract.Require(sym != nil, "sym")
	contract.Assert(ctx.Symbols[node] == nil)
	if glog.V(7) {
		glog.V(7).Infof("Registered definition symbol: '%v' => %v", node.GetKind(), sym.Name())
	}
	ctx.Symbols[node] = sym
}

func (ctx *Context) RequireFunction(fnc ast.Function) symbols.Function {
	return ctx.RequireSymbol(fnc).(symbols.Function)
}

func (ctx *Context) RequireVariable(fnc ast.Variable) symbols.Variable {
	return ctx.RequireSymbol(fnc).(symbols.Variable)
}
