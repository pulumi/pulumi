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
	slot       *rt.Environment
	parent     rt.Environment // the parent to restore when popping this scope.
	activation bool           // if a top-level frame, searches won't reach beyond this scope.
	lexical    *binder.Scope  // the binding scope tells us at what level, lexically, a local resides.
	values     rt.Slots       // the values map contains the value for a variable so long as it exists.
}

var _ rt.Environment = (*localScope)(nil)

func newLocalScope(slot *rt.Environment, activation bool, lex *binder.Scope) *localScope {
	s := &localScope{
		slot:       slot,
		parent:     *slot,
		activation: activation,
		lexical:    lex,
		values:     make(rt.Slots),
	}
	*slot = s
	return s
}

func (s *localScope) Parent() rt.Environment { return s.parent }
func (s *localScope) Activation() bool       { return s.activation }
func (s *localScope) Lexical() *binder.Scope { return s.lexical }
func (s *localScope) Values() rt.Slots       { return s.values }

func (s *localScope) Push(frame bool) rt.Environment {
	lex := s.lexical.Push(frame)
	return newLocalScope(s.slot, frame, lex)
}

func (s *localScope) Pop() {
	contract.Assert(*s.slot == s)
	s.lexical.Pop()
	*s.slot = s.parent
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
	lex := s.lexical
	var env rt.Environment = s
outer:
	for {
		// Search this frame for a match.
		for _, lexloc := range lex.Locals {
			if lexloc == sym {
				break outer
			}
		}

		// If we hit the top of the activation records, quit looking.
		if env.Activation() {
			break
		}

		env = env.Parent()
		lex = lex.Parent
		contract.Assert(env.Lexical() == lex)
		contract.Assert(env.Activation() == lex.Activation)
	}
	contract.Assert(env != nil)
	contract.Assert(lex != nil)

	slots := env.Values()
	if ref, has := slots[sym]; has {
		contract.Assertf(place == nil, "Expected an empty value slot, given init usage; it was non-nil: %v", sym)
		return ref
	}
	if place != nil {
		slots[sym] = place
		return place
	}
	if init {
		ref := rt.NewPointer(nil, sym.Readonly())
		slots[sym] = ref
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
