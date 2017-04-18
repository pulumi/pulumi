// Copyright 2017 Pulumi, Inc. All rights reserved.

package eval

import (
	"github.com/pulumi/coconut/pkg/compiler/binder"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/compiler/types"
	"github.com/pulumi/coconut/pkg/eval/rt"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// localScope holds variable values that correspond to a specific lexical scope.
type localScope struct {
	Slot    **localScope
	Parent  *localScope   // the parent to restore when popping this scope.
	Frame   bool          // if a top-level frame, searches won't reach beyond this scope.
	Lexical *binder.Scope // the binding scope tells us at what level, lexically, a local resides.
	Values  valueMap      // the values map contains the value for a variable so long as it exists.
}

// valueMap maps variables to their current known object value in this scope (if any).
type valueMap map[*symbols.LocalVariable]*rt.Pointer

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
func (s *localScope) GetValue(sym *symbols.LocalVariable) *rt.Object {
	if ref := s.GetValueAddr(sym, false); ref != nil {
		return ref.Obj()
	}
	return nil
}

// GetValueAddr returns a reference to the object for the given symbol.  If init is true, and the value doesn't
// exist, a new slot will be allocated.  Otherwise, the return value is nil.
func (s *localScope) GetValueAddr(sym *symbols.LocalVariable, init bool) *rt.Pointer {
	contract.Require(sym != nil, "sym")
	return s.lookupValueAddr(sym, nil, init)
}

// InitValue registers a reference for a variable, and asserts that none previously existed.
func (s *localScope) InitValueAddr(sym *symbols.LocalVariable, ref *rt.Pointer) {
	contract.Require(sym != nil, "sym")
	contract.Require(ref != nil, "ref")
	s.lookupValueAddr(sym, ref, false)
}

// lookupValueAddr is used to lookup and initialize references using a single, shared routine.
func (s *localScope) lookupValueAddr(sym *symbols.LocalVariable, place *rt.Pointer, init bool) *rt.Pointer {
	contract.Require(sym != nil, "sym")

	// To get a value's reference, we must first find the position in the shadowed frames, so that its lifetime equals
	// the actual variable symbol's lifetime.  This ensures that once that frame is popped, so too is any value
	// associated with it; and similarly, that its value won't be popped until the frame containing the variable is.
	lex := s.Lexical
outer:
	for {
		// Search this frame for a match.
		for _, lexloc := range lex.Locals {
			if lexloc == sym {
				break outer
			}
		}

		// If we hit the top of the activation records, quit looking.
		if s.Frame {
			break
		}

		s = s.Parent
		lex = lex.Parent
		contract.Assert(s.Lexical == lex)
		contract.Assert(s.Frame == lex.Frame)
	}
	contract.Assert(s != nil)
	contract.Assert(lex != nil)

	if ref, has := s.Values[sym]; has {
		contract.Assertf(place == nil, "Expected an empty value slot, given init usage; it was non-nil: %v", sym)
		return ref
	}
	if place != nil {
		s.Values[sym] = place
		return place
	}
	if init {
		ref := rt.NewPointer(nil, sym.Readonly())
		s.Values[sym] = ref
		return ref
	}
	return nil
}

// SetValue overwrites the current value, or adds a new entry, for the given symbol.
func (s *localScope) SetValue(sym *symbols.LocalVariable, obj *rt.Object) {
	contract.Assert(obj == nil || types.CanConvert(obj.Type(), sym.Type()))
	ptr := s.GetValueAddr(sym, true)
	ptr.Set(obj)
}
