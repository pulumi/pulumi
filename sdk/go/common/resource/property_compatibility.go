// Copyright 2016-2024, Pulumi Corporation.
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

package resource

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Translate a [property.Map] into a [PropertyMap].
//
// This is a lossless transition, such that this will be true:
//
//	FromResourcePropertyMap(ToResourcePropertyMap(m)).Equals(m)
func ToResourcePropertyMap(v property.Map) PropertyMap {
	vMap := v.AsMap()
	rMap := make(PropertyMap, len(vMap))
	for k, vElem := range vMap {
		rMap[PropertyKey(k)] = ToResourcePropertyValue(vElem)
	}
	return rMap
}

// Translate a Value into a PropertyValue.
//
// This is a lossless transition, such that this will be true:
//
//	FromResourcePropertyValue(ToResourcePropertyValue(v)).Equals(v)
func ToResourcePropertyValue(v property.Value) PropertyValue {
	var r PropertyValue
	switch {
	case v.IsBool():
		r = NewBoolProperty(v.AsBool())
	case v.IsNumber():
		r = NewNumberProperty(v.AsNumber())
	case v.IsString():
		r = NewStringProperty(v.AsString())
	case v.IsArray():
		vArr := v.AsArray().AsSlice()
		arr := make([]PropertyValue, len(vArr))
		for i, vElem := range vArr {
			arr[i] = ToResourcePropertyValue(vElem)
		}
		r = NewArrayProperty(arr)
	case v.IsMap():
		r = NewObjectProperty(ToResourcePropertyMap(v.AsMap()))
	case v.IsAsset():
		r = NewAssetProperty(v.AsAsset())
	case v.IsArchive():
		r = NewArchiveProperty(v.AsArchive())
	case v.IsResourceReference():
		ref := v.AsResourceReference()
		r = NewResourceReferenceProperty(ResourceReference{
			URN:            ref.URN,
			ID:             ToResourcePropertyValue(ref.ID),
			PackageVersion: ref.PackageVersion,
		})
	case v.IsNull():
		r = NewNullProperty()
	}

	switch {
	case len(v.Dependencies()) > 0 || (v.Secret() && v.IsComputed()):
		r = NewOutputProperty(Output{
			Element:      r,
			Known:        !v.IsComputed(),
			Secret:       v.Secret(),
			Dependencies: v.Dependencies(),
		})
	case v.Secret():
		r = MakeSecret(r)
	case v.IsComputed():
		r = MakeComputed(NewProperty(""))
	}

	return r
}

// Translate a [PropertyValue] into a [property.Value].
//
// This is a normalizing transition, such that the last expression will be true:
//
//	normalized := ToResourcePropertyMap(FromResourcePropertyMap(m))
//	normalized.DeepEquals(ToResourcePropertyMap(FromResourcePropertyMap(m)))
func FromResourcePropertyMap(v PropertyMap) property.Map {
	rMap := make(map[string]property.Value, len(v))
	for k, v := range v {
		rMap[string(k)] = FromResourcePropertyValue(v)
	}
	return property.NewMap(rMap)
}

// Translate a PropertyValue into a Value.
//
// This is a normalizing transition, such that the last expression will be true:
//
//	normalized := ToResourcePropertyValue(FromResourcePropertyValue(v))
//	normalized.DeepEquals(ToResourcePropertyValue(FromResourcePropertyValue(v)))
func FromResourcePropertyValue(v PropertyValue) property.Value {
	switch {
	// Value types
	case v.IsBool():
		return property.New(v.BoolValue())
	case v.IsNumber():
		return property.New(v.NumberValue())
	case v.IsString():
		return property.New(v.StringValue())
	case v.IsArray():
		vArr := v.ArrayValue()
		arr := make([]property.Value, len(vArr))
		for i, v := range vArr {
			arr[i] = FromResourcePropertyValue(v)
		}
		return property.New(arr)
	case v.IsObject():
		return property.New(FromResourcePropertyMap(v.ObjectValue()))
	case v.IsAsset():
		return property.New(v.AssetValue())
	case v.IsArchive():
		return property.New(v.ArchiveValue())
	case v.IsResourceReference():
		r := v.ResourceReferenceValue()

		return property.New(property.ResourceReference{
			URN:            r.URN,
			ID:             FromResourcePropertyValue(r.ID),
			PackageVersion: r.PackageVersion,
		})
	case v.IsNull():
		return property.Value{}

	// Flavor types
	case v.IsComputed():
		return property.New(property.Computed).WithSecret(
			v.Input().Element.IsSecret() ||
				(v.Input().Element.IsOutput() && v.Input().Element.OutputValue().Secret))
	case v.IsSecret():
		return FromResourcePropertyValue(v.SecretValue().Element).WithSecret(true)
	case v.IsOutput():
		o := v.OutputValue()
		var elem property.Value
		if !o.Known {
			elem = property.New(property.Computed)
		} else {
			elem = FromResourcePropertyValue(o.Element)
		}

		// If the value is already secret, we leave it secret, otherwise we take
		// the value from Output.
		if o.Secret {
			elem = elem.WithSecret(true)
		}

		return elem.WithDependencies(o.Dependencies)

	default:
		contract.Failf("Unknown property value type %T", v.V)
		return property.Value{}
	}
}
