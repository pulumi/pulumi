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
	"context"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func (b *binder) bindReadResource(ctx context.Context, node *ReadResource) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	typeDiags := b.bindReadResourceTypes(ctx, node)
	diagnostics = append(diagnostics, typeDiags...)

	bodyDiags := b.bindReadResourceBody(node)
	diagnostics = append(diagnostics, bodyDiags...)

	return diagnostics
}

func (b *binder) bindReadResourceTypes(ctx context.Context, node *ReadResource) hcl.Diagnostics {
	node.InputType, node.OutputType = model.DynamicType, model.DynamicType

	token, tokenRange := node.GetToken()

	makeResourceDynamic := func() {
		node.token = token
		node.OutputType = model.DynamicType
		inferredInputProperties := map[string]model.Type{}
		for _, attr := range node.Inputs {
			inferredInputProperties[attr.Name] = attr.Type()
		}
		node.InputType = model.NewObjectType(inferredInputProperties)
	}

	res, resolvedToken, diagnostics := b.resolveSchemaResourceForBind(
		ctx, token, tokenRange, false, false, makeResourceDynamic)
	if diagnostics.HasErrors() || res == nil {
		return diagnostics
	}

	node.Schema = res
	node.token = resolvedToken

	var stateInputs []*schema.Property
	if res.StateInputs != nil {
		stateInputs = res.StateInputs.Properties
	}
	stateInputs = b.resolveBaseResourceInputUnionTypes(node, stateInputs)
	inputProperties := append([]*schema.Property{{
		Name: "id",
		Type: schema.StringType,
	}}, stateInputs...)
	node.InputType, node.OutputType, _ = b.computeBaseResourceInputOutputTypes(node, inputProperties, res.Properties)

	return diagnostics
}

func (b *binder) bindReadResourceBody(node *ReadResource) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	node.LenientTraversal = b.options.skipResourceTypecheck
	var rangeKey, rangeValue model.Type
	node.VariableType, rangeKey, rangeValue, diagnostics = b.computeBaseResourceVariableTypeFromRange(
		node, node.OutputType,
	)

	block, inputs, options, logicalName, bindDiags := b.bindAndCollectBaseResourceBlock(node, rangeKey, rangeValue)
	diagnostics = append(diagnostics, bindDiags...)
	node.Inputs = inputs
	if logicalName != "" {
		node.logicalName = logicalName
	}

	resourceProperties := make(map[string]schema.Type)
	resourceProperties["id"] = schema.StringType
	if node.Schema != nil && node.Schema.StateInputs != nil {
		for _, property := range node.Schema.StateInputs.Properties {
			resourceProperties[property.Name] = property.Type
		}
	}

	diagnostics = append(diagnostics,
		b.typecheckBaseResourceAttributes(node.InputType, node.Inputs, resourceProperties, node.token, block)...)

	if options != nil {
		readOptions, optionsDiags := bindResourceOptions(options)
		diagnostics = append(diagnostics, optionsDiags...)
		node.Options = readOptions
	}

	node.Definition = block
	return diagnostics
}
