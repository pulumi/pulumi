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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	"google.golang.org/protobuf/types/known/structpb"
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

func ctyToPropertyValue(value cty.Value) (resource.PropertyValue, error) {
	var inner func(cty.Value) (resource.PropertyValue, error)
	inner = func(value cty.Value) (resource.PropertyValue, error) {
		// First check for dependencies as that will lift this to an output type
		var dependencies []resource.URN
		value, dependency := unmark[dependencyMark](value)
		for dependency != nil {
			dependencies = append(dependencies, dependency.dependency)
			value, dependency = unmark[dependencyMark](value)
		}
		if dependencies != nil {
			pv, err := inner(value)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			return resource.NewProperty(resource.Output{
				Element:      pv,
				Known:        true,
				Dependencies: dependencies,
			}), nil
		}

		if !value.IsKnown() {
			return resource.NewProperty(resource.Computed{Element: resource.NewProperty("")}), nil
		}
		if value.IsNull() {
			return resource.NewNullProperty(), nil
		}

		if value.Type().Equals(assetType) {
			assetValue, ok := value.EncapsulatedValue().(*asset.Asset)
			if !ok {
				return resource.PropertyValue{}, errors.New("unexpected non-asset capsule value")
			}
			return resource.NewProperty(assetValue), nil
		}

		if value.Type().Equals(archiveType) {
			archiveValue, ok := value.EncapsulatedValue().(*archive.Archive)
			if !ok {
				return resource.PropertyValue{}, errors.New("unexpected non-archive capsule value")
			}
			return resource.NewProperty(archiveValue), nil
		}

		switch value.Type() {
		case cty.String:
			pv := resource.NewProperty(value.AsString())
			return pv, nil
		case cty.Bool:
			pv := resource.NewProperty(value.True())
			return pv, nil
		case cty.Number:
			f, _ := value.AsBigFloat().Float64()
			pv := resource.NewProperty(f)
			return pv, nil
		}

		switch {
		case value.Type().IsListType() || value.Type().IsTupleType():
			var elements []resource.PropertyValue
			it := value.ElementIterator()
			for it.Next() {
				_, v := it.Element()
				pv, err := ctyToPropertyValue(v)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				elements = append(elements, pv)
			}
			pv := resource.NewProperty(elements)
			return pv, nil
		case value.Type().IsMapType() || value.Type().IsObjectType():
			result := resource.PropertyMap{}
			it := value.ElementIterator()
			for it.Next() {
				k, v := it.Element()
				pv, err := ctyToPropertyValue(v)
				if err != nil {
					return resource.PropertyValue{}, err
				}
				result[resource.PropertyKey(k.AsString())] = pv
			}
			pv := resource.NewProperty(result)
			return pv, nil
		}

		return resource.PropertyValue{}, fmt.Errorf("unsupported value type %s", value.Type().FriendlyName())
	}

	value, poison := unmark[poisonMark](value)
	if poison != nil {
		return resource.PropertyValue{}, &poisonError{name: poison.name}
	}

	value, secret := unmark[secretMark](value)
	pv, err := inner(value)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	if secret != nil {
		return resource.MakeSecret(pv), nil
	}
	return pv, nil
}

func convertInvokeInputObject(args resource.PropertyMap, inputType *schema.ObjectType) (resource.PropertyMap, error) {
	schemaProperties := make(map[string]schema.Type, len(inputType.Properties))
	for _, prop := range inputType.Properties {
		schemaProperties[prop.Name] = prop.Type
	}

	converted := make(resource.PropertyMap, len(args))
	for key, value := range args {
		targetType, ok := schemaProperties[string(key)]
		if !ok {
			converted[key] = value
			continue
		}

		convertedValue, err := convertPropertyValueForSchemaType(value, targetType)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", key, err)
		}
		converted[key] = convertedValue
	}

	return converted, nil
}

func convertPropertyValueForSchemaType(
	value resource.PropertyValue, targetType schema.Type,
) (resource.PropertyValue, error) {
	targetType = codegen.UnwrapType(targetType)

	if value.IsSecret() {
		converted, err := convertPropertyValueForSchemaType(value.SecretValue().Element, targetType)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.MakeSecret(converted), nil
	}

	if value.IsOutput() {
		out := value.OutputValue()
		copied := resource.Output{
			Element:      out.Element,
			Known:        out.Known,
			Secret:       out.Secret,
			Dependencies: out.Dependencies,
		}

		if copied.Known {
			converted, err := convertPropertyValueForSchemaType(copied.Element, targetType)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			copied.Element = converted
		}
		return resource.NewProperty(copied), nil
	}

	if value.IsComputed() {
		return value, nil
	}

	switch t := targetType.(type) {
	case *schema.ArrayType:
		if !value.IsArray() {
			return value, nil
		}
		arr := value.ArrayValue()
		converted := make([]resource.PropertyValue, len(arr))
		for i, elem := range arr {
			v, err := convertPropertyValueForSchemaType(elem, t.ElementType)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("array index %d: %w", i, err)
			}
			converted[i] = v
		}
		return resource.NewProperty(converted), nil
	case *schema.MapType:
		if !value.IsObject() {
			return value, nil
		}
		obj := value.ObjectValue()
		converted := make(resource.PropertyMap, len(obj))
		for key, elem := range obj {
			v, err := convertPropertyValueForSchemaType(elem, t.ElementType)
			if err != nil {
				return resource.PropertyValue{}, fmt.Errorf("map key %q: %w", key, err)
			}
			converted[key] = v
		}
		return resource.NewProperty(converted), nil
	case *schema.ObjectType:
		if !value.IsObject() {
			return value, nil
		}
		converted, err := convertInvokeInputObject(value.ObjectValue(), t)
		if err != nil {
			return resource.PropertyValue{}, err
		}
		return resource.NewProperty(converted), nil
	case *schema.UnionType:
		// Prefer the original value if it already matches the target type, otherwise try to convert to each element
		// type in turn.
		var first *resource.PropertyValue
		var errs []error
		for _, elementType := range t.ElementTypes {
			converted, err := convertPropertyValueForSchemaType(value, elementType)
			if err != nil {
				errs = append(errs, err)
			} else {
				if converted.DeepEquals(value) {
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
		return resource.PropertyValue{}, fmt.Errorf("cannot convert to any type in union: %v", errs)
	case *schema.ResourceType:
		return value, nil
	}

	switch targetType {
	case schema.BoolType:
		if value.IsBool() {
			return value, nil
		}
		if value.IsString() {
			converted, err := strconv.ParseBool(value.StringValue())
			if err != nil {
				return value, nil
			}
			return resource.NewProperty(converted), nil
		}
	case schema.IntType, schema.NumberType:
		if value.IsNumber() {
			return value, nil
		}
		if value.IsString() {
			converted, err := strconv.ParseFloat(value.StringValue(), 64)
			if err != nil {
				return value, nil
			}
			return resource.NewProperty(converted), nil
		}
	case schema.StringType:
		if value.IsString() {
			return value, nil
		}
		if value.IsBool() {
			return resource.NewProperty(strconv.FormatBool(value.BoolValue())), nil
		}
		if value.IsNumber() {
			return resource.NewProperty(strconv.FormatFloat(value.NumberValue(), 'f', -1, 64)), nil
		}
	}

	// If we couldn't convert to the target type, just try and pass the value as is.
	return value, nil
}

func propertyValueToCty(
	ctx context.Context,
	monitor pulumirpc.ResourceMonitorClient,
	value resource.PropertyValue,
) (cty.Value, error) {
	switch {
	case value.IsAsset():
		a := value.AssetValue()
		return cty.CapsuleVal(assetType, a), nil
	case value.IsArchive():
		a := value.ArchiveValue()
		return cty.CapsuleVal(archiveType, a), nil
	case value.IsSecret():
		ctyVal, err := propertyValueToCty(ctx, monitor, value.SecretValue().Element)
		if err != nil {
			return cty.NilVal, err
		}
		return ctyVal.Mark(secretMark{}), nil
	case value.IsOutput():
		output := value.OutputValue()
		var ctyVal cty.Value
		if !output.Known {
			ctyVal = cty.UnknownVal(cty.DynamicPseudoType)
		} else {
			var err error
			ctyVal, err = propertyValueToCty(ctx, monitor, output.Element)
			if err != nil {
				return cty.NilVal, err
			}
		}
		if output.Secret {
			ctyVal = ctyVal.Mark(secretMark{})
		}
		for _, dep := range output.Dependencies {
			ctyVal = ctyVal.Mark(dependencyMark{dependency: dep})
		}
		return ctyVal, nil
	case value.IsComputed():
		return cty.UnknownVal(cty.DynamicPseudoType), nil
	case value.IsNull():
		return cty.NullVal(cty.DynamicPseudoType), nil
	case value.IsBool():
		return cty.BoolVal(value.BoolValue()), nil
	case value.IsString():
		return cty.StringVal(value.StringValue()), nil
	case value.IsNumber():
		return cty.NumberFloatVal(value.NumberValue()), nil
	case value.IsArray():
		array := value.ArrayValue()
		vals := make([]cty.Value, len(array))
		for i, elem := range array {
			ctyElem, err := propertyValueToCty(ctx, monitor, elem)
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
	case value.IsObject():
		obj := value.ObjectValue()
		vals := map[string]cty.Value{}
		for k, v := range obj {
			ctyVal, err := propertyValueToCty(ctx, monitor, v)
			if err != nil {
				return cty.NilVal, err
			}
			vals[string(k)] = ctyVal
		}
		return cty.ObjectVal(vals), nil
	case value.IsResourceReference():
		// We need to expand the resource into a resource object
		ref := value.ResourceReferenceValue()

		args, err := structpb.NewStruct(map[string]any{
			"urn": string(ref.URN),
		})
		contract.AssertNoErrorf(err, "failed to create structpb for resource reference")

		resp, err := monitor.Invoke(ctx, &pulumirpc.ResourceInvokeRequest{
			Tok:             "pulumi:pulumi:getResource",
			Args:            args,
			AcceptResources: true,
		})
		if err != nil {
			return cty.NilVal, fmt.Errorf("invoke getResource for %s: %w", ref.URN, err)
		}

		marshalOpts := plugin.MarshalOptions{
			KeepUnknowns:  true,
			KeepSecrets:   true,
			KeepResources: true,
		}
		outputs, err := plugin.UnmarshalProperties(resp.Return, marshalOpts)
		if err != nil {
			return cty.NilVal, fmt.Errorf("unmarshal stack outputs: %w", err)
		}
		outputs = outputs["state"].ObjectValue()

		outputs["id"] = ref.ID
		outputs["urn"] = resource.NewProperty(string(ref.URN))
		outputs["__name"] = resource.NewProperty(ref.URN.Name())
		outputs["__type"] = resource.NewProperty(string(ref.URN.Type()))

		return propertyValueToCty(ctx, monitor, resource.NewProperty(outputs))
	}

	return cty.NilVal, errors.New("unsupported property value")
}
