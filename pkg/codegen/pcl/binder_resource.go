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
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

func getResourceToken(node *Resource) (string, hcl.Range) {
	return node.syntax.Labels[1], node.syntax.LabelRanges[1]
}

func (b *binder) bindResource(node *Resource) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	typeDiags := b.bindResourceTypes(node)
	diagnostics = append(diagnostics, typeDiags...)

	bodyDiags := b.bindResourceBody(node)
	diagnostics = append(diagnostics, bodyDiags...)

	return diagnostics
}

func annotateAttributeValue(expr model.Expression, attributeType schema.Type) model.Expression {
	if optionalType, ok := attributeType.(*schema.OptionalType); ok {
		return annotateAttributeValue(expr, optionalType.ElementType)
	}

	switch attrValue := expr.(type) {
	case *model.ObjectConsExpression:
		if schemaObjectType, ok := attributeType.(*schema.ObjectType); ok {
			schemaProperties := make(map[string]schema.Type)
			for _, schemaProperty := range schemaObjectType.Properties {
				schemaProperties[schemaProperty.Name] = schemaProperty.Type
			}

			for _, item := range attrValue.Items {
				keyLiteral, isLit := item.Key.(*model.LiteralValueExpression)
				if isLit {
					correspondingSchemaType, ok := schemaProperties[keyLiteral.Value.AsString()]
					if ok {
						item.Value = annotateAttributeValue(item.Value, correspondingSchemaType)
					}
				}
			}
			return attrValue.WithType(func(attrValueType model.Type) *model.ObjectConsExpression {
				annotateObjectProperties(attrValueType, attributeType)
				return attrValue
			})
		}

		return attrValue

	case *model.TupleConsExpression:
		if schemaArrayType, ok := attributeType.(*schema.ArrayType); ok {
			elementType := schemaArrayType.ElementType
			for _, arrayExpr := range attrValue.Expressions {
				annotateAttributeValue(arrayExpr, elementType)
			}
		}

		return attrValue
	case *model.FunctionCallExpression:
		if attrValue.Name == IntrinsicConvert {
			converterArg := attrValue.Args[0]
			annotateAttributeValue(converterArg, attributeType)
		}

		return attrValue
	default:
		return expr
	}
}

func AnnotateAttributeValue(expr model.Expression, attributeType schema.Type) model.Expression {
	return annotateAttributeValue(expr, attributeType)
}

func AnnotateResourceInputs(node *Resource) {
	if node.Schema == nil {
		// skip annotations for resource which don't have a schema
		return
	}
	resourceProperties := make(map[string]*schema.Property)
	for _, property := range node.Schema.Properties {
		resourceProperties[property.Name] = property
	}

	// add type annotations to the attributes
	// and their nested objects
	for index := range node.Inputs {
		attr := node.Inputs[index]
		if property, ok := resourceProperties[attr.Name]; ok {
			node.Inputs[index] = &model.Attribute{
				Tokens: attr.Tokens,
				Name:   attr.Name,
				Syntax: attr.Syntax,
				Value:  AnnotateAttributeValue(attr.Value, property.Type),
			}
		}
	}
}

// bindResourceTypes binds the input and output types for a resource.
func (b *binder) bindResourceTypes(node *Resource) hcl.Diagnostics {
	// Set the input and output types to dynamic by default.
	node.InputType, node.OutputType = model.DynamicType, model.DynamicType

	// Find the resource's schema.
	token, tokenRange := getResourceToken(node)
	pkg, module, name, diagnostics := DecomposeToken(token, tokenRange)
	if diagnostics.HasErrors() {
		return diagnostics
	}

	makeResourceDynamic := func() {
		// make the inputs and outputs of the resource dynamic
		node.Token = token
		node.OutputType = model.DynamicType
		inferredInputProperties := map[string]model.Type{}
		for _, attr := range node.Inputs {
			inferredInputProperties[attr.Name] = attr.Type()
		}
		node.InputType = model.NewObjectType(inferredInputProperties)
	}

	isProvider := false
	if pkg == "pulumi" && module == "providers" {
		pkg, isProvider = name, true
	}
	var pkgSchema *packageSchema

	// It is important that we call `loadPackageSchema` instead of `getPackageSchema` here
	// because the the version may be wrong. When the version should not be empty,
	// `loadPackageSchema` will load the default version while `getPackageSchema` will
	// simply fail. We can't give a populated version field since we have not processed
	// the body, and thus the version yet.
	pkgSchema, err := b.options.packageCache.loadPackageSchema(b.options.loader, pkg, "")
	if err != nil {
		e := unknownPackage(pkg, tokenRange)
		e.Detail = err.Error()

		if b.options.skipResourceTypecheck {
			makeResourceDynamic()
			return hcl.Diagnostics{asWarningDiagnostic(e)}
		}

		return hcl.Diagnostics{e}
	}

	var res *schema.Resource
	var inputProperties, properties []*schema.Property
	if isProvider {
		r, err := pkgSchema.schema.Provider()
		if err != nil {
			if b.options.skipResourceTypecheck {
				makeResourceDynamic()
				return diagnostics
			}
			return hcl.Diagnostics{resourceLoadError(token, err, tokenRange)}
		}
		res = r
	} else {
		r, tk, ok, err := pkgSchema.LookupResource(token)
		if err != nil {
			if b.options.skipResourceTypecheck {
				makeResourceDynamic()
				return diagnostics
			}

			return hcl.Diagnostics{resourceLoadError(token, err, tokenRange)}
		} else if !ok {
			if b.options.skipResourceTypecheck {
				makeResourceDynamic()
				return diagnostics
			}

			return hcl.Diagnostics{unknownResourceType(token, tokenRange)}
		}
		res = r
		token = tk
	}
	node.Schema = res
	inputProperties, properties = res.InputProperties, res.Properties
	node.Token = token

	// Create input and output types for the schema.
	inputType := b.schemaTypeToType(&schema.ObjectType{Properties: inputProperties})

	outputProperties := map[string]model.Type{
		"id":  model.NewOutputType(model.StringType),
		"urn": model.NewOutputType(model.StringType),
	}
	for _, prop := range properties {
		outputProperties[prop.Name] = model.NewOutputType(b.schemaTypeToType(prop.Type))
	}
	outputType := model.NewObjectType(outputProperties, &schema.ObjectType{Properties: properties})

	node.InputType, node.OutputType = inputType, outputType

	return diagnostics
}

type resourceScopes struct {
	root      *model.Scope
	withRange *model.Scope
	resource  *Resource
}

func newResourceScopes(root *model.Scope, resource *Resource, rangeKey, rangeValue model.Type) model.Scopes {
	scopes := &resourceScopes{
		root:      root,
		withRange: root,
		resource:  resource,
	}
	if rangeValue != nil {
		properties := map[string]model.Type{
			"value": rangeValue,
		}
		if rangeKey != nil {
			properties["key"] = rangeKey
		}

		scopes.withRange = root.Push(syntax.None)
		scopes.withRange.Define("range", &model.Variable{
			Name:         "range",
			VariableType: model.NewObjectType(properties),
		})
	}
	return scopes
}

func (s *resourceScopes) GetScopesForBlock(block *hclsyntax.Block) (model.Scopes, hcl.Diagnostics) {
	if block.Type == "options" {
		return &optionsScopes{root: s.root, resource: s.resource}, nil
	}
	return model.StaticScope(s.withRange), nil
}

func (s *resourceScopes) GetScopeForAttribute(attr *hclsyntax.Attribute) (*model.Scope, hcl.Diagnostics) {
	return s.withRange, nil
}

type optionsScopes struct {
	root     *model.Scope
	resource *Resource
}

func (s *optionsScopes) GetScopesForBlock(block *hclsyntax.Block) (model.Scopes, hcl.Diagnostics) {
	return model.StaticScope(s.root), nil
}

func (s *optionsScopes) GetScopeForAttribute(attr *hclsyntax.Attribute) (*model.Scope, hcl.Diagnostics) {
	if attr.Name == "ignoreChanges" {
		obj, ok := model.ResolveOutputs(s.resource.InputType).(*model.ObjectType)
		if !ok {
			return nil, nil
		}
		scope := model.NewRootScope(syntax.None)
		for k, t := range obj.Properties {
			scope.Define(k, &ResourceProperty{
				Path:         hcl.Traversal{hcl.TraverseRoot{Name: k}},
				PropertyType: t,
			})
		}
		return scope, nil
	}
	return s.root, nil
}

func bindResourceOptions(options *model.Block) (*ResourceOptions, hcl.Diagnostics) {
	resourceOptions := &ResourceOptions{}
	var diagnostics hcl.Diagnostics
	for _, item := range options.Body.Items {
		switch item := item.(type) {
		case *model.Attribute:
			var t model.Type
			switch item.Name {
			case "range":
				t = model.NewUnionType(model.BoolType, model.NumberType, model.NewListType(model.DynamicType),
					model.NewMapType(model.DynamicType))
				resourceOptions.Range = item.Value
			case "parent":
				t = model.DynamicType
				resourceOptions.Parent = item.Value
			case "provider":
				t = model.DynamicType
				resourceOptions.Provider = item.Value
			case "dependsOn":
				t = model.NewListType(model.DynamicType)
				resourceOptions.DependsOn = item.Value
			case "protect":
				t = model.BoolType
				resourceOptions.Protect = item.Value
			case "retainOnDelete":
				t = model.BoolType
				resourceOptions.RetainOnDelete = item.Value
			case "ignoreChanges":
				t = model.NewListType(ResourcePropertyType)
				resourceOptions.IgnoreChanges = item.Value
			case "version":
				t = model.StringType
				resourceOptions.Version = item.Value
			case "pluginDownloadURL":
				t = model.StringType
				resourceOptions.PluginDownloadURL = item.Value
			default:
				diagnostics = append(diagnostics, unsupportedAttribute(item.Name, item.Syntax.NameRange))
				continue
			}
			if model.InputType(t).ConversionFrom(item.Value.Type()) == model.NoConversion {
				diagnostics = append(diagnostics, model.ExprNotConvertible(model.InputType(t), item.Value))
			}
		case *model.Block:
			diagnostics = append(diagnostics, unsupportedBlock(item.Type, item.Syntax.TypeRange))
		}
	}
	return resourceOptions, diagnostics
}

// bindResourceBody binds the body of a resource.
func (b *binder) bindResourceBody(node *Resource) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	// Allow for lenient traversal when we choose to skip resource type-checking.
	node.LenientTraversal = b.options.skipResourceTypecheck
	node.VariableType = node.OutputType
	// If the resource has a range option, we need to know the type of the collection being ranged over. Pre-bind the
	// range expression now, but ignore the diagnostics.
	var rangeKey, rangeValue model.Type
	for _, block := range node.syntax.Body.Blocks {
		if block.Type == "options" {
			if rng, hasRange := block.Body.Attributes["range"]; hasRange {
				expr, _ := model.BindExpression(rng.Expr, b.root, b.tokens, b.options.modelOptions()...)
				typ := model.ResolveOutputs(expr.Type())

				resourceVar := &model.Variable{
					Name:         "r",
					VariableType: node.VariableType,
				}
				switch {
				case model.InputType(model.BoolType).ConversionFrom(typ) == model.SafeConversion:
					condExpr := &model.ConditionalExpression{
						Condition:  expr,
						TrueResult: model.VariableReference(resourceVar),
						FalseResult: model.ConstantReference(&model.Constant{
							Name:          "null",
							ConstantValue: cty.NullVal(cty.DynamicPseudoType),
						}),
					}
					diags := condExpr.Typecheck(false)
					contract.Assertf(len(diags) == 0, "failed to typecheck conditional expression: %v", diags)

					node.VariableType = condExpr.Type()
				case model.InputType(model.NumberType).ConversionFrom(typ) == model.SafeConversion:
					functions := pulumiBuiltins(b.options)
					rangeArgs := []model.Expression{expr}
					rangeSig, _ := functions["range"].GetSignature(rangeArgs)

					rangeExpr := &model.ForExpression{
						ValueVariable: &model.Variable{
							Name:         "_",
							VariableType: model.NumberType,
						},
						Collection: &model.FunctionCallExpression{
							Name:      "range",
							Signature: rangeSig,
							Args:      rangeArgs,
						},
						Value: model.VariableReference(resourceVar),
					}
					diags := rangeExpr.Typecheck(false)
					contract.Assertf(len(diags) == 0, "failed to typecheck range expression: %v", diags)

					rangeValue = model.IntType

					node.VariableType = rangeExpr.Type()
				default:
					strictCollectionType := !b.options.skipRangeTypecheck
					rk, rv, diags := model.GetCollectionTypes(typ, rng.Range(), strictCollectionType)
					rangeKey, rangeValue, diagnostics = rk, rv, append(diagnostics, diags...)

					iterationExpr := &model.ForExpression{
						ValueVariable: &model.Variable{
							Name:         "_",
							VariableType: rangeValue,
						},
						Collection:                   expr,
						Value:                        model.VariableReference(resourceVar),
						StrictCollectionTypechecking: strictCollectionType,
					}
					diags = iterationExpr.Typecheck(false)
					contract.Ignore(diags) // Any relevant diagnostics were reported by GetCollectionTypes.

					node.VariableType = iterationExpr.Type()
				}
			}
		}
	}

	// Bind the resource's body.
	scopes := newResourceScopes(b.root, node, rangeKey, rangeValue)
	block, blockDiags := model.BindBlock(node.syntax, scopes, b.tokens, b.options.modelOptions()...)
	diagnostics = append(diagnostics, blockDiags...)

	var options *model.Block
	for _, item := range block.Body.Items {
		switch item := item.(type) {
		case *model.Attribute:
			if item.Name == LogicalNamePropertyKey {
				logicalName, lDiags := getStringAttrValue(item)
				if lDiags != nil {
					diagnostics = diagnostics.Append(lDiags)
				} else {
					node.logicalName = logicalName
				}
				continue
			}
			node.Inputs = append(node.Inputs, item)
		case *model.Block:
			switch item.Type {
			case "options":
				if options != nil {
					diagnostics = append(diagnostics, duplicateBlock(item.Type, item.Syntax.TypeRange))
				} else {
					options = item
				}
			default:
				diagnostics = append(diagnostics, unsupportedBlock(item.Type, item.Syntax.TypeRange))
			}
		}
	}

	// Typecheck the attributes.
	if objectType, ok := node.InputType.(*model.ObjectType); ok {
		diag := func(d *hcl.Diagnostic) {
			if b.options.skipResourceTypecheck && d.Severity == hcl.DiagError {
				d.Severity = hcl.DiagWarning
			}
			diagnostics = append(diagnostics, d)
		}
		attrNames := codegen.StringSet{}
		for _, attr := range node.Inputs {
			attrNames.Add(attr.Name)

			if typ, ok := objectType.Properties[attr.Name]; ok {
				conversion := typ.ConversionFrom(attr.Value.Type())
				if !conversion.Exists() {
					diag(model.ExprNotConvertible(typ, attr.Value))
				}
			} else {
				diag(unsupportedAttribute(attr.Name, attr.Syntax.NameRange))
			}
		}

		for _, k := range codegen.SortedKeys(objectType.Properties) {
			typ := objectType.Properties[k]
			if model.IsOptionalType(typ) || attrNames.Has(k) {
				// The type is present or optional. No error.
				continue
			}
			if model.IsConstType(objectType.Properties[k]) {
				// The type is const, so the value is implied. No error.
				continue
			}
			diag(missingRequiredAttribute(k, block.Body.Syntax.MissingItemRange()))
		}
	}

	// Typecheck the options block.
	if options != nil {
		resourceOptions, optionsDiags := bindResourceOptions(options)
		diagnostics = append(diagnostics, optionsDiags...)
		node.Options = resourceOptions
	}

	node.Definition = block
	return diagnostics
}
