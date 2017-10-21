// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"reflect"
	"sort"

	"github.com/golang/glog"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// MarshalOptions controls the marshaling of RPC structures.
type MarshalOptions struct {
	SkipNulls          bool // true to skip nulls altogether in the resulting map.
	AllowUnknowns      bool // true if we are allowing unknown values (otherwise an error results).
	ComputeAssetHashes bool // true if we are computing missing asset hashes on the fly.
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
func MarshalProperties(props resource.PropertyMap, opts MarshalOptions) (*structpb.Struct, error) {
	fields := make(map[string]*structpb.Value)
	for _, key := range props.StableKeys() {
		v := props[key]
		glog.V(9).Infof("Marshaling property for RPC: %v=%v", key, v)
		if v.IsOutput() {
			glog.V(9).Infof("Skipping output property %v", key)
		} else if opts.SkipNulls && v.IsNull() {
			glog.V(9).Infof("Skipping null property %v (as requested)", key)
		} else {
			m, err := MarshalPropertyValue(v, opts)
			if err != nil {
				return nil, err
			}
			fields[string(key)] = m
		}
	}
	return &structpb.Struct{
		Fields: fields,
	}, nil
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.
func MarshalPropertyValue(v resource.PropertyValue, opts MarshalOptions) (*structpb.Value, error) {
	if v.IsNull() {
		return MarshalNull(opts), nil
	} else if v.IsBool() {
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				BoolValue: v.BoolValue(),
			},
		}, nil
	} else if v.IsNumber() {
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: v.NumberValue(),
			},
		}, nil
	} else if v.IsString() {
		return MarshalString(v.StringValue(), opts), nil
	} else if v.IsArray() {
		var elems []*structpb.Value
		for _, elem := range v.ArrayValue() {
			e, err := MarshalPropertyValue(elem, opts)
			if err != nil {
				return nil, err
			}
			elems = append(elems, e)
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: elems},
			},
		}, nil
	} else if v.IsAsset() {
		return MarshalAsset(v.AssetValue(), opts)
	} else if v.IsArchive() {
		return MarshalArchive(v.ArchiveValue(), opts)
	} else if v.IsObject() {
		obj, err := MarshalProperties(v.ObjectValue(), opts)
		if err != nil {
			return nil, err
		}
		return MarshalStruct(obj, opts), nil
	} else if v.IsComputed() {
		elem := v.ComputedValue().Element
		if opts.AllowUnknowns {
			return marshalUnknownProperty(elem, opts), nil
		}
		return nil, errors.Errorf("unpexected computed property during marshaling: %v", elem)
	} else if v.IsOutput() {
		// Note that at the moment we don't differentiate between computed and output properties on the wire.  As
		// a result, they will show up as computed on the other end.  This distinction isn't currently interesting.
		elem := v.ComputedValue().Element
		if opts.AllowUnknowns {
			return marshalUnknownProperty(v.OutputValue().Element, opts), nil
		}
		return nil, errors.Errorf("unexpected output property during marshaling: %v", elem)
	}

	contract.Failf("Unrecognized property value: %v (type=%v)", v.V, reflect.TypeOf(v.V))
	return nil, nil
}

// marshalUnknownProperty marshals an unknown property in a way that lets us recover its type on the other end.
func marshalUnknownProperty(elem resource.PropertyValue, opts MarshalOptions) *structpb.Value {
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
func UnmarshalProperties(props *structpb.Struct, opts MarshalOptions) (resource.PropertyMap, error) {
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
		v, err := UnmarshalPropertyValue(props.Fields[key], opts)
		if err != nil {
			return nil, err
		}
		glog.V(9).Infof("Unmarshaling property for RPC: %v=%v", key, v)
		if opts.SkipNulls && v.IsNull() {
			glog.V(9).Infof("Skipping unmarshaling of %v (it is null)", key)
		} else {
			result[pk] = v
		}
	}

	return result, nil
}

// UnmarshalPropertyValue unmarshals a single "JSON-like" value into a new property value.
func UnmarshalPropertyValue(v *structpb.Value, opts MarshalOptions) (resource.PropertyValue, error) {
	contract.Assert(v != nil)

	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		return resource.NewNullProperty(), nil
	case *structpb.Value_BoolValue:
		return resource.NewBoolProperty(v.GetBoolValue()), nil
	case *structpb.Value_NumberValue:
		return resource.NewNumberProperty(v.GetNumberValue()), nil
	case *structpb.Value_StringValue:
		// If it's a string, it could be an unknown property, or just a regular string.
		s := v.GetStringValue()
		if unk, isunk := unmarshalUnknownPropertyValue(s, opts); isunk {
			if opts.AllowUnknowns {
				return unk, nil
			}
			return resource.PropertyValue{},
				errors.Errorf("unexpected unknown property during unmarshaling: %v", unk)
		}
		return resource.NewStringProperty(s), nil
	case *structpb.Value_ListValue:
		// If there's already an array, prefer to swap elements within it.
		var elems []resource.PropertyValue
		lst := v.GetListValue()
		for i, elem := range lst.GetValues() {
			if i == len(elems) {
				elems = append(elems, resource.PropertyValue{})
			}
			contract.Assert(len(elems) > i)
			e, err := UnmarshalPropertyValue(elem, opts)
			if err != nil {
				return resource.PropertyValue{}, err
			}
			elems[i] = e
		}
		return resource.NewArrayProperty(elems), nil
	case *structpb.Value_StructValue:
		// Start by unmarshaling.
		obj, err := UnmarshalProperties(v.GetStructValue(), opts)
		if err != nil {
			return resource.PropertyValue{}, err
		}

		// Before returning it as an object, check to see if it's a known recoverable type.
		objmap := obj.Mappable()
		asset, isasset, err := resource.DeserializeAsset(objmap, false)
		if err != nil {
			return resource.PropertyValue{}, err
		} else if isasset {
			if opts.ComputeAssetHashes {
				if err = asset.EnsureHash(); err != nil {
					return resource.PropertyValue{}, errors.Wrapf(err, "failed to compute asset hash")
				}
			} else if asset.Hash == "" {
				return resource.PropertyValue{}, errors.New("asset missing hash, and no compute requested")
			}
			return resource.NewAssetProperty(asset), nil
		}
		archive, isarchive, err := resource.DeserializeArchive(objmap, false)
		if err != nil {
			return resource.PropertyValue{}, err
		} else if isarchive {
			if opts.ComputeAssetHashes {
				if err = archive.EnsureHash(); err != nil {
					return resource.PropertyValue{}, errors.Wrapf(err, "failed to compute archive hash")
				}
			} else if archive.Hash == "" {
				return resource.PropertyValue{}, errors.New("archive missing hash, and no compute requested")
			}
			return resource.NewArchiveProperty(archive), nil
		}
		return resource.NewObjectProperty(obj), nil

	default:
		contract.Failf("Unrecognized structpb value kind: %v", reflect.TypeOf(v.Kind))
		return resource.PropertyValue{}, nil
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
		elem, unknown = resource.NewAssetProperty(&resource.Asset{}), true
	case UnknownArchiveValue:
		elem, unknown = resource.NewArchiveProperty(&resource.Archive{}), true
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
func MarshalAsset(v *resource.Asset, opts MarshalOptions) (*structpb.Value, error) {
	// Ensure a hash is present if needed.
	if v.Hash == "" && opts.ComputeAssetHashes {
		if err := v.EnsureHash(); err != nil {
			return nil, errors.Wrapf(err, "failed to compute asset hash")
		}
	}

	// To marshal an asset, we need to first serialize it, and then marshal that.
	sera := v.Serialize()
	serap := resource.NewPropertyMapFromMap(sera)
	return MarshalPropertyValue(resource.NewObjectProperty(serap), opts)
}

// MarshalArchive marshals an archive into its wire form for resource provider plugins.
func MarshalArchive(v *resource.Archive, opts MarshalOptions) (*structpb.Value, error) {
	// Ensure a hash is present if needed.
	if v.Hash == "" && opts.ComputeAssetHashes {
		if err := v.EnsureHash(); err != nil {
			return nil, errors.Wrapf(err, "failed to compute archive hash")
		}
	}

	// To marshal an archive, we need to first serialize it, and then marshal that.
	sera := v.Serialize()
	serap := resource.NewPropertyMapFromMap(sera)
	return MarshalPropertyValue(resource.NewObjectProperty(serap), opts)
}
