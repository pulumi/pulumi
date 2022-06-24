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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Encode encodes a strongly typed struct into a weakly typed JSON-like property bag.
func (md *mapper) Encode(source interface{}) (map[string]interface{}, MappingError) {
	if source == nil {
		return nil, nil
	}
	return md.encode(reflect.ValueOf(source))
}

func (md *mapper) encode(vsrc reflect.Value) (map[string]interface{}, MappingError) {
	contract.Assert(vsrc.IsValid())

	// Fetch the type; if it's a pointer, do a quick nil check, otherwise operate on its underlying type.
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
			v, err := md.encodeValue(fld)
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
	if v == nil {
		return nil, nil
	}
	return md.encodeValue(reflect.ValueOf(v))
}

func (md *mapper) encodeValue(vsrc reflect.Value) (interface{}, MappingError) {
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
		return md.encodeValue(vsrc.Elem())

	// Slices and maps:
	case reflect.Slice:
		if vsrc.IsNil() {
			return nil, nil
		}

		slice := make([]interface{}, vsrc.Len())
		var errs []error
		for i := 0; i < vsrc.Len(); i++ {
			ev := vsrc.Index(i)
			if elem, err := md.encodeValue(ev); err != nil {
				errs = append(errs, err.Failures()...)
			} else {
				slice[i] = elem
			}
		}
		if errs == nil {
			return slice, nil
		}
		return nil, NewMappingError(errs)
	case reflect.Map:
		if vsrc.IsNil() {
			return nil, nil
		}
		contract.Assert(vsrc.Type().Key().Kind() == reflect.String)

		iter := vsrc.MapRange()
		mmap := make(map[string]interface{}, vsrc.Len())
		var errs []error
		for iter.Next() {
			if val, err := md.encodeValue(iter.Value()); err != nil {
				errs = append(errs, err.Failures()...)
			} else {
				mmap[iter.Key().String()] = val
			}
		}
		if errs == nil {
			return mmap, nil
		}
		return nil, NewMappingError(errs)

	// Structs and interface{}:
	case reflect.Struct:
		return md.encode(vsrc)
	case reflect.Interface:
		return md.encodeValue(vsrc.Elem())
	default:
		contract.Failf("Unrecognized field type '%v' during encoding", k)
	}

	return nil, nil
}
