// Copyright 2017 Pulumi, Inc. All rights reserved.

package binder

import (
	"github.com/pulumi/coconut/pkg/compiler/ast"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Scope facilitates storing information that obeys lexically nested scoping rules.
type Scope struct {
	Ctx    *Context // the shared context object for errors, etc.
	Slot   **Scope  // the slot rooting the tree of current scopes for pushing/popping.
	Frame  bool     // if this scope represents the top of an activation frame.
	Parent *Scope   // the parent scope to restore upon pop, or nil if this is the top.
	Locals LocalMap // the current scope's locals map (name to local symbol).
}

// LocalMap maps the name of locals to their corresponding symbols, for the few places we need name-based lookup.
type LocalMap map[tokens.Name]*symbols.LocalVariable

// NewScope allocates and returns a fresh scope using the given slot, populating it.
func NewScope(ctx *Context, frame bool) *Scope {
	slot := &ctx.Scope
	scope := &Scope{
		Ctx:    ctx,
		Slot:   slot,
		Frame:  frame,
		Parent: *slot,
		Locals: make(LocalMap),
	}
	*slot = scope
	return scope
}

// Push creates a new scope with an empty symbol table parented to the existing one.
func (s *Scope) Push(frame bool) *Scope {
	return NewScope(s.Ctx, frame)
}

// Pop restores the prior scope into the underlying slot, tossing away the current symbol table.
func (s *Scope) Pop() {
	contract.Assert(*s.Slot == s)
	*s.Slot = s.Parent
}

// Lookup finds a variable underneath the given name, issuing an error and returning nil if not found.
func (s *Scope) Lookup(nm tokens.Name) *symbols.LocalVariable {
	for s != nil {
		if sym, exists := s.Locals[nm]; exists {
			contract.Assert(sym != nil)
			return sym
		}

		// If not in this scope, keep looking at the ancestral chain, if one exists.
		if s.Frame {
			s = nil
		} else {
			s = s.Parent
		}
	}
	return nil
}

// Register registers a local variable with a given name; if it already exists, the function returns false.
func (s *Scope) Register(sym *symbols.LocalVariable) bool {
	nm := sym.Name()
	if _, exists := s.Locals[nm]; exists {
		// TODO: this won't catch "shadowing" for parent scopes; do we care about this?
		return false
	}

	s.Locals[nm] = sym
	return true
}

// MustRegister registers a local with the given name; if it already exists, the function abandons.
func (s *Scope) MustRegister(sym *symbols.LocalVariable) {
	if !s.Register(sym) {
		contract.Failf("Symbol already exists: %v", sym.Name)
	}
}

// TryRegister registers a local with the given name; if it already exists, a compiler error is emitted.
func (s *Scope) TryRegister(node ast.Node, sym *symbols.LocalVariable) {
	if !s.Register(sym) {
		s.Ctx.Diag.Errorf(errors.ErrorSymbolAlreadyExists.At(node), sym.Name)
	}
}
