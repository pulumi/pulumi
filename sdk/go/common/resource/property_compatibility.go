// Copyright 2016, Pulumi Corporation.
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

// Translate a [property.Value] into a [PropertyValue].
//
// This is a lossless transition, such that this will be true:
//
//	FromResourcePropertyValue(ToResourcePropertyValue(v)).Equals(v)
func ToResourcePropertyValue(v property.Value) PropertyValue {
	var r PropertyValue
	switch {
	case v.IsBool():
		r = NewProperty(v.AsBool())
	case v.IsNumber():
		r = NewProperty(v.AsNumber())
	case v.IsString():
		r = NewProperty(v.AsString())
	case v.IsArray():
		vArr := v.AsArray().AsSlice()
		arr := make([]PropertyValue, len(vArr))
		for i, vElem := range vArr {
			arr[i] = ToResourcePropertyValue(vElem)
		}
		r = NewProperty(arr)
	case v.IsMap():
		r = NewProperty(ToResourcePropertyMap(v.AsMap()))
	case v.IsAsset():
		r = NewProperty(v.AsAsset())
	case v.IsArchive():
		r = NewProperty(v.AsArchive())
	case v.IsResourceReference():
		ref := v.AsResourceReference()
		r = NewProperty(ResourceReference{
			URN:            ref.URN,
			Name:           ref.Name,
			Type:           ref.Type,
			ID:             ToResourcePropertyValue(ref.ID),
			PackageVersion: ref.PackageVersion,
		})
	case v.IsNull():
		r = NewNullProperty()
	case v.IsComputed():
		r = MakeComputed(NewProperty(""))
	}

	switch {
	case len(v.Dependencies()) > 0:
		r = NewProperty(Output{
			Element:      r,
			Known:        !v.IsComputed(),
			Secret:       v.Secret(),
			Dependencies: v.Dependencies(),
		})
	case v.Secret():
		r = MakeSecret(r)
	}

	return r
}

// Translate a [PropertyMap] into a [property.Map].
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

// Translate a [PropertyMap] into a [*property.Map].
//
// See FromResourcePropertyMap for details on the translation.
func FromResourcePropertyMapPtr(v PropertyMap) *property.Map {
	if v == nil {
		return nil
	}
	m := FromResourcePropertyMap(v)
	return &m
}

// Translate a [PropertyValue] into a [property.Value].
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
			Name:           r.Name,
			Type:           r.Type,
			ID:             FromResourcePropertyValue(r.ID),
			PackageVersion: r.PackageVersion,
		})
	case v.IsNull():
		return property.Value{}

	// Flavor types
	case v.IsComputed():
		return property.New(property.Computed).WithSecret(
			v.Input().Element.IsSecret() ||
				(v.Input().Element.IsOutput() && v.Input().Element.OutputValue().Secret),
		)
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

func FromResourcePropertyPath(v PropertyPath) property.Path {
	str, err := v.MarshalText()
	contract.AssertNoErrorf(err, "Failed to marshal PropertyPath %v", v)
	var p property.Path
	if err := p.UnmarshalText(str); err != nil {
		contract.Failf("Failed to unmarshal property.Path %v: %v", v, err)
	}
	return p
}

func ToResourcePropertyPath(v property.Path) PropertyPath {
	str, err := v.MarshalText()
	contract.AssertNoErrorf(err, "Failed to marshal property.Path %v", v)
	var p PropertyPath
	if err := p.UnmarshalText(str); err != nil {
		contract.Failf("Failed to unmarshal PropertyPath %v: %v", v, err)
	}
	return p
}

func toResourceArrayDiff(v *property.ArrayDiff) *ArrayDiff {
	if v == nil {
		return nil
	}

	adds := make(map[int]PropertyValue, len(v.Adds))
	for k, v := range v.Adds {
		adds[k] = ToResourcePropertyValue(v)
	}

	deletes := make(map[int]PropertyValue, len(v.Deletes))
	for k, v := range v.Deletes {
		deletes[k] = ToResourcePropertyValue(v)
	}

	sames := make(map[int]PropertyValue, len(v.Sames))
	for k, v := range v.Sames {
		sames[k] = ToResourcePropertyValue(v)
	}

	updates := make(map[int]ValueDiff, len(v.Updates))
	for k, v := range v.Updates {
		updates[k] = ValueDiff{
			Old:    ToResourcePropertyValue(v.Old),
			New:    ToResourcePropertyValue(v.New),
			Array:  toResourceArrayDiff(v.Array),
			Object: ToResourceObjectDiff(v.Object),
		}
	}

	return &ArrayDiff{
		Adds:    adds,
		Deletes: deletes,
		Sames:   sames,
		Updates: updates,
	}
}

// Translate a [property.ObjectDiff] into an [ObjectDiff].
func ToResourceObjectDiff(v *property.ObjectDiff) *ObjectDiff {
	if v == nil {
		return nil
	}

	adds := make(map[PropertyKey]PropertyValue, len(v.Adds))
	for k, v := range v.Adds {
		adds[PropertyKey(k)] = ToResourcePropertyValue(v)
	}

	deletes := make(map[PropertyKey]PropertyValue, len(v.Deletes))
	for k, v := range v.Deletes {
		deletes[PropertyKey(k)] = ToResourcePropertyValue(v)
	}

	sames := make(map[PropertyKey]PropertyValue, len(v.Sames))
	for k, v := range v.Sames {
		sames[PropertyKey(k)] = ToResourcePropertyValue(v)
	}

	updates := make(map[PropertyKey]ValueDiff, len(v.Updates))
	for k, v := range v.Updates {
		updates[PropertyKey(k)] = ValueDiff{
			Old:    ToResourcePropertyValue(v.Old),
			New:    ToResourcePropertyValue(v.New),
			Array:  toResourceArrayDiff(v.Array),
			Object: ToResourceObjectDiff(v.Object),
		}
	}

	return &ObjectDiff{
		Adds:    adds,
		Deletes: deletes,
		Sames:   sames,
		Updates: updates,
	}
}
