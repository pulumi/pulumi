// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"reflect"
	"sort"

	"github.com/golang/glog"
	structpb "github.com/golang/protobuf/ptypes/struct"

	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
)

// MarshalOptions controls the marshaling of RPC structures.
type MarshalOptions struct {
	SkipNulls        bool // true to skip nulls altogether in the resulting map.
	DisallowUnknowns bool // true if we are disallowing unknown values (results in assertion failures).
}

const (
	// UnknownBoolValue is a sentinel indicating that a bool property's value is not known, because it depends on
	// a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownBoolValue = "1c4a061d-8072-4f0a-a4cb-0ff528b18fe7"
	// UnknownNumberValue is a sentinel indicating that a number property's value is not known, because it depends on
	// a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownNumberValue = "3eeb2bf0-c639-47a8-9e75-3b44932eb421"
	// UnknownStringValue is a sentinel indicating that a string property's value is not known, because it depends on
	// a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownStringValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
	// UnknownArrayValue is a sentinel indicating that an array property's value is not known, because it depends on
	// a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownArrayValue = "6a19a0b0-7e62-4c92-b797-7f8e31da9cc2"
	// UnknownAssetValue is a sentinel indicating that an asset property's value is not known, because it depends on
	// a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownAssetValue = "030794c1-ac77-496b-92df-f27374a8bd58"
	// UnknownArchiveValue is a sentinel indicating that an archive property's value is not known, because it depends
	// on a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownArchiveValue = "e48ece36-62e2-4504-bad9-02848725956a"
	// UnknownObjectValue is a sentinel indicating that an archive property's value is not known, because it depends
	// on a computation with values whose values themselves are not yet known (e.g., dependent upon an output property).
	UnknownObjectValue = "dd056dcd-154b-4c76-9bd3-c8f88648b5ff"
)

// MarshalProperties marshals a resource's property map as a "JSON-like" protobuf structure.
func MarshalProperties(props resource.PropertyMap, opts MarshalOptions) *structpb.Struct {
	fields := make(map[string]*structpb.Value)
	for _, key := range props.StableKeys() {
		v := props[key]
		glog.V(9).Infof("Marshaling property for RPC: %v=%v", key, v)
		if v.IsOutput() {
			glog.V(9).Infof("Skipping output property %v", key)
		} else if opts.SkipNulls && v.IsNull() {
			glog.V(9).Infof("Skipping null property %v (as requested)", key)
		} else {
			fields[string(key)] = MarshalPropertyValue(v, opts)
		}
	}
	return &structpb.Struct{
		Fields: fields,
	}
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.
func MarshalPropertyValue(v resource.PropertyValue, opts MarshalOptions) *structpb.Value {
	if v.IsNull() {
		return MarshalNull(opts)
	} else if v.IsBool() {
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				BoolValue: v.BoolValue(),
			},
		}
	} else if v.IsNumber() {
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: v.NumberValue(),
			},
		}
	} else if v.IsString() {
		return MarshalString(v.StringValue(), opts)
	} else if v.IsArray() {
		var elems []*structpb.Value
		for _, elem := range v.ArrayValue() {
			elems = append(elems, MarshalPropertyValue(elem, opts))
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: elems},
			},
		}
	} else if v.IsAsset() {
		return MarshalAsset(v.AssetValue(), opts)
	} else if v.IsArchive() {
		return MarshalArchive(v.ArchiveValue(), opts)
	} else if v.IsObject() {
		obj := MarshalProperties(v.ObjectValue(), opts)
		return MarshalStruct(obj, opts)
	} else if v.IsComputed() {
		return marshalUnknownProperty(v.ComputedValue().Element, opts)
	} else if v.IsOutput() {
		// Note that at the moment we don't differentiate between computed and output properties on the wire.  As
		// a result, they will show up as computed on the other end.  This distinction isn't currently interesting.
		return marshalUnknownProperty(v.OutputValue().Element, opts)
	}

	contract.Failf("Unrecognized property value: %v (type=%v)", v.V, reflect.TypeOf(v.V))
	return nil
}

// marshalUnknownProperty marshals an unknown property in a way that lets us recover its type on the other end.
func marshalUnknownProperty(elem resource.PropertyValue, opts MarshalOptions) *structpb.Value {
	contract.Assertf(!opts.DisallowUnknowns, "Unexpected unknown value when opts.DisallowUnknowns")
	// Normal cases, these get sentinels.
	if elem.IsBool() {
		return MarshalString(UnknownBoolValue, opts)
	} else if elem.IsNumber() {
		return MarshalString(UnknownNumberValue, opts)
	} else if elem.IsString() {
		return MarshalString(UnknownStringValue, opts)
	} else if elem.IsArray() {
		return MarshalString(UnknownArrayValue, opts)
	} else if elem.IsAsset() {
		return MarshalString(UnknownAssetValue, opts)
	} else if elem.IsArchive() {
		return MarshalString(UnknownArchiveValue, opts)
	} else if elem.IsObject() {
		return MarshalString(UnknownObjectValue, opts)
	}

	// If for some reason we end up with a recursive computed/output, just keep digging.
	if elem.IsComputed() {
		return marshalUnknownProperty(elem.ComputedValue().Element, opts)
	} else if elem.IsOutput() {
		return marshalUnknownProperty(elem.OutputValue().Element, opts)
	}

	// Finally, if a null, we can guess its value!  (the one and only...)
	if elem.IsNull() {
		return MarshalNull(opts)
	}

	contract.Failf("Unexpected output/computed property element: %v", elem)
	return nil
}

// UnmarshalProperties unmarshals a "JSON-like" protobuf structure into a new resource property map.
func UnmarshalProperties(props *structpb.Struct, opts MarshalOptions) resource.PropertyMap {
	result := make(resource.PropertyMap)

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	if props != nil {
		for k := range props.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}

	// And now unmarshal every field it into the map.
	for _, key := range keys {
		pk := resource.PropertyKey(key)
		v := UnmarshalPropertyValue(props.Fields[key], opts)
		glog.V(9).Infof("Unmarshaling property for RPC: %v=%v", key, v)
		if opts.SkipNulls && v.IsNull() {
			glog.V(9).Infof("Skipping unmarshaling of %v (it is null)", key)
		} else {
			result[pk] = v
		}
	}

	return result
}

// UnmarshalPropertyValue unmarshals a single "JSON-like" value into a new property value.
func UnmarshalPropertyValue(v *structpb.Value, opts MarshalOptions) resource.PropertyValue {
	contract.Assert(v != nil)

	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		return resource.NewNullProperty()
	case *structpb.Value_BoolValue:
		return resource.NewBoolProperty(v.GetBoolValue())
	case *structpb.Value_NumberValue:
		return resource.NewNumberProperty(v.GetNumberValue())
	case *structpb.Value_StringValue:
		// If it's a string, it could be an unknown property, or just a regular string.
		s := v.GetStringValue()
		if unk, isunk := unmarshalUnknownPropertyValue(s, opts); isunk {
			return unk
		}
		return resource.NewStringProperty(s)
	case *structpb.Value_ListValue:
		// If there's already an array, prefer to swap elements within it.
		var elems []resource.PropertyValue
		lst := v.GetListValue()
		for i, elem := range lst.GetValues() {
			if i == len(elems) {
				elems = append(elems, resource.PropertyValue{})
			}
			contract.Assert(len(elems) > i)
			elems[i] = UnmarshalPropertyValue(elem, opts)
		}

		return resource.NewArrayProperty(elems)
	case *structpb.Value_StructValue:
		// Start by unmarshaling.
		obj := UnmarshalProperties(v.GetStructValue(), opts)

		// Before returning it as an object, check to see if it's a known recoverable type.
		objmap := obj.Mappable()
		if asset, isasset := resource.DeserializeAsset(objmap); isasset {
			return resource.NewAssetProperty(asset)
		} else if archive, isarchive := resource.DeserializeArchive(objmap); isarchive {
			return resource.NewArchiveProperty(archive)
		}
		return resource.NewObjectProperty(obj)

	default:
		contract.Failf("Unrecognized structpb value kind: %v", reflect.TypeOf(v.Kind))
		return resource.NewNullProperty()
	}
}

func unmarshalUnknownPropertyValue(s string, opts MarshalOptions) (resource.PropertyValue, bool) {
	var elem resource.PropertyValue
	var unknown bool
	switch s {
	case UnknownBoolValue:
		elem, unknown = resource.NewBoolProperty(false), true
	case UnknownNumberValue:
		elem, unknown = resource.NewNumberProperty(0), true
	case UnknownStringValue:
		elem, unknown = resource.NewStringProperty(""), true
	case UnknownArrayValue:
		elem, unknown = resource.NewArrayProperty([]resource.PropertyValue{}), true
	case UnknownAssetValue:
		elem, unknown = resource.NewAssetProperty(resource.Asset{}), true
	case UnknownArchiveValue:
		elem, unknown = resource.NewArchiveProperty(resource.Archive{}), true
	case UnknownObjectValue:
		elem, unknown = resource.NewObjectProperty(make(resource.PropertyMap)), true
	}
	if unknown {
		comp := resource.Computed{Element: elem}
		return resource.NewComputedProperty(comp), true
	}
	return resource.PropertyValue{}, false
}

// MarshalNull marshals a nil to its protobuf form.
func MarshalNull(opts MarshalOptions) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_NullValue{
			NullValue: structpb.NullValue_NULL_VALUE,
		},
	}
}

// MarshalString marshals a string to its protobuf form.
func MarshalString(s string, opts MarshalOptions) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: s,
		},
	}
}

// MarshalStruct marshals a struct for use in a protobuf field where a value is expected.
func MarshalStruct(obj *structpb.Struct, opts MarshalOptions) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: obj,
		},
	}
}

// MarshalAsset marshals an asset into its wire form for resource provider plugins.
func MarshalAsset(v resource.Asset, opts MarshalOptions) *structpb.Value {
	// To marshal an asset, we need to first serialize it, and then marshal that.
	sera := v.Serialize()
	serap := resource.NewPropertyMapFromMap(sera)
	return MarshalPropertyValue(resource.NewObjectProperty(serap), opts)
}

// MarshalArchive marshals an archive into its wire form for resource provider plugins.
func MarshalArchive(v resource.Archive, opts MarshalOptions) *structpb.Value {
	// To marshal an archive, we need to first serialize it, and then marshal that.
	sera := v.Serialize()
	serap := resource.NewPropertyMapFromMap(sera)
	return MarshalPropertyValue(resource.NewObjectProperty(serap), opts)
}
