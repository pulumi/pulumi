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

// Scope enables lookups and symbols to obey traditional language scoping rules.
type Scope struct {
	ctx    *core.Context
	slot   **Scope
	parent *Scope
	symtbl SymbolTable
}

// SymbolTable is a mapping from symbol token to an actual symbol object representing it.
type SymbolTable map[tokens.Token]symbols.Symbol

// NewScope allocates and returns a fresh scope using the given slot, populating it.
func NewScope(ctx *core.Context, slot **Scope) *Scope {
	scope := &Scope{
		ctx:    ctx,
		slot:   slot,
		parent: *slot,
		symtbl: make(SymbolTable),
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
	*s.slot = s.parent
}

// Lookup finds a symbol registered underneath the given token, issuing an error and returning nil if not found.
func (s *Scope) Lookup(tok tokens.Token) symbols.Symbol {
	for s != nil {
		if sym, exists := s.symtbl[tok]; exists {
			contract.Assert(sym != nil)
			return sym
		}

		// If not in this scope, keep looking at the ancestral chain, if one exists.
		s = s.parent
	}

	// TODO: issue an error about a missing symbol.
	return nil
}

// LookupModule finds a module symbol registered underneath the given token, issuing an error and returning nil if
// missing.  If the symbol exists, but is of the wrong type (i.e., not a *symbols.Module), then an error is issued.
func (s *Scope) LookupModule(tok tokens.Module) *symbols.Module {
	if sym := s.Lookup(tokens.Token(tok)); sym != nil {
		if typsym, ok := sym.(*symbols.Module); ok {
			return typsym
		}
		// TODO: issue an error about an incorrect symbol type.
	} else {
		// TODO: issue an error about a missing symbol.
	}
	return nil
}

// LookupType finds a type symbol registered underneath the given token, issuing an error and returning nil if missing.
// If the symbol exists, but is of the wrong type (i.e., not a symbols.Type), then an error is issued.
func (s *Scope) LookupType(tok tokens.Type) symbols.Type {
	if sym := s.Lookup(tokens.Token(tok)); sym != nil {
		if typsym, ok := sym.(symbols.Type); ok {
			return typsym
		}
		// TODO: issue an error about an incorrect symbol type.
	} else {
		// TODO: issue an error about a missing symbol.
	}
	return nil
}

// Register registers a symbol with a given name; if it already exists, the function returns false.
func (s *Scope) Register(sym symbols.Symbol) bool {
	tok := sym.Token()
	if _, exists := s.symtbl[tok]; exists {
		// TODO: this won't catch "shadowing" for parent scopes; do we care about this?
		return false
	}

	s.symtbl[tok] = sym
	return true
}

// MustRegister registers a symbol with a given name; if it already exists, the function fail-fasts.
func (s *Scope) MustRegister(sym symbols.Symbol) {
	ok := s.Register(sym)
	contract.Assertf(ok, "Expected symbol %v to be unique; entry already found in this scope", sym.Token())
}

// TryRegister registers a symbol with the given name; if it already exists, a compiler error is emitted.
func (s *Scope) TryRegister(node ast.Node, sym symbols.Symbol) {
	if !s.Register(sym) {
		s.ctx.Diag.Errorf(errors.ErrorSymbolAlreadyExists.At(node), sym.Name)
	}
}
