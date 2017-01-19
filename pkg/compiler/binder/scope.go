// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Scope enables lookups and symbols to obey traditional language scoping rules.
type Scope struct {
	ctx    *core.Context
	parent *Scope
	symtbl SymbolTable
}

// SymbolTable is a mapping from symbol token to an actual symbol object representing it.
type SymbolTable map[tokens.Token]symbols.Symbol

// NewScope allocates and returns a fresh scope with the optional parent scope attached to it.
func NewScope(ctx *core.Context, parent *Scope) *Scope {
	return &Scope{
		ctx:    ctx,
		parent: parent,
		symtbl: make(SymbolTable),
	}
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
