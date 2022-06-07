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
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/zclconf/go-cty/cty"
)

// Definition represents a single definition in a Scope.
type Definition interface {
	Traversable

	SyntaxNode() hclsyntax.Node
}

// A Keyword is a non-traversable definition that allows scope traversals to bind to arbitrary keywords.
type Keyword string

// Traverse attempts to traverse the keyword, and always fails.
func (kw Keyword) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return DynamicType, hcl.Diagnostics{cannotTraverseKeyword(string(kw), traverser.SourceRange())}
}

// SyntaxNode returns the syntax node for the keyword, which is always syntax.None.
func (kw Keyword) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

// A Variable is a traversable, typed definition that represents a named value.
type Variable struct {
	// The syntax node associated with the variable definition, if any.
	Syntax hclsyntax.Node

	// The name of the variable.
	Name string
	// The type of the variable.
	VariableType Type
}

// Traverse attempts to traverse the variable's type.
func (v *Variable) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return v.VariableType.Traverse(traverser)
}

// SyntaxNode returns the variable's syntax node or syntax.None.
func (v *Variable) SyntaxNode() hclsyntax.Node {
	return syntaxOrNone(v.Syntax)
}

// Type returns the type of the variable.
func (v *Variable) Type() Type {
	return v.VariableType
}

func (v *Variable) Value(context *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	if value, hasValue := context.Variables[v.Name]; hasValue {
		return value, nil
	}
	return cty.DynamicVal, nil
}

// A Constant is a traversable, typed definition that represents a named constant.
type Constant struct {
	// The syntax node associated with the constant definition, if any.
	Syntax hclsyntax.Node

	// The name of the constant.
	Name string
	// The value of the constant.
	ConstantValue cty.Value

	typ Type
}

// Tracerse attempts to traverse the constant's value.
func (c *Constant) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	v, diags := traverser.TraversalStep(c.ConstantValue)
	return &Constant{ConstantValue: v}, diags
}

// SyntaxNode returns the constant's syntax node or syntax.None.
func (c *Constant) SyntaxNode() hclsyntax.Node {
	return syntaxOrNone(c.Syntax)
}

// Type returns the type of the constant.
func (c *Constant) Type() Type {
	if c.typ == nil {
		if c.ConstantValue.IsNull() {
			c.typ = NoneType
		} else {
			c.typ = ctyTypeToType(c.ConstantValue.Type(), false)
		}
	}
	return c.typ
}

func (c *Constant) Value(context *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	return c.ConstantValue, nil
}

// A Scope is used to map names to definitions during expression binding.
//
// A scope has two namespaces: one that is exclusive to functions and one that contains both variables and functions.
// When binding a reference, only the latter is checked; when binding a function, only the former is checked.
type Scope struct {
	parent    *Scope
	syntax    hclsyntax.Node
	defs      map[string]Definition
	functions map[string]*Function
}

// NewRootScope returns a new unparented scope associated with the given syntax node.
func NewRootScope(syntax hclsyntax.Node) *Scope {
	return &Scope{
		syntax:    syntax,
		defs:      map[string]Definition{},
		functions: map[string]*Function{},
	}
}

// Traverse attempts to traverse the scope using the given traverser. If the traverser is a literal string that refers
// to a name defined within the scope or one of its ancestors, the traversal returns the corresponding definition.
func (s *Scope) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	name, nameType := GetTraverserKey(traverser)
	if nameType != StringType {
		// TODO(pdg): return a better error here
		return DynamicType, hcl.Diagnostics{undefinedVariable("", traverser.SourceRange())}
	}

	memberName := name.AsString()
	member, hasMember := s.BindReference(memberName)
	if !hasMember {
		return DynamicType, hcl.Diagnostics{undefinedVariable(memberName, traverser.SourceRange())}
	}
	return member, nil
}

// SyntaxNode returns the syntax node associated with the scope, if any.
func (s *Scope) SyntaxNode() hclsyntax.Node {
	if s != nil {
		return syntaxOrNone(s.syntax)
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
// definition is not overwritten and Define returns false.
func (s *Scope) Define(name string, def Definition) bool {
	if s != nil {
		if _, hasDef := s.defs[name]; !hasDef {
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
			child := &Scope{
				parent:    s,
				syntax:    syntax,
				defs:      map[string]Definition{},
				functions: map[string]*Function{},
			}
			s.defs[name] = child
			return child, true
		}
	}
	return nil, false
}

// Push defines an anonymous child scope associated with the given syntax node.
func (s *Scope) Push(syntax hclsyntax.Node) *Scope {
	return &Scope{
		parent:    s,
		syntax:    syntax,
		defs:      map[string]Definition{},
		functions: map[string]*Function{},
	}
}

// Pop returns this scope's parent.
func (s *Scope) Pop() *Scope {
	return s.parent
}

// Scopes is the interface that is used fetch the scope that should be used when binding a block or attribute.
type Scopes interface {
	// GetScopesForBlock returns the Scopes that should be used when binding the given block.
	GetScopesForBlock(block *hclsyntax.Block) (Scopes, hcl.Diagnostics)
	// GetScopeForAttribute returns the *Scope that should be used when binding the given attribute.
	GetScopeForAttribute(attribute *hclsyntax.Attribute) (*Scope, hcl.Diagnostics)
}

type staticScope struct {
	scope *Scope
}

// GetScopesForBlock returns the scopes to use when binding the given block.
func (s staticScope) GetScopesForBlock(block *hclsyntax.Block) (Scopes, hcl.Diagnostics) {
	return s, nil
}

// GetScopeForAttribute returns the scope to use when binding the given attribute.
func (s staticScope) GetScopeForAttribute(attribute *hclsyntax.Attribute) (*Scope, hcl.Diagnostics) {
	return s.scope, nil
}

// StaticScope returns a Scopes that uses the given *Scope for all blocks and attributes.
func StaticScope(scope *Scope) Scopes {
	return staticScope{scope: scope}
}
