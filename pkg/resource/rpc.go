// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"reflect"
	"sort"

	"github.com/golang/glog"
	structpb "github.com/golang/protobuf/ptypes/struct"

	"github.com/pulumi/lumi/pkg/util/contract"
)

// MarshalOptions controls the marshaling of RPC structures.
type MarshalOptions struct {
	PermitOlds bool // true to permit old URNs in the properties (e.g., for pre-update).
	RawURNs    bool // true to marshal URNs "as-is"; often used when ID mappings aren't known yet.
}

// MarshalProperties marshals a resource's property map as a "JSON-like" protobuf structure.  Any URNs are replaced
// with their resource IDs during marshaling; it is an error to marshal a URN for a resource without an ID.
func MarshalProperties(ctx *Context, props PropertyMap, opts MarshalOptions) *structpb.Struct {
	result := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}
	for _, key := range StablePropertyKeys(props) {
		if v, use := MarshalPropertyValue(ctx, props[key], opts); use {
			result.Fields[string(key)] = v
		}
	}
	return result
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.
func MarshalPropertyValue(ctx *Context, v PropertyValue, opts MarshalOptions) (*structpb.Value, bool) {
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
		var elems []*structpb.Value
		for _, elem := range v.ArrayValue() {
			if elemv, use := MarshalPropertyValue(ctx, elem, opts); use {
				elems = append(elems, elemv)
			}
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: elems},
			},
		}, true
	} else if v.IsObject() {
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: MarshalProperties(ctx, v.ObjectValue(), opts),
			},
		}, true
	} else if v.IsResource() {
		var wire string
		m := v.ResourceValue()
		if opts.RawURNs {
			wire = string(m)
		} else {
			contract.Assertf(ctx != nil, "Resource encountered with a nil context; URN not recoverable")
			var id ID
			if res, has := ctx.URNRes[m]; has {
				id = res.ID() // found a new resource with this ID, use it.
			} else if oldid, has := ctx.URNOldIDs[m]; opts.PermitOlds && has {
				id = oldid // found an old resource, maybe deleted, so use that.
			} else {
				contract.Failf("Expected resource URN '%v' to exist at marshal time", m)
			}
			contract.Assertf(id != "", "Expected resource URN '%v' to have an ID at marshal time", m)
			wire = string(id)
		}
		glog.V(7).Infof("Serializing resource URN '%v' as '%v' (raw=%v)", m, wire, opts.RawURNs)
		return &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: wire,
			},
		}, true
	}

	contract.Failf("Unrecognized property value: %v (type=%v)", v.V, reflect.TypeOf(v.V))
	return nil, true
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
			// TODO: we have no way of determining that this is a resource ID; consider tagging.
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
