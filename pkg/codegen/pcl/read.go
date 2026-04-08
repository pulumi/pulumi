// Copyright 2026, Pulumi Corporation.
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

package pcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/zclconf/go-cty/cty"
)

var _ resourceLike = (*Read)(nil)

func (r *Read) getSyntax() *hclsyntax.Block   { return r.syntax }
func (r *Read) getOptions() **ResourceOptions { return &r.Options }
func (r *Read) getVariableType() *model.Type  { return &r.VariableType }
func (r *Read) getInput() model.Type          { return r.StateType }

// Read represents a resource read inside of a program or component.
type Read struct {
	node

	syntax *hclsyntax.Block

	// The name visible to API calls related to the resource. Used as the Name argument in resource
	// constructors, and through those calls to RegisterResource. Must not be modified during code
	// generation to ensure that resources are not renamed (deleted and recreated).
	logicalName string

	// The definition of the resource.
	Definition *model.Block

	// When set to true, allows traversing unknown properties through a resource. i.e. `resource.unknownProperty`
	// will be valid and the type of the traversal is dynamic. This property is set to false by default
	LenientTraversal bool

	// Token is the type token for this resource.
	Token string

	// Schema is the schema definition for this resource, if any.
	Schema *schema.Resource

	// The type of the resource's state. This will always be either Any or an object type.
	StateType model.Type

	// The type of the resource's outputs. This will always be either Any or an object type.
	OutputType model.Type

	// The type of the resource variable.
	VariableType model.Type

	// The resource's state attributes, in source order.
	State []*model.Attribute

	// The resource's options, if any.
	Options *ResourceOptions
}

// SyntaxNode returns the syntax node associated with the resource.
func (r *Read) SyntaxNode() hclsyntax.Node {
	return r.syntax
}

// Type returns the type of the resource.
func (r *Read) Type() model.Type {
	return r.VariableType
}

func (r *Read) VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics {
	return model.VisitExpressions(r.Definition, pre, post)
}

func (r *Read) Value(context *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	if value, hasValue := hcl2.LookupVariable(context, r.Name()); hasValue {
		return value, nil
	}
	return cty.DynamicVal, nil
}

func (r *Read) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	if r == nil || r.VariableType == nil {
		return model.DynamicType.Traverse(traverser)
	}

	traversable, diags := r.VariableType.Traverse(traverser)

	if diags.HasErrors() && r.LenientTraversal {
		return model.DynamicType.Traverse(traverser)
	}

	return traversable, diags
}

// Deprecated: Name returns the variable or declaration name of the resource.
func (r *Read) Name() string {
	return r.Definition.Labels[0]
}

// Returns the unique name of the resource; if the resource has an unique name it is formatted with
// the format string and returned, otherwise the defaultValue is returned as is.
func (r *Read) LogicalName() string {
	if r.logicalName != "" {
		return r.logicalName
	}

	return r.Name()
}

// DecomposeToken attempts to decompose the resource's type token into its package, module, and type. If decomposition
// fails, a description of the failure is returned in the diagnostics.
func (r *Read) DecomposeToken() (string, string, string, hcl.Diagnostics) {
	return DecomposeToken(r.Token, r.syntax.LabelRanges[1])
}
