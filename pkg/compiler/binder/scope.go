// Copyright 2016 Marapongo, Inc. All rights reserved.

package binder

import (
	"github.com/marapongo/mu/pkg/compiler/legacy"
	"github.com/marapongo/mu/pkg/compiler/legacy/ast"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
)

// PushScope creates a new scope with an empty symbol table parented to the existing one.
func (b *binder) PushScope() {
	b.scope = &scope{parent: b.scope, symtbl: make(map[tokens.Name]*legacy.Symbol)}
}

// PopScope replaces the current scope with its parent.
func (b *binder) PopScope() {
	contract.Assertf(b.scope != nil, "Unexpected empty binding scope during pop")
	b.scope = b.scope.parent
}

// scope enables lookups and symbols to obey traditional language scoping rules.
type scope struct {
	parent *scope
	symtbl map[tokens.Name]*legacy.Symbol
}

// LookupService binds a name to a Service type.
func (s *scope) LookupService(nm tokens.Name) (*ast.Service, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == legacy.SymKindService {
		return sym.Real.(*ast.Service), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupStack binds a name to a Stack type.
func (s *scope) LookupStack(nm tokens.Name) (*ast.Stack, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == legacy.SymKindStack {
		return sym.Real.(*ast.Stack), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupUninstStack binds a name to a UninstStack type.
func (s *scope) LookupUninstStack(nm tokens.Name) (*ast.UninstStack, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == legacy.SymKindUninstStack {
		return sym.Real.(*ast.UninstStack), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupSchema binds a name to a Schema type.
func (s *scope) LookupSchema(nm tokens.Name) (*ast.Schema, bool) {
	sym, exists := s.LookupSymbol(nm)
	if exists && sym.Kind == legacy.SymKindSchema {
		return sym.Real.(*ast.Schema), true
	}
	// TODO: we probably need to issue an error for this condition (wrong expected symbol type).
	return nil, false
}

// LookupSymbol binds a name to any kind of Symbol.
func (s *scope) LookupSymbol(nm tokens.Name) (*legacy.Symbol, bool) {
	for s != nil {
		if sym, exists := s.symtbl[nm]; exists {
			return sym, true
		}
		s = s.parent
	}
	return nil, false
}

// RegisterSymbol registers a symbol with the given name; if it already exists, the function returns false.
func (s *scope) RegisterSymbol(sym *legacy.Symbol) bool {
	nm := sym.Name
	if _, exists := s.symtbl[nm]; exists {
		// TODO: this won't catch "shadowing" for parent scopes; do we care about this?
		return false
	}

	s.symtbl[nm] = sym
	return true
}
