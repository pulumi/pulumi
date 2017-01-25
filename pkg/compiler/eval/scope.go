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
type valueMap map[*symbols.LocalVariable]*Object

func newLocalScope(slot **localScope, frame bool, lex *binder.Scope) *localScope {
	return &localScope{
		Slot:    slot,
		Parent:  *slot,
		Frame:   frame,
		Lexical: lex,
		Values:  make(valueMap),
	}
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

func (s *localScope) GetValue(sym *symbols.LocalVariable) *Object {
	for s != nil {
		if val, has := s.Values[sym]; has {
			return val
		}

		// Keep looking into parent scopes so long as we didn't hit the top of an activation frame.
		if s.Frame {
			s = nil
		} else {
			s = s.Parent
		}
	}
	return nil
}

func (s *localScope) SetValue(sym *symbols.LocalVariable, obj *Object) {
	contract.Assert(obj == nil || types.CanConvert(obj.Type, sym.Type()))

	// To set a value, we must first find the position in the shadowed frames, so that the value's lifetime is identical
	// to the actual local variable symbol's lifetime.  This ensures that once that frame is popped, so too is any value
	// associated with it; and similarly, that its value won't be popped until the frame containing the variable is.
	lex := s.Lexical
	for {
		for _, lexloc := range lex.Locals {
			if lexloc == sym {
				break
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
	s.Values[sym] = obj
}
