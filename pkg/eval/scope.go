// Copyright 2017 Pulumi, Inc. All rights reserved.

package eval

import (
	"github.com/pulumi/coconut/pkg/compiler/binder"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/compiler/types"
	"github.com/pulumi/coconut/pkg/eval/rt"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// localScope holds variable values that correspond to a specific lexical scope.
type localScope struct {
	root       *rt.Environment // a pointer to the root of all scopes.
	parent     rt.Environment  // the parent to restore when popping this scope.
	activation bool            // if a top-level frame, searches won't reach beyond this scope.
	lexical    *binder.Scope   // the binding scope tells us at what level, lexically, a local resides.
	slots      rt.Slots        // the values map contains the value for a variable so long as it exists.
}

var _ rt.Environment = (*localScope)(nil)

func newLocalScope(root *rt.Environment, activation bool, lex *binder.Scope) *localScope {
	s := &localScope{
		root:       root,
		parent:     *root,
		activation: activation,
		lexical:    lex,
		slots:      make(rt.Slots),
	}
	*root = s
	return s
}

func (s *localScope) Parent() rt.Environment { return s.parent }
func (s *localScope) Activation() bool       { return s.activation }
func (s *localScope) Lexical() *binder.Scope { return s.lexical }
func (s *localScope) Slots() rt.Slots        { return s.slots }

func (s *localScope) Push(activation bool) rt.Environment {
	lex := s.lexical.Push(activation)
	return newLocalScope(s.root, activation, lex)
}

func (s *localScope) Pop() {
	contract.Assert(*s.root == s)
	s.lexical.Pop()
	*s.root = s.parent
}

// Swap replaces a current scope with a new run, returning a popper function.
func (s *localScope) Swap(other rt.Environment) func() {
	*s.root = other
	*s.lexical.Root = other.Lexical()
	return func() {
		*s.root = s
		*s.lexical.Root = s.Lexical()
	}
}

// Lookup locates an existing variable by name in the current scope.
func (s *localScope) Lookup(nm tokens.Name) *symbols.LocalVariable {
	return s.lexical.Lookup(nm)
}

// Register registers a new variable in the scope.
func (s *localScope) Register(sym *symbols.LocalVariable) bool {
	return s.lexical.Register(sym)
}

// MustRegister registers a new variable in the scope, failing if it collides with an existing one.
func (s *localScope) MustRegister(sym *symbols.LocalVariable) {
	s.lexical.MustRegister(sym)
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

	slots := env.Slots()
	if ref, has := slots[sym]; has {
		contract.Assertf(place == nil, "Expected an empty value slot, given init usage; it was non-nil: %v", sym)
		return ref
	}
	if place != nil {
		slots[sym] = place
		return place
	}
	if init {
		ref := rt.NewPointer(nil, sym.Readonly(), nil, nil)
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
