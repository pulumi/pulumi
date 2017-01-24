// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/golang/glog"

	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context holds binder-specific context information, like symbol and type binding information.
type Context struct {
	*core.Context         // inherits all of the other context info.
	Scope         *Scope  // the current (mutable) scope.
	Types         TypeMap // the type-checked type symbols for expressions.
}

func NewContextFrom(ctx *core.Context) *Context {
	bctx := &Context{
		Context: ctx,
		Types:   make(TypeMap),
	}

	// Create a global scope and populate it with all of the predefined type names.  This one's never popped.
	NewScope(ctx, &bctx.Scope)
	for _, prim := range types.Primitives {
		bctx.Scope.MustRegister(prim)
	}

	return bctx
}

// TypeMap maps AST nodes to their corresponding type.  The semantics of this differ based on the kind of node.  For
// example, an ast.Expression's type is the type of its evaluation; an ast.LocalVariable's type is the bound type of its
// value.  And so on.  This is used during binding, type checking, and evaluation, to perform type-sensitive operations.
// This avoids needing to recreate scopes and/or storing type information on every single node in the AST.
type TypeMap map[ast.Node]symbols.Type

// RequireType requires that a type exists for the given AST node.
func (ctx *Context) RequireType(node ast.Node) symbols.Type {
	ty := ctx.Types[node]
	contract.Assertf(ty != nil, "Expected a typemap entry for %v node", node.GetKind())
	return ty
}

// RequireExprType fetches an expression's non-nil type.
func (ctx *Context) RequireExprType(node ast.Expression) symbols.Type {
	return ctx.RequireType(node)
}

// RegisterExprType registers an expression's type.
func (ctx *Context) RegisterExprType(node ast.Expression, tysym symbols.Type) {
	contract.Require(tysym != nil, "tysym")
	contract.Assert(ctx.Types[node] == nil)
	if glog.V(7) {
		glog.V(7).Infof("Registered expression type: '%v' => %v", node.GetKind(), tysym.Name())
	}
	ctx.Types[node] = tysym
}

// RequireFunctionType fetches the non-nil registered type for a given function.
func (ctx *Context) RequireFunctionType(node ast.Function) *symbols.FunctionType {
	ty := ctx.RequireType(node)
	fty, ok := ty.(*symbols.FunctionType)
	contract.Assertf(ok, "Expected function type for %v; got %v", node.GetKind(), fty.Token())
	return fty
}

// RegisterFunctionType understands how to turn any function node into a type, and adds it to the type table.  This
// works for any kind of function-like AST node: module property, class property, or lambda.
func (ctx *Context) RegisterFunctionType(node ast.Function) *symbols.FunctionType {
	// Make a function type and inject it into the type table.
	var params []symbols.Type
	np := node.GetParameters()
	if np != nil {
		for _, param := range *np {
			var ptysym symbols.Type

			// If there was an explicit type, look it up.
			if param.Type != nil {
				ptysym = ctx.Scope.LookupType(param.Type.Tok)
			}

			// If either the parameter's type was unknown, or the lookup failed (leaving an error), use the any type.
			if ptysym == nil {
				ptysym = types.Any
			}

			params = append(params, ptysym)
		}
	}

	var ret symbols.Type
	nr := node.GetReturnType()
	if nr != nil {
		ret = ctx.Scope.LookupType(nr.Tok)
	}

	tysym := symbols.NewFunctionType(params, ret)
	if glog.V(7) {
		glog.V(7).Infof("Registered function type: '%v' => %v", node.GetName().Ident, tysym.Name())
	}
	ctx.Types[node] = tysym

	return tysym
}

// RequireVariableType fetches the non-nil registered type for a given variable.
func (ctx *Context) RequireVariableType(node ast.Variable) symbols.Type {
	return ctx.RequireType(node)
}

// RegisterVariableType understands how to turn any variable node into a type, and adds it to the type table.  This
// works for any kind of variable-like AST node: module property, class property, parameter, or local variable.
func (ctx *Context) RegisterVariableType(node ast.Variable) symbols.Type {
	var tysym symbols.Type

	// If there is an explicit node type, use it.
	nt := node.GetType()
	if nt != nil {
		tysym = ctx.Scope.LookupType(nt.Tok)
	}

	// Otherwise, either there was no type, or the lookup failed (leaving behind an error); use the any type.
	if tysym == nil {
		tysym = types.Any
	}

	if glog.V(7) {
		glog.V(7).Infof("Registered variable type: '%v' => %v", node.GetName().Ident, tysym.Name())
	}
	ctx.Types[node] = tysym

	return tysym
}
