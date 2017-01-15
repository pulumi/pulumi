// Copyright 2016 Marapongo, Inc. All rights reserved.

package encoding

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/marapongo/mu/pkg/util/contract"
)

type object map[string]interface{}
type array []interface{}

func noExcess(tree object, ty string, fields ...string) error {
	m := make(map[string]bool)
	for _, f := range fields {
		m[f] = true
	}
	for k := range tree {
		if !m[k] {
			return fmt.Errorf("Unrecognized %v field `%v`", ty, k)
		}
	}
	return nil
}

func errMissing(ty reflect.Type, field string) error {
	return fmt.Errorf("Missing required %v field `%v`", ty, field)
}

func errWrongType(ty reflect.Type, field string, expect reflect.Type, actual reflect.Type) error {
	return fmt.Errorf("%v `%v` must be a `%v`, got `%v`", ty, field, expect, actual)
}

// adjustValue converts if possible to produce the target type.
func adjustValue(val reflect.Value, to reflect.Type) (reflect.Value, error) {
	for !val.Type().AssignableTo(to) {
		var emptyObject map[string]interface{}
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
		} else if val.Kind() == reflect.Interface && to.Kind() != reflect.Interface {
			// It could be that the source is an interface{} with the right element type (or the right element type
			// through a series of successive conversions); go ahead and give it a try.
			val = val.Elem()
		} else if (val.Type() == reflect.TypeOf(object{}) || val.Type() == reflect.TypeOf(emptyObject)) &&
			(to.Kind() == reflect.Struct || (to.Kind() == reflect.Ptr && to.Elem().Kind() == reflect.Struct)) {
			// If we're assigning to a custom struct, and the source is a JSON object, try to map it.
			var tree map[string]interface{}
			mi := val.Interface()
			if ma, ok := mi.(object); ok {
				tree = ma
			} else {
				tree = mi.(map[string]interface{})
			}
			var target interface{}
			if to.Kind() == reflect.Ptr {
				target = reflect.New(to.Elem()).Interface()
			} else {
				target = reflect.New(to).Interface()
			}
			if err := decode(tree, target); err != nil {
				return val, err
			}
			val = reflect.ValueOf(target).Elem()
		} else {
			break
		}
	}
	return val, nil
}

// decodeField decodes primitive fields.  For fields of complex types, we use custom deserialization.
func decodeField(tree object, ty reflect.Type, key string, target interface{}, optional bool) error {
	vdst := reflect.ValueOf(target)
	contract.AssertMF(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", vdst.Type())
	if v, has := tree[key]; has {
		// The field exists; okay, try to map it to the right type.
		vsrc := reflect.ValueOf(v)
		vdstType := vdst.Type().Elem()

		// So long as the target element is a pointer, we have a pointer to pointer; keep digging through until we
		// bottom out on the non-pointer type that matches the source.  This assumes the source isn't itself a pointer!
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
		var err error
		if vsrc, err = adjustValue(vsrc, vdstType); err != nil {
			return err
		}

		// If the source is an interface, convert it to its element type.
		if !vsrc.Type().AssignableTo(vdstType) && vsrc.Type().Kind() == reflect.Interface {
			vsrc = vsrc.Elem()
		}

		if !vsrc.Type().AssignableTo(vdstType) {
			// If the source and destination types don't match, after depointerizing the type above, something's up.
			if vsrc.Type().Kind() == vdstType.Kind() {
				switch vsrc.Type().Kind() {
				case reflect.Slice:
					// If a slice, everything's ok so long as the elements are compatible.
					arr := reflect.New(vdstType).Elem()
					for i := 0; i < vsrc.Len(); i++ {
						elem := vsrc.Index(i)
						if !elem.Type().AssignableTo(vdstType.Elem()) {
							if elem, err = adjustValue(elem, vdstType.Elem()); err != nil {
								return err
							}
							if !elem.Type().AssignableTo(vdstType.Elem()) {
								return errWrongType(ty, fmt.Sprintf("%v[%v]", key, i), vdstType.Elem(), elem.Type())
							}
						}
						arr = reflect.Append(arr, elem)
					}
					vsrc = arr
				case reflect.Map:
					// Similarly, if a map, everything's ok so long as elements and keys are compatible.
					m := reflect.MakeMap(vdstType)
					for _, k := range vsrc.MapKeys() {
						val := vsrc.MapIndex(k)
						if !k.Type().AssignableTo(vdstType.Key()) {
							if k, err = adjustValue(k, vdstType.Key()); err != nil {
								return err
							}
							if !k.Type().AssignableTo(vdstType.Key()) {
								return errWrongType(ty,
									fmt.Sprintf("%v[%v] key", key, k.Interface()), vdstType.Key(), k.Type())
							}
						}
						if !val.Type().AssignableTo(vdstType.Elem()) {
							if val, err = adjustValue(val, vdstType.Elem()); err != nil {
								return err
							}
							if !val.Type().AssignableTo(vdstType.Elem()) {
								return errWrongType(ty,
									fmt.Sprintf("%v[%v] value", key, k.Interface()), vdstType.Elem(), val.Type())
							}
						}
						m.SetMapIndex(k, val)
					}
					vsrc = m
				}
			}
		}

		// Finally, provided everything is kosher, go ahead and store the value; otherwise, issue an error.
		if vsrc.Type().AssignableTo(vdstType) {
			vdst.Elem().Set(vsrc)
		} else {
			return errWrongType(ty, key, vdstType, vsrc.Type())
		}
	} else if !optional {
		// The field doesn't exist and yet it is required; issue an error.
		return errMissing(ty, key)
	}
	return nil
}

// decode decodes an entire map into a target object, using tag-directed mappings.
func decode(tree object, target interface{}) error {
	vdst := reflect.ValueOf(target)
	contract.AssertMF(vdst.Kind() == reflect.Ptr && !vdst.IsNil() && vdst.Elem().CanSet(),
		"Target %v must be a non-nil, settable pointer", vdst.Type())
	vdstType := vdst.Type().Elem()
	contract.AssertMF(vdstType.Kind() == reflect.Struct && !vdst.IsNil(),
		"Target %v must be a struct type with `json:\"x\"` tags to direct decoding", vdstType)

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
			var custom bool   // true if custom marshaling is used.

			// Decode the tag.
			tagparts := strings.Split(tag, ",")
			contract.AssertMF(len(tagparts) > 0,
				"Expected >0 tagparts on field %v.%v; got %v", vdstType.Name(), fldinfo.Name, len(tagparts))
			key = tagparts[0]
			for i := 1; i < len(tagparts); i++ {
				switch tagparts[i] {
				case "omitempty":
					optional = true
				case "custom":
					custom = true
				default:
					contract.FailMF("Unrecognized tagpart on field %v.%v: %v", vdstType.Name(), fldinfo.Name, tagparts[i])
				}
			}

			// Now use the tag to direct unmarshaling.
			fld := vdst.Elem().FieldByName(fldinfo.Name)
			if !custom {
				if err := decodeField(tree, vdstType, key, fld.Addr().Interface(), optional); err != nil {
					return err
				}
			}

			// Remember this key so we can be sure not to reject it later when checking for unrecognized fields.
			flds[key] = true
		}
	}

	// Afterwards, if there are any unrecognized fields, issue an error.
	for k := range tree {
		if !flds[k] {
			return fmt.Errorf("Unrecognized %v field `%v`", vdstType.Name(), k)
		}
	}

	return nil
}
