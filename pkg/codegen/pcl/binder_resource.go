// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

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
				// annotate the nested object properties
				// here when the key is a literal such as { key = <inner value> }
				keyLiteral, isLit := item.Key.(*model.LiteralValueExpression)
				if isLit {
					correspondingSchemaType, ok := schemaProperties[keyLiteral.Value.AsString()]
					if ok {
						item.Value = annotateAttributeValue(item.Value, correspondingSchemaType)
					}
				}

				// here when the key is a quoted literal such as { "key" = <inner value> }
				if templateExpression, ok := item.Key.(*model.TemplateExpression); ok && len(templateExpression.Parts) == 1 {
					if literalValue, ok := templateExpression.Parts[0].(*model.LiteralValueExpression); ok {
						if correspondingSchemaType, ok := schemaProperties[literalValue.Value.AsString()]; ok {
							item.Value = annotateAttributeValue(item.Value, correspondingSchemaType)
						}
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

// resolveUnionOfObjects takes an object expression and its corresponding schema union type,
// then tries to find out which type from the union is the one that matches the object expression.
// We do this based on the discriminator field in the object expression.
func resolveUnionOfObjects(objectExpr *model.ObjectConsExpression, union *schema.UnionType) schema.Type {
	var discriminatorValue string
	for _, item := range objectExpr.Items {
		if key, ok := item.Key.(*model.LiteralValueExpression); ok {
			if key.Value.AsString() == union.Discriminator {
				if value, ok := item.Value.(*model.LiteralValueExpression); ok {
					discriminatorValue = value.Value.AsString()
					break
				}

				if value, ok := item.Value.(*model.TemplateExpression); ok && len(value.Parts) == 1 {
					if literalValue, ok := value.Parts[0].(*model.LiteralValueExpression); ok {
						discriminatorValue = literalValue.Value.AsString()
						break
					}
				}
			}
		}
	}

	if discriminatorValue == "" {
		return union
	}

	if correspondingTypeToken, ok := union.Mapping[discriminatorValue]; ok {
		for _, schemaType := range union.ElementTypes {
			if schemaObjectType, ok := codegen.UnwrapType(schemaType).(*schema.ObjectType); ok {
				parsedTypeToken, err := tokens.ParseTypeToken(correspondingTypeToken)
				if err != nil {
					continue
				}

				parsedObjectToken, err := tokens.ParseTypeToken(schemaObjectType.Token)
				if err != nil {
					continue
				}

				if string(parsedTypeToken.Name()) == string(parsedObjectToken.Name()) {
					// found the corresponding object type
					return schemaObjectType
				}
			}
		}
	}

	return union
}

// resolveInputUnions will take input expressions and their corresponding schema type,
// if the schema type is a union of objects and the input is an object, we use the
// the discriminator field to determine which object type to use and reduce the union into
// just an object type. This way program generators can easily work with the object type
// directly instead of working out which object type they should be using based on the union
func (b *binder) resolveInputUnions(
	inputs map[string]model.Expression,
	inputProperties []*schema.Property,
) []*schema.Property {
	resolvedProperties := make([]*schema.Property, len(inputProperties))
	for i, property := range inputProperties {
		resolvedType := property.Type
		if value, ok := inputs[property.Name]; ok {
			switch schemaType := codegen.UnwrapType(property.Type).(type) {
			case *schema.UnionType:
				if objectExpr, ok := value.(*model.ObjectConsExpression); ok {
					resolvedType = resolveUnionOfObjects(objectExpr, schemaType)
					switch resolvedType := resolvedType.(type) {
					case *schema.ObjectType:
						// found the corresponding object type for this PCL expression, resolve nested unions if any
						nestedInputs := map[string]model.Expression{}
						for _, item := range objectExpr.Items {
							if key, ok := literalExprValue(item.Key); ok && key.Type().Equals(cty.String) {
								nestedInputs[key.AsString()] = item.Value
							}
						}

						b.resolveInputUnions(nestedInputs, resolvedType.Properties)
					}
				}
			}
		}

		resolvedProperties[i] = &schema.Property{
			Type:                 resolvedType,
			Name:                 property.Name,
			DeprecationMessage:   property.DeprecationMessage,
			ConstValue:           property.ConstValue,
			Secret:               property.Secret,
			Plain:                property.Plain,
			Language:             property.Language,
			Comment:              property.Comment,
			DefaultValue:         property.DefaultValue,
			ReplaceOnChanges:     property.ReplaceOnChanges,
			WillReplaceOnChanges: property.WillReplaceOnChanges,
		}
	}

	return resolvedProperties
}

// rawResourceInputs returns the raw inputs for a resource. This is useful when we need to resolve unions of objects
// and reduce them to just an object when possible. The inputs of a resource contain the discriminator field of which
// the value is used to determine which object type to use and thus reduce unions into objects.
func (b *binder) rawResourceInputs(node *Resource) map[string]model.Expression {
	inputs := map[string]model.Expression{}
	scopes := newResourceScopes(b.root, node, nil, nil)
	block, _ := model.BindBlock(node.syntax, scopes, b.tokens, b.options.modelOptions()...)
	for _, item := range block.Body.Items {
		switch item := item.(type) {
		case *model.Attribute:
			inputs[item.Name] = item.Value
		}
	}

	return inputs
}

// reduceInputUnionTypes reduces the input types of a resource which are unions of objects
// into just an object when possible. We use the actual inputs of the resource to determine which type object we should
// use because objects in a union have a discriminator field which is used to determine which object type to use.
func (b *binder) reduceInputUnionTypes(node *Resource, inputProperties []*schema.Property) []*schema.Property {
	inputs := b.rawResourceInputs(node)
	return b.resolveInputUnions(inputs, inputProperties)
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
	var err error
	// It is important that we call `loadPackageSchema`/`loadPackageSchemaFromDescriptor`
	// instead of `getPackageSchema` here  because the version may be wrong. When the version should not be empty,
	// `loadPackageSchema` will load the default version while `getPackageSchema` will
	// simply fail. We can't give a populated version field since we have not processed
	// the body, and thus the version yet.
	if packageDescriptor, ok := b.packageDescriptors[pkg]; ok {
		pkgSchema, err = b.options.packageCache.loadPackageSchemaFromDescriptor(b.options.loader, packageDescriptor)
	} else {
		pkgSchema, err = b.options.packageCache.loadPackageSchema(b.options.loader, pkg, "")
	}

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
	// first reduce property types which are unions of objects into just an object when possible
	inputObjectType := &schema.ObjectType{Properties: b.reduceInputUnionTypes(node, inputProperties)}
	inputType := b.schemaTypeToType(inputObjectType)

	outputProperties := map[string]model.Type{
		"id":  model.NewOutputType(model.StringType),
		"urn": model.NewOutputType(model.StringType),
	}
	for _, prop := range properties {
		outputProperties[prop.Name] = model.NewOutputType(b.schemaTypeToType(prop.Type))
	}
	outputType := model.NewObjectType(
		outputProperties,
		&ResourceAnnotation{node},
		&schema.ObjectType{Properties: properties},
	)

	node.InputType, node.OutputType = inputType, outputType

	findTransitivePackageReferences := func(schemaType schema.Type) {
		if objectType, ok := schemaType.(*schema.ObjectType); ok && objectType.PackageReference != nil {
			ref := objectType.PackageReference
			if _, found := b.referencedPackages[ref.Name()]; !found {
				b.referencedPackages[ref.Name()] = ref
			}
		}
	}

	codegen.VisitTypeClosure(inputProperties, findTransitivePackageReferences)
	codegen.VisitTypeClosure(properties, findTransitivePackageReferences)

	return diagnostics
}

// ResourceAnnotation is a type that can be used to annotate ObjectTypes that represent resources with their
// corresponding Resource node. We define a wrapper type that does not implement any interfaces so as to reduce the
// chance of the annotation being plucked out by an interface-type query by accident.
type ResourceAnnotation struct {
	Node *Resource
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
			case "deletedWith":
				t = model.DynamicType
				resourceOptions.DeletedWith = item.Value
			case "import":
				t = model.StringType
				resourceOptions.ImportID = item.Value
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

// unwrapOptionalType returns T from an optional type that is modelled as Union[T, None] or Union[None, T]
// if the input type is not optional, it returns the input type as is.
func unwrapOptionalType(t model.Type) model.Type {
	if union, ok := t.(*model.UnionType); ok && len(union.ElementTypes) == 2 {
		if union.ElementTypes[0] == model.NoneType {
			return union.ElementTypes[1]
		} else if union.ElementTypes[1] == model.NoneType {
			return union.ElementTypes[0]
		}
	}
	return t
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
				if model.IsOptionalType(typ) {
					// if the range expression type is wrapped as an optional type
					// due to the range expression being a conditional.
					// unwrap it to get the actual type
					typ = unwrapOptionalType(typ)
				}

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

	resourceProperties := make(map[string]schema.Type)
	if node.Schema != nil {
		for _, property := range node.Schema.Properties {
			resourceProperties[property.Name] = property.Type
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
					if propertyType, ok := resourceProperties[attr.Name]; ok {
						attributeRange := attr.Value.SyntaxNode().Range()
						diag(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Subject:  &attributeRange,
							Detail: fmt.Sprintf("Cannot assign value %s to attribute of type %q for resource %q",
								attr.Value.Type().Pretty().String(),
								propertyType.String(),
								node.Token),
						})
					}
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
