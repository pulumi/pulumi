// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"reflect"
	"sort"

	"github.com/golang/glog"
	structpb "github.com/golang/protobuf/ptypes/struct"

	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// MarshalOptions controls the marshaling of RPC structures.
type MarshalOptions struct {
	OldURNs      bool // true to permit old URNs in the properties (e.g., for pre-update).
	RawResources bool // true to marshal resources "as-is"; often used when ID mappings aren't known yet.
}

// MarshalPropertiesWithUnknowns marshals a resource's property map as a "JSON-like" protobuf structure.  Any URNs are
// replaced with their resource IDs during marshaling; it is an error to marshal a URN for a resource without an ID.  A
// map of any unknown properties encountered during marshaling (latent values) is returned on the side; these values are
// marshaled using the default value in the returned structure and so this map is essential for interpreting results.
func MarshalPropertiesWithUnknowns(
	ctx *Context, props resource.PropertyMap, opts MarshalOptions) (*structpb.Struct, map[string]bool) {
	var unk map[string]bool
	result := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}
	for _, key := range props.StableKeys() {
		if v := props[key]; !v.IsOutput() {
			glog.V(9).Infof("Marshaling property for RPC: %v=%v", key, v)
			mv, known := MarshalPropertyValue(ctx, v, opts)
			result.Fields[string(key)] = mv

			// If the property was unknown, note it, so that we may tell the provider.
			if !known {
				if unk == nil {
					unk = make(map[string]bool)
				}
				unk[string(key)] = true
			}
		}
	}
	return result, unk
}

// MarshalProperties performs ordinary marshaling of a resource's properties but then validates afterwards that all
// fields were known (in other words, no latent properties were encountered).
func MarshalProperties(ctx *Context, props resource.PropertyMap, opts MarshalOptions) *structpb.Struct {
	pstr, unks := MarshalPropertiesWithUnknowns(ctx, props, opts)
	contract.Assertf(unks == nil, "Unexpected unknown properties during final marshaling")
	return pstr
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.  The
// boolean return value indicates whether the value was known (true) or unknown (false).
func MarshalPropertyValue(ctx *Context, v resource.PropertyValue, opts MarshalOptions) (*structpb.Value, bool) {
	if v.IsNull() {
		return &structpb.Value{
			Kind: &structpb.Value_NullValue{
				NullValue: structpb.NullValue_NULL_VALUE,
			},
		}, true
	} else if v.IsBool() {
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				BoolValue: v.BoolValue(),
			},
		}, true
	} else if v.IsNumber() {
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: v.NumberValue(),
			},
		}, true
	} else if v.IsString() {
		return &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: v.StringValue(),
			},
		}, true
	} else if v.IsArray() {
		outcome := true
		var elems []*structpb.Value
		for _, elem := range v.ArrayValue() {
			elemv, known := MarshalPropertyValue(ctx, elem, opts)
			outcome = outcome && known
			elems = append(elems, elemv)
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: elems},
			},
		}, outcome
	} else if v.IsObject() {
		obj, unks := MarshalPropertiesWithUnknowns(ctx, v.ObjectValue(), opts)
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: obj,
			},
		}, unks == nil
	} else if v.IsComputed() {
		e := v.ComputedValue().Element
		contract.Assert(!e.IsComputed())
		w, known := MarshalPropertyValue(ctx, e, opts)
		contract.Assert(known)
		return w, false
	} else if v.IsOutput() {
		e := v.OutputValue().Element
		contract.Assert(!e.IsComputed())
		w, known := MarshalPropertyValue(ctx, e, opts)
		contract.Assert(known)
		return w, false
	}

	contract.Failf("Unrecognized property value: %v (type=%v)", v.V, reflect.TypeOf(v.V))
	return nil, true
}

// UnmarshalProperties unmarshals a "JSON-like" protobuf structure into a new resource property map.
func UnmarshalProperties(ctx *Context, props *structpb.Struct, opts MarshalOptions) resource.PropertyMap {
	result := make(resource.PropertyMap)
	if props != nil {
		UnmarshalPropertiesInto(ctx, props, result, opts)
	}
	return result
}

// UnmarshalPropertiesInto unmarshals a "JSON-like" protobuf structure into an existing resource property map.
func UnmarshalPropertiesInto(ctx *Context, props *structpb.Struct, t resource.PropertyMap, opts MarshalOptions) {
	contract.Assert(props != nil)

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	for k := range props.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// And now unmarshal every field it into the map.
	for _, k := range keys {
		pk := resource.PropertyKey(k)
		v := t[pk]
		UnmarshalPropertyValueInto(ctx, props.Fields[k], &v, opts)
		contract.Assert(!v.IsComputed())
		t[pk] = v
	}
}

// UnmarshalPropertyValue unmarshals a single "JSON-like" value into a new property value.
func UnmarshalPropertyValue(ctx *Context, v *structpb.Value, opts MarshalOptions) resource.PropertyValue {
	var result resource.PropertyValue
	UnmarshalPropertyValueInto(ctx, v, &result, opts)
	return result
}

// UnmarshalPropertyValueInto unmarshals a single "JSON-like" value into an existing property value slot.  The existing
// slot may be used to drive transformations, if necessary, such as recovering resource URNs on the receiver side.
func UnmarshalPropertyValueInto(ctx *Context, v *structpb.Value, t *resource.PropertyValue, opts MarshalOptions) {
	contract.Assert(v != nil)

	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		contract.Assert(t.CanNull())
		*t = resource.NewNullProperty()
	case *structpb.Value_BoolValue:
		contract.Assert(t.CanBool())
		*t = resource.NewBoolProperty(v.GetBoolValue())
	case *structpb.Value_NumberValue:
		contract.Assert(t.CanNumber())
		*t = resource.NewNumberProperty(v.GetNumberValue())
	case *structpb.Value_StringValue:
		*t = resource.NewStringProperty(v.GetStringValue())
	case *structpb.Value_ListValue:
		contract.Assert(t.CanArray())

		// If there's already an array, prefer to swap elements within it.
		var elems []resource.PropertyValue
		if t.IsArray() {
			elems = t.ArrayValue()
		}

		lst := v.GetListValue()
		for i, elem := range lst.GetValues() {
			if i == len(elems) {
				elems = append(elems, resource.PropertyValue{})
			}
			contract.Assert(len(elems) > i)
			UnmarshalPropertyValueInto(ctx, elem, &elems[i], opts)
		}
		*t = resource.NewArrayProperty(elems)
	case *structpb.Value_StructValue:
		contract.Assert(t.CanObject())

		// If there's already an object, prefer to swap existing properties.
		var obj resource.PropertyMap
		if t.IsObject() {
			obj = t.ObjectValue()
		} else {
			obj = make(resource.PropertyMap)
			*t = resource.NewObjectProperty(obj)
		}

		UnmarshalPropertiesInto(ctx, v.GetStructValue(), obj, opts)
	default:
		contract.Failf("Unrecognized structpb value kind: %v", reflect.TypeOf(v.Kind))
	}
}
