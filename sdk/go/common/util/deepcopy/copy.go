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

package deepcopy

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Copy returns a deep copy of the provided value.
//
// If there are multiple references to the same value inside the provided value, the multiply-referenced value will be
// copied multiple times.
//
// NOTE: Unexported members of structs will *not* be copied.
func Copy(i any) any {
	if i == nil {
		return nil
	}
	return deepCopy(reflect.ValueOf(i)).Interface()
}

func copyPropertyMap(m property.Map) property.Map {
	values := m.AsMap()
	for k, v := range values {
		values[k] = copyPropertyValue(v)
	}
	return property.NewMap(values)
}

func copyPropertyArray(a property.Array) property.Array {
	values := a.AsSlice()
	for i, v := range values {
		values[i] = copyPropertyValue(v)
	}
	return property.NewArray(values)
}

func copyPropertyValue(v property.Value) property.Value {
	var result property.Value
	switch {
	case v.IsBool():
		result = property.New(v.AsBool())
	case v.IsNumber():
		result = property.New(v.AsNumber())
	case v.IsString():
		result = property.New(v.AsString())
	case v.IsArray():
		result = property.New(copyPropertyArray(v.AsArray()))
	case v.IsMap():
		result = property.New(copyPropertyMap(v.AsMap()))
	case v.IsAsset():
		result = property.New(v.AsAsset())
	case v.IsArchive():
		result = property.New(v.AsArchive())
	case v.IsResourceReference():
		ref := v.AsResourceReference()
		ref.ID = copyPropertyValue(ref.ID)
		result = property.New(ref)
	case v.IsComputed():
		result = property.New(property.Computed)
	case v.IsNull():
		result = property.New(property.Null)
	default:
		contract.Failf("unknown property value type %T", v)
	}

	if v.Secret() {
		result = result.WithSecret(true)
	}
	if deps := v.Dependencies(); len(deps) > 0 {
		result = result.WithDependencies(deps)
	}
	return result
}

func deepCopy(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	if v.Type() == reflect.TypeOf(internal.OutputState{}) {
		contract.Failf("Outputs cannot be deep copied")
	}

	typ := v.Type()
	switch typ.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String,
		reflect.Func:
		// These all have value semantics. Return them as-is.
		return v
	case reflect.Chan:
		// Channels have referential semantics, but deep-copying them has no meaning. Return them as-is.
		return v
	case reflect.Interface:
		rv := reflect.New(typ).Elem()
		if !v.IsNil() {
			rv.Set(deepCopy(v.Elem()))
		}
		return rv
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.New(typ).Elem()
		}
		elem := deepCopy(v.Elem())
		if elem.CanAddr() {
			return elem.Addr()
		}
		rv := reflect.New(typ.Elem())
		rv.Set(elem)
		return rv
	case reflect.Array:
		rv := reflect.New(typ).Elem()
		for i := 0; i < v.Len(); i++ {
			rv.Index(i).Set(deepCopy(v.Index(i)))
		}
		return rv
	case reflect.Slice:
		rv := reflect.New(typ).Elem()
		if !v.IsNil() {
			rv.Set(reflect.MakeSlice(typ, v.Len(), v.Cap()))
			for i := 0; i < v.Len(); i++ {
				rv.Index(i).Set(deepCopy(v.Index(i)))
			}
		}
		return rv
	case reflect.Map:
		rv := reflect.New(typ).Elem()
		if !v.IsNil() {
			rv.Set(reflect.MakeMap(typ))
			iter := v.MapRange()
			for iter.Next() {
				rv.SetMapIndex(deepCopy(iter.Key()), deepCopy(iter.Value()))
			}
		}
		return rv
	case reflect.Struct:
		// Special case property.Value and it's ilk as they are made of private fields, but we still want to be able to
		// copy them. As they are immutable they can be copied by value.
		switch typ {
		case reflect.TypeFor[property.Value]():
		case reflect.TypeFor[property.Array]():
		case reflect.TypeFor[property.Map]():
			return v
		}

		rv := reflect.New(typ).Elem()
		for i := 0; i < typ.NumField(); i++ {
			if f := rv.Field(i); f.CanSet() {
				f.Set(deepCopy(v.Field(i)))
			}
		}
		return rv
	case reflect.Invalid, reflect.UnsafePointer:
		panic("unexpected kind " + typ.Kind().String())
	default:
		panic("unexpected kind " + typ.Kind().String())
	}
}
