// Copyright 2023, Pulumi Corporation.
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

package encoding

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/pulumi/esc/syntax"
)

// DecodeValue decodes a plain Go value into a syntax.Node.
//
// Decoding uses the following rules:
//
//   - Interface and pointer values decode as their element value. Nil interface and pointer values decode as
//     *syntax.NullNode.
//   - Boolean values decode as *syntax.BooleanNode.
//   - Floating point and integer values decode as *syntax.NumberNode.
//   - json.Number values decode as *syntax.NumberNode.
//   - String values decode as *syntax.StringNode.
//   - Arrays and slices decode as *syntax.ListNode. Nil slices are decoded as *syntax.NullNode.
//   - Maps are decoded as *syntax.ObjectNode. Map keys must be strings. Nil maps are deocded as *syntax.NullNode.
//   - Structs are decoded as *syntax.ObjectNode. Exported struct fields decode into object properties using the name of
//     the field as the property's key. The name of the struct field can be customized using a struct tag of the form
//     `object:"name"`. If a field's value decodes as *syntax.NullNode, that field is omitted from the result.
func DecodeValue(v interface{}) (syntax.Node, syntax.Diagnostics) {
	return decodeValue(reflect.ValueOf(v))
}

func decodeValue(v reflect.Value) (syntax.Node, syntax.Diagnostics) {
	if !v.IsValid() {
		return syntax.Null(), nil
	}

	for {
		switch v.Kind() {
		case reflect.Interface, reflect.Ptr:
			if v.IsNil() {
				return syntax.Null(), nil
			}
			v = v.Elem()
		case reflect.Bool:
			return syntax.Boolean(v.Bool()), nil
		case reflect.Float32, reflect.Float64:
			return syntax.Number(v.Float()), nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return syntax.Number(v.Int()), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return syntax.Number(v.Uint()), nil
		case reflect.String:
			if v.Type() == jsonNumberType {
				return syntax.Number(json.Number(v.String())), nil
			}
			return syntax.String(v.String()), nil
		case reflect.Array, reflect.Slice:
			if v.IsNil() {
				return syntax.Null(), nil
			}

			var elements []syntax.Node
			var diags syntax.Diagnostics
			if v.Len() != 0 {
				elements = make([]syntax.Node, v.Len())
				for i := 0; i < v.Len(); i++ {
					e, ediags := decodeValue(v.Index(i))
					diags.Extend(ediags...)

					elements[i] = e
				}
			}
			return syntax.Array(elements...), diags
		case reflect.Map:
			if v.Type().Key().Kind() != reflect.String {
				return nil, syntax.Diagnostics{syntax.Error(nil, fmt.Sprintf("cannot decode value of type %v (map keys must be strings)", v.Type()), "")}
			}

			if v.IsNil() {
				return syntax.Null(), nil
			}

			var entries []syntax.ObjectPropertyDef
			var diags syntax.Diagnostics
			if v.Len() != 0 {
				keys := make([]string, 0, v.Len())
				for iter := v.MapRange(); iter.Next(); {
					keys = append(keys, iter.Key().String())
				}
				sort.Strings(keys)

				entries = make([]syntax.ObjectPropertyDef, v.Len())
				for i, k := range keys {
					kn := syntax.String(k)

					vn, vdiags := decodeValue(v.MapIndex(reflect.ValueOf(k)))
					diags.Extend(vdiags...)

					entries[i] = syntax.ObjectProperty(kn, vn)
				}
			}
			return syntax.Object(entries...), diags
		case reflect.Struct:
			var entries []syntax.ObjectPropertyDef
			var diags syntax.Diagnostics
			if t := v.Type(); t.NumField() != 0 {
				entries = make([]syntax.ObjectPropertyDef, 0, t.NumField())

				for i := 0; i < t.NumField(); i++ {
					ft := t.Field(i)

					k := ft.Name
					if tag, ok := ft.Tag.Lookup("syntax"); ok {
						if tag == "-" {
							continue
						}
						k = tag
					}

					vn, fdiags := decodeValue(v.Field(i))
					diags.Extend(fdiags...)

					if _, isNull := vn.(*syntax.NullNode); isNull || vn == nil {
						continue
					}

					if obj, ok := vn.(*syntax.ObjectNode); ok && ft.Anonymous {
						for i := 0; i < obj.Len(); i++ {
							entries = append(entries, obj.Index(i))
						}
						continue
					}

					kn := syntax.String(k)

					entries = append(entries, syntax.ObjectProperty(kn, vn))
				}
			}
			return syntax.Object(entries...), diags
		default:
			return nil, syntax.Diagnostics{syntax.Error(nil, fmt.Sprintf("cannot decode value of type %v", v.Type()), "")}
		}
	}
}

// EncodeValue encodes a syntax.Node into a plain Go value. The encoding rules are the inverse of the decoding rules
// described in the documentation for DecodeValue.
func EncodeValue(n syntax.Node, v interface{}) syntax.Diagnostics {
	return encodeValue(n, reflect.ValueOf(v))
}

func getStructFields(fields map[string]reflect.Value, v reflect.Value) reflect.Value {
	var nodeField reflect.Value

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			nf := getStructFields(fields, v.Field(i))
			if !nodeField.IsValid() {
				nodeField = nf
			}
		} else {
			k := f.Name
			if tag, ok := f.Tag.Lookup("syntax"); ok {
				if tag == "-" {
					nodeField = v.Field(i)
					continue
				}
				k = tag
			}

			if fv := v.Field(i); fv.CanSet() {
				fields[k] = fv
			}
		}
	}

	return nodeField
}

var nodeType = reflect.TypeOf((*syntax.Node)(nil)).Elem()
var jsonNumberType = reflect.TypeOf((*json.Number)(nil)).Elem()

func encodeValue(n syntax.Node, v reflect.Value) syntax.Diagnostics {
	if _, isNull := n.(*syntax.NullNode); isNull {
		if v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
		return nil
	}

	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			if !v.CanSet() {
				return nil
			}
			el := reflect.New(v.Type().Elem())
			v.Set(el)
		}
		v = v.Elem()
	}

	if v.Type().AssignableTo(nodeType) {
		if v.CanSet() {
			nv := reflect.ValueOf(n)
			if !nv.Type().AssignableTo(v.Type()) {
				rng := n.Syntax().Range()
				return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot encode %v into location of type %v", nv.Type(), v.Type()), "")}
			}
			v.Set(nv)
			return nil
		}
	}

	if !v.CanSet() {
		return nil
	}

	switch n := n.(type) {
	case *syntax.BooleanNode:
		if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
			ev := reflect.New(reflect.TypeOf((*bool)(nil)).Elem()).Elem()
			defer v.Set(ev)
			v = ev
		}

		if v.Kind() != reflect.Bool {
			rng := n.Syntax().Range()
			return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot encode boolean into location of type %v", v.Type()), "")}
		}
		v.SetBool(n.Value())
		return nil
	case *syntax.NumberNode:
		if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
			ev := reflect.New(jsonNumberType).Elem()
			defer v.Set(ev)
			v = ev
		}

		reprError := func() syntax.Diagnostics {
			rng := n.Syntax().Range()
			return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot represent %v as type %v", n.Value(), v.Type()), "")}
		}

		parseFloat := func(bitSize int) syntax.Diagnostics {
			x, err := strconv.ParseFloat(string(n.Value()), bitSize)
			if err != nil {
				return reprError()
			}
			v.SetFloat(x)
			return nil
		}

		parseInt := func(bitSize int) syntax.Diagnostics {
			x, err := strconv.ParseInt(string(n.Value()), 10, bitSize)
			if err != nil {
				return reprError()
			}
			v.SetInt(x)
			return nil
		}

		parseUint := func(bitSize int) syntax.Diagnostics {
			x, err := strconv.ParseUint(string(n.Value()), 10, bitSize)
			if err != nil {
				return reprError()
			}
			v.SetUint(x)
			return nil
		}

		switch v.Kind() {
		case reflect.Float32:
			return parseFloat(32)
		case reflect.Float64:
			return parseFloat(64)
		case reflect.Int:
			return parseInt(64)
		case reflect.Int8:
			return parseInt(8)
		case reflect.Int16:
			return parseInt(16)
		case reflect.Int32:
			return parseInt(32)
		case reflect.Int64:
			return parseInt(64)
		case reflect.Uint:
			return parseUint(64)
		case reflect.Uint8:
			return parseUint(8)
		case reflect.Uint16:
			return parseUint(16)
		case reflect.Uint32:
			return parseUint(32)
		case reflect.Uint64:
			return parseUint(64)
		case reflect.Uintptr:
			return parseUint(64)
		default:
			if v.Type() == jsonNumberType {
				v.SetString(n.Value().String())
				return nil
			}

			rng := n.Syntax().Range()
			return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot encode number into location of type %v", v.Type()), "")}
		}
	case *syntax.StringNode:
		if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
			ev := reflect.New(reflect.TypeOf((*string)(nil)).Elem()).Elem()
			defer v.Set(ev)
			v = ev
		}

		if v.Kind() != reflect.String {
			rng := n.Syntax().Range()
			return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot encode string into location of type %v", v.Type()), "")}
		}
		v.SetString(n.Value())
		return nil
	case *syntax.ArrayNode:
		if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
			ev := reflect.New(reflect.TypeOf((*[]interface{})(nil)).Elem()).Elem()
			defer v.Set(ev)
			v = ev
		}

		switch v.Kind() {
		case reflect.Array:
			// OK
		case reflect.Slice:
			v.Set(reflect.MakeSlice(v.Type(), n.Len(), n.Len()))
		default:
			rng := n.Syntax().Range()
			return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot encode list into location of type %v", v.Type()), "")}
		}

		l := n.Len()
		if v.Len() < l {
			l = v.Len()
		}

		var diags syntax.Diagnostics
		for i := 0; i < l; i++ {
			ediags := encodeValue(n.Index(i), v.Index(i))
			diags.Extend(ediags...)
		}
		return diags
	case *syntax.ObjectNode:
		if v.Kind() == reflect.Interface && v.NumMethod() == 0 {
			ev := reflect.New(reflect.TypeOf((*map[string]interface{})(nil)).Elem()).Elem()
			defer v.Set(ev)
			v = ev
		}

		switch v.Kind() {
		case reflect.Map:
			t := v.Type()

			v.Set(reflect.MakeMap(t))

			var diags syntax.Diagnostics
			for i := 0; i < n.Len(); i++ {
				kvp := n.Index(i)

				var key string
				kdiags := EncodeValue(kvp.Key, &key)
				diags.Extend(kdiags...)
				if len(kdiags) != 0 {
					continue
				}

				value := reflect.New(t.Elem()).Elem()
				vdiags := encodeValue(kvp.Value, value)
				diags.Extend(vdiags...)
				if len(vdiags) != 0 {
					continue
				}

				v.SetMapIndex(reflect.ValueOf(key), value)
			}
			return diags
		case reflect.Struct:
			fields := map[string]reflect.Value{}
			nodeField := getStructFields(fields, v)
			if nodeField.IsValid() && nodeField.CanSet() && nodeField.Type() == nodeType {
				nodeField.Set(reflect.ValueOf(n))
			}

			var diags syntax.Diagnostics
			for i := 0; i < n.Len(); i++ {
				kvp := n.Index(i)

				var key string
				kdiags := EncodeValue(kvp.Key, &key)
				diags.Extend(kdiags...)
				if len(kdiags) != 0 {
					continue
				}

				field, ok := fields[key]
				if !ok {
					continue
				}

				value := reflect.New(field.Type()).Elem()
				vdiags := encodeValue(kvp.Value, value)
				diags.Extend(vdiags...)
				if len(vdiags) != 0 {
					continue
				}

				field.Set(value)
			}
			return diags
		default:
			syn := n.Syntax()
			rng := syn.Range()
			path := syn.Path()
			return syntax.Diagnostics{syntax.Error(rng, fmt.Sprintf("cannot encode object into location of type %v", v.Type()), path)}
		}
	default:
		panic("unreachable")
	}
}
