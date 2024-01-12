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

package property

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Translate a Value into a resource.PropertyValue.
//
// This is a lossless transition, such that this will be true:
//
//	FromResourcePropertyValue(ToResourcePropertyValue(v)).Equals(v)
func ToResourcePropertyValue(v Value) resource.PropertyValue {
	var r resource.PropertyValue
	switch {
	case v.IsBool():
		r = resource.NewBoolProperty(v.AsBool())
	case v.IsNumber():
		r = resource.NewNumberProperty(v.AsNumber())
	case v.IsString():
		r = resource.NewStringProperty(v.AsString())
	case v.IsArray():
		vArr := v.AsArray()
		arr := make([]resource.PropertyValue, len(vArr))
		for i, vElem := range vArr {
			arr[i] = ToResourcePropertyValue(vElem)
		}
		r = resource.NewArrayProperty(arr)
	case v.IsMap():
		vMap := v.AsMap()
		rMap := make(resource.PropertyMap, len(vMap))
		for k, vElem := range vMap {
			rMap[resource.PropertyKey(k)] = ToResourcePropertyValue(vElem)
		}
		r = resource.NewObjectProperty(rMap)
	case v.IsAsset():
		r = resource.NewAssetProperty(v.AsAsset())
	case v.IsArchive():
		r = resource.NewArchiveProperty(v.AsArchive())
	case v.IsResourceReference():
		r = resource.NewResourceReferenceProperty(v.AsResourceReference())
	case v.IsNull():
		r = resource.NewNullProperty()
	}

	switch {
	case len(v.dependencies) > 0 || (v.isSecret && v.isComputed):
		r = resource.NewOutputProperty(resource.Output{
			Element:      ToResourcePropertyValue(Value{v: v.v}),
			Known:        v.isComputed,
			Secret:       v.isSecret,
			Dependencies: v.dependencies,
		})
	case v.isSecret:
		r = resource.MakeSecret(r)
	case v.isComputed:
		r = resource.MakeComputed(r)
	}

	return r
}

// Translate a resource.PropertyValue into a Value.
//
// This is a normalizing transition, such that the last expression will be true:
//
//	normalized := ToResourcePropertyValue(FromResourcePropertyValue(v))
//	normalized.DeepEquals(ToResourcePropertyValue(FromResourcePropertyValue(v)))
func FromResourcePropertyValue(v resource.PropertyValue) Value {
	switch {
	// Value types
	case v.IsBool():
		return Of(v.BoolValue())
	case v.IsNumber():
		return Of(v.NumberValue())
	case v.IsString():
		return Of(v.StringValue())
	case v.IsArray():
		vArr := v.ArrayValue()
		arr := make(Array, len(vArr))
		for i, v := range vArr {
			arr[i] = FromResourcePropertyValue(v)
		}
		return Of(arr)
	case v.IsObject():
		vMap := v.ObjectValue()
		rMap := make(Map, len(vMap))
		for k, v := range vMap {
			rMap[MapKey(k)] = FromResourcePropertyValue(v)
		}
		return Of(rMap)
	case v.IsAsset():
		return Of(v.AssetValue())
	case v.IsArchive():
		return Of(v.ArchiveValue())
	case v.IsResourceReference():
		return Of(v.ResourceReferenceValue())
	case v.IsNull():
		return Value{}

	// Flavor types
	case v.IsComputed():
		elem := FromResourcePropertyValue(v.Input().Element)
		elem.isComputed = true
		return elem
	case v.IsSecret():
		elem := FromResourcePropertyValue(v.SecretValue().Element)
		elem.isSecret = true
		return elem
	case v.IsOutput():
		o := v.OutputValue()
		elem := FromResourcePropertyValue(o.Element)
		// If the value is already secret, we leave it secret, otherwise we take
		// the value from Output.
		if !elem.isSecret {
			elem.isSecret = o.Secret
		}

		if !elem.isComputed {
			elem.isComputed = !o.Known
		}

		elem.dependencies = o.Dependencies

		return elem

	default:
		contract.Failf("Unknown property value type %T", v.V)
		return Value{}
	}
}
