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
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// PackageReferenceType is the type of a bare reference to a package block,
// valid only as a resource or invoke `provider` option.
var PackageReferenceType model.Type = model.NewOpaqueType("PackageReference")

// PackageVariable represents a package block. Referencing it by name as a
// resource or invoke `provider` option resolves the token against the
// referenced package's schema and routes the registration to that package's
// default provider. Package variables are scope definitions rather than
// program nodes: they can be referenced by expressions but do not appear in
// Program.Nodes.
type PackageVariable struct {
	syntax *hclsyntax.Block

	// LocalName is the name by which the package block is referenced in the
	// program: the name of the package it declares.
	LocalName string

	// Descriptor describes the package the block declares.
	Descriptor *schema.PackageDescriptor
}

// SyntaxNode returns the syntax node associated with the package variable.
func (pv *PackageVariable) SyntaxNode() hclsyntax.Node {
	return pv.syntax
}

func (pv *PackageVariable) Traverse(traverser hcl.Traverser) (model.Traversable, hcl.Diagnostics) {
	return PackageReferenceType.Traverse(traverser)
}

func (pv *PackageVariable) Name() string {
	return pv.LocalName
}

// Type returns the type of the package variable.
func (pv *PackageVariable) Type() model.Type {
	return PackageReferenceType
}

// ReferencesPackageBlock reports whether the expression is a bare reference to
// a package block (see PackageVariable).
func ReferencesPackageBlock(expr model.Expression) bool {
	traversal, ok := expr.(*model.ScopeTraversalExpression)
	if !ok || len(traversal.Parts) != 1 {
		return false
	}
	_, ok = traversal.Parts[0].(*PackageVariable)
	return ok
}

// GeneratedInvokeOptions returns the invoke option items that have a generated
// form. A provider option naming a package block selects the package the
// token resolves against, which the SDK carries as the package reference
// rather than an invoke option.
func GeneratedInvokeOptions(options *model.ObjectConsExpression) []model.ObjectConsItem {
	items := make([]model.ObjectConsItem, 0, len(options.Items))
	for _, item := range options.Items {
		if LiteralValueString(item.Key) == "provider" && ReferencesPackageBlock(item.Value) {
			continue
		}
		items = append(items, item)
	}
	return items
}
