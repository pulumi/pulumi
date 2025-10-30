package model

import model "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model"

// Definition represents a single definition in a Scope.
type Definition = model.Definition

// A Keyword is a non-traversable definition that allows scope traversals to bind to arbitrary keywords.
type Keyword = model.Keyword

// A Variable is a traversable, typed definition that represents a named value.
type Variable = model.Variable

// A Constant is a traversable, typed definition that represents a named constant.
type Constant = model.Constant

// A Scope is used to map names to definitions during expression binding.
// 
// A scope has three namespaces:
//   - one that is exclusive to functions
//   - one that is exclusive to output variables
//   - and one that contains config variables, local variables and resource definitions.
// 
// When binding a reference, we check `defs` and `outputs`. When binding a function, we check `functions`.
// Definitions within a namespace such as `defs`, `outputs` or `functions` are expected to have a unique identifier
// and cannot be redeclared.
type Scope = model.Scope

// Scopes is the interface that is used fetch the scope that should be used when binding a block or attribute.
type Scopes = model.Scopes

// NewRootScope returns a new unparented scope associated with the given syntax node.
func NewRootScope(syntax hclsyntax.Node) *Scope {
	return model.NewRootScope(syntax)
}

// StaticScope returns a Scopes that uses the given *Scope for all blocks and attributes.
func StaticScope(scope *Scope) Scopes {
	return model.StaticScope(scope)
}

