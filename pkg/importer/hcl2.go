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

package importer

import (
	"fmt"
	"math"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/zclconf/go-cty/cty"
)

// Null represents Pulumi HCL2's `null` variable.
var Null = &model.Variable{
	Name:         "null",
	VariableType: model.NoneType,
}

// GenerateHCL2Definition generates a Pulumi HCL2 definition for a given resource.
func GenerateHCL2Definition(loader schema.Loader, state *resource.State, names NameTable) (*model.Block, error) {
	// TODO: pull the package version from the resource's provider
	pkg, err := schema.LoadPackageReference(loader, string(state.Type.Package()), nil)
	if err != nil {
		return nil, err
	}

	r, ok, err := pkg.Resources().Get(string(state.Type))
	if err != nil {
		return nil, fmt.Errorf("loading resource '%v': %w", state.Type, err)
	}
	if !ok {
		return nil, fmt.Errorf("unknown resource type '%v'", r)
	}

	var items []model.BodyItem
	for _, p := range r.InputProperties {
		x, err := generatePropertyValue(p, state.Inputs[resource.PropertyKey(p.Name)])
		if err != nil {
			return nil, err
		}
		if x != nil {
			items = append(items, &model.Attribute{
				Name:  p.Name,
				Value: x,
			})
		}
	}

	resourceOptions, err := makeResourceOptions(state, names)
	if err != nil {
		return nil, err
	}
	if resourceOptions != nil {
		items = append(items, resourceOptions)
	}

	typ, name := state.URN.Type(), state.URN.Name()
	return &model.Block{
		Tokens: syntax.NewBlockTokens("resource", name, string(typ)),
		Type:   "resource",
		Labels: []string{name, string(typ)},
		Body: &model.Body{
			Items: items,
		},
	}, nil
}

func newVariableReference(name string) model.Expression {
	return model.VariableReference(&model.Variable{
		Name:         name,
		VariableType: model.DynamicType,
	})
}

func appendResourceOption(block *model.Block, name string, value model.Expression) *model.Block {
	if block == nil {
		block = &model.Block{
			Tokens: syntax.NewBlockTokens("options"),
			Type:   "options",
			Body:   &model.Body{},
		}
	}
	block.Body.Items = append(block.Body.Items, &model.Attribute{
		Tokens: syntax.NewAttributeTokens(name),
		Name:   name,
		Value:  value,
	})
	return block
}

func makeResourceOptions(state *resource.State, names NameTable) (*model.Block, error) {
	var resourceOptions *model.Block
	if state.Parent != "" && state.Parent.Type() != resource.RootStackType {
		name, ok := names[state.Parent]
		if !ok {
			return nil, fmt.Errorf("no name for parent %v", state.Parent)
		}
		resourceOptions = appendResourceOption(resourceOptions, "parent", newVariableReference(name))
	}
	if state.Provider != "" {
		ref, err := providers.ParseReference(state.Provider)
		if err != nil {
			return nil, fmt.Errorf("invalid provider reference %v: %w", state.Provider, err)
		}
		if !providers.IsDefaultProvider(ref.URN()) {
			name, ok := names[ref.URN()]
			if !ok {
				return nil, fmt.Errorf("no name for provider %v", state.Provider)
			}
			resourceOptions = appendResourceOption(resourceOptions, "provider", newVariableReference(name))
		}
	}
	if len(state.Dependencies) != 0 {
		deps := make([]model.Expression, len(state.Dependencies))
		for i, d := range state.Dependencies {
			name, ok := names[d]
			if !ok {
				return nil, fmt.Errorf("no name for resource %v", d)
			}
			deps[i] = newVariableReference(name)
		}
		resourceOptions = appendResourceOption(resourceOptions, "dependsOn", &model.TupleConsExpression{
			Tokens:      syntax.NewTupleConsTokens(len(deps)),
			Expressions: deps,
		})
	}
	if state.Protect {
		resourceOptions = appendResourceOption(resourceOptions, "protect", &model.LiteralValueExpression{
			Tokens: syntax.NewLiteralValueTokens(cty.True),
			Value:  cty.True,
		})
	}
	return resourceOptions, nil
}

// typeRank orders types by their simplicity.
func typeRank(t schema.Type) int {
	switch t {
	case schema.BoolType:
		return 1
	case schema.IntType:
		return 2
	case schema.NumberType:
		return 3
	case schema.StringType:
		return 4
	case schema.AssetType:
		return 5
	case schema.ArchiveType:
		return 6
	case schema.JSONType:
		return 7
	case schema.AnyType:
		return 13
	default:
		switch t := t.(type) {
		case *schema.TokenType:
			return 8
		case *schema.ArrayType:
			return 9
		case *schema.MapType:
			return 10
		case *schema.ObjectType:
			return 11
		case *schema.UnionType:
			return 12
		case *schema.InputType:
			return typeRank(t.ElementType)
		case *schema.OptionalType:
			return typeRank(t.ElementType)
		default:
			return int(math.MaxInt32)
		}
	}
}

// simplerType returns true if T is simpler than U.
//
// The first-order ranking is:
//
//	bool < int < number < string < archive < asset < json < token < array < map < object < union < any
//
// Additional rules apply to composite types of the same kind:
//   - array(T) is simpler than array(U) if T is simpler than U
//   - map(T) is simpler than map(U) if T is simpler than U
//   - object({ ... }) is simpler than object({ ... }) if the former has a greater number of required properties that
//     are simpler than the latter's required properties
//   - union(...) is simpler than union(...) if the former's simplest element type is simpler than the latter's simplest
//     element type
func simplerType(t, u schema.Type) bool {
	tRank, uRank := typeRank(t), typeRank(u)
	if tRank < uRank {
		return true
	} else if tRank > uRank {
		return false
	}

	t, u = codegen.UnwrapType(t), codegen.UnwrapType(u)

	// At this point we know that t and u have the same concrete type.
	switch t := t.(type) {
	case *schema.TokenType:
		u := u.(*schema.TokenType)
		if t.UnderlyingType != nil && u.UnderlyingType != nil {
			return simplerType(t.UnderlyingType, u.UnderlyingType)
		}
		return false
	case *schema.ArrayType:
		return simplerType(t.ElementType, u.(*schema.ArrayType).ElementType)
	case *schema.MapType:
		return simplerType(t.ElementType, u.(*schema.MapType).ElementType)
	case *schema.ObjectType:
		// Count how many of T's required properties are simpler than U's required properties and vice versa.
		uu := u.(*schema.ObjectType)
		tscore, nt, uscore := 0, 0, 0
		for _, p := range t.Properties {
			if p.IsRequired() {
				nt++
				for _, q := range uu.Properties {
					if q.IsRequired() {
						if simplerType(p.Type, q.Type) {
							tscore++
						}
						if simplerType(q.Type, p.Type) {
							uscore++
						}
					}
				}
			}
		}

		// If the number of T's required properties that are simpler that U's required properties exceeds the number
		// of U's required properties that are simpler than T's required properties, T is simpler.
		if tscore > uscore {
			return true
		}
		if tscore < uscore {
			return false
		}

		// If the above counts are equal, T is simpler if it has fewer required properties.
		nu := 0
		for _, q := range uu.Properties {
			if q.IsRequired() {
				nu++
			}
		}

		return nt < nu
	case *schema.UnionType:
		// Pick whichever has the simplest element type.
		var simplestElementType schema.Type
		for _, u := range u.(*schema.UnionType).ElementTypes {
			if simplestElementType == nil || simplerType(u, simplestElementType) {
				simplestElementType = u
			}
		}
		for _, t := range t.ElementTypes {
			if simplestElementType == nil || simplerType(t, simplestElementType) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// zeroValue constructs a zero value of the given type.
func zeroValue(t schema.Type) model.Expression {
	switch t := t.(type) {
	case *schema.OptionalType:
		return model.VariableReference(Null)
	case *schema.InputType:
		return zeroValue(t.ElementType)
	case *schema.MapType:
		return &model.ObjectConsExpression{}
	case *schema.ArrayType:
		return &model.TupleConsExpression{}
	case *schema.UnionType:
		// If there is a default type, create a value of that type.
		if t.DefaultType != nil {
			return zeroValue(t.DefaultType)
		}
		// Otherwise, pick the simplest type in the list.
		var simplestType schema.Type
		for _, t := range t.ElementTypes {
			if simplestType == nil || simplerType(t, simplestType) {
				simplestType = t
			}
		}
		return zeroValue(simplestType)
	case *schema.ObjectType:
		var items []model.ObjectConsItem
		for _, p := range t.Properties {
			if p.IsRequired() {
				items = append(items, model.ObjectConsItem{
					Key: &model.LiteralValueExpression{
						Value: cty.StringVal(p.Name),
					},
					Value: zeroValue(p.Type),
				})
			}
		}
		return &model.ObjectConsExpression{Items: items}
	case *schema.TokenType:
		if t.UnderlyingType != nil {
			return zeroValue(t.UnderlyingType)
		}
		return model.VariableReference(Null)
	}
	switch t {
	case schema.BoolType:
		x, err := generateValue(t, resource.NewBoolProperty(false))
		contract.IgnoreError(err)
		return x
	case schema.IntType, schema.NumberType:
		x, err := generateValue(t, resource.NewNumberProperty(0))
		contract.IgnoreError(err)
		return x
	case schema.StringType:
		x, err := generateValue(t, resource.NewStringProperty(""))
		contract.IgnoreError(err)
		return x
	case schema.ArchiveType, schema.AssetType:
		return model.VariableReference(Null)
	case schema.JSONType, schema.AnyType:
		return &model.ObjectConsExpression{}
	default:
		contract.Failf("unexpected schema type %v", t)
		return nil
	}
}

// generatePropertyValue generates the value for the given property. If the value is absent and the property is
// required, a zero value for the property's type is generated. If the value is absent and the property is not
// required, no value is generated (i.e. this function returns nil).
func generatePropertyValue(property *schema.Property, value resource.PropertyValue) (model.Expression, error) {
	if !value.HasValue() {
		if !property.IsRequired() {
			return nil, nil
		}
		return zeroValue(property.Type), nil
	}

	return generateValue(property.Type, value)
}

// valueStructurallyTypedAs returns true if the given value is structurally typed as the given schema type.
func valueStructurallyTypedAs(value resource.PropertyValue, schemaType schema.Type) bool {
	if union, ok := schemaType.(*schema.UnionType); ok {
		schemaType = reduceUnionType(union, value)
	}

	switch {
	case value.IsObject():
		switch arg := schemaType.(type) {
		case *schema.ObjectType:
			schemaProperties := make(map[string]schema.Type)
			for _, schemaProperty := range arg.Properties {
				schemaProperties[schemaProperty.Name] = schemaProperty.Type
			}

			objectProperties := value.ObjectValue()
			// check that each property is present in the schema and that the value is structurally typed as well
			for propertyKey, propertyValue := range objectProperties {
				propertyValueSchema, ok := arg.Property(string(propertyKey))
				if !ok {
					// unknown property
					return false
				}

				if !valueStructurallyTypedAs(propertyValue, propertyValueSchema.Type) {
					return false
				}
			}

			// check that all required properties from the schema are present in the object properties
			for _, schemaProperty := range arg.Properties {
				if schemaProperty.IsRequired() {
					if _, ok := objectProperties[resource.PropertyKey(schemaProperty.Name)]; !ok {
						// the required property was not present in the object
						return false
					}
				}
			}

			// all properties are present and structurally typed
			return true

		case *schema.UnionType:
			// make sure that at least of the union element types is structurally typed
			for _, unionElement := range arg.ElementTypes {
				if valueStructurallyTypedAs(value, unionElement) {
					return true
				}
			}
		}

	case value.IsString():
		// basic case
		if schemaType == schema.StringType {
			return true
		}

		// for unions: check that at least of one of the element types is also a string
		// collapsing unions of unions as necessary recursively
		if union, ok := schemaType.(*schema.UnionType); ok {
			for _, elementType := range union.ElementTypes {
				if valueStructurallyTypedAs(value, elementType) {
					return true
				}
			}
		}

	case value.IsBool():
		// basic case
		if schemaType == schema.BoolType {
			return true
		}

		// for unions: check that at least of one of the element types is also a bool
		// collapsing unions of unions as necessary recursively
		if union, ok := schemaType.(*schema.UnionType); ok {
			for _, elementType := range union.ElementTypes {
				if valueStructurallyTypedAs(value, elementType) {
					return true
				}
			}
		}

	case value.IsNumber():
		// basic case
		if schemaType == schema.NumberType || schemaType == schema.IntType {
			return true
		}

		// for unions: check that at least of one of the element types is also a number
		// collapsing unions of unions as necessary recursively
		if union, ok := schemaType.(*schema.UnionType); ok {
			for _, elementType := range union.ElementTypes {
				if valueStructurallyTypedAs(value, elementType) {
					return true
				}
			}
		}

	case value.IsArray():
		// basic case: check that each element in the array is structurally typed as the element type of the schenma array
		switch arg := schemaType.(type) {
		case *schema.ArrayType:
			for _, element := range value.ArrayValue() {
				if !valueStructurallyTypedAs(element, arg.ElementType) {
					return false
				}
			}

			// all elements are structurally typed
			return true
		case *schema.UnionType:
			// make sure that at least of the union element types is structurally typed
			for _, unionElement := range arg.ElementTypes {
				if valueStructurallyTypedAs(value, unionElement) {
					return true
				}
			}
		}
	}

	return false
}

// reduceUnionType reduces the given union type to a simpler type that potentially matches the value.
// When the value type is primitive, choose the first element type of the union elements that is of the same type.
// When the value is an object, use the discriminator to choose the element type.
func reduceUnionType(schemaUnion *schema.UnionType, value resource.PropertyValue) schema.Type {
	switch {
	case value.IsObject():
		// return the first element type that matches structurally fits the value
		findBestFitType := func() schema.Type {
			for _, t := range schemaUnion.ElementTypes {
				if valueStructurallyTypedAs(value, t) {
					return t
				}
			}

			// if we still couldn't find a type that fits the value
			return nil
		}

		// If the value is an object, use the discriminator to choose the element type.
		if schemaUnion.Discriminator == "" {
			return findBestFitType()
		}

		obj := value.ObjectValue()
		discriminatorValue, ok := obj[resource.PropertyKey(schemaUnion.Discriminator)]
		if !ok {
			// discriminator property is not present
			// return the first type that fits the value
			return findBestFitType()
		}

		if !discriminatorValue.IsString() {
			// discriminator property value is not a string,
			// so we can't select a type from the union mapping
			return findBestFitType()
		}

		correspondingTypeToken, ok := schemaUnion.Mapping[discriminatorValue.StringValue()]
		if !ok {
			// discriminator property value is not a key in the union mapping,
			return findBestFitType()
		}

		for _, elementType := range schemaUnion.ElementTypes {
			// found the type token
			// match it against the element type which should be an object
			elementTypeObject, ok := codegen.UnwrapType(elementType).(*schema.ObjectType)
			if ok {
				elementTypeToken, parseError := tokens.ParseTypeToken(elementTypeObject.Token)
				if parseError != nil {
					continue
				}

				foundTypeToken, parseError := tokens.ParseTypeToken(correspondingTypeToken)

				if parseError != nil {
					continue
				}

				typeName := string(elementTypeToken.Name())
				foundTypeName := string(foundTypeToken.Name())
				if typeName == foundTypeName {
					return elementTypeObject
				}
			}
		}

	default:
		for _, t := range schemaUnion.ElementTypes {
			if unionType, ok := t.(*schema.UnionType); ok {
				t = reduceUnionType(unionType, value)
			}

			if valueStructurallyTypedAs(value, t) {
				return t
			}
		}
	}

	// anything else, we don't know
	return nil
}

// generateValue generates a value from the given property value. The given type may or may not match the shape of the
// given value.
func generateValue(typ schema.Type, value resource.PropertyValue) (model.Expression, error) {
	typ = codegen.UnwrapType(typ)

	if unionType, ok := typ.(*schema.UnionType); ok {
		typ = reduceUnionType(unionType, value)
	}

	switch {
	case value.IsArchive():
		return nil, fmt.Errorf("NYI: archives")
	case value.IsArray():
		elementType := schema.AnyType
		if typ, ok := typ.(*schema.ArrayType); ok {
			elementType = typ.ElementType
		}

		arr := value.ArrayValue()
		exprs := make([]model.Expression, len(arr))
		for i, v := range arr {
			x, err := generateValue(elementType, v)
			if err != nil {
				return nil, err
			}
			exprs[i] = x
		}
		return &model.TupleConsExpression{
			Tokens:      syntax.NewTupleConsTokens(len(exprs)),
			Expressions: exprs,
		}, nil
	case value.IsAsset():
		return nil, fmt.Errorf("NYI: assets")
	case value.IsBool():
		return &model.LiteralValueExpression{
			Value: cty.BoolVal(value.BoolValue()),
		}, nil
	case value.IsComputed() || value.IsOutput():
		return nil, fmt.Errorf("cannot define computed values")
	case value.IsNull():
		return model.VariableReference(Null), nil
	case value.IsNumber():
		return &model.LiteralValueExpression{
			Value: cty.NumberFloatVal(value.NumberValue()),
		}, nil
	case value.IsObject():
		obj := value.ObjectValue()
		items := slice.Prealloc[model.ObjectConsItem](len(obj))

		switch arg := typ.(type) {
		case *schema.ObjectType:
			for _, p := range arg.Properties {
				x, err := generatePropertyValue(p, obj[resource.PropertyKey(p.Name)])
				if err != nil {
					return nil, err
				}
				if x != nil {
					items = append(items, model.ObjectConsItem{
						Key: &model.LiteralValueExpression{
							Value: cty.StringVal(p.Name),
						},
						Value: x,
					})
				}
			}

		default:
			elementType := schema.AnyType
			if mapType, ok := typ.(*schema.MapType); ok {
				elementType = mapType.ElementType
			}

			for _, k := range obj.StableKeys() {
				// Ignore internal properties.
				if strings.HasPrefix(string(k), "__") {
					continue
				}

				x, err := generateValue(elementType, obj[k])
				if err != nil {
					return nil, err
				}

				// Always quote the key in case it includes invalid identifier characters (like '/' or ':')
				propKey := fmt.Sprintf("%q", string(k))

				items = append(items, model.ObjectConsItem{
					Key: &model.LiteralValueExpression{
						Value: cty.StringVal(propKey),
					},
					Value: x,
				})
			}
		}
		return &model.ObjectConsExpression{
			Tokens: syntax.NewObjectConsTokens(len(items)),
			Items:  items,
		}, nil
	case value.IsSecret():
		arg, err := generateValue(typ, value.SecretValue().Element)
		if err != nil {
			return nil, err
		}
		return &model.FunctionCallExpression{
			Name: "secret",
			Signature: model.StaticFunctionSignature{
				Parameters: []model.Parameter{{
					Name: "value",
					Type: arg.Type(),
				}},
				ReturnType: model.NewOutputType(arg.Type()),
			},
			Args: []model.Expression{arg},
		}, nil
	case value.IsString():
		x := &model.TemplateExpression{
			Parts: []model.Expression{
				&model.LiteralValueExpression{
					Value: cty.StringVal(value.StringValue()),
				},
			},
		}
		switch typ {
		case schema.ArchiveType:
			return &model.FunctionCallExpression{
				Name: "fileArchive",
				Args: []model.Expression{x},
			}, nil
		case schema.AssetType:
			return &model.FunctionCallExpression{
				Name: "fileAsset",
				Args: []model.Expression{x},
			}, nil
		default:
			return x, nil
		}
	default:
		contract.Failf("unexpected property value %v", value)
		return nil, nil
	}
}
