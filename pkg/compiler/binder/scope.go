// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/ast"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Scope facilitates storing information that obeys lexically nested scoping rules.
type Scope struct {
	ctx    *core.Context // the shared context object for errors, etc.
	slot   **Scope       // the slot rooting the tree of current scopes for pushing/popping.
	parent *Scope        // the parent scope to restore upon pop, or nil if this is the top.
	locals LocalMap      // the current scope's locals map (name to local symbol).
}

// LocalMap maps the name of locals to their corresponding symbols, for the few places we need name-based lookup.
type LocalMap map[tokens.Name]*symbols.LocalVariable

// NewScope allocates and returns a fresh scope using the given slot, populating it.
func NewScope(ctx *core.Context, slot **Scope) *Scope {
	scope := &Scope{
		ctx:    ctx,
		slot:   slot,
		parent: *slot,
		locals: make(LocalMap),
	}
	*slot = scope
	return scope
}

// Push creates a new scope with an empty symbol table parented to the existing one.
func (s *Scope) Push() *Scope {
	return NewScope(s.ctx, s.slot)
}

// Pop restores the prior scope into the underlying slot, tossing away the current symbol table.
func (s *Scope) Pop() {
	contract.Assert(*s.slot == s)
	*s.slot = s.parent
}

// Lookup finds a variable underneath the given name, issuing an error and returning nil if not found.
func (s *Scope) Lookup(nm tokens.Name) *symbols.LocalVariable {
	for s != nil {
		if sym, exists := s.locals[nm]; exists {
			contract.Assert(sym != nil)
			return sym
		}

		// If not in this scope, keep looking at the ancestral chain, if one exists.
		s = s.parent
	}

	// TODO: issue an error about a missing symbol.
	return nil
}

// Register registers a local variable with a given name; if it already exists, the function returns false.
func (s *Scope) Register(sym *symbols.LocalVariable) bool {
	nm := sym.Name()
	if _, exists := s.locals[nm]; exists {
		// TODO: this won't catch "shadowing" for parent scopes; do we care about this?
		return false
	}

	s.locals[nm] = sym
	return true
}

// TryRegister registers a local with the given name; if it already exists, a compiler error is emitted.
func (s *Scope) TryRegister(node ast.Node, sym *symbols.LocalVariable) {
	if !s.Register(sym) {
		s.ctx.Diag.Errorf(errors.ErrorSymbolAlreadyExists.At(node), sym.Name)
	}
}
