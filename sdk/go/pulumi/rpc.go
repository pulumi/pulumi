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

package pulumi

import (
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

func mapStructTypes(from, to reflect.Type) func(reflect.Value, int) (reflect.StructField, reflect.Value) {
	contract.Assert(from.Kind() == reflect.Struct)
	contract.Assert(to.Kind() == reflect.Struct)

	if from == to {
		return func(v reflect.Value, i int) (reflect.StructField, reflect.Value) {
			if !v.IsValid() {
				return to.Field(i), reflect.Value{}
			}
			return to.Field(i), v.Field(i)
		}
	}

	nameToIndex := map[string]int{}
	numFields := to.NumField()
	for i := 0; i < numFields; i++ {
		nameToIndex[to.Field(i).Name] = i
	}

	return func(v reflect.Value, i int) (reflect.StructField, reflect.Value) {
		fieldName := from.Field(i).Name
		j, ok := nameToIndex[fieldName]
		if !ok {
			panic(errors.Errorf("unknown field %v when marshaling inputs of type %v to %v", fieldName, from, to))
		}

		field := to.Field(j)
		if !v.IsValid() {
			return field, reflect.Value{}
		}
		return field, v.Field(j)
	}
}

// marshalInputs turns resource property inputs into a map suitable for marshaling.
func marshalInputs(props Input) (resource.PropertyMap, map[string][]URN, []URN, error) {
	var depURNs []URN
	depset := map[URN]bool{}
	pmap, pdeps := resource.PropertyMap{}, map[string][]URN{}

	if props == nil {
		return pmap, pdeps, depURNs, nil
	}

	pv := reflect.ValueOf(props)
	if pv.Kind() == reflect.Ptr {
		pv = pv.Elem()
	}
	pt := pv.Type()
	contract.Assert(pt.Kind() == reflect.Struct)

	// We use the resolved type to decide how to convert inputs to outputs.
	rt := props.ElementType()
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	contract.Assert(rt.Kind() == reflect.Struct)

	getMappedField := mapStructTypes(pt, rt)

	// Now, marshal each field in the input.
	numFields := pt.NumField()
	for i := 0; i < numFields; i++ {
		destField, _ := getMappedField(reflect.Value{}, i)
		tag := destField.Tag.Get("pulumi")
		if tag == "" {
			continue
		}

		// Get the underlying value, possibly waiting for an output to arrive.
		v, resourceDeps, err := marshalInput(pv.Field(i).Interface(), destField.Type, true)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "awaiting input property %s", tag)
		}

		pmap[resource.PropertyKey(tag)] = v

		// Record all dependencies accumulated from reading this property.
		var deps []URN
		pdepset := map[URN]bool{}
		for _, dep := range resourceDeps {
			depURN, _, err := dep.URN().awaitURN(context.TODO())
			if err != nil {
				return nil, nil, nil, err
			}
			if !pdepset[depURN] {
				deps = append(deps, depURN)
				pdepset[depURN] = true
			}
			if !depset[depURN] {
				depURNs = append(depURNs, depURN)
				depset[depURN] = true
			}
		}
		if len(deps) > 0 {
			pdeps[tag] = deps
		}
	}

	return pmap, pdeps, depURNs, nil
}

// `gosec` thinks these are credentials, but they are not.
// nolint: gosec
const rpcTokenUnknownValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

const cannotAwaitFmt = "cannot marshal Output value of type %T; please use Apply to access the Output's value"

// marshalInput marshals an input value, returning its raw serializable value along with any dependencies.
func marshalInput(v interface{}, destType reflect.Type, await bool) (resource.PropertyValue, []Resource, error) {
	for {
		// If v is nil, just return that.
		if v == nil {
			return resource.PropertyValue{}, nil, nil
		}

		valueType := reflect.TypeOf(v)

		// If this is an Input, make sure it is of the proper type and await it if it is an output/
		var deps []Resource
		if input, ok := v.(Input); ok {
			valueType = input.ElementType()

			// If the element type of the input is not identical to the type of the destination and the destination is
			// not the any type (i.e. interface{}), attempt to convert the input to an appropriately-typed output.
			if valueType != destType && destType != anyType {
				if newOutput, ok := callToOutputMethod(context.TODO(), reflect.ValueOf(input), destType); ok {
					// We were able to convert the input. Use the result as the new input value.
					input, valueType = newOutput, destType
				} else if !valueType.AssignableTo(destType) {
					err := errors.Errorf("cannot marshal an input of type %T as a value of type %v", input, destType)
					return resource.PropertyValue{}, nil, err
				}
			}

			// If the input is an Output, await its value. The returned value is fully resolved.
			if output, ok := input.(Output); ok {
				if !await {
					return resource.PropertyValue{}, nil, errors.Errorf(cannotAwaitFmt, output)
				}

				// Await the output.
				ov, known, err := output.await(context.TODO())
				if err != nil {
					return resource.PropertyValue{}, nil, err
				}

				// If the value is unknown, return the appropriate sentinel.
				if !known {
					return resource.MakeComputed(resource.NewStringProperty("")), output.dependencies(), nil
				}

				v, deps = ov, output.dependencies()
			}
		}

		// Look for some well known types.
		switch v := v.(type) {
		case *asset:
			return resource.NewAssetProperty(&resource.Asset{
				Path: v.Path(),
				Text: v.Text(),
				URI:  v.URI(),
			}), deps, nil
		case *archive:
			var assets map[string]interface{}
			if as := v.Assets(); as != nil {
				assets = make(map[string]interface{})
				for k, a := range as {
					aa, _, err := marshalInput(a, anyType, await)
					if err != nil {
						return resource.PropertyValue{}, nil, err
					}
					assets[k] = aa.V
				}
			}
			return resource.NewArchiveProperty(&resource.Archive{
				Assets: assets,
				Path:   v.Path(),
				URI:    v.URI(),
			}), deps, nil
		case CustomResource:
			deps = append(deps, v)

			// Resources aren't serializable; instead, serialize a reference to ID, tracking as a dependency.
			e, d, err := marshalInput(v.ID(), idType, await)
			if err != nil {
				return resource.PropertyValue{}, nil, err
			}
			return e, append(deps, d...), nil
		}

		contract.Assertf(valueType.AssignableTo(destType), "%v: cannot assign %v to %v", v, valueType, destType)

		if destType.Kind() == reflect.Interface {
			destType = valueType
		}

		rv := reflect.ValueOf(v)
		switch rv.Type().Kind() {
		case reflect.Bool:
			return resource.NewBoolProperty(rv.Bool()), deps, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return resource.NewNumberProperty(float64(rv.Int())), deps, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return resource.NewNumberProperty(float64(rv.Uint())), deps, nil
		case reflect.Float32, reflect.Float64:
			return resource.NewNumberProperty(rv.Float()), deps, nil
		case reflect.Ptr, reflect.Interface:
			// Dereference non-nil pointers and interfaces.
			if rv.IsNil() {
				return resource.PropertyValue{}, deps, nil
			}
			v, destType = rv.Elem().Interface(), destType.Elem()
			continue
		case reflect.String:
			return resource.NewStringProperty(rv.String()), deps, nil
		case reflect.Array, reflect.Slice:
			if rv.IsNil() {
				return resource.PropertyValue{}, deps, nil
			}

			destElem := destType.Elem()

			// If an array or a slice, create a new array by recursing into elements.
			var arr []resource.PropertyValue
			for i := 0; i < rv.Len(); i++ {
				elem := rv.Index(i)
				e, d, err := marshalInput(elem.Interface(), destElem, await)
				if err != nil {
					return resource.PropertyValue{}, nil, err
				}
				if !e.IsNull() {
					arr = append(arr, e)
				}
				deps = append(deps, d...)
			}
			return resource.NewArrayProperty(arr), deps, nil
		case reflect.Map:
			if rv.Type().Key().Kind() != reflect.String {
				return resource.PropertyValue{}, nil,
					errors.Errorf("expected map keys to be strings; got %v", rv.Type().Key())
			}

			if rv.IsNil() {
				return resource.PropertyValue{}, deps, nil
			}

			destElem := destType.Elem()

			// For maps, only support string-based keys, and recurse into the values.
			obj := resource.PropertyMap{}
			for _, key := range rv.MapKeys() {
				value := rv.MapIndex(key)
				mv, d, err := marshalInput(value.Interface(), destElem, await)
				if err != nil {
					return resource.PropertyValue{}, nil, err
				}
				if !mv.IsNull() {
					obj[resource.PropertyKey(key.String())] = mv
				}
				deps = append(deps, d...)
			}
			return resource.NewObjectProperty(obj), deps, nil
		case reflect.Struct:
			obj := resource.PropertyMap{}
			typ := rv.Type()
			getMappedField := mapStructTypes(typ, destType)
			for i := 0; i < typ.NumField(); i++ {
				destField, _ := getMappedField(reflect.Value{}, i)
				tag := destField.Tag.Get("pulumi")
				if tag == "" {
					continue
				}

				fv, d, err := marshalInput(rv.Field(i).Interface(), destField.Type, await)
				if err != nil {
					return resource.PropertyValue{}, nil, err
				}

				if !fv.IsNull() {
					obj[resource.PropertyKey(tag)] = fv
				}
				deps = append(deps, d...)
			}
			return resource.NewObjectProperty(obj), deps, nil
		}
		return resource.PropertyValue{}, nil, errors.Errorf("unrecognized input property type: %v (%T)", v, v)
	}
}

func unmarshalPropertyValue(v resource.PropertyValue) (interface{}, error) {
	switch {
	case v.IsComputed() || v.IsOutput():
		return nil, nil
	case v.IsSecret():
		return nil, errors.New("this version of the Pulumi SDK does not support first-class secrets")
	case v.IsArray():
		arr := v.ArrayValue()
		rv := make([]interface{}, len(arr))
		for i, e := range arr {
			ev, err := unmarshalPropertyValue(e)
			if err != nil {
				return nil, err
			}
			rv[i] = ev
		}
		return rv, nil
	case v.IsObject():
		m := make(map[string]interface{})
		for k, e := range v.ObjectValue() {
			ev, err := unmarshalPropertyValue(e)
			if err != nil {
				return nil, err
			}
			m[string(k)] = ev
		}
		return m, nil
	case v.IsAsset():
		asset := v.AssetValue()
		switch {
		case asset.IsPath():
			return NewFileAsset(asset.Path), nil
		case asset.IsText():
			return NewStringAsset(asset.Text), nil
		case asset.IsURI():
			return NewRemoteAsset(asset.URI), nil
		}
		return nil, errors.New("expected asset to be one of File, String, or Remote; got none")
	case v.IsArchive():
		archive := v.ArchiveValue()
		switch {
		case archive.IsAssets():
			as := make(map[string]interface{})
			for k, v := range archive.Assets {
				a, err := unmarshalPropertyValue(resource.NewPropertyValue(v))
				if err != nil {
					return nil, err
				}
				as[k] = a
			}
			return NewAssetArchive(as), nil
		case archive.IsPath():
			return NewFileArchive(archive.Path), nil
		case archive.IsURI():
			return NewRemoteArchive(archive.URI), nil
		default:
		}
		return nil, errors.New("expected asset to be one of File, String, or Remote; got none")
	default:
		return v.V, nil
	}
}

// unmarshalOutput unmarshals a single output variable into its runtime representation.
func unmarshalOutput(v resource.PropertyValue, dest reflect.Value) error {
	contract.Assert(dest.CanSet())

	// Check for nils and unknowns. The destination will be left with the zero value.
	if v.IsNull() || v.IsComputed() || v.IsOutput() {
		return nil
	}

	// Allocate storage as necessary.
	for dest.Kind() == reflect.Ptr {
		elem := reflect.New(dest.Type().Elem())
		dest.Set(elem)
		dest = elem.Elem()
	}

	// In the case of assets and archives, turn these into real asset and archive structures.
	switch {
	case v.IsAsset():
		if !assetType.AssignableTo(dest.Type()) {
			return errors.Errorf("expected a %s, got an asset", dest.Type())
		}

		asset, err := unmarshalPropertyValue(v)
		if err != nil {
			return err
		}
		dest.Set(reflect.ValueOf(asset))
		return nil
	case v.IsArchive():
		if !archiveType.AssignableTo(dest.Type()) {
			return errors.Errorf("expected a %s, got an archive", dest.Type())
		}

		archive, err := unmarshalPropertyValue(v)
		if err != nil {
			return err
		}
		dest.Set(reflect.ValueOf(archive))
		return nil
	case v.IsSecret():
		return errors.New("this version of the Pulumi SDK does not support first-class secrets")
	}

	// Unmarshal based on the desired type.
	switch dest.Kind() {
	case reflect.Bool:
		if !v.IsBool() {
			return errors.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetBool(v.BoolValue())
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !v.IsNumber() {
			return errors.Errorf("expected an %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetInt(int64(v.NumberValue()))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !v.IsNumber() {
			return errors.Errorf("expected an %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetUint(uint64(v.NumberValue()))
		return nil
	case reflect.Float32, reflect.Float64:
		if !v.IsNumber() {
			return errors.Errorf("expected an %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetFloat(v.NumberValue())
		return nil
	case reflect.String:
		if !v.IsString() {
			return errors.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetString(v.StringValue())
		return nil
	case reflect.Slice:
		if !v.IsArray() {
			return errors.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}
		arr := v.ArrayValue()
		slice := reflect.MakeSlice(dest.Type(), len(arr), len(arr))
		for i, e := range arr {
			if err := unmarshalOutput(e, slice.Index(i)); err != nil {
				return err
			}
		}
		dest.Set(slice)
		return nil
	case reflect.Map:
		if !v.IsObject() {
			return errors.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}

		keyType, elemType := dest.Type().Key(), dest.Type().Elem()
		if keyType.Kind() != reflect.String {
			return errors.Errorf("map keys must be assignable from type string")
		}

		result := reflect.MakeMap(dest.Type())
		for k, e := range v.ObjectValue() {
			elem := reflect.New(elemType).Elem()
			if err := unmarshalOutput(e, elem); err != nil {
				return err
			}

			key := reflect.New(keyType).Elem()
			key.SetString(string(k))

			result.SetMapIndex(key, elem)
		}
		dest.Set(result)
		return nil
	case reflect.Interface:
		if !anyType.Implements(dest.Type()) {
			return errors.Errorf("cannot unmarshal into non-empty interface type %v", dest.Type())
		}

		// If we're unmarshaling into the empty interface type, use the property type as the type of the result.
		result, err := unmarshalPropertyValue(v)
		if err != nil {
			return err
		}
		dest.Set(reflect.ValueOf(result))
		return nil
	case reflect.Struct:
		if !v.IsObject() {
			return errors.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}

		obj := v.ObjectValue()
		typ := dest.Type()
		for i := 0; i < typ.NumField(); i++ {
			fieldV := dest.Field(i)
			if !fieldV.CanSet() {
				continue
			}

			tag := typ.Field(i).Tag.Get("pulumi")
			if tag == "" {
				continue
			}

			e, ok := obj[resource.PropertyKey(tag)]
			if !ok {
				continue
			}

			if err := unmarshalOutput(e, fieldV); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.Errorf("cannot unmarshal into type %v", dest.Type())
	}
}
