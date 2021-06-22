// Copyright 2016-2018, Pulumi Corporation.
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

package plugin

import (
	"reflect"
	"sort"

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// MarshalOptions controls the marshaling of RPC structures.
type MarshalOptions struct {
	Label              string // an optional label for debugging.
	SkipNulls          bool   // true to skip nulls altogether in the resulting map.
	KeepUnknowns       bool   // true if we are keeping unknown values (otherwise we skip them).
	RejectUnknowns     bool   // true if we should return errors on unknown values. Takes precedence over KeepUnknowns.
	ElideAssetContents bool   // true if we are eliding the contents of assets.
	ComputeAssetHashes bool   // true if we are computing missing asset hashes on the fly.
	KeepSecrets        bool   // true if we are keeping secrets (otherwise we replace them with their underlying value).
	RejectAssets       bool   // true if we should return errors on Asset and Archive values.
	KeepResources      bool   // true if we are keeping resoures (otherwise we return raw urn).
	SkipInternalKeys   bool   // true to skip internal property keys (keys that start with "__") in the resulting map.
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
		logging.V(9).Infof("Marshaling property for RPC[%s]: %s=%v", opts.Label, key, v)
		if v.IsOutput() {
			logging.V(9).Infof("Skipping output property for RPC[%s]: %v", opts.Label, key)
		} else if opts.SkipNulls && v.IsNull() {
			logging.V(9).Infof("Skipping null property for RPC[%s]: %s (as requested)", opts.Label, key)
		} else if opts.SkipInternalKeys && resource.IsInternalPropertyKey(key) {
			logging.V(9).Infof("Skipping internal property for RPC[%s]: %s (as requested)", opts.Label, key)
		} else {
			m, err := MarshalPropertyValue(v, opts)
			if err != nil {
				return nil, err
			} else if m != nil {
				fields[string(key)] = m
			}
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
			if e != nil {
				elems = append(elems, e)
			}
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: elems},
			},
		}, nil
	} else if v.IsAsset() {
		if opts.RejectAssets {
			return nil, errors.New("unexpected Asset property value")
		}
		return MarshalAsset(v.AssetValue(), opts)
	} else if v.IsArchive() {
		if opts.RejectAssets {
			return nil, errors.New("unexpected Asset Archive property value")
		}
		return MarshalArchive(v.ArchiveValue(), opts)
	} else if v.IsObject() {
		obj, err := MarshalProperties(v.ObjectValue(), opts)
		if err != nil {
			return nil, err
		}
		return MarshalStruct(obj, opts), nil
	} else if v.IsComputed() {
		if opts.RejectUnknowns {
			return nil, errors.New("unexpected unknown property value")
		} else if opts.KeepUnknowns {
			return marshalUnknownProperty(v.Input().Element, opts), nil
		}
		return nil, nil // return nil and the caller will ignore it.
	} else if v.IsOutput() {
		// Note that at the moment we don't differentiate between computed and output properties on the wire.  As
		// a result, they will show up as computed on the other end.  This distinction isn't currently interesting.
		if opts.KeepUnknowns {
			return marshalUnknownProperty(v.OutputValue().Element, opts), nil
		}
		return nil, nil // return nil and the caller will ignore it.
	} else if v.IsSecret() {
		if !opts.KeepSecrets {
			logging.V(5).Infof("marshalling secret value as raw value as opts.KeepSecrets is false")
			return MarshalPropertyValue(v.SecretValue().Element, opts)
		}
		secret := resource.NewObjectProperty(resource.PropertyMap{
			resource.SigKey: resource.NewStringProperty(resource.SecretSig),
			"value":         v.SecretValue().Element,
		})
		return MarshalPropertyValue(secret, opts)
	} else if v.IsResourceReference() {
		ref := v.ResourceReferenceValue()
		if !opts.KeepResources {
			val := string(ref.URN)
			if !ref.ID.IsNull() {
				return MarshalPropertyValue(ref.ID, opts)
			}
			logging.V(5).Infof("marshalling resource value as raw URN or ID as opts.KeepResources is false")
			return MarshalString(val, opts), nil
		}
		m := resource.PropertyMap{
			resource.SigKey: resource.NewStringProperty(resource.ResourceReferenceSig),
			"urn":           resource.NewStringProperty(string(ref.URN)),
		}
		if id, hasID := ref.IDString(); hasID {
			m["id"] = resource.NewStringProperty(id)
		}
		if ref.PackageVersion != "" {
			m["packageVersion"] = resource.NewStringProperty(ref.PackageVersion)
		}
		return MarshalPropertyValue(resource.NewObjectProperty(m), opts)
	}

	contract.Failf("Unrecognized property value in RPC[%s]: %v (type=%v)", opts.Label, v.V, reflect.TypeOf(v.V))
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
		return marshalUnknownProperty(elem.Input().Element, opts)
	} else if elem.IsOutput() {
		return marshalUnknownProperty(elem.OutputValue().Element, opts)
	}

	// Finally, if a null, we can guess its value!  (the one and only...)
	if elem.IsNull() {
		return MarshalNull(opts)
	}

	contract.Failf("Unexpected output/computed property element in RPC[%s]: %v", opts.Label, elem)
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
		} else if v != nil {
			logging.V(9).Infof("Unmarshaling property for RPC[%s]: %s=%v", opts.Label, key, v)
			if opts.SkipNulls && v.IsNull() {
				logging.V(9).Infof("Skipping unmarshaling for RPC[%s]: %s is null", opts.Label, key)
			} else if opts.SkipInternalKeys && resource.IsInternalPropertyKey(pk) {
				logging.V(9).Infof("Skipping unmarshaling for RPC[%s]: %s is internal", opts.Label, key)
			} else {
				result[pk] = *v
			}
		}
	}

	return result, nil
}

// UnmarshalPropertyValue unmarshals a single "JSON-like" value into a new property value.
func UnmarshalPropertyValue(v *structpb.Value, opts MarshalOptions) (*resource.PropertyValue, error) {
	contract.Assert(v != nil)

	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		m := resource.NewNullProperty()
		return &m, nil
	case *structpb.Value_BoolValue:
		m := resource.NewBoolProperty(v.GetBoolValue())
		return &m, nil
	case *structpb.Value_NumberValue:
		m := resource.NewNumberProperty(v.GetNumberValue())
		return &m, nil
	case *structpb.Value_StringValue:
		// If it's a string, it could be an unknown property, or just a regular string.
		s := v.GetStringValue()
		if unk, isunk := unmarshalUnknownPropertyValue(s, opts); isunk {
			if opts.RejectUnknowns {
				return nil, errors.New("unexpected unknown property value")
			} else if opts.KeepUnknowns {
				return &unk, nil
			}
			return nil, nil
		}
		m := resource.NewStringProperty(s)
		return &m, nil
	case *structpb.Value_ListValue:
		lst := v.GetListValue()
		elems := make([]resource.PropertyValue, len(lst.GetValues()))
		for i, elem := range lst.GetValues() {
			e, err := UnmarshalPropertyValue(elem, opts)
			if err != nil {
				return nil, err
			} else if e != nil {
				if i == len(elems) {
					elems = append(elems, *e)
				} else {
					elems[i] = *e
				}
			}
		}
		m := resource.NewArrayProperty(elems)
		return &m, nil
	case *structpb.Value_StructValue:
		// Start by unmarshaling.
		obj, err := UnmarshalProperties(v.GetStructValue(), opts)
		if err != nil {
			return nil, err
		}

		// Before returning it as an object, check to see if it's a known recoverable type.
		objmap := obj.Mappable()
		sig, hasSig := objmap[string(resource.SigKey)]
		if !hasSig {
			// This is a weakly-typed object map.
			m := resource.NewObjectProperty(obj)
			return &m, nil
		}

		switch sig {
		case resource.AssetSig:
			if opts.RejectAssets {
				return nil, errors.New("unexpected Asset property value")
			}
			asset, isasset, err := resource.DeserializeAsset(objmap)
			if err != nil {
				return nil, err
			}
			// This can only be false with a non-nil error if there is a signature match. We've already verified the
			// signature.
			contract.Assert(isasset)
			if opts.ComputeAssetHashes {
				if err = asset.EnsureHash(); err != nil {
					return nil, errors.Wrapf(err, "failed to compute asset hash")
				}
			}
			m := resource.NewAssetProperty(asset)
			return &m, nil
		case resource.ArchiveSig:
			if opts.RejectAssets {
				return nil, errors.New("unexpected Asset Archive property value")
			}
			archive, isarchive, err := resource.DeserializeArchive(objmap)
			if err != nil {
				return nil, err
			}
			// This can only be false with a non-nil error if there is a signature match. We've already verified the
			// signature.
			contract.Assert(isarchive)
			if opts.ComputeAssetHashes {
				if err = archive.EnsureHash(); err != nil {
					return nil, errors.Wrapf(err, "failed to compute archive hash")
				}
			}
			m := resource.NewArchiveProperty(archive)
			return &m, nil
		case resource.SecretSig:
			value, ok := obj["value"]
			if !ok {
				return nil, errors.New("malformed RPC secret: missing value")
			}
			if !opts.KeepSecrets {
				logging.V(5).Infof("unmarshalling secret as raw value, as opts.KeepSecrets is false")
				return &value, nil
			}
			s := resource.MakeSecret(value)
			return &s, nil
		case resource.ResourceReferenceSig:
			urn, ok := obj["urn"]
			if !ok {
				return nil, errors.New("malformed resource reference: missing urn")
			}
			if !urn.IsString() {
				return nil, errors.New("malformed resource reference: urn not a string")
			}

			id, hasID := "", false
			if idProp, ok := obj["id"]; ok {
				hasID = true
				switch {
				case idProp.IsString():
					id = idProp.StringValue()
				case idProp.IsComputed():
					// Leave the ID empty to indicate that it is unknown.
				default:
					return nil, errors.New("malformed resource reference: id not a string")
				}
			}

			var packageVersion string
			if packageVersionProp, ok := obj["packageVersion"]; ok {
				if !packageVersionProp.IsString() {
					return nil, errors.New("malformed resource reference: packageVersion not a string")
				}
				packageVersion = packageVersionProp.StringValue()
			}

			if !opts.KeepResources {
				value := urn.StringValue()
				if hasID {
					isIDUnknown := id == ""
					if isIDUnknown && opts.KeepUnknowns {
						v := structpb.Value{
							Kind: &structpb.Value_StringValue{StringValue: UnknownStringValue},
						}
						return UnmarshalPropertyValue(&v, opts)
					}
					value = id
				}
				r := resource.NewStringProperty(value)
				return &r, nil
			}

			var ref resource.PropertyValue
			if hasID {
				ref = resource.MakeCustomResourceReference(resource.URN(urn.StringValue()), resource.ID(id), packageVersion)
			} else {
				ref = resource.MakeComponentResourceReference(resource.URN(urn.StringValue()), packageVersion)
			}
			return &ref, nil
		default:
			return nil, errors.Errorf("unrecognized signature '%v' in property map", sig)
		}

	default:
		contract.Failf("Unrecognized structpb value kind in RPC[%s]: %v", opts.Label, reflect.TypeOf(v.Kind))
		return nil, nil
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
	// If we are not providing access to an asset's contents, we simply need to record the fact that this asset existed.
	// Serialize the asset with only its hash (if present).
	if opts.ElideAssetContents {
		v = &resource.Asset{Hash: v.Hash}
	} else {
		// Ensure a hash is present if needed.
		if v.Hash == "" && opts.ComputeAssetHashes {
			if err := v.EnsureHash(); err != nil {
				return nil, errors.Wrapf(err, "failed to compute asset hash")
			}
		}
	}

	// To marshal an asset, we need to first serialize it, and then marshal that.
	sera := v.Serialize()
	serap := resource.NewPropertyMapFromMap(sera)
	return MarshalPropertyValue(resource.NewObjectProperty(serap), opts)
}

// MarshalArchive marshals an archive into its wire form for resource provider plugins.
func MarshalArchive(v *resource.Archive, opts MarshalOptions) (*structpb.Value, error) {
	// If we are not providing access to an asset's contents, we simply need to record the fact that this asset existed.
	// Serialize the asset with only its hash (if present).
	if opts.ElideAssetContents {
		v = &resource.Archive{Hash: v.Hash}
	} else {
		// Ensure a hash is present if needed.
		if v.Hash == "" && opts.ComputeAssetHashes {
			if err := v.EnsureHash(); err != nil {
				return nil, errors.Wrapf(err, "failed to compute archive hash")
			}
		}
	}

	// To marshal an archive, we need to first serialize it, and then marshal that.
	sera := v.Serialize()
	serap := resource.NewPropertyMapFromMap(sera)
	return MarshalPropertyValue(resource.NewObjectProperty(serap), opts)
}
