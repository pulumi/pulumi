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

	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"golang.org/x/net/context"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/sdk/go/pulumi/asset"
)

// marshalInputs turns resource property inputs into a gRPC struct suitable for marshaling.
func marshalInputs(props map[string]interface{}) (*structpb.Struct, map[string][]URN, []URN, error) {
	var depURNs []URN
	pmap, pdeps := make(map[string]interface{}), make(map[string][]URN)
	for key := range props {
		// Get the underlying value, possibly waiting for an output to arrive.
		v, resourceDeps, err := marshalInput(props[key])
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "awaiting input property %s", key)
		}

		pmap[key] = v

		// Record all dependencies accumulated from reading this property.
		deps := make([]URN, 0, len(resourceDeps))
		for _, dep := range resourceDeps {
			depURN, _, err := dep.URN().await(context.TODO())
			if err != nil {
				return nil, nil, nil, err
			}
			deps = append(deps, depURN)
		}
		pdeps[key] = deps

		depURNs = append(depURNs, deps...)
	}

	// Marshal all properties for the RPC call.
	m, err := plugin.MarshalProperties(
		resource.NewPropertyMapFromMap(pmap),
		plugin.MarshalOptions{KeepUnknowns: true},
	)
	return m, pdeps, depURNs, err
}

// `gosec` thinks these are credentials, but they are not.
// nolint: gosec
const (
	rpcTokenSpecialSigKey     = "4dabf18193072939515e22adb298388d"
	rpcTokenSpecialAssetSig   = "c44067f5952c0a294b673a41bacd8c17"
	rpcTokenSpecialArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7"
	rpcTokenSpecialSecretSig  = "1b47061264138c4ac30d75fd1eb44270"
	rpcTokenUnknownValue      = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"
)

// marshalInput marshals an input value, returning its raw serializable value along with any dependencies.
func marshalInput(v interface{}) (interface{}, []Resource, error) {
	for {
		// If v is nil, just return that.
		if v == nil {
			return nil, nil, nil
		}

		// If this is an Output, recurse.
		if out, ok := isOutput(v); ok {
			return marshalInputOutput(out)
		}

		// Next, look for some well known types.
		switch v := v.(type) {
		case asset.Asset:
			return map[string]interface{}{
				rpcTokenSpecialSigKey: rpcTokenSpecialAssetSig,
				"path":                v.Path(),
				"text":                v.Text(),
				"uri":                 v.URI(),
			}, nil, nil
		case asset.Archive:
			var assets map[string]interface{}
			if as := v.Assets(); as != nil {
				assets = make(map[string]interface{})
				for k, a := range as {
					aa, _, err := marshalInput(a)
					if err != nil {
						return nil, nil, err
					}
					assets[k] = aa
				}
			}

			return map[string]interface{}{
				rpcTokenSpecialSigKey: rpcTokenSpecialAssetSig,
				"assets":              assets,
				"path":                v.Path(),
				"uri":                 v.URI(),
			}, nil, nil
		case CustomResource:
			// Resources aren't serializable; instead, serialize a reference to ID, tracking as a dependency.
			e, d, err := marshalInput(v.ID())
			if err != nil {
				return nil, nil, err
			}
			return e, append([]Resource{v}, d...), nil
		}

		rv := reflect.ValueOf(v)
		switch rv.Type().Kind() {
		case reflect.Bool:
			return rv.Bool(), nil, nil
		case reflect.Int:
			return int(rv.Int()), nil, nil
		case reflect.Int8:
			return int8(rv.Int()), nil, nil
		case reflect.Int16:
			return int16(rv.Int()), nil, nil
		case reflect.Int32:
			return int32(rv.Int()), nil, nil
		case reflect.Int64:
			return rv.Int(), nil, nil
		case reflect.Uint:
			return uint(rv.Uint()), nil, nil
		case reflect.Uint8:
			return uint8(rv.Uint()), nil, nil
		case reflect.Uint16:
			return uint16(rv.Uint()), nil, nil
		case reflect.Uint32:
			return uint32(rv.Uint()), nil, nil
		case reflect.Uint64:
			return rv.Uint(), nil, nil
		case reflect.Float32:
			return float32(rv.Float()), nil, nil
		case reflect.Float64:
			return rv.Float(), nil, nil
		case reflect.Ptr, reflect.Interface:
			// Dereference non-nil pointers and interfaces.
			if rv.IsNil() {
				return nil, nil, nil
			}
			rv = rv.Elem()
		case reflect.Array, reflect.Slice:
			// If an array or a slice, create a new array by recursing into elements.
			var arr []interface{}
			var deps []Resource
			for i := 0; i < rv.Len(); i++ {
				elem := rv.Index(i)
				e, d, err := marshalInput(elem.Interface())
				if err != nil {
					return nil, nil, err
				}
				arr = append(arr, e)
				deps = append(deps, d...)
			}
			return arr, deps, nil
		case reflect.Map:
			// For maps, only support string-based keys, and recurse into the values.
			obj := make(map[string]interface{})
			var deps []Resource
			for _, key := range rv.MapKeys() {
				k, ok := key.Interface().(string)
				if !ok {
					return nil, nil,
						errors.Errorf("expected map keys to be strings; got %v", reflect.TypeOf(key.Interface()))
				}
				value := rv.MapIndex(key)
				mv, d, err := marshalInput(value.Interface())
				if err != nil {
					return nil, nil, err
				}

				obj[k] = mv
				deps = append(deps, d...)
			}
			return obj, deps, nil
		case reflect.String:
			return rv.String(), nil, nil
		default:
			return nil, nil, errors.Errorf("unrecognized input property type: %v (%T)", v, v)
		}
		v = rv.Interface()
	}

}

func marshalInputOutput(out *Output) (interface{}, []Resource, error) {
	// Await the value and return its raw value.
	ov, known, err := out.s.await(context.TODO())
	if err != nil {
		return nil, nil, err
	}

	// If the value is known, marshal it.
	if known {
		e, d, merr := marshalInput(ov)
		if merr != nil {
			return nil, nil, merr
		}
		return e, append(out.s.deps, d...), nil
	}

	// Otherwise, simply return the unknown value sentinel.
	return rpcTokenUnknownValue, out.s.deps, nil
}

// unmarshalOutputs unmarshals all the outputs into a simple map.
func unmarshalOutputs(outs *structpb.Struct) (map[string]interface{}, error) {
	outprops, err := plugin.UnmarshalProperties(outs, plugin.MarshalOptions{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for k, v := range outprops.Mappable() {
		result[k], err = unmarshalOutput(v)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// unmarshalOutput unmarshals a single output variable into its runtime representation.  For the most part, this just
// returns the raw value.  In a small number of cases, we need to change a type.
func unmarshalOutput(v interface{}) (interface{}, error) {
	// Check for nils and unknowns.
	if v == nil || v == rpcTokenUnknownValue {
		return nil, nil
	}

	// In the case of assets and archives, turn these into real asset and archive structures.
	if m, ok := v.(map[string]interface{}); ok {
		if sig, hasSig := m[rpcTokenSpecialSigKey]; hasSig {
			switch sig {
			case rpcTokenSpecialAssetSig:
				if path := m["path"]; path != nil {
					return asset.NewFileAsset(cast.ToString(path)), nil
				} else if text := m["text"]; text != nil {
					return asset.NewStringAsset(cast.ToString(text)), nil
				} else if uri := m["uri"]; uri != nil {
					return asset.NewRemoteAsset(cast.ToString(uri)), nil
				}
				return nil, errors.New("expected asset to be one of File, String, or Remote; got none")
			case rpcTokenSpecialArchiveSig:
				if assets := m["assets"]; assets != nil {
					as := make(map[string]interface{})
					for k, v := range assets.(map[string]interface{}) {
						a, err := unmarshalOutput(v)
						if err != nil {
							return nil, err
						}
						as[k] = a
					}
					return asset.NewAssetArchive(as), nil
				} else if path := m["path"]; path != nil {
					return asset.NewFileArchive(cast.ToString(path)), nil
				} else if uri := m["uri"]; uri != nil {
					return asset.NewRemoteArchive(cast.ToString(uri)), nil
				}
				return nil, errors.New("expected asset to be one of File, String, or Remote; got none")
			case rpcTokenSpecialSecretSig:
				return nil, errors.New("this version of the Pulumi SDK does not support first-class secrets")
			default:
				return nil, errors.Errorf("unrecognized signature '%v' in output value", sig)
			}
		}
	}

	// For arrays and maps, just make sure to transform them deeply.
	rv := reflect.ValueOf(v)
	switch rk := rv.Type().Kind(); rk {
	case reflect.Array, reflect.Slice:
		// If an array or a slice, create a new array by recursing into elements.
		var arr []interface{}
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i)
			e, err := unmarshalOutput(elem.Interface())
			if err != nil {
				return nil, err
			}
			arr = append(arr, e)
		}
		return arr, nil
	case reflect.Map:
		// For maps, only support string-based keys, and recurse into the values.
		obj := make(map[string]interface{})
		for _, key := range rv.MapKeys() {
			k, ok := key.Interface().(string)
			if !ok {
				return nil, errors.Errorf("expected map keys to be strings; got %v", reflect.TypeOf(key.Interface()))
			}
			value := rv.MapIndex(key)
			mv, err := unmarshalOutput(value)
			if err != nil {
				return nil, err
			}

			obj[k] = mv
		}
		return obj, nil
	}

	return v, nil
}
