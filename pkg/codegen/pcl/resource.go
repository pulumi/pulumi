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

package pcl

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// ResourceOptions represents a resource instantiation's options.
type ResourceOptions struct {
	// The definition of the resource options.
	Definition *model.Block

	// An expression to range over when instantiating the resource.
	Range model.Expression
	// The resource's parent, if any.
	Parent model.Expression
	// The provider to use.
	Provider model.Expression
	// The explicit dependencies of the resource.
	DependsOn model.Expression
	// Whether or not the resource is protected.
	Protect model.Expression
	// Whether the resource should be left in the cloud provider
	// when it's deleted from the Pulumi state.
	RetainOnDelete model.Expression
	// A list of properties that are not considered when diffing the resource.
	IgnoreChanges model.Expression
	// The version of the provider for this resource.
	Version model.Expression
	// The plugin download URL for this resource.
	PluginDownloadURL model.Expression
	// If set, the provider's Delete method will not be called for this resource if the specified resource is being
	// deleted as well.
	DeletedWith model.Expression
	// If the resource was imported, the id that was imported.
	ImportID model.Expression
}

// Resource represents a resource instantiation inside of a program or component.
type Resource struct {
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

	// The type of the resource's inputs. This will always be either Any or an object type.
	InputType model.Type
	// The type of the resource's outputs. This will always be either Any or an object type.
	OutputType model.Type

	// The type of the resource variable.
	VariableType model.Type

	// The resource's input attributes, in source order.
	Inputs []*model.Attribute

	// The resource's options, if any.
	Options *ResourceOptions
}

// SyntaxNode returns the syntax node associated with the resource.
func (r *Resource) SyntaxNode() hclsyntax.Node {
	return r.syntax
}

// Type returns the type of the resource.
func (r *Resource) Type() model.Type {
	return r.VariableType
}

func (r *Resource) VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics {
	return model.VisitExpressions(r.Definition, pre, post)
}

func (r *Resource) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
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
func (r *Resource) Name() string {
	return r.Definition.Labels[0]
}

// Returns the unique name of the resource; if the resource has an unique name it is formatted with
// the format string and returned, otherwise the defaultValue is returned as is.
func (r *Resource) LogicalName() string {
	if r.logicalName != "" {
		return r.logicalName
	}

	return r.Name()
}

// DecomposeToken attempts to decompose the resource's type token into its package, module, and type. If decomposition
// fails, a description of the failure is returned in the diagnostics.
func (r *Resource) DecomposeToken() (string, string, string, hcl.Diagnostics) {
	_, tokenRange := getResourceToken(r)
	return DecomposeToken(r.Token, tokenRange)
}

// ResourceProperty represents a resource property.
type ResourceProperty struct {
	Path         hcl.Traversal
	PropertyType model.Type
}

func (*ResourceProperty) SyntaxNode() hclsyntax.Node {
	return syntax.None
}

func (p *ResourceProperty) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	propertyType, diagnostics := p.PropertyType.Traverse(traverser)
	return &ResourceProperty{
		Path:         append(p.Path, traverser),
		PropertyType: propertyType.(model.Type),
	}, diagnostics
}

func (p *ResourceProperty) Type() model.Type {
	return ResourcePropertyType
}
