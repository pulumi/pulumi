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
	SkipNulls    bool // true to skip nulls altogether in the resulting map.
	OldURNs      bool // true to permit old URNs in the properties (e.g., for pre-update).
	RawResources bool // true to marshal resources "as-is"; often used when ID mappings aren't known yet.
}

// MarshalPropertiesWithUnknowns marshals a resource's property map as a "JSON-like" protobuf structure.  Any URNs are
// replaced with their resource IDs during marshaling; it is an error to marshal a URN for a resource without an ID.  A
// map of any unknown properties encountered during marshaling (latent values) is returned on the side; these values are
// marshaled using the default value in the returned structure and so this map is essential for interpreting results.
func MarshalPropertiesWithUnknowns(
	props resource.PropertyMap, opts MarshalOptions) (*structpb.Struct, map[string]bool) {
	var unk map[string]bool
	result := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}
	for _, key := range props.StableKeys() {
		v := props[key]
		glog.V(9).Infof("Marshaling property for RPC: %v=%v", key, v)
		if v.IsOutput() {
			glog.V(9).Infof("Skipping output property %v", key)
			continue // skip output properties.
		} else if opts.SkipNulls && v.IsNull() {
			glog.V(9).Infof("Skipping null property %v (as requested)", key)
			continue // skip nulls if requested.
		}

		mv, known := MarshalPropertyValue(v, opts)
		result.Fields[string(key)] = mv

		// If the property was unknown, note it, so that we may tell the provider.
		if !known {
			if unk == nil {
				unk = make(map[string]bool)
			}
			unk[string(key)] = true
		}
	}
	return result, unk
}

// MarshalProperties performs ordinary marshaling of a resource's properties but then validates afterwards that all
// fields were known (in other words, no latent properties were encountered).
func MarshalProperties(props resource.PropertyMap, opts MarshalOptions) *structpb.Struct {
	pstr, unks := MarshalPropertiesWithUnknowns(props, opts)
	contract.Assertf(unks == nil, "Unexpected unknown properties during final marshaling")
	return pstr
}

// MarshalPropertyValue marshals a single resource property value into its "JSON-like" value representation.  The
// boolean return value indicates whether the value was known (true) or unknown (false).
func MarshalPropertyValue(v resource.PropertyValue, opts MarshalOptions) (*structpb.Value, bool) {
	if v.IsNull() {
		return MarshalNull(opts), true
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
		return MarshalString(v.StringValue(), opts), true
	} else if v.IsArray() {
		outcome := true
		var elems []*structpb.Value
		for _, elem := range v.ArrayValue() {
			elemv, known := MarshalPropertyValue(elem, opts)
			outcome = outcome && known
			elems = append(elems, elemv)
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: elems},
			},
		}, outcome
	} else if v.IsAsset() {
		return MarshalAsset(v.AssetValue(), opts), true
	} else if v.IsArchive() {
		return MarshalArchive(v.ArchiveValue(), opts), true
	} else if v.IsObject() {
		obj, unks := MarshalPropertiesWithUnknowns(v.ObjectValue(), opts)
		return MarshalStruct(obj, opts), unks == nil
	} else if v.IsComputed() {
		e := v.ComputedValue().Element
		contract.Assert(!e.IsComputed())
		w, known := MarshalPropertyValue(e, opts)
		contract.Assert(known)
		return w, false
	} else if v.IsOutput() {
		e := v.OutputValue().Element
		contract.Assert(!e.IsComputed())
		w, known := MarshalPropertyValue(e, opts)
		contract.Assert(known)
		return w, false
	}

	contract.Failf("Unrecognized property value: %v (type=%v)", v.V, reflect.TypeOf(v.V))
	return nil, true
}

// UnmarshalProperties unmarshals a "JSON-like" protobuf structure into a new resource property map.
func UnmarshalProperties(props *structpb.Struct, opts MarshalOptions) resource.PropertyMap {
	result := make(resource.PropertyMap)

	// First sort the keys so we enumerate them in order (in case errors happen, we want determinism).
	var keys []string
	for k := range props.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// And now unmarshal every field it into the map.
	for _, key := range keys {
		pk := resource.PropertyKey(key)
		v := UnmarshalPropertyValue(props.Fields[key], opts)
		glog.V(9).Infof("Unmarshaling property for RPC: %v=%v", key, v)
		contract.Assert(!v.IsComputed())
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
		return resource.NewStringProperty(v.GetStringValue())
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
		if asset, isasset := TryUnmarshalAsset(obj); isasset {
			return resource.NewAssetProperty(*asset)
		} else if archive, isarchive := TryUnmarshalArchive(obj); isarchive {
			return resource.NewArchiveProperty(*archive)
		}
		return resource.NewObjectProperty(obj)

	default:
		contract.Failf("Unrecognized structpb value kind: %v", reflect.TypeOf(v.Kind))
		return resource.NewNullProperty()
	}
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

// MarshalOptString marshals a string to its protobuf form (either a null or string payload).
func MarshalOptString(s *string, opts MarshalOptions) *structpb.Value {
	if s == nil {
		return MarshalNull(opts)
	}
	return MarshalString(*s, opts)
}

// MarshalStruct marshals a struct for use in a protobuf field where a value is expected.
func MarshalStruct(obj *structpb.Struct, opts MarshalOptions) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: obj,
		},
	}
}

/// a few random hashes to avoid accidental conflicts with real human-authored properties and values.
var (
	sigProperty = "sig-4dabf18193072939515e22adb298388d" // a special hidden sig property.
	assetSig    = "c44067f5952c0a294b673a41bacd8c17"     // serialized assets.
	archiveSig  = "0def7320c3a5731c473e5ecbe6d01bc7"     // serialized archives.
)

func hasSig(obj resource.PropertyMap, match string) bool {
	if sig, hassig := obj[resource.PropertyKey(sigProperty)]; hassig {
		return sig.IsString() && sig.StringValue() == match
	}
	return false
}

// MarshalAsset marshals an asset into its wire form for resource provider plugins.
func MarshalAsset(a resource.Asset, opts MarshalOptions) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					sigProperty:                MarshalString(assetSig, opts),
					resource.AssetTextProperty: MarshalOptString(a.Text, opts),
					resource.AssetPathProperty: MarshalOptString(a.Path, opts),
					resource.AssetURIProperty:  MarshalOptString(a.URI, opts),
				},
			},
		},
	}
}

// TryUnmarshalAsset tests whether the object is a marshaled asset and, if so, recovers its state.
func TryUnmarshalAsset(obj resource.PropertyMap) (*resource.Asset, bool) {
	if hasSig(obj, assetSig) {
		text := obj[resource.AssetTextProperty].StringValue()
		path := obj[resource.AssetPathProperty].StringValue()
		uri := obj[resource.AssetURIProperty].StringValue()
		return &resource.Asset{
			Text: &text,
			Path: &path,
			URI:  &uri,
		}, true
	}
	return nil, false
}

// MarshalArchive marshals an archive into its wire form for resource provider plugins.
func MarshalArchive(a resource.Archive, opts MarshalOptions) *structpb.Value {
	var assets *structpb.Struct
	if a.Assets != nil {
		assets = &structpb.Struct{
			Fields: make(map[string]*structpb.Value),
		}
		for name, asset := range *a.Assets {
			assets.Fields[name] = MarshalAsset(asset, opts)
		}
	}
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					sigProperty:                    MarshalString(assetSig, opts),
					resource.ArchiveAssetsProperty: MarshalStruct(assets, opts),
					resource.ArchivePathProperty:   MarshalOptString(a.Path, opts),
					resource.ArchiveURIProperty:    MarshalOptString(a.URI, opts),
				},
			},
		},
	}
}

// TryUnmarshalArchive tests whether the object is a marshaled archive and, if so, recovers its state.
func TryUnmarshalArchive(obj resource.PropertyMap) (*resource.Archive, bool) {
	if hasSig(obj, archiveSig) {
		var assets map[string]resource.Asset
		if rawAssets, has := obj[resource.ArchiveAssetsProperty]; has {
			assets = make(map[string]resource.Asset)
			for aname, avalue := range rawAssets.ObjectValue() {
				asset, isasset := TryUnmarshalAsset(avalue.ObjectValue())
				contract.Assert(isasset)
				assets[string(aname)] = *asset
			}
		}
		path := obj[resource.ArchivePathProperty].StringValue()
		uri := obj[resource.ArchiveURIProperty].StringValue()
		return &resource.Archive{
			Assets: &assets,
			Path:   &path,
			URI:    &uri,
		}, true
	}
	return nil, false
}
