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

package hcl2

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/codegen"
	"github.com/pulumi/pulumi/pkg/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/codegen/schema"
)

func getResourceToken(node *Resource) (string, hcl.Range) {
	return node.Syntax.Labels[1], node.Syntax.LabelRanges[1]
}

func (b *binder) bindResource(node *Resource) hcl.Diagnostics {
	return b.bindResourceBody(node)
}

// bindResourceTypes binds the input and output types for a resource.
func (b *binder) bindResourceTypes(node *Resource) hcl.Diagnostics {
	// Set the input and output types to dynamic by default.
	node.InputType, node.OutputType = model.DynamicType, model.DynamicType

	// Find the resource's schema.
	token, tokenRange := getResourceToken(node)
	pkg, _, _, diagnostics := decomposeToken(token, tokenRange)
	if diagnostics.HasErrors() {
		return diagnostics
	}

	pkgSchema, ok := b.packageSchemas[pkg]
	if !ok {
		return hcl.Diagnostics{unknownPackage(pkg, tokenRange)}
	}

	token = canonicalizeToken(token, pkgSchema.schema)
	res, ok := pkgSchema.resources[token]
	if !ok {
		return hcl.Diagnostics{unknownResourceType(token, tokenRange)}
	}
	node.Token = token

	// Create input and output types for the schema.
	inputType := model.InputType(schemaTypeToType(&schema.ObjectType{Properties: res.InputProperties}))

	outputProperties := map[string]model.Type{
		"id":  model.NewOutputType(model.StringType),
		"urn": model.NewOutputType(model.StringType),
	}
	for _, prop := range res.Properties {
		outputProperties[prop.Name] = model.NewOutputType(schemaTypeToType(prop.Type))
	}
	outputType := model.NewObjectType(outputProperties)

	node.InputType, node.OutputType = inputType, outputType
	return diagnostics
}

// bindResourceBody binds the body of a resource.
func (b *binder) bindResourceBody(node *Resource) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	// Bind the resource's body.
	block, blockDiags := model.BindBlock(node.Syntax, model.StaticScope(b.root), b.tokens)
	diagnostics = append(diagnostics, blockDiags...)

	for _, item := range block.Body.Items {
		switch item := item.(type) {
		case *model.Attribute:
			node.Inputs = append(node.Inputs, item)
		case *model.Block:
			switch item.Type {
			case "options":
				node.Options = item
			default:
				diagnostics = append(diagnostics, unsupportedBlock(item.Type, item.Syntax.TypeRange))
			}
		}
	}

	// Typecheck the attributes.
	if objectType, ok := node.InputType.(*model.ObjectType); ok {
		attrNames := codegen.StringSet{}
		for _, attr := range node.Inputs {
			attrNames.Add(attr.Name)

			if typ, ok := objectType.Properties[attr.Name]; ok {
				if !typ.ConversionFrom(attr.Value.Type()).Exists() {
					diagnostics = append(diagnostics, model.ExprNotConvertible(typ, attr.Value))
				}
			} else {
				diagnostics = append(diagnostics, unsupportedAttribute(attr.Name, attr.Syntax.NameRange))
			}
		}

		for _, k := range codegen.SortedKeys(objectType.Properties) {
			if !model.IsOptionalType(objectType.Properties[k]) && !attrNames.Has(k) {
				diagnostics = append(diagnostics, missingRequiredAttribute(k, node.Body.Syntax.MissingItemRange()))
			}
		}
	}

	// TODO(pdg): typecheck the options block

	return diagnostics
}
