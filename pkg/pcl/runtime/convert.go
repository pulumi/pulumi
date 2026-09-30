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

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

type (
	secretMark     struct{}
	dependencyMark struct {
		dependency resource.URN
	}
	poisonMark struct {
		// The name of the resource that caused this value to be poisoned.
		name string
	}
)

var (
	assetType   = cty.Capsule("asset", reflect.TypeFor[asset.Asset]())
	archiveType = cty.Capsule("archive", reflect.TypeFor[archive.Archive]())
)

func unmark[T any](value cty.Value) (cty.Value, *T) {
	unmarked, marks := value.Unmark()
	if marks == nil {
		return value, nil
	}
	var found *T
	for mark := range marks {
		if t, ok := mark.(T); ok {
			found = &t
			continue
		}
		unmarked = unmarked.Mark(mark)
	}
	return unmarked, found
}

func unwrapModelType(typ model.Type) model.Type {
	if model.IsOptionalType(typ) {
		if union, ok := typ.(*model.UnionType); ok {
			for _, elem := range union.ElementTypes {
				if elem != model.NoneType {
					return unwrapModelType(elem)
				}
			}
		}
		return model.DynamicType
	}
	switch t := typ.(type) {
	case *model.PromiseType:
		return unwrapModelType(t.ElementType)
	case *model.OutputType:
		return unwrapModelType(t.ElementType)
	default:
		return typ
	}
}

func modelTypeToCty(typ model.Type) (cty.Type, error) {
	if typ == nil {
		return cty.DynamicPseudoType, nil
	}
	typ = unwrapModelType(typ)
	switch t := typ.(type) {
	case *model.ListType:
		el, err := modelTypeToCty(t.ElementType)
		if err != nil {
			return cty.DynamicPseudoType, err
		}
		return cty.List(el), nil
	case *model.MapType:
		el, err := modelTypeToCty(t.ElementType)
		if err != nil {
			return cty.DynamicPseudoType, err
		}
		return cty.Map(el), nil
	case *model.ObjectType:
		fields := map[string]cty.Type{}
		for key, prop := range t.Properties {
			ctyProp, err := modelTypeToCty(prop)
			if err != nil {
				return cty.DynamicPseudoType, err
			}
			fields[key] = ctyProp
		}
		return cty.Object(fields), nil
	case *model.TupleType:
		elems := make([]cty.Type, 0, len(t.ElementTypes))
		for _, elem := range t.ElementTypes {
			ctyElem, err := modelTypeToCty(elem)
			if err != nil {
				return cty.DynamicPseudoType, err
			}
			elems = append(elems, ctyElem)
		}
		return cty.Tuple(elems), nil
	case *model.UnionType:
		return cty.DynamicPseudoType, nil
	case *model.ConstType:
		return cty.DynamicPseudoType, nil
	}

	switch typ {
	case model.StringType:
		return cty.String, nil
	case model.BoolType:
		return cty.Bool, nil
	case model.IntType:
		return cty.Number, nil
	case model.NumberType:
		return cty.Number, nil
	case model.DynamicType:
		return cty.DynamicPseudoType, nil
	}

	return cty.DynamicPseudoType, fmt.Errorf("unsupported model type %T", typ)
}

func parseConfigValue(raw string, typ model.Type) (cty.Value, hcl.Diagnostics) {
	if typ == nil {
		return cty.StringVal(raw), nil
	}
	ctyType, err := modelTypeToCty(typ)
	if err != nil {
		return cty.NilVal, hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		}}
	}

	if ctyType == cty.String {
		return cty.StringVal(raw), nil
	}
	if ctyType == cty.Bool {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return cty.NilVal, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("invalid boolean %q", raw),
			}}
		}
		return cty.BoolVal(parsed), nil
	}
	if ctyType == cty.Number {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return cty.NilVal, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("invalid number %q", raw),
			}}
		}
		return cty.NumberFloatVal(parsed), nil
	}

	if ctyType == cty.DynamicPseudoType {
		var obj ctyjson.SimpleJSONValue
		err := json.Unmarshal([]byte(raw), &obj)
		if err == nil {
			return obj.Value, nil
		}
		return cty.StringVal(raw), nil
	}

	v, err := ctyjson.Unmarshal([]byte(raw), ctyType)
	if err != nil {
		return cty.NilVal, hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("invalid JSON value %q", raw),
		}}
	}
	return v, nil
}

type poisonError struct {
	name string
}

func (e *poisonError) Error() string {
	return "poisoned value from resource " + e.name
}

func makePoisonValue(name string) cty.Value {
	return cty.DynamicVal.Mark(poisonMark{name: name})
}

func ctyToPropertyValue(value cty.Value) (out property.Value, _ error) {
	// The accessors below panic on a marked value, so move the marks onto the result.
	value, marks := value.Unmark()
	var dependencies []resource.URN
	for mark := range marks {
		switch mark := mark.(type) {
		case poisonMark:
			return property.Value{}, &poisonError{name: mark.name}
		case dependencyMark:
			dependencies = append(dependencies, mark.dependency)
		}
	}
	_, secret := marks[secretMark{}]
	defer func() { out = out.WithSecret(secret).WithDependencies(dependencies) }()

	if !value.IsKnown() {
		return property.New(property.Computed), nil
	}
	if value.IsNull() {
		return property.New(property.Null), nil
	}

	if value.Type().Equals(assetType) {
		assetValue, ok := value.EncapsulatedValue().(*asset.Asset)
		if !ok {
			return property.Value{}, errors.New("unexpected non-asset capsule value")
		}
		return property.New(assetValue), nil
	}

	if value.Type().Equals(archiveType) {
		archiveValue, ok := value.EncapsulatedValue().(*archive.Archive)
		if !ok {
			return property.Value{}, errors.New("unexpected non-archive capsule value")
		}
		return property.New(archiveValue), nil
	}

	switch value.Type() {
	case cty.String:
		return property.New(value.AsString()), nil
	case cty.Bool:
		return property.New(value.True()), nil
	case cty.Number:
		f, _ := value.AsBigFloat().Float64()
		return property.New(f), nil
	}

	switch {
	case value.Type().IsListType() || value.Type().IsTupleType():
		elements := make([]property.Value, 0, value.LengthInt())
		it := value.ElementIterator()
		for it.Next() {
			_, v := it.Element()
			pv, err := ctyToPropertyValue(v)
			if err != nil {
				return property.Value{}, err
			}
			elements = append(elements, pv)
		}
		return property.New(elements), nil
	case value.Type().IsMapType() || value.Type().IsObjectType():
		result := make(map[string]property.Value, value.LengthInt())
		it := value.ElementIterator()
		for it.Next() {
			k, v := it.Element()
			pv, err := ctyToPropertyValue(v)
			if err != nil {
				return property.Value{}, err
			}
			result[k.AsString()] = pv
		}
		return property.New(result), nil
	}

	return property.Value{}, fmt.Errorf("unsupported value type %s", value.Type().FriendlyName())
}

// applySchemaInputs returns a new PropertyMap derived from inputs by, for each schema
// property: coercing the input value to the property's type, filling in the schema default
// when no input is provided, and wrapping the value in a secret marker when the property
// is declared secret. Inputs whose key does not match any schema property pass through
// unchanged.
//
// Recursion through nested objects also applies defaults and conversions, but suppresses
// secret-marking when an ancestor is already a secret — the outer wrap covers the inner
// values, so adding redundant inner marks would be wasted (and trips up callers that walk
// the snapshot, like the language conformance suite). User-supplied inner secrets are
// preserved untouched.
func applySchemaInputs(
	inputs property.Map, properties []*schema.Property,
) (property.Map, error) {
	return applySchemaInputsInner(inputs, properties, false)
}

// fillSchemaOutputs ensures that every schema-declared property is present in outputs,
// recursively. Missing properties are filled with computed (when fillUnknown is set, i.e.
// during previews and for skipped creates) or null. Nested
// object-typed properties are also walked so that programs traversing into an optional
// inner field (e.g. `res.data.kubeconfig` where `data` is a required output object whose
// inner fields are optional) see a typed attribute rather than a missing-key error from
// the HCL runtime.
//
// Only the structural set of attributes is materialised; values returned by the provider
// are not transformed. Properties whose key does not match any schema property pass
// through unchanged.
func fillSchemaOutputs(outputs property.Map, properties []*schema.Property, fillUnknown bool) property.Map {
	for _, prop := range properties {
		key := prop.Name
		v, ok := outputs.GetOk(key)
		if !ok {
			if fillUnknown {
				outputs = outputs.Set(key, property.New(property.Computed).WithSecret(prop.Secret))
			} else {
				outputs = outputs.Set(key, property.New(property.Null).WithSecret(prop.Secret))
			}
			continue
		}
		outputs = outputs.Set(key, fillSchemaOutputValue(v, prop.Type, fillUnknown))
	}
	return outputs
}

func fillSchemaOutputValue(value property.Value, targetType schema.Type, fillUnknown bool) property.Value {
	switch t := codegen.UnwrapType(targetType).(type) {
	case *schema.ObjectType:
		if value.IsMap() {
			return property.WithGoValue(value, fillSchemaOutputs(value.AsMap(), t.Properties, fillUnknown))
		}
	case *schema.ArrayType:
		if value.IsArray() {
			array := value.AsArray()
			arr := make([]property.Value, array.Len())
			for i, elem := range array.All {
				arr[i] = fillSchemaOutputValue(elem, t.ElementType, fillUnknown)
			}
			return property.WithGoValue(value, arr)
		}
	case *schema.MapType:
		if value.IsMap() {
			m := value.AsMap().AsMap()
			for k, elem := range m {
				m[k] = fillSchemaOutputValue(elem, t.ElementType, fillUnknown)
			}
			return property.WithGoValue(value, m)
		}
	}
	return value
}

func applySchemaInputsInner(
	inputs property.Map, properties []*schema.Property, insideSecret bool,
) (property.Map, error) {
	converted := make(map[string]property.Value, inputs.Len())
	seen := make(map[string]struct{}, len(properties))

	for _, prop := range properties {
		key := prop.Name
		seen[key] = struct{}{}

		// Anything nested below a secret-marked property is itself "inside a secret".
		nestedInsideSecret := insideSecret || prop.Secret

		var val property.Value
		if input, hasInput := inputs.GetOk(key); hasInput {
			v, err := applySchemaInputConversion(input, prop.Type, nestedInsideSecret)
			if err != nil {
				return property.Map{}, fmt.Errorf("property %q: %w", key, err)
			}
			val = v
		} else if prop.DefaultValue != nil {
			var err error
			val, err = property.Any(prop.DefaultValue.Value)
			contract.AssertNoErrorf(err, "invalid default value of type %T", prop.DefaultValue.Value)
		} else {
			continue
		}

		// Only add a fresh secret marker at the outermost level — once inside a secret,
		// schema-driven marks would just duplicate the outer wrap.
		if !insideSecret && prop.Secret && !val.Secret() {
			val = val.WithSecret(true)
		}
		converted[key] = val
	}

	for key, value := range inputs.All {
		if _, ok := seen[key]; !ok {
			converted[key] = value
		}
	}

	return property.NewMap(converted), nil
}

func applySchemaInputConversion(
	value property.Value, targetType schema.Type, insideSecret bool,
) (out property.Value, _ error) {
	targetType = codegen.UnwrapType(targetType)

	if value.Secret() {
		insideSecret = true
		// Anything inside the secret wrap is by definition "inside a secret".
		defer func() { out = out.WithSecret(true) }()
	}
	if d := value.Dependencies(); len(d) > 0 {
		defer func() { out = out.WithDependencies(append(d, out.Dependencies()...)) }()
	}

	if value.IsComputed() {
		return value, nil
	}

	switch t := targetType.(type) {
	case *schema.ArrayType:
		if !value.IsArray() {
			return value, nil
		}
		arr := value.AsArray()
		converted := make([]property.Value, arr.Len())
		for i, elem := range arr.All {
			v, err := applySchemaInputConversion(elem, t.ElementType, insideSecret)
			if err != nil {
				return property.Value{}, fmt.Errorf("array index %d: %w", i, err)
			}
			converted[i] = v
		}
		return property.New(converted), nil
	case *schema.MapType:
		if !value.IsMap() {
			return value, nil
		}
		obj := value.AsMap()
		converted := make(map[string]property.Value, obj.Len())
		for key, elem := range obj.All {
			v, err := applySchemaInputConversion(elem, t.ElementType, insideSecret)
			if err != nil {
				return property.Value{}, fmt.Errorf("map key %q: %w", key, err)
			}
			converted[key] = v
		}
		return property.New(converted), nil
	case *schema.ObjectType:
		if !value.IsMap() {
			return value, nil
		}
		// Recurse with the full helper so nested objects also fill in schema defaults and
		// mark schema-secret properties. Pass insideSecret through so the inner pass knows
		// to suppress redundant marks when the outer is already a secret.
		converted, err := applySchemaInputsInner(value.AsMap(), t.Properties, insideSecret)
		if err != nil {
			return property.Value{}, err
		}
		return property.New(converted), nil
	case *schema.UnionType:
		// Prefer the original value if it already matches the target type, otherwise try to convert to each element
		// type in turn.
		var first *property.Value
		var errs []error
		for _, elementType := range t.ElementTypes {
			converted, err := applySchemaInputConversion(value, elementType, insideSecret)
			if err != nil {
				errs = append(errs, err)
			} else {
				if converted.Equals(value) {
					return value, nil
				}
				if first == nil {
					first = &converted
				}
			}
		}
		// If we got here we didn't no-op convert in the list above, so just return the first successful conversion if
		// there was one.
		if first != nil {
			return *first, nil
		}
		// Else return what errors we saw in trying to convert to each element type, if any.
		return property.Value{}, fmt.Errorf("cannot convert to any type in union: %v", errs)
	case *schema.ResourceType:
		return value, nil
	}

	switch targetType {
	case schema.BoolType:
		if value.IsBool() {
			return value, nil
		}
		if value.IsString() {
			converted, err := strconv.ParseBool(value.AsString())
			if err != nil {
				return property.Value{}, fmt.Errorf(
					"cannot convert string %q to bool: %w", value.AsString(), err)
			}
			return property.New(converted), nil
		}
	case schema.IntType, schema.NumberType:
		if value.IsNumber() {
			return value, nil
		}
		if value.IsString() {
			converted, err := strconv.ParseFloat(value.AsString(), 64)
			if err != nil {
				return property.Value{}, fmt.Errorf(
					"cannot convert string %q to number: %w", value.AsString(), err)
			}
			return property.New(converted), nil
		}
	case schema.StringType:
		if value.IsString() {
			return value, nil
		}
		if value.IsBool() {
			return property.New(strconv.FormatBool(value.AsBool())), nil
		}
		if value.IsNumber() {
			return property.New(strconv.FormatFloat(value.AsNumber(), 'f', -1, 64)), nil
		}
	}

	// If we couldn't convert to the target type, just try and pass the value as is.
	return value, nil
}

func propertyValueToCty(
	ctx context.Context,
	getResource func(context.Context, property.ResourceReference) (property.Map, error),
	value property.Value,
) (out cty.Value, _ error) {
	if value.Secret() {
		defer func() { out = out.Mark(secretMark{}) }()
	}
	if d := value.Dependencies(); len(d) > 0 {
		defer func() {
			for _, dep := range d {
				out = out.Mark(dependencyMark{dependency: dep})
			}
		}()
	}
	switch {
	case value.IsAsset():
		a := value.AsAsset()
		return cty.CapsuleVal(assetType, a), nil
	case value.IsArchive():
		a := value.AsArchive()
		return cty.CapsuleVal(archiveType, a), nil
	case value.IsComputed():
		return cty.UnknownVal(cty.DynamicPseudoType), nil
	case value.IsNull():
		return cty.NullVal(cty.DynamicPseudoType), nil
	case value.IsBool():
		return cty.BoolVal(value.AsBool()), nil
	case value.IsString():
		return cty.StringVal(value.AsString()), nil
	case value.IsNumber():
		return cty.NumberFloatVal(value.AsNumber()), nil
	case value.IsArray():
		array := value.AsArray()
		vals := make([]cty.Value, array.Len())
		for i, elem := range array.All {
			ctyElem, err := propertyValueToCty(ctx, getResource, elem)
			if err != nil {
				return cty.NilVal, err
			}
			vals[i] = ctyElem
		}
		if len(vals) == 0 {
			return cty.ListValEmpty(cty.DynamicPseudoType), nil
		}
		first := vals[0].Type()
		for _, v := range vals[1:] {
			if !v.Type().Equals(first) {
				return cty.TupleVal(vals), nil
			}
		}
		return cty.ListVal(vals), nil
	case value.IsMap():
		obj := value.AsMap()
		vals := map[string]cty.Value{}
		for k, v := range obj.All {
			ctyVal, err := propertyValueToCty(ctx, getResource, v)
			if err != nil {
				return cty.NilVal, err
			}
			vals[string(k)] = ctyVal
		}
		return cty.ObjectVal(vals), nil
	case value.IsResourceReference():
		// We need to expand the resource into a resource object
		ref := value.AsResourceReference()

		outputs, err := getResource(ctx, ref)
		if err != nil {
			return cty.NilVal, fmt.Errorf("get resource for %s: %w", ref.URN, err)
		}

		return propertyValueToCty(ctx, getResource, property.New(outputs))
	}

	return cty.NilVal, errors.New("unsupported property value")
}
