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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
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
			panic(fmt.Errorf("unknown field %v when marshaling inputs of type %v to %v", fieldName, from, to))
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

	marshalProperty := func(pname string, pv interface{}, pt reflect.Type) error {
		// Get the underlying value, possibly waiting for an output to arrive.
		v, resourceDeps, err := marshalInput(pv, pt, true)
		if err != nil {
			return fmt.Errorf("awaiting input property %s: %w", pname, err)
		}

		// Record all dependencies accumulated from reading this property.
		var deps []URN
		pdepset := map[URN]bool{}
		for _, dep := range resourceDeps {
			depURN, _, _, err := dep.URN().awaitURN(context.TODO())
			if err != nil {
				return err
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
			pdeps[pname] = deps
		}

		if !v.IsNull() || len(deps) > 0 {
			pmap[resource.PropertyKey(pname)] = v
		}
		return nil
	}

	pv := reflect.ValueOf(props)
	if pv.Kind() == reflect.Ptr {
		if pv.IsNil() {
			return pmap, pdeps, depURNs, nil
		}
		pv = pv.Elem()
	}
	pt := pv.Type()

	rt := props.ElementType()
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	switch pt.Kind() {
	case reflect.Struct:
		contract.Assert(rt.Kind() == reflect.Struct)
		// We use the resolved type to decide how to convert inputs to outputs.
		rt := props.ElementType()
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		getMappedField := mapStructTypes(pt, rt)
		// Now, marshal each field in the input.
		numFields := pt.NumField()
		for i := 0; i < numFields; i++ {
			destField, _ := getMappedField(reflect.Value{}, i)
			tag := destField.Tag.Get("pulumi")
			if tag == "" {
				continue
			}
			err := marshalProperty(tag, pv.Field(i).Interface(), destField.Type)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	case reflect.Map:
		contract.Assert(rt.Key().Kind() == reflect.String)
		for _, key := range pv.MapKeys() {
			keyname := key.Interface().(string)
			val := pv.MapIndex(key).Interface()
			err := marshalProperty(keyname, val, rt.Elem())
			if err != nil {
				return nil, nil, nil, err
			}
		}
	default:
		return nil, nil, nil, fmt.Errorf("cannot marshal Input that is not a struct or map, saw type %s", pt.String())
	}

	return pmap, pdeps, depURNs, nil
}

// `gosec` thinks these are credentials, but they are not.
// nolint: gosec
const rpcTokenUnknownValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

const cannotAwaitFmt = "cannot marshal Output value of type %T; please use Apply to access the Output's value"

// marshalInput marshals an input value, returning its raw serializable value along with any dependencies.
func marshalInput(v interface{}, destType reflect.Type, await bool) (resource.PropertyValue, []Resource, error) {
	val, deps, secret, err := marshalInputAndDetermineSecret(v, destType, await)
	if err != nil {
		return val, deps, err
	}

	if secret {
		return resource.MakeSecret(val), deps, nil
	}

	return val, deps, nil
}

// marshalInputAndDetermineSecret marshals an input value with information about secret status
func marshalInputAndDetermineSecret(v interface{},
	destType reflect.Type,
	await bool) (resource.PropertyValue, []Resource, bool, error) {
	secret := false
	for {
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
					err := fmt.Errorf("cannot marshal an input of type %T as a value of type %v", input, destType)
					return resource.PropertyValue{}, nil, false, err
				}
			}

			// If the input is an Output, await its value. The returned value is fully resolved.
			if output, ok := input.(Output); ok {
				if !await {
					return resource.PropertyValue{}, nil, false, fmt.Errorf(cannotAwaitFmt, output)
				}

				// Await the output.
				ov, known, outputSecret, outputDeps, err := output.await(context.TODO())
				if err != nil {
					return resource.PropertyValue{}, nil, false, err
				}
				secret = outputSecret

				// If the value is unknown, return the appropriate sentinel.
				if !known {
					return resource.MakeComputed(resource.NewStringProperty("")), outputDeps, secret, nil
				}

				v, deps = ov, outputDeps
			}
		}

		// If v is nil, just return that.
		if v == nil {
			return resource.PropertyValue{}, nil, secret, nil
		}

		// Look for some well known types.
		switch v := v.(type) {
		case *asset:
			return resource.NewAssetProperty(&resource.Asset{
				Path: v.Path(),
				Text: v.Text(),
				URI:  v.URI(),
			}), deps, secret, nil
		case *archive:
			var assets map[string]interface{}
			if as := v.Assets(); as != nil {
				assets = make(map[string]interface{})
				for k, a := range as {
					aa, _, err := marshalInput(a, anyType, await)
					if err != nil {
						return resource.PropertyValue{}, nil, false, err
					}
					assets[k] = aa.V
				}
			}
			return resource.NewArchiveProperty(&resource.Archive{
				Assets: assets,
				Path:   v.Path(),
				URI:    v.URI(),
			}), deps, secret, nil
		case Resource:
			deps = append(deps, v)

			urn, known, secretURN, err := v.URN().awaitURN(context.Background())
			if err != nil {
				return resource.PropertyValue{}, nil, false, err
			}
			contract.Assert(known)
			contract.Assert(!secretURN)

			var id ID
			if custom, ok := v.(CustomResource); ok {
				resID, known, secretID, err := custom.ID().awaitID(context.Background())
				if err != nil {
					return resource.PropertyValue{}, nil, false, err
				}
				contract.Assert(!secretID)
				if !known {
					return resource.MakeComputed(resource.NewStringProperty("")), deps, secret, nil
				}
				id = resID
			}

			return resource.MakeResourceReference(resource.URN(urn), resource.ID(id), ""), deps, secret, nil
		}

		contract.Assertf(valueType.AssignableTo(destType) || valueType.ConvertibleTo(destType),
			"%v: cannot assign %v to %v", v, valueType, destType)

		if destType.Kind() == reflect.Interface {
			// This happens in the case of Any.
			if valueType.Kind() == reflect.Interface {
				valueType = reflect.TypeOf(v)
			}
			destType = valueType
		}

		rv := reflect.ValueOf(v)
		switch rv.Type().Kind() {
		case reflect.Bool:
			return resource.NewBoolProperty(rv.Bool()), deps, secret, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return resource.NewNumberProperty(float64(rv.Int())), deps, secret, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return resource.NewNumberProperty(float64(rv.Uint())), deps, secret, nil
		case reflect.Float32, reflect.Float64:
			return resource.NewNumberProperty(rv.Float()), deps, secret, nil
		case reflect.Ptr, reflect.Interface:
			// Dereference non-nil pointers and interfaces.
			if rv.IsNil() {
				return resource.PropertyValue{}, deps, secret, nil
			}
			if destType.Kind() == reflect.Ptr {
				destType = destType.Elem()
			}
			v = rv.Elem().Interface()
			continue
		case reflect.String:
			return resource.NewStringProperty(rv.String()), deps, secret, nil
		case reflect.Array, reflect.Slice:
			if rv.IsNil() {
				return resource.PropertyValue{}, deps, secret, nil
			}

			destElem := destType.Elem()

			// If an array or a slice, create a new array by recursing into elements.
			var arr []resource.PropertyValue
			for i := 0; i < rv.Len(); i++ {
				elem := rv.Index(i)
				e, d, err := marshalInput(elem.Interface(), destElem, await)
				if err != nil {
					return resource.PropertyValue{}, nil, false, err
				}
				if !e.IsNull() {
					arr = append(arr, e)
				}
				deps = append(deps, d...)
			}
			return resource.NewArrayProperty(arr), deps, secret, nil
		case reflect.Map:
			if rv.Type().Key().Kind() != reflect.String {
				return resource.PropertyValue{}, nil, false,
					fmt.Errorf("expected map keys to be strings; got %v", rv.Type().Key())
			}

			if rv.IsNil() {
				return resource.PropertyValue{}, deps, secret, nil
			}

			destElem := destType.Elem()

			// For maps, only support string-based keys, and recurse into the values.
			obj := resource.PropertyMap{}
			for _, key := range rv.MapKeys() {
				value := rv.MapIndex(key)
				mv, d, err := marshalInput(value.Interface(), destElem, await)
				if err != nil {
					return resource.PropertyValue{}, nil, false, err
				}
				if !mv.IsNull() {
					obj[resource.PropertyKey(key.String())] = mv
				}
				deps = append(deps, d...)
			}
			return resource.NewObjectProperty(obj), deps, secret, nil
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
					return resource.PropertyValue{}, nil, false, err
				}

				if !fv.IsNull() {
					obj[resource.PropertyKey(tag)] = fv
				}
				deps = append(deps, d...)
			}
			return resource.NewObjectProperty(obj), deps, secret, nil
		}
		return resource.PropertyValue{}, nil, false, fmt.Errorf("unrecognized input property type: %v (%T)", v, v)
	}
}

func unmarshalPropertyValue(v resource.PropertyValue) (interface{}, bool, error) {
	switch {
	case v.IsComputed() || v.IsOutput():
		return nil, false, nil
	case v.IsSecret():
		sv, _, err := unmarshalPropertyValue(v.SecretValue().Element)
		if err != nil {
			return nil, false, err
		}
		return sv, true, nil
	case v.IsArray():
		arr := v.ArrayValue()
		rv := make([]interface{}, len(arr))
		secret := false
		for i, e := range arr {
			ev, esecret, err := unmarshalPropertyValue(e)
			secret = secret || esecret
			if err != nil {
				return nil, false, err
			}
			rv[i] = ev
		}
		return rv, secret, nil
	case v.IsObject():
		m := make(map[string]interface{})
		secret := false
		for k, e := range v.ObjectValue() {
			ev, esecret, err := unmarshalPropertyValue(e)
			secret = secret || esecret
			if err != nil {
				return nil, false, err
			}
			m[string(k)] = ev
		}
		return m, secret, nil
	case v.IsAsset():
		asset := v.AssetValue()
		switch {
		case asset.IsPath():
			return NewFileAsset(asset.Path), false, nil
		case asset.IsText():
			return NewStringAsset(asset.Text), false, nil
		case asset.IsURI():
			return NewRemoteAsset(asset.URI), false, nil
		}
		return nil, false, errors.New("expected asset to be one of File, String, or Remote; got none")
	case v.IsArchive():
		archive := v.ArchiveValue()
		secret := false
		switch {
		case archive.IsAssets():
			as := make(map[string]interface{})
			for k, v := range archive.Assets {
				a, asecret, err := unmarshalPropertyValue(resource.NewPropertyValue(v))
				secret = secret || asecret
				if err != nil {
					return nil, false, err
				}
				as[k] = a
			}
			return NewAssetArchive(as), secret, nil
		case archive.IsPath():
			return NewFileArchive(archive.Path), secret, nil
		case archive.IsURI():
			return NewRemoteArchive(archive.URI), secret, nil
		}
		return nil, false, errors.New("expected asset to be one of File, String, or Remote; got none")
	case v.IsResourceReference():
		ref := v.ResourceReferenceValue()

		resName := ref.URN.Name()
		resType := ref.URN.Type()

		var resource Resource
		var err error

		isProvider := tokens.Token(resType).HasModuleMember() && resType.Module() == "pulumi:providers"
		if isProvider {
			pkgName := resType.Name()
			resourcePackageV, ok := resourcePackages.Load(packageKey(string(pkgName), ref.PackageVersion))
			if !ok {
				err := fmt.Errorf("unable to deserialize provider %v, no resource package is registered for %v",
					ref.URN, pkgName)
				return nil, false, err
			}
			resourcePackage := resourcePackageV.(ResourcePackage)
			resource, err = resourcePackage.ConstructProvider(string(resName), string(resType), nil, string(ref.URN))
		} else {
			modName := resType.Module()
			resourceModuleV, ok := resourceModules.Load(packageKey(string(modName), ref.PackageVersion))
			if !ok {
				err := fmt.Errorf("unable to deserialize resource %v, no module is registered for %v", ref.URN, modName)
				return nil, false, err
			}
			resourceModule := resourceModuleV.(ResourceModule)
			resource, err = resourceModule.Construct(string(resName), string(resType), nil, string(ref.URN))
		}
		if err != nil {
			return nil, false, err
		}
		return resource, false, nil
	default:
		return v.V, false, nil
	}
}

// unmarshalOutput unmarshals a single output variable into its runtime representation.
// returning a bool that indicates secretness
func unmarshalOutput(v resource.PropertyValue, dest reflect.Value) (bool, error) {
	contract.Assert(dest.CanSet())

	// Check for nils and unknowns. The destination will be left with the zero value.
	if v.IsNull() || v.IsComputed() || v.IsOutput() {
		return false, nil
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
			return false, fmt.Errorf("expected a %s, got an asset", dest.Type())
		}

		asset, secret, err := unmarshalPropertyValue(v)
		if err != nil {
			return false, err
		}
		dest.Set(reflect.ValueOf(asset))
		return secret, nil
	case v.IsArchive():
		if !archiveType.AssignableTo(dest.Type()) {
			return false, fmt.Errorf("expected a %s, got an archive", dest.Type())
		}

		archive, secret, err := unmarshalPropertyValue(v)
		if err != nil {
			return false, err
		}
		dest.Set(reflect.ValueOf(archive))
		return secret, nil
	case v.IsSecret():
		if _, err := unmarshalOutput(v.SecretValue().Element, dest); err != nil {
			return false, err
		}
		return true, nil
	case v.IsResourceReference():
		res, secret, err := unmarshalPropertyValue(v)
		if err != nil {
			return false, err
		}
		resV := reflect.ValueOf(res).Elem()
		if !resV.Type().AssignableTo(dest.Type()) {
			return false, fmt.Errorf("expected a %s, got a resource of type %s", dest.Type(), resV.Type())
		}
		dest.Set(resV)
		return secret, nil
	}

	// Unmarshal based on the desired type.
	switch dest.Kind() {
	case reflect.Bool:
		if !v.IsBool() {
			return false, fmt.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetBool(v.BoolValue())
		return false, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !v.IsNumber() {
			return false, fmt.Errorf("expected an %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetInt(int64(v.NumberValue()))
		return false, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !v.IsNumber() {
			return false, fmt.Errorf("expected an %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetUint(uint64(v.NumberValue()))
		return false, nil
	case reflect.Float32, reflect.Float64:
		if !v.IsNumber() {
			return false, fmt.Errorf("expected an %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetFloat(v.NumberValue())
		return false, nil
	case reflect.String:
		if !v.IsString() {
			return false, fmt.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}
		dest.SetString(v.StringValue())
		return false, nil
	case reflect.Slice:
		if !v.IsArray() {
			return false, fmt.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}
		arr := v.ArrayValue()
		slice := reflect.MakeSlice(dest.Type(), len(arr), len(arr))
		secret := false
		for i, e := range arr {
			isecret, err := unmarshalOutput(e, slice.Index(i))
			if err != nil {
				return false, err
			}
			secret = secret || isecret
		}
		dest.Set(slice)
		return secret, nil
	case reflect.Map:
		if !v.IsObject() {
			return false, fmt.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}

		keyType, elemType := dest.Type().Key(), dest.Type().Elem()
		if keyType.Kind() != reflect.String {
			return false, fmt.Errorf("map keys must be assignable from type string")
		}

		result := reflect.MakeMap(dest.Type())
		secret := false
		for k, e := range v.ObjectValue() {
			// ignore properties internal to the pulumi engine
			if strings.HasPrefix(string(k), "__") {
				continue
			}
			elem := reflect.New(elemType).Elem()
			esecret, err := unmarshalOutput(e, elem)
			if err != nil {
				return false, err
			}
			secret = secret || esecret

			key := reflect.New(keyType).Elem()
			key.SetString(string(k))

			result.SetMapIndex(key, elem)
		}
		dest.Set(result)
		return secret, nil
	case reflect.Interface:
		if !anyType.Implements(dest.Type()) {
			return false, fmt.Errorf("cannot unmarshal into non-empty interface type %v", dest.Type())
		}

		// If we're unmarshaling into the empty interface type, use the property type as the type of the result.
		result, secret, err := unmarshalPropertyValue(v)
		if err != nil {
			return false, err
		}
		dest.Set(reflect.ValueOf(result))
		return secret, nil
	case reflect.Struct:
		if !v.IsObject() {
			return false, fmt.Errorf("expected a %v, got a %s", dest.Type(), v.TypeString())
		}

		obj := v.ObjectValue()
		typ := dest.Type()
		secret := false
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

			osecret, err := unmarshalOutput(e, fieldV)
			secret = secret || osecret
			if err != nil {
				return false, err
			}
		}
		return secret, nil
	default:
		return false, fmt.Errorf("cannot unmarshal into type %v", dest.Type())
	}
}

type ResourcePackage interface {
	ConstructProvider(name, typ string, args map[string]interface{}, urn string) (ProviderResource, error)
}

var resourcePackages sync.Map // map[string]ResourcePackage

func packageKey(name, version string) string {
	return fmt.Sprintf("%s@%s", name, version)
}

// RegisterResourcePackage register a resource package with the Pulumi runtime.
func RegisterResourcePackage(name, version string, pkg ResourcePackage) {
	key := packageKey(name, version)
	existing, hasExisting := resourcePackages.LoadOrStore(key, pkg)
	if hasExisting {
		panic(fmt.Errorf("a resource package for %v is already registered: %v", key, existing))
	}
}

type ResourceModule interface {
	Construct(name, typ string, args map[string]interface{}, urn string) (Resource, error)
}

var resourceModules sync.Map // map[string]ResourceModule

func moduleKey(name, version string) string {
	return fmt.Sprintf("%s@%s", name, version)
}

// RegisterResourceModule register a resource module with the Pulumi runtime.
func RegisterResourceModule(name, version string, module ResourceModule) {
	key := moduleKey(name, version)
	existing, hasExisting := resourceModules.LoadOrStore(key, module)
	if hasExisting {
		panic(fmt.Errorf("a resource module for %v is already registered: %v", key, existing))
	}
}
