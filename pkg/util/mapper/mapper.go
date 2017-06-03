// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/lumi/pkg/util/contract"
)

type Mapper interface {
	// Decode decodes a JSON-like object into the target pointer to a structure.
	Decode(tree Object, target interface{}) DecodeError
	// DecodeField decodes a single JSON-like value (with a given type and name) into a target pointer to a structure.
	DecodeField(tree Object, ty reflect.Type, key string, target interface{}, optional bool) FieldError
	// Encode encodes an object into a JSON-like in-memory object.
	Encode(obj interface{}) Object
}

func New(opts *Opts) Mapper {
	var initOpts Opts
	if opts != nil {
		initOpts = *opts
	}
	if initOpts.CustomDecoders == nil {
		initOpts.CustomDecoders = make(Decoders)
	}
	return &mapper{opts: initOpts}
}

type Opts struct {
	CustomDecoders     Decoders // custom decoders.
	IgnoreMissing      bool     // ignore missing required fields.
	IgnoreUnrecognized bool     // ignore unrecognized fields.
}

type mapper struct {
	opts Opts
}

// Decoder is a func that knows how to decode into particular type.
type Decoder func(m Mapper, tree Object) (interface{}, error)

// Decoders is a map from type to a decoder func that understands how to decode that type.
type Decoders map[reflect.Type]Decoder

// Decode decodes an entire map into a target object, using tag-directed mappings.
func (md *mapper) Decode(tree Object, target interface{}) DecodeError {
	// Fetch the destination types and validate that we can store into the target (i.e., a valid lval).
	vdst := reflect.ValueOf(target)
	contract.Assertf(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", vdst.Type())
	vdstType := vdst.Type().Elem()
	contract.Assertf(vdstType.Kind() == reflect.Struct && !vdst.IsNil(),
		"Target %v must be a struct type with `json:\"x\"` tags to direct decoding", vdstType)

	// Keep track of any errors that result.
	var errs []FieldError

	// For each field in the struct that has a `json:"name"`, look it up in the map by that `name`, issuing an error if
	// it is missing or of the wrong type.  For each field that has a tag with omitempty specified, i.e.,
	// `json:"name,omitempty"`, do the same, but permit it to be missing without issuing an error.

	// We need to pass over the struct first to build up the set of infos, so we can dig into embedded structs.
	fldtypes := []reflect.Type{vdstType}
	var fldinfos []reflect.StructField
	for len(fldtypes) > 0 {
		fldtype := fldtypes[0]
		fldtypes = fldtypes[1:]
		for i := 0; i < fldtype.NumField(); i++ {
			fldinfo := fldtype.Field(i)
			if fldinfo.Anonymous {
				// If an embedded struct, push it onto the queue to visit.
				fldtypes = append(fldtypes, fldinfo.Type)
			} else {
				// Otherwise, we will go ahead and consider this field in our decoding.
				fldinfos = append(fldinfos, fldinfo)
			}
		}
	}

	// Now go through, read and parse the "json" tags, and actually perform the decoding for each one.
	flds := make(map[string]bool)
	for _, fldinfo := range fldinfos {
		if tag := fldinfo.Tag.Get("json"); tag != "" {
			var key string    // the JSON key name.
			var optional bool // true if this can be missing.
			var skip bool     // true if we should skip auto-marshaling.

			// Decode the tag.
			tagparts := strings.Split(tag, ",")
			contract.Assertf(len(tagparts) > 0,
				"Expected >0 tagparts on field %v.%v; got %v", vdstType.Name(), fldinfo.Name, len(tagparts))
			key = tagparts[0]
			if key == "-" {
				skip = true // a name of "-" means skip
			}
			for i := 1; i < len(tagparts); i++ {
				switch tagparts[i] {
				case "omitempty":
					optional = true
				case "skip":
					skip = true
				default:
					contract.Failf("Unrecognized tagpart on field %v.%v: %v", vdstType.Name(), fldinfo.Name, tagparts[i])
				}
			}

			// Now use the tag to direct unmarshaling.
			fld := vdst.Elem().FieldByName(fldinfo.Name)
			if !skip {
				if err := md.DecodeField(tree, vdstType, key, fld.Addr().Interface(), optional); err != nil {
					errs = append(errs, err)
				}
			}

			// Remember this key so we can be sure not to reject it later when checking for unrecognized fields.
			flds[key] = true
		}
	}

	// Afterwards, if there are any unrecognized fields, issue an error.
	if !md.opts.IgnoreUnrecognized {
		for k := range tree {
			if !flds[k] {
				err := NewUnrecognizedErr(vdstType, k)
				errs = append(errs, err)
			}
		}
	}

	// If there are no errors, return nil; else manufacture a decode error object.
	if len(errs) == 0 {
		return nil
	}

	return NewDecodeErr(errs)
}

// DecodeField decodes primitive type fields.  For fields of complex types, we use custom deserialization.
func (md *mapper) DecodeField(tree Object, ty reflect.Type, key string,
	target interface{}, optional bool) FieldError {
	vdst := reflect.ValueOf(target)
	contract.Assertf(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", vdst.Type())
	if v, has := tree[key]; has {
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
			if vsrc, err = md.adjustValue(vsrc, vdstType, ty, key); err != nil {
				return err
			}

			// Finally, provided everything is kosher, go ahead and store the value; otherwise, issue an error.
			if vsrc.Type().AssignableTo(vdstType) {
				vdst.Elem().Set(vsrc)
				return nil
			}

			return NewWrongTypeErr(ty, key, vdstType, vsrc.Type())
		}
	}

	if !optional && !md.opts.IgnoreMissing {
		// The field doesn't exist and yet it is required; issue an error.
		return NewMissingErr(ty, key)
	}

	return nil
}

var emptyObject map[string]interface{}
var textUnmarshalerType = reflect.TypeOf(new(encoding.TextUnmarshaler)).Elem()

// adjustValue converts if possible to produce the target type.
func (md *mapper) adjustValue(val reflect.Value,
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
					if elem, err = md.adjustValue(elem, to.Elem(), ty, ekey); err != nil {
						return val, err
					}
					if !elem.Type().AssignableTo(to.Elem()) {
						return val, NewWrongTypeErr(ty, ekey, to.Elem(), elem.Type())
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
					if k, err = md.adjustValue(k, to.Key(), ty, kkey); err != nil {
						return val, err
					}
					if !k.Type().AssignableTo(to.Key()) {
						return val, NewWrongTypeErr(ty, kkey, to.Key(), k.Type())
					}
				}
				if !entry.Type().AssignableTo(to.Elem()) {
					ekey := fmt.Sprintf("%v[%v] value", key, k.Interface())
					var err FieldError
					if entry, err = md.adjustValue(entry, to.Elem(), ty, ekey); err != nil {
						return val, err
					}
					if !entry.Type().AssignableTo(to.Elem()) {
						return val, NewWrongTypeErr(ty, ekey, to.Elem(), entry.Type())
					}
				}
				m.SetMapIndex(k, entry)
			}
			val = m
		} else if val.Type() == reflect.TypeOf(Object{}) || val.Type() == reflect.TypeOf(emptyObject) {
			// The value is an object and needs to be decoded into a value.
			var tree map[string]interface{}
			mi := val.Interface()
			if ma, ok := mi.(Object); ok {
				tree = ma
			} else {
				tree = mi.(map[string]interface{})
			}

			if decode, has := md.opts.CustomDecoders[to]; has {
				// A custom decoder exists; use it to unmarshal the type.
				target, err := decode(md, tree)
				if err != nil {
					return val, NewFieldErr(ty, key, err)
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
				if err := md.Decode(tree, target); err != nil {
					return val, NewFieldErr(ty, key, err)
				}
				val = reflect.ValueOf(target).Elem()
			} else {
				return val, NewFieldErr(ty, key,
					errors.Errorf(
						"Cannot decode Object{} to type %v; it isn't a struct, and no custom decoder exists", to))
			}
		} else if val.Type().Kind() == reflect.String {
			// If the source is a string, see if the target implements encoding.TextUnmarshaler.
			target := reflect.New(to)
			if target.Type().Implements(textUnmarshalerType) {
				um := target.Interface().(encoding.TextUnmarshaler)
				if err := um.UnmarshalText([]byte(val.String())); err != nil {
					return val, NewFieldErr(ty, key, err)
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

func (md *mapper) Encode(obj interface{}) Object {
	// TODO[pulumi/lumi#183]: implement this more efficiently.
	data, err := json.Marshal(obj)
	contract.Assert(err == nil)
	var result Object
	err = json.Unmarshal(data, &result)
	return result
}

// Map decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.
func Map(tree Object, target interface{}) DecodeError {
	return New(nil).Decode(tree, target)
}

// Unmap translates an already mapped target object into a raw, unmapped form.
func Unmap(obj interface{}) Object {
	return New(nil).Encode(obj)
}

// MapI decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any missing required fields in the payload in addition to any unrecognized fields.
func MapI(tree Object, target interface{}) DecodeError {
	return New(&Opts{
		IgnoreMissing:      true,
		IgnoreUnrecognized: true,
	}).Decode(tree, target)
}

// MapIM decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any missing required fields in the payload.
func MapIM(tree Object, target interface{}) DecodeError {
	return New(&Opts{IgnoreMissing: true}).Decode(tree, target)
}

// MapIU decodes an entire map into a target object, using an anonymous decoder and tag-directed mappings.  This variant
// ignores any unrecognized fields in the payload.
func MapIU(tree Object, target interface{}) DecodeError {
	return New(&Opts{IgnoreUnrecognized: true}).Decode(tree, target)
}
