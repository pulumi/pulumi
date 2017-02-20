// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"reflect"
	"sort"

	structpb "github.com/golang/protobuf/ptypes/struct"

	"github.com/marapongo/mu/pkg/util/contract"
)

// MarshalProperties marshals a resource's property map as a "JSON-like" protobuf structure.
func MarshalProperties(props PropertyMap) *structpb.Struct {
	result := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}
	for _, key := range StablePropertyKeys(props) {
		result.Fields[string(key)] = MarshalPropertyValue(props[key])
	}
	return result
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.
func MarshalPropertyValue(v PropertyValue) *structpb.Value {
	if v.IsNull() {
		return &structpb.Value{
			Kind: &structpb.Value_NullValue{
				structpb.NullValue_NULL_VALUE,
			},
		}
	} else if v.IsBool() {
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				v.BoolValue(),
			},
		}
	} else if v.IsNumber() {
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				v.NumberValue(),
			},
		}
	} else if v.IsString() {
		return &structpb.Value{
			Kind: &structpb.Value_StringValue{
				v.StringValue(),
			},
		}
	} else if v.IsArray() {
		var elems []*structpb.Value
		for _, elem := range v.ArrayValue() {
			elems = append(elems, MarshalPropertyValue(elem))
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				&structpb.ListValue{elems},
			},
		}
	} else if v.IsObject() {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				MarshalProperties(v.ObjectValue()),
			},
		}
	} else if v.IsResource() {
		// TODO: consider a tag so that the other end knows they are monikers.  These just look like strings.
		return &structpb.Value{
			Kind: &structpb.Value_StringValue{
				string(v.ResourceValue()),
			},
		}
	} else {
		contract.Failf("Unrecognized property value: %v (type=%v)", v.V, reflect.TypeOf(v.V))
		return nil
	}
}

// UnmarshalProperties unmarshals a "JSON-like" protobuf structure into a resource property map.
func UnmarshalProperties(props *structpb.Struct) PropertyMap {
	result := make(PropertyMap)
	if props == nil {
		return result
	}

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	for k := range props.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// And now unmarshal every field it into the map.
	for _, k := range keys {
		result[PropertyKey(k)] = UnmarshalPropertyValue(props.Fields[k])
	}

	return result
}

// UnmarshalPropertyValue unmarshals a single "JSON-like" value into its property form.
func UnmarshalPropertyValue(v *structpb.Value) PropertyValue {
	if v != nil {
		switch v.Kind.(type) {
		case *structpb.Value_NullValue:
			return NewPropertyNull()
		case *structpb.Value_BoolValue:
			return NewPropertyBool(v.GetBoolValue())
		case *structpb.Value_NumberValue:
			return NewPropertyNumber(v.GetNumberValue())
		case *structpb.Value_StringValue:
			// TODO: we have no way of determining that this is a moniker; consider tagging.
			return NewPropertyString(v.GetStringValue())
		case *structpb.Value_ListValue:
			var elems []PropertyValue
			lst := v.GetListValue()
			for _, elem := range lst.GetValues() {
				elems = append(elems, UnmarshalPropertyValue(elem))
			}
			return NewPropertyArray(elems)
		case *structpb.Value_StructValue:
			props := UnmarshalProperties(v.GetStructValue())
			return NewPropertyObject(props)
		default:
			contract.Failf("Unrecognized structpb value kind: %v", reflect.TypeOf(v.Kind))
		}
	}
	return NewPropertyNull()
}
