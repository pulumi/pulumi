// Copyright 2016-2026, Pulumi Corporation.
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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

type secretMark struct{}

func markSecret(value cty.Value) cty.Value {
	return value.Mark(secretMark{})
}

func unmarkSecret(value cty.Value) (cty.Value, bool) {
	unmarked, marks := value.Unmark()
	if marks == nil {
		return value, false
	}
	secret := false
	for mark := range marks {
		if _, ok := mark.(secretMark); ok {
			secret = true
			continue
		}
		unmarked = unmarked.Mark(mark)
	}
	return unmarked, secret
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

func ctyToPropertyValue(value cty.Value) (resource.PropertyValue, error) {
	unmarked, secret := unmarkSecret(value)
	if !unmarked.IsKnown() {
		return resource.NewProperty(resource.Computed{}), nil
	}
	if unmarked.IsNull() {
		pv := resource.NewNullProperty()
		if secret {
			return resource.MakeSecret(pv), nil
		}
		return pv, nil
	}

	switch unmarked.Type() {
	case cty.String:
		pv := resource.NewProperty(unmarked.AsString())
		if secret {
			return resource.MakeSecret(pv), nil
		}
		return pv, nil
	case cty.Bool:
		pv := resource.NewProperty(unmarked.True())
		if secret {
			return resource.MakeSecret(pv), nil
		}
		return pv, nil
	case cty.Number:
		f, _ := unmarked.AsBigFloat().Float64()
		pv := resource.NewProperty(f)
		if secret {
			return resource.MakeSecret(pv), nil
		}
		return pv, nil
	}

	switch {
	case unmarked.Type().IsListType() || unmarked.Type().IsTupleType():
		var elements []resource.PropertyValue
		it := unmarked.ElementIterator()
		for it.Next() {
			_, v := it.Element()
			pv, err := ctyToPropertyValue(v)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			elements = append(elements, pv)
		}
		pv := resource.NewProperty(elements)
		if secret {
			return resource.MakeSecret(pv), nil
		}
		return pv, nil
	case unmarked.Type().IsMapType() || unmarked.Type().IsObjectType():
		result := resource.PropertyMap{}
		it := unmarked.ElementIterator()
		for it.Next() {
			k, v := it.Element()
			pv, err := ctyToPropertyValue(v)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			result[resource.PropertyKey(k.AsString())] = pv
		}
		pv := resource.NewProperty(result)
		if secret {
			return resource.MakeSecret(pv), nil
		}
		return pv, nil
	}

	return resource.PropertyValue{}, fmt.Errorf("unsupported value type %s", unmarked.Type().FriendlyName())
}

func propertyValueToCty(value resource.PropertyValue) (cty.Value, error) {
	switch {
	case value.IsSecret():
		ctyVal, err := propertyValueToCty(value.SecretValue().Element)
		if err != nil {
			return cty.NilVal, err
		}
		return markSecret(ctyVal), nil
	case value.IsOutput():
		output := value.OutputValue()
		ctyVal, err := propertyValueToCty(output.Element)
		if err != nil {
			return cty.NilVal, err
		}
		if !output.Known {
			return cty.DynamicVal, nil
		}
		if output.Secret {
			return markSecret(ctyVal), nil
		}
		return ctyVal, nil
	case value.IsComputed():
		return cty.DynamicVal, nil
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
			ctyElem, err := propertyValueToCty(elem)
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
			ctyVal, err := propertyValueToCty(v)
			if err != nil {
				return cty.NilVal, err
			}
			vals[string(k)] = ctyVal
		}
		return cty.ObjectVal(vals), nil
	}

	return cty.NilVal, fmt.Errorf("unsupported property value")
}
