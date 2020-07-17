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

package mapper

import (
	"reflect"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// Encode encodes a strongly typed struct into a weakly typed JSON-like property bag.
func (md *mapper) Encode(source interface{}) (map[string]interface{}, MappingError) {
	// Fetch the type and value; if it's a pointer, do a quick nil check, otherwise operate on its underlying type.
	vsrc := reflect.ValueOf(source)
	vsrcType := vsrc.Type()
	if vsrcType.Kind() == reflect.Ptr {
		if vsrc.IsNil() {
			return nil, nil
		}
		vsrc = vsrc.Elem()
		vsrcType = vsrc.Type()
	}
	contract.Assertf(vsrcType.Kind() == reflect.Struct,
		"Source %v must be a struct type with `pulumi:\"x\"` tags to direct encoding (kind %v)",
		vsrcType, vsrcType.Kind())

	// Fetch the source type, allocate a fresh object, and start encoding into it.
	var errs []error
	obj := make(map[string]interface{})
	for _, fldtag := range md.structFieldsTags(vsrc.Type()) {
		if !fldtag.Skip {
			key := fldtag.Key
			fld := vsrc.FieldByName(fldtag.Info.Name)
			v, err := md.EncodeValue(fld.Interface())
			if err != nil {
				errs = append(errs, err.Failures()...)
			} else if v == nil {
				if !fldtag.Optional && !md.opts.IgnoreMissing {
					// The field doesn't exist and yet it is required; issue an error.
					errs = append(errs, NewMissingError(vsrcType, key))
				}
			} else {
				obj[key] = v
			}
		}
	}

	// If there are no errors, return nil; else manufacture a decode error object.
	var err MappingError
	if len(errs) > 0 {
		err = NewMappingError(errs)
	}
	return obj, err
}

// EncodeValue decodes primitive type fields.  For fields of complex types, we use custom deserialization.
func (md *mapper) EncodeValue(v interface{}) (interface{}, MappingError) {
	vsrc := reflect.ValueOf(v)
	contract.Assert(vsrc.IsValid())

	// Otherwise, try to map to the closest JSON-like destination type we can.
	switch k := vsrc.Kind(); k {
	// Primitive types:
	case reflect.Bool:
		return vsrc.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(vsrc.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(vsrc.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return vsrc.Float(), nil
	case reflect.String:
		return vsrc.String(), nil

	// Pointers:
	case reflect.Ptr:
		if vsrc.IsNil() {
			return nil, nil
		}
		return md.EncodeValue(vsrc.Elem().Interface())

	// Slices and maps:
	case reflect.Slice:
		var slice []interface{}
		var errs []error
		for i := 0; i < vsrc.Len(); i++ {
			ev := vsrc.Index(i).Interface()
			if elem, err := md.EncodeValue(ev); err != nil {
				errs = append(errs, err.Failures()...)
			} else {
				slice = append(slice, elem)
			}
		}
		if errs == nil {
			return slice, nil
		}
		return nil, NewMappingError(errs)
	case reflect.Map:
		keys := vsrc.MapKeys()
		mmap := make(map[string]interface{})
		var errs []error
		for _, key := range keys {
			contract.Assert(key.Kind() == reflect.String)
			if val, err := md.EncodeValue(vsrc.MapIndex(key).Interface()); err != nil {
				errs = append(errs, err.Failures()...)
			} else {
				mmap[key.String()] = val
			}
		}
		if errs == nil {
			return mmap, nil
		}
		return nil, NewMappingError(errs)

	// Structs and interface{}:
	case reflect.Struct:
		return md.Encode(vsrc.Interface())
	case reflect.Interface:
		return md.EncodeValue(vsrc.Elem().Interface())
	default:
		contract.Failf("Unrecognized field type '%v' during encoding", k)
	}

	return nil, nil
}
