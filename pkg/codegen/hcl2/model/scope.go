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
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/syntax"
	"github.com/zclconf/go-cty/cty"
)

type Definition interface {
	Traversable

	SyntaxNode() hclsyntax.Node
}

type Keyword string

func (kw Keyword) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return AnyType, hcl.Diagnostics{cannotTraverseKeyword(string(kw), traverser.SourceRange())}
}

func (kw Keyword) SyntaxNode() hclsyntax.Node {
	return &hclsyntax.LiteralValueExpr{Val: cty.StringVal(string(kw))}
}

type Variable struct {
	Syntax hclsyntax.Node

	Name         string
	VariableType Type
}

func (v *Variable) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return v.VariableType, nil
}

func (v *Variable) SyntaxNode() hclsyntax.Node {
	return v.Syntax
}

// A Scope is used to map names to definitions during expression binding.
type Scope struct {
	parent    *Scope
	syntax    hclsyntax.Node
	defs      map[string]Definition
	functions map[string]*Function
}

func NewRootScope(syntax hclsyntax.Node) *Scope {
	return &Scope{syntax: syntax, defs: map[string]Definition{}, functions: map[string]*Function{}}
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
	if s != nil {
		return s.syntax
	}
	return syntax.None
}

// BindReference returns the definition that corresponds to the given name, if any. Each parent scope is checked until
// a definition is found or no parent scope remains.
func (s *Scope) BindReference(name string) (Definition, bool) {
	if s != nil {
		if def, ok := s.defs[name]; ok {
			return def, true
		}
		if s.parent != nil {
			return s.parent.BindReference(name)
		}
	}
	return nil, false
}

// BindFunctionReference returns the function definition that corresponds to the given name, if any. Each parent scope
// is checked until a definition is found or no parent scope remains.
func (s *Scope) BindFunctionReference(name string) (*Function, bool) {
	if s != nil {
		if fn, ok := s.functions[name]; ok {
			return fn, true
		}
		if s.parent != nil {
			return s.parent.BindFunctionReference(name)
		}
	}
	return nil, false
}

// Define maps the given name to the given definition. If the name is already defined in this scope, the existing
// definition is not overwritten and Define returns false. If the definition is a function, it is also added to the
// scope's set of functions unless a function with the given name already exists.
func (s *Scope) Define(name string, def Definition) bool {
	if s != nil {
		if _, hasDef := s.defs[name]; !hasDef {
			if fn, isFunc := def.(*Function); isFunc && !s.DefineFunction(name, fn) {
				return false
			}
			s.defs[name] = def
			return true
		}
	}
	return false
}

// DefineFunction maps the given function name to the given function definition. If the function is alreadu defined in
// this scope, the definition is not overwritten and DefineFunction returns false.
func (s *Scope) DefineFunction(name string, def *Function) bool {
	if s != nil {
		if _, hasFunc := s.functions[name]; !hasFunc {
			s.functions[name] = def
			return true
		}
	}
	return false
}

// DefineScope defines a child scope with the given name. If the name is already defined in this scope, the existing
// definition is not overwritten and DefineScope returns false.
func (s *Scope) DefineScope(name string, syntax hclsyntax.Node) (*Scope, bool) {
	if s != nil {
		if _, exists := s.defs[name]; !exists {
			child := &Scope{parent: s, syntax: syntax, defs: map[string]Definition{}, functions: map[string]*Function{}}
			s.defs[name] = child
			return child, true
		}
	}
	return nil, false
}

// PushScope defines an anonymous child scope associated with the given syntax node.
func (s *Scope) PushScope(syntax hclsyntax.Node) *Scope {
	return &Scope{parent: s, syntax: syntax, defs: map[string]Definition{}, functions: map[string]*Function{}}
}

// Pop returns this scope's parent.
func (s *Scope) Pop() *Scope {
	return s.parent
}
