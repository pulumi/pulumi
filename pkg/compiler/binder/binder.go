// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/metadata"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

// Binder annotates an existing parse tree with semantic information.
type Binder interface {
	core.Phase

	// BindPackages takes a package AST, resolves all dependencies and tokens inside of it, and returns a fully bound
	// package symbol that can be used for semantic operations (like interpretation and evaluation).
	BindPackage(pkg *pack.Package) *symbols.Package
}

// TypeMap maps AST nodes to their corresponding type.  The semantics of this differ based on the kind of node.  For
// example, an ast.Expression's type is the type of its evaluation; an ast.LocalVariable's type is the bound type of its
// value.  And so on.  This is used during binding, type checking, and evaluation, to perform type-sensitive operations.
// This avoids needing to recreate scopes and/or storing type information on every single node in the AST.
type TypeMap map[ast.Node]symbols.Type

// New allocates a fresh binder object with the given workspace, context, and metadata reader.
func New(w workspace.W, ctx *core.Context, reader metadata.Reader) Binder {
	// Create a new binder and a new scope with an empty symbol table.
	b := &binder{
		w:      w,
		ctx:    ctx,
		reader: reader,
		types:  make(TypeMap),
	}

	// Create a global scope and populate it with all of the predefined type names.
	b.PushScope()
	for _, prim := range symbols.Primitives {
		b.scope.MustRegister(prim)
	}

	return b
}

type binder struct {
	w      workspace.W     // a workspace in which this compilation is happening.
	ctx    *core.Context   // a context shared across all phases of compilation.
	reader metadata.Reader // a metadata reader (in case we encounter package references).
	scope  *Scope          // the current (mutable) scope.
	types  TypeMap         // the typechecked types for expressions (see TypeMap's comments above).
}

func (b *binder) Diag() diag.Sink {
	return b.ctx.Diag
}

// PushScope creates a new scope with an empty symbol table parented to the existing one.
func (b *binder) PushScope() {
	b.scope = NewScope(b.ctx, b.scope)
}

// PopScope replaces the current scope with its parent.
func (b *binder) PopScope() {
	contract.Assertf(b.scope != nil, "Unexpected empty binding scope during PopScope")
	b.scope = b.scope.parent
}

// registerFunctionType understands how to turn any function node into a type, and adds it to the type table.  This
// works for any kind of function-like AST node: module property, class property, or lambda.
func (b *binder) registerFunctionType(node ast.Function) {
	// Make a function type and inject it into the type table.
	var params *[]symbols.Type
	np := node.GetParameters()
	if np != nil {
		*params = make([]symbols.Type, len(*np))
		for i, param := range *np {
			var ptysym symbols.Type

			// If there was an explicit type, look it up.
			if param.Type != nil {
				ptysym = b.scope.LookupType(*param.Type)
			}

			// If either the parameter's type was unknown, or the lookup failed (leaving an error), use the any type.
			if ptysym == nil {
				ptysym = symbols.AnyType
			}

			(*params)[i] = ptysym
		}
	}

	var ret *symbols.Type
	nr := node.GetReturnType()
	if nr != nil {
		*ret = b.scope.LookupType(*nr)
	}

	b.types[node] = symbols.NewFunctionType(params, ret)
}

// registerVariableType understands how to turn any variable node into a type, and adds it to the type table.  This
// works for any kind of variable-like AST node: module property, class property, parameter, or local variable.
func (b *binder) registerVariableType(node ast.Variable) {
	var tysym symbols.Type

	// If there is an explicit node type, use it.
	nt := node.GetType()
	if nt != nil {
		tysym = b.scope.LookupType(*nt)
	}

	// Otherwise, either there was no type, or the lookup failed (leaving behind an error); use the any type.
	if tysym == nil {
		tysym = symbols.AnyType
	}

	b.types[node] = tysym
}
