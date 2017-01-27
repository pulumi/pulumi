// Copyright 2016 Marapongo, Inc. All rights reserved.

package eval

import (
	"github.com/marapongo/mu/pkg/compiler/binder"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/compiler/types"
	"github.com/marapongo/mu/pkg/util/contract"
)

// localScope is a kind of scope that holds local variable values.
type localScope struct {
	Slot    **localScope
	Parent  *localScope   // the parent to restore when popping this scope.
	Frame   bool          // if a top-level frame, searches won't reach beyond this scope.
	Lexical *binder.Scope // the binding scope tells us at what level, lexically, a local resides.
	Values  valueMap      // the values map contains the value for a variable so long as it exists.
}

// valueMap maps local variables to their current known object value (if any).
type valueMap map[*symbols.LocalVariable]*Reference

func newLocalScope(slot **localScope, frame bool, lex *binder.Scope) *localScope {
	s := &localScope{
		Slot:    slot,
		Parent:  *slot,
		Frame:   frame,
		Lexical: lex,
		Values:  make(valueMap),
	}
	*slot = s
	return s
}

func (s *localScope) Push(frame bool) *localScope {
	lex := s.Lexical.Push(frame)
	return newLocalScope(s.Slot, frame, lex)
}

func (s *localScope) Pop() {
	contract.Assert(*s.Slot == s)
	s.Lexical.Pop()
	*s.Slot = s.Parent
}

// GetValue returns the object value for the given symbol.
func (s *localScope) GetValue(sym *symbols.LocalVariable) *Object {
	if ref := s.GetValueReference(sym, false); ref != nil {
		return ref.Get()
	}
	return nil
}

// GetValueReference returns a reference to the object for the given symbol.  If init is true, and the value doesn't
// exist, a new slot will be allocated.  Otherwise, the return value is nil.
func (s *localScope) GetValueReference(sym *symbols.LocalVariable, init bool) *Reference {
	return s.lookupValueReference(sym, nil, init)
}

// InitValue registers a reference for a local variable, and asserts that none previously existed.
func (s *localScope) InitValueReference(sym *symbols.LocalVariable, ref *Reference) {
	s.lookupValueReference(sym, ref, false)
}

// lookupValueReference is used to lookup and initialize references using a single, shared routine.
func (s *localScope) lookupValueReference(sym *symbols.LocalVariable, place *Reference, init bool) *Reference {
	// To get a value's reference, we must first find the position in the shadowed frames, so that its lifetime equals
	// the actual local variable symbol's lifetime.  This ensures that once that frame is popped, so too is any value
	// associated with it; and similarly, that its value won't be popped until the frame containing the variable is.
	lex := s.Lexical
outer:
	for {
		for _, lexloc := range lex.Locals {
			if lexloc == sym {
				break outer
			}
		}
		contract.Assert(!s.Frame)
		contract.Assert(!lex.Frame)
		s = s.Parent
		lex = lex.Parent
		contract.Assert(s.Lexical == lex)
	}
	contract.Assert(s != nil)
	contract.Assert(lex != nil)

	if ref, has := s.Values[sym]; has {
		contract.Assertf(place == nil, "Expected an empty value slot, given init usage; it was non-nil: %v", sym)
		return ref
	} else if place != nil {
		s.Values[sym] = place
		return place
	} else if init {
		ref := &Reference{}
		s.Values[sym] = ref
		return ref
	} else {
		return nil
	}
}

// SetValue overwrites the current value, or adds a new entry, for the given symbol.
func (s *localScope) SetValue(sym *symbols.LocalVariable, obj *Object) {
	contract.Assert(obj == nil || types.CanConvert(obj.Type, sym.Type()))
	ref := s.GetValueReference(sym, true)
	ref.Set(obj)
}
