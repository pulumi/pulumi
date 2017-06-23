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

package binder

import (
	"github.com/pulumi/lumi/pkg/compiler/ast"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Scope facilitates storing information that obeys lexically nested scoping rules.
type Scope struct {
	Root       **Scope  // the slot rooting the tree of current scopes for pushing/popping.
	Ctx        *Context // the shared context object for errors, etc.
	Activation bool     // if this scope represents the top of an activation frame.
	Parent     *Scope   // the parent scope to restore upon pop, or nil if this is the top.
	Locals     LocalMap // the current scope's locals map (name to local symbol).
}

// LocalMap maps the name of locals to their corresponding symbols, for the few places we need name-based lookup.
type LocalMap map[tokens.Name]*symbols.LocalVariable

// NewScope allocates and returns a fresh scope using the given slot, populating it.
func NewScope(ctx *Context, activation bool) *Scope {
	root := &ctx.Scope
	scope := &Scope{
		Root:       root,
		Ctx:        ctx,
		Activation: activation,
		Parent:     *root,
		Locals:     make(LocalMap),
	}
	*root = scope
	return scope
}

// Push creates a new scope with an empty symbol table parented to the existing one.
func (s *Scope) Push(activation bool) *Scope {
	return NewScope(s.Ctx, activation)
}

// Pop restores the prior scope into the underlying slot, tossing away the current symbol table.
func (s *Scope) Pop() {
	contract.Assert(*s.Root == s)
	*s.Root = s.Parent
}

// Lookup finds a variable underneath the given name, issuing an error and returning nil if not found.
func (s *Scope) Lookup(nm tokens.Name) *symbols.LocalVariable {
	for s != nil {
		if sym, exists := s.Locals[nm]; exists {
			contract.Assert(sym != nil)
			return sym
		}

		// If not in this scope, keep looking at the ancestral chain, if one exists.
		if s.Activation {
			break
		}

		s = s.Parent
	}
	return nil
}

// Register registers a local variable with a given name; if it already exists, the function returns false.
func (s *Scope) Register(sym *symbols.LocalVariable) bool {
	nm := sym.Name()
	if _, exists := s.Locals[nm]; exists {
		// TODO[pulumi/lumi#176]: this won't catch "shadowing" for parent scopes; do we care about this?
		return false
	}

	s.Locals[nm] = sym
	return true
}

// MustRegister registers a local with the given name; if it already exists, the function abandons.
func (s *Scope) MustRegister(sym *symbols.LocalVariable) {
	if !s.Register(sym) {
		contract.Failf("Symbol already exists: %v", sym.Name)
	}
}

// TryRegister registers a local with the given name; if it already exists, a compiler error is emitted.
func (s *Scope) TryRegister(node ast.Node, sym *symbols.LocalVariable) {
	if !s.Register(sym) {
		s.Ctx.Diag.Errorf(errors.ErrorSymbolAlreadyExists.At(node), sym.Name)
	}
}
