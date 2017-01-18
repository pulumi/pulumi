// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/legacy/ast"
	"github.com/marapongo/mu/pkg/compiler/metadata"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/workspace"
)

// Binder annotates an existing parse tree with semantic information.
type Binder interface {
	core.Phase

	// BindPackages takes a package AST, resolves all dependencies and tokens inside of it, and returns a fully bound
	// package symbol that can be used for semantic operations (like interpretation and evaluation).
	BindPackage(pkg *pack.Package) *symbols.Package

	// PrepareStack prepares the AST for binding.  It returns a list of all unresolved dependency references.  These
	// must be bound and supplied to the BindStack function as the deps argument.
	PrepareStack(stack *ast.Stack) []tokens.Ref
	// BindStack takes an AST, and its set of dependencies, and binds all names inside, mutating it in place.  It
	// returns a full list of all dependency Stacks that this Stack depends upon (which must then be bound).
	BindStack(stack *ast.Stack, deprefs ast.DependencyRefs) []*ast.Stack
	// ValidateStack runs last, after all transitive dependencies have been bound, to perform last minute validation.
	ValidateStack(stack *ast.Stack)
}

func New(w workspace.W, ctx *core.Context, reader metadata.Reader) Binder {
	// Create a new binder and a new scope with an empty symbol table.
	b := &binder{ctx: ctx, w: w}
	b.PushScope()
	return b
}

type binder struct {
	w      workspace.W     // a workspace in which this compilation is happening.
	ctx    *core.Context   // a context shared across all phases of compilation.
	reader metadata.Reader // a metadata reader (in case we encounter package references).
	scope  *scope          // the current (mutable) scope.
}

func (b *binder) Diag() diag.Sink {
	return b.ctx.Diag
}
