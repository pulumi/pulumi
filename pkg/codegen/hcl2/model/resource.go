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
)

// Resource represents a resource instantiation inside of a program or component.
type Resource struct {
	node

	// The syntax node associated with the resource instantiation.
	Syntax *hclsyntax.Block
	// The syntax tokens associated with the resource instantiation.
	Tokens syntax.BlockTokens

	// The type of the resource's inputs. This will always be either Any or an object type.
	InputType Type
	// The type of the resource's outputs. This will always be either Any or an object type.
	OutputType Type

	// The inputs to this resource. This will always be an ObjectConsExpression or an ErrorExpression.
	Inputs Expression
	// The range expression for this resource, if any. TODO: unimplemented.
	Range Expression

	// TODO: Resource options
}

// SyntaxNode returns the syntax node associated with the resource.
func (r *Resource) SyntaxNode() hclsyntax.Node {
	return r.Syntax
}

// Type returns the type of the resource.
func (r *Resource) Type() Type {
	return r.OutputType
}

func (r *Resource) Traverse(traverser hcl.Traverser) (Traversable, hcl.Diagnostics) {
	return r.OutputType.Traverse(traverser)
}

// Name returns the name of the resource.
func (r *Resource) Name() string {
	return r.Syntax.Labels[0]
}

// DecomposeToken attempts to decompose the resource's type token into its package, module, and type. If decomposition
// fails, a description of the failure is returned in the diagnostics.
func (r *Resource) DecomposeToken() (string, string, string, hcl.Diagnostics) {
	token, tokenRange := getResourceToken(r)
	return decomposeToken(token, tokenRange)
}
