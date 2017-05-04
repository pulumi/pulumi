// Copyright 2017 Pulumi, Inc. All rights reserved.

package rt

import (
	"github.com/pulumi/coconut/pkg/compiler/binder"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
)

// Environment represents a current chained collection of frames, forming an environment of variables.
type Environment interface {
	// Parent is an optional frame to which this parent is chained (and is restored upon popping).
	Parent() Environment
	// Activation indicates whether this frame is a top-level activation record, constraining lookups.
	Activation() bool
	// Lexical is the corresponding binding scope for this object, telling us at what level a local is, lexically.
	Lexical() *binder.Scope
	// Values is a map of variable to slot.
	Values() Slots

	// Push creates a new frame chained to the current one.  If activation is true, it is the top of a record.
	Push(activation bool) Environment
	// Pop restores the previous frame.
	Pop()

	// GetValue fetches the runtime object for the given variable.
	GetValue(sym *symbols.LocalVariable) *Object
	// GetValueAddr fetches a pointer to the slot holding the given variable.
	GetValueAddr(sym *symbols.LocalVariable, init bool) *Pointer
	// InitValueAddr registers a reference for a variable, asserting that none previously existed.
	InitValueAddr(sym *symbols.LocalVariable, ref *Pointer)
	// SetValue unconditionally overwrites the current value for a variable's slot.
	SetValue(sym *symbols.LocalVariable, obj *Object)
}

// Slots maps variables to their current known object value in this scope (if any).
type Slots map[*symbols.LocalVariable]*Pointer
