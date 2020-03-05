// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type Definition interface {
	Traversable

	SyntaxNode() hclsyntax.Node
}

// A Scope is used to map names to definitions during expression binding.
type Scope struct {
	parent *Scope
	syntax hclsyntax.Node
	defs   map[string]Definition
}

func (s *Scope) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	name, nameType := GetTraverserKey(traverser)
	if nameType != StringType {
		return AnyType, hcl.Diagnostics{undefinedVariable(traverser.SourceRange())}
	}

	memberName := name.AsString()
	member, hasMember := s.BindReference(memberName)
	if !hasMember {
		return AnyType, hcl.Diagnostics{undefinedVariable(traverser.SourceRange())}
	}
	return member, nil
}

func (s *Scope) SyntaxNode() hclsyntax.Node {
	return s.syntax
}

// BindReference returns the definition that corresponds to the given name, if any. Each parent scope is checked until
// a definition is found or no parent scope remains.
func (s *Scope) BindReference(name string) (Definition, bool) {
	if def, ok := s.defs[name]; ok {
		return def, ok
	}
	if s.parent != nil {
		return s.parent.BindReference(name)
	}
	return nil, false
}

// Define maps the given name to the given definition. If the name is already defined in this scope, the existing
// definition is not overwritten and Define returns false.
func (s *Scope) Define(name string, node Node) bool {
	if _, exists := s.defs[name]; exists {
		return false
	}
	s.defs[name] = node
	return true
}

// DefineScope defines a child scope with the given name. If the name is already defined in this scope, the existing
// definition is not overwritten and DefineScope returns false.
func (s *Scope) DefineScope(name string, syntax hclsyntax.Node) (*Scope, bool) {
	if _, exists := s.defs[name]; exists {
		return nil, false
	}
	child := &Scope{parent: s, syntax: syntax, defs: map[string]Definition{}}
	s.defs[name] = child
	return child, true
}

// Scopes tracks a stack of scopes that is used to map names to definitions when binding scope traversal expressions.
// Though it is not legal for a name to be defined twice in a given scope, it is legal for a child scope to redefine a
// name present in its ancestors.
type Scopes struct {
	stack []*Scope
}

// NewScopes creates a new scope stack and pushes a root scope on the stack.
func NewScopes(rootSyntax hclsyntax.Node) (*Scopes, *Scope) {
	scopes := &Scopes{}
	return scopes, scopes.Push(rootSyntax)
}

// Push creates a new scope and pushes it on the scope stack.
func (s *Scopes) Push(syntax hclsyntax.Node) *Scope {
	next := &Scope{parent: s.stack[len(s.stack)-1], syntax: syntax, defs: map[string]Definition{}}
	s.stack = append(s.stack, next)
	return next
}

// Pop removes the top of the scope stack.
func (s *Scopes) Pop() {
	s.stack = s.stack[:len(s.stack)-1]
}

// BindReference returns the definition that corresponds to the given name, if any. Each scope on the stack is checked
// in LIFO order.
func (s *Scopes) BindReference(name string) (Definition, bool) {
	return s.stack[len(s.stack)-1].BindReference(name)
}
