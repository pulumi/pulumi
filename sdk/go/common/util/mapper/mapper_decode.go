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
	"encoding"
	"fmt"
	"reflect"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Decoder is a func that knows how to decode into particular type.
type Decoder func(m Mapper, obj map[string]interface{}) (interface{}, error)

// Decoders is a map from type to a decoder func that understands how to decode that type.
type Decoders map[reflect.Type]Decoder

// Decode decodes an entire map into a target object, using tag-directed mappings.
func (md *mapper) Decode(obj map[string]interface{}, target interface{}) MappingError {
	// Fetch the destination types and validate that we can store into the target (i.e., a valid lval).
	vdst := reflect.ValueOf(target)
	contract.Assertf(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", vdst.Type())
	vdstType := vdst.Type().Elem()
	contract.Assertf(vdstType.Kind() == reflect.Struct && !vdst.IsNil(),
		"Target %v must be a struct type with `pulumi:\"x\"` tags to direct decoding", vdstType)

	// Keep track of any errors that result.
	var errs []error

	// For each field in the struct that has a `pulumi:"name"`, look it up in the map by that `name`, issuing an error
	// if it is missing or of the wrong type.  For each field that is marked optional, e.g. `pulumi:"name,optional"`,
	// do the same, but permit it to be missing without issuing an error.
	flds := make(map[string]bool)
	for _, fldtag := range md.structFieldsTags(vdstType) {
		// Use the tag to direct unmarshaling.
		key := fldtag.Key
		if !fldtag.Skip {
			fld := vdst.Elem().FieldByName(fldtag.Info.Name)
			if err := md.DecodeValue(obj, vdstType, key, fld.Addr().Interface(), fldtag.Optional); err != nil {
				errs = append(errs, err)
			}
		}

		// Remember this key so we can be sure not to reject it later when checking for unrecognized fields.
		flds[key] = true
	}

	// Afterwards, if there are any unrecognized fields, issue an error.
	if !md.opts.IgnoreUnrecognized {
		for k := range obj {
			if !flds[k] {
				err := NewUnrecognizedError(vdstType, k)
				errs = append(errs, err)
			}
		}
	}

	// If there are no errors, return nil; else manufacture a decode error object.
	if len(errs) == 0 {
		return nil
	}

	return NewMappingError(errs)
}

// DecodeValue decodes primitive type fields.  For fields of complex types, we use custom deserialization.
func (md *mapper) DecodeValue(obj map[string]interface{}, ty reflect.Type, key string,
	target interface{}, optional bool) FieldError {
	vdst := reflect.ValueOf(target)
	contract.Assertf(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", vdst.Type())
	if v, has := obj[key]; has {
		// The field exists; okay, try to map it to the right type.
		vsrc := reflect.ValueOf(v)
		// Ensure the source is valid; this is false if the value reflects the zero value.
		if vsrc.IsValid() {
			vdstType := vdst.Type().Elem()

			// So long as the target element is a pointer, we have a pointer to pointer; dig through until we bottom out
			// on the non-pointer type that matches the source.  This assumes the source isn't itself a pointer!
			contract.Assert(vsrc.Type().Kind() != reflect.Ptr)
			for vdstType.Kind() == reflect.Ptr {
				vdst = vdst.Elem()
				vdstType = vdstType.Elem()
				if !vdst.Elem().CanSet() {
					// If the pointer is nil, initialize it so we can set it below.
					contract.Assert(vdst.IsNil())
					vdst.Set(reflect.New(vdstType))
				}
			}

			// Adjust the value if necessary; this handles recursive struct marshaling, interface unboxing, and more.
			var err FieldError
			if vsrc, err = md.adjustValueForAssignment(vsrc, vdstType, ty, key); err != nil {
				return err
			}

			// Finally, provided everything is kosher, go ahead and store the value; otherwise, issue an error.
			if vsrc.Type().AssignableTo(vdstType) {
				vdst.Elem().Set(vsrc)
				return nil
			}

			return NewWrongTypeError(ty, key, vdstType, vsrc.Type())
		}
	}

	if !optional && !md.opts.IgnoreMissing {
		// The field doesn't exist and yet it is required; issue an error.
		return NewMissingError(ty, key)
	}

	return nil
}

var (
	emptyObject         = map[string]interface{}{}
	textUnmarshalerType = reflect.TypeOf(new(encoding.TextUnmarshaler)).Elem()
)

// adjustValueForAssignment converts if possible to produce the target type.
func (md *mapper) adjustValueForAssignment(val reflect.Value,
	to reflect.Type, ty reflect.Type, key string) (reflect.Value, FieldError) {
	for !val.Type().AssignableTo(to) {
		// The source cannot be assigned directly to the destination.  Go through all known conversions.

		if val.Type().ConvertibleTo(to) {
			// A simple conversion exists to make this right.
			val = val.Convert(to)
		} else if to.Kind() == reflect.Ptr && val.Type().AssignableTo(to.Elem()) {
			// If the target is a pointer, turn the target into a pointer.  If it's not addressable, make a copy.
			if val.CanAddr() {
				val = val.Addr()
			} else {
				slot := reflect.New(val.Type().Elem())
				copy := reflect.ValueOf(val.Interface())
				contract.Assert(copy.CanAddr())
				slot.Set(copy)
				val = slot
			}
		} else if val.Kind() == reflect.Interface {
			// It could be that the source is an interface{} with the right element type (or the right element type
			// through a series of successive conversions); go ahead and give it a try.
			val = val.Elem()
		} else if val.Type().Kind() == reflect.Slice && to.Kind() == reflect.Slice {
			// If a slice, everything's ok so long as the elements are compatible.
			arr := reflect.New(to).Elem()
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				if !elem.Type().AssignableTo(to.Elem()) {
					ekey := fmt.Sprintf("%v[%v]", key, i)
					var err FieldError
					if elem, err = md.adjustValueForAssignment(elem, to.Elem(), ty, ekey); err != nil {
						return val, err
					}
					if !elem.Type().AssignableTo(to.Elem()) {
						return val, NewWrongTypeError(ty, ekey, to.Elem(), elem.Type())
					}
				}
				arr = reflect.Append(arr, elem)
			}
			val = arr
		} else if val.Type().Kind() == reflect.Map && to.Kind() == reflect.Map {
			// Similarly, if a map, everything's ok so long as elements and keys are compatible.
			m := reflect.MakeMap(to)
			for _, k := range val.MapKeys() {
				entry := val.MapIndex(k)
				if !k.Type().AssignableTo(to.Key()) {
					kkey := fmt.Sprintf("%v[%v] key", key, k.Interface())
					var err FieldError
					if k, err = md.adjustValueForAssignment(k, to.Key(), ty, kkey); err != nil {
						return val, err
					}
					if !k.Type().AssignableTo(to.Key()) {
						return val, NewWrongTypeError(ty, kkey, to.Key(), k.Type())
					}
				}
				if !entry.Type().AssignableTo(to.Elem()) {
					ekey := fmt.Sprintf("%v[%v] value", key, k.Interface())
					var err FieldError
					if entry, err = md.adjustValueForAssignment(entry, to.Elem(), ty, ekey); err != nil {
						return val, err
					}
					if !entry.Type().AssignableTo(to.Elem()) {
						return val, NewWrongTypeError(ty, ekey, to.Elem(), entry.Type())
					}
				}
				m.SetMapIndex(k, entry)
			}
			val = m
		} else if val.Type() == reflect.TypeOf(emptyObject) {
			// The value is an object and needs to be decoded into a value.
			obj := val.Interface().(map[string]interface{})
			if decode, has := md.opts.CustomDecoders[to]; has {
				// A custom decoder exists; use it to unmarshal the type.
				target, err := decode(md, obj)
				if err != nil {
					return val, NewTypeFieldError(ty, key, err)
				}
				val = reflect.ValueOf(target)
			} else if to.Kind() == reflect.Struct || (to.Kind() == reflect.Ptr && to.Elem().Kind() == reflect.Struct) {
				// If the target is a struct, we can use the built-in decoding logic.
				var target interface{}
				if to.Kind() == reflect.Ptr {
					target = reflect.New(to.Elem()).Interface()
				} else {
					target = reflect.New(to).Interface()
				}
				if err := md.Decode(obj, target); err != nil {
					return val, NewTypeFieldError(ty, key, err)
				}
				val = reflect.ValueOf(target).Elem()
			} else {
				return val, NewTypeFieldError(ty, key,
					errors.Errorf(
						"Cannot decode Object{} to type %v; it isn't a struct, and no custom decoder exists", to))
			}
		} else if val.Type().Kind() == reflect.String {
			// If the source is a string, see if the target implements encoding.TextUnmarshaler.
			target := reflect.New(to)
			if target.Type().Implements(textUnmarshalerType) {
				um := target.Interface().(encoding.TextUnmarshaler)
				if err := um.UnmarshalText([]byte(val.String())); err != nil {
					return val, NewTypeFieldError(ty, key, err)
				}
				val = target.Elem()
			} else {
				break
			}
		} else {
			break
		}
	}
	return val, nil
}
