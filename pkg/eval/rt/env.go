// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rt

import (
	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/tokens"
)

// Environment represents a current chained collection of frames, forming an environment of variables.
type Environment interface {
	// Parent is an optional frame to which this parent is chained (and is restored upon popping).
	Parent() Environment
	// Activation indicates whether this frame is a top-level activation record, constraining lookups.
	Activation() bool
	// Lexical is the corresponding binding scope for this object, telling us at what level a local is, lexically.
	Lexical() *binder.Scope
	// Slots is a map of variable to slot.
	Slots() Slots

	// Push creates a new frame chained to the current one.  If activation is true, it is the top of a record.
	Push(activation bool) Environment
	// Pop restores the previous frame.
	Pop()
	// Swap replaces a current scope with a new run, returning a popper function.
	Swap(other Environment) func()

	// Lookup locates an existing variable by name in the current scope.
	Lookup(nm tokens.Name) *symbols.LocalVariable
	// Register registers a new variable in the scope.
	Register(sym *symbols.LocalVariable) bool
	// MustRegister registers a new variable in the scope, failing if it collides with an existing one.
	MustRegister(sym *symbols.LocalVariable)

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
