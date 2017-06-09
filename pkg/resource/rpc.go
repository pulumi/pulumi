// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	OldURNs      bool // true to permit old URNs in the properties (e.g., for pre-update).
	RawResources bool // true to marshal resources "as-is"; often used when ID mappings aren't known yet.
}

// MarshalPropertiesWithUnknowns marshals a resource's property map as a "JSON-like" protobuf structure.  Any URNs are
// replaced with their resource IDs during marshaling; it is an error to marshal a URN for a resource without an ID.  A
// map of any unknown properties encountered during marshaling (latent values) is returned on the side; these values are
// marshaled using the default value in the returned structure and so this map is essential for interpreting results.
func MarshalPropertiesWithUnknowns(
	ctx *Context, props PropertyMap, opts MarshalOptions) (*structpb.Struct, map[string]bool) {
	var unk map[string]bool
	result := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}
	for _, key := range StablePropertyKeys(props) {
		if v := props[key]; !v.IsOutput() {
			mv, known := MarshalPropertyValue(ctx, props[key], opts)
			if known {
				result.Fields[string(key)] = mv
			} else {
				if unk == nil {
					unk = make(map[string]bool)
				}
				unk[string(key)] = true // remember that this property was unknown, tainting this whole object.
			}
		}
	}
	return result, unk
}

// MarshalProperties performs ordinary marshaling of a resource's properties but then validates afterwards that all
// fields were known (in other words, no latent properties were encountered).
func MarshalProperties(ctx *Context, props PropertyMap, opts MarshalOptions) *structpb.Struct {
	pstr, unks := MarshalPropertiesWithUnknowns(ctx, props, opts)
	contract.Assertf(unks == nil, "Unexpected unknown properties during final marshaling")
	return pstr
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.  The
// boolean return value indicates whether the value was known (true) or unknown (false).
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
	} else if v.IsResource() {
		// During marshaling, the wire format of a resource URN is the provider ID; in some circumstances, this is
		// not desirable -- such as when the provider is marshaling data for its own use, or when the IDs are not
		// yet ready -- in which case RawResources == true and we simply marshal the URN as-is.
		var wire string
		m := v.ResourceValue()
		if opts.RawResources {
			wire = string(m)
		} else {
			contract.Assertf(ctx != nil, "Resource encountered with a nil context; URN not recoverable")
			var id ID
			if res, has := ctx.URNRes[m]; has {
				id = res.ID() // found a new resource with this ID, use it.
			} else if oldid, has := ctx.URNOldIDs[m]; opts.OldURNs && has {
				id = oldid // found an old resource, maybe deleted, so use that.
			} else {
				contract.Failf("Expected resource URN '%v' to exist at marshal time", m)
			}
			contract.Assertf(id != "", "Expected resource URN '%v' to have an ID at marshal time", m)
			wire = string(id)
		}
		glog.V(7).Infof("Serializing resource URN '%v' as '%v' (raw=%v)", m, wire, opts.RawResources)
		return &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: wire,
			},
		}, true
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
func UnmarshalProperties(ctx *Context, props *structpb.Struct, opts MarshalOptions) PropertyMap {
	result := make(PropertyMap)
	if props != nil {
		UnmarshalPropertiesInto(ctx, props, result, opts)
	}
	return result
}

// UnmarshalPropertiesInto unmarshals a "JSON-like" protobuf structure into an existing resource property map.
func UnmarshalPropertiesInto(ctx *Context, props *structpb.Struct, t PropertyMap, opts MarshalOptions) {
	contract.Assert(props != nil)

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	for k := range props.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// And now unmarshal every field it into the map.
	for _, k := range keys {
		pk := PropertyKey(k)
		v := t[pk]
		UnmarshalPropertyValueInto(ctx, props.Fields[k], &v, opts)
		contract.Assert(!v.IsComputed())
		t[pk] = v
	}
}

// UnmarshalPropertyValue unmarshals a single "JSON-like" value into a new property value.
func UnmarshalPropertyValue(ctx *Context, v *structpb.Value, opts MarshalOptions) PropertyValue {
	var result PropertyValue
	UnmarshalPropertyValueInto(ctx, v, &result, opts)
	return result
}

// UnmarshalPropertyValueInto unmarshals a single "JSON-like" value into an existing property value slot.  The existing
// slot may be used to drive transformations, if necessary, such as recovering resource URNs on the receiver side.
func UnmarshalPropertyValueInto(ctx *Context, v *structpb.Value, t *PropertyValue, opts MarshalOptions) {
	contract.Assert(v != nil)

	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		contract.Assert(t.CanNull())
		*t = NewNullProperty()
	case *structpb.Value_BoolValue:
		contract.Assert(t.CanBool())
		*t = NewBoolProperty(v.GetBoolValue())
	case *structpb.Value_NumberValue:
		contract.Assert(t.CanNumber())
		*t = NewNumberProperty(v.GetNumberValue())
	case *structpb.Value_StringValue:
		// In the case of a string, the target may be either a resource URN or a real string.
		s := v.GetStringValue()
		if t.CanString() || opts.RawResources {
			*t = NewStringProperty(s)
		} else {
			// During unmarshaling, we must translate from the expected resource wire format of a provider ID, back
			// into its Lumi side URN.  There are cases when this is not desirable, most notably, when a resource
			// provider is unmarshaling data for its own consumption (as is the case for receiving payloads from Lumi,
			// for instance).  In such cases, RawResources == true, and those will end up as simple strings (above).
			contract.Assert(t.CanResource())
			contract.Assertf(ctx != nil, "Resource encountered with a nil context; URN not recoverable")
			id := ID(s)
			urn, has := ctx.IDURN[id]
			contract.Assertf(has, "Expected resource ID '%v' to have a URN; none found", id)
			glog.V(7).Infof("Deserializing resource ID '%v' as URN '%v' (raw=%v)", id, urn, opts.RawResources)
			*t = NewResourceProperty(urn)
		}
	case *structpb.Value_ListValue:
		contract.Assert(t.CanArray())

		// If there's already an array, prefer to swap elements within it.
		var elems []PropertyValue
		if t.IsArray() {
			elems = t.ArrayValue()
		}

		lst := v.GetListValue()
		for i, elem := range lst.GetValues() {
			if i == len(elems) {
				elems = append(elems, PropertyValue{})
			}
			contract.Assert(len(elems) > i)
			UnmarshalPropertyValueInto(ctx, elem, &elems[i], opts)
		}
		*t = NewArrayProperty(elems)
	case *structpb.Value_StructValue:
		contract.Assert(t.CanObject())

		// If there's already an object, prefer to swap existing properties.
		var obj PropertyMap
		if t.IsObject() {
			obj = t.ObjectValue()
		} else {
			obj = make(PropertyMap)
			*t = NewObjectProperty(obj)
		}

		UnmarshalPropertiesInto(ctx, v.GetStructValue(), obj, opts)
	default:
		contract.Failf("Unrecognized structpb value kind: %v", reflect.TypeOf(v.Kind))
	}
}
