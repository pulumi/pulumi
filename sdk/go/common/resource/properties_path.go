// Copyright 2019-2024, Pulumi Corporation.
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

package resource

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// PropertyPath represents a path to a nested property. The path may be composed of strings (which access properties
// in ObjectProperty values) and integers (which access elements of ArrayProperty values).
type PropertyPath []interface{}

// ParsePropertyPath parses a property path into a PropertyPath value.
//
// A property path string is essentially a Javascript property access expression in which all elements are literals.
// Valid property paths obey the following EBNF-ish grammar:
//
//	propertyName := [a-zA-Z_$] { [a-zA-Z0-9_$] }
//	quotedPropertyName := '"' ( '\' '"' | [^"] ) { ( '\' '"' | [^"] ) } '"'
//	arrayIndex := { [0-9] }
//
//	propertyIndex := '[' ( quotedPropertyName | arrayIndex ) ']'
//	rootProperty := ( propertyName | propertyIndex )
//	propertyAccessor := ( ( '.' propertyName ) |  propertyIndex )
//	path := rootProperty { propertyAccessor }
//
// Examples of valid paths:
// - root
// - root.nested
// - root["nested"]
// - root.double.nest
// - root["double"].nest
// - root["double"]["nest"]
// - root.array[0]
// - root.array[100]
// - root.array[0].nested
// - root.array[0][1].nested
// - root.nested.array[0].double[1]
// - root["key with \"escaped\" quotes"]
// - root["key with a ."]
// - ["root key with \"escaped\" quotes"].nested
// - ["root key with a ."][100]
// - root.array[*].field
// - root.array["*"].field
func parsePropertyPath(path string, strict bool) (PropertyPath, error) {
	// We interpret the grammar above a little loosely in order to keep things simple. Specifically, we will accept
	// something close to the following:
	// pathElement := { '.' } [a-zA-Z_$][a-zA-Z0-9_$]
	// pathIndex := '[' ( [0-9]+ | '"' ('\' '"' | [^"] )+ '"' ']'
	// path := { pathElement | pathIndex }
	var elements []interface{}
	if len(path) > 0 && path[0] == '.' {
		return nil, errors.New("expected property path to start with a name or index")
	}
	for len(path) > 0 {
		switch path[0] {
		case '.':
			path = path[1:]
			if len(path) == 0 {
				return nil, errors.New("expected property path to end with a name or index")
			}
			if path[0] == '[' && strict {
				return nil, errors.New("expected property name after '.'")
			} else if path[0] == '[' {
				// We tolerate a '.' followed by a '[', which is not strictly legal, but is common from old providers.
				logging.V(10).Infof("property path '%s' contains a '.' followed by a '['; this is not strictly legal", path)
			}
		case '[':
			// If the character following the '[' is a '"', parse a string key.
			var pathElement interface{}
			if len(path) > 1 && path[1] == '"' {
				var propertyKey []byte
				var i int
				for i = 2; ; {
					if i >= len(path) {
						return nil, errors.New("missing closing quote in property name")
					} else if path[i] == '"' {
						i++
						break
					} else if path[i] == '\\' && i+1 < len(path) && path[i+1] == '"' {
						propertyKey = append(propertyKey, '"')
						i += 2
					} else {
						propertyKey = append(propertyKey, path[i])
						i++
					}
				}
				if i >= len(path) || path[i] != ']' {
					return nil, errors.New("missing closing bracket in property access")
				}
				pathElement, path = string(propertyKey), path[i:]
			} else {
				// Look for a closing ']'
				rbracket := strings.IndexRune(path, ']')
				if rbracket == -1 {
					return nil, errors.New("missing closing bracket in array index")
				}

				segment := path[1:rbracket]
				if segment == "*" {
					pathElement, path = "*", path[rbracket:]
				} else {
					index, err := strconv.ParseInt(segment, 10, 0)
					if err != nil {
						return nil, fmt.Errorf("invalid array index: %w", err)
					}
					pathElement, path = int(index), path[rbracket:]
				}
			}
			elements, path = append(elements, pathElement), path[1:]
		default:
			for i := 0; ; i++ {
				if i == len(path) || path[i] == '.' || path[i] == '[' {
					elements, path = append(elements, path[:i]), path[i:]
					break
				}
			}
		}
	}
	return PropertyPath(elements), nil
}

func ParsePropertyPath(path string) (PropertyPath, error) {
	return parsePropertyPath(path, false)
}

func ParsePropertyPathStrict(path string) (PropertyPath, error) {
	return parsePropertyPath(path, true)
}

// Get attempts to get the value located by the PropertyPath inside the given PropertyValue. If any component of the
// path does not exist, this function will return (NullPropertyValue, false).
func (p PropertyPath) Get(v PropertyValue) (PropertyValue, bool) {
	for _, key := range p {
		switch {
		case v.IsArray():
			index, ok := key.(int)
			if !ok || index < 0 || index >= len(v.ArrayValue()) {
				return PropertyValue{}, false
			}
			v = v.ArrayValue()[index]
		case v.IsObject():
			k, ok := key.(string)
			if !ok {
				return PropertyValue{}, false
			}
			v, ok = v.ObjectValue()[PropertyKey(k)]
			if !ok {
				return PropertyValue{}, false
			}
		default:
			return PropertyValue{}, false
		}
	}
	return v, true
}

// Set attempts to set the location inside a PropertyValue indicated by the PropertyPath to the given value. If any
// component of the path besides the last component does not exist, this function will return false.
func (p PropertyPath) Set(dest, v PropertyValue) bool {
	if len(p) == 0 {
		return false
	}

	dest, ok := p[:len(p)-1].Get(dest)
	if !ok {
		return false
	}

	key := p[len(p)-1]
	switch {
	case dest.IsArray():
		index, ok := key.(int)
		if !ok || index < 0 || index >= len(dest.ArrayValue()) {
			return false
		}
		dest.ArrayValue()[index] = v
	case dest.IsObject():
		k, ok := key.(string)
		if !ok {
			return false
		}
		dest.ObjectValue()[PropertyKey(k)] = v
	default:
		return false
	}
	return true
}

// Add sets the location inside a PropertyValue indicated by the PropertyPath to the given value. Any components
// referred to by the path that do not exist will be created. If there is a mismatch between the type of an existing
// component and a key that traverses that component, this function will return false. If the destination is a null
// property value, this function will create and return a new property value.
func (p PropertyPath) Add(dest, v PropertyValue) (PropertyValue, bool) {
	if len(p) == 0 {
		return PropertyValue{}, false
	}

	// set sets the destination referred to by the last element of the path to the given value.
	rv := dest
	set := func(v PropertyValue) {
		dest, rv = v, v
	}
	for _, key := range p {
		switch key := key.(type) {
		case int:
			// This key is an int, so we expect an array.
			switch {
			case dest.IsNull():
				// If the destination array does not exist, create a new array with enough room to store the value at
				// the requested index.
				dest = NewArrayProperty(make([]PropertyValue, key+1))
				set(dest)
			case dest.IsArray():
				// If the destination array does exist, ensure that it is large enough to accommodate the requested
				// index.
				if arr := dest.ArrayValue(); key >= len(arr) {
					dest = NewArrayProperty(append(make([]PropertyValue, key+1-len(arr)), arr...))
					set(dest)
				}
			default:
				return PropertyValue{}, false
			}
			destV := dest.ArrayValue()
			set = func(v PropertyValue) {
				destV[key] = v
			}
			dest = destV[key]
		case string:
			// This key is a string, so we expect an object.
			switch {
			case dest.IsNull():
				// If the destination does not exist, create a new object.
				dest = NewObjectProperty(PropertyMap{})
				set(dest)
			case dest.IsObject():
				// OK
			default:
				return PropertyValue{}, false
			}
			destV := dest.ObjectValue()
			set = func(v PropertyValue) {
				destV[PropertyKey(key)] = v
			}
			dest = destV[PropertyKey(key)]
		default:
			return PropertyValue{}, false
		}
	}

	set(v)
	return rv, true
}

// Delete attempts to delete the value located by the PropertyPath inside the given PropertyValue. If any component
// of the path does not exist, this function will return false.
func (p PropertyPath) Delete(dest PropertyValue) bool {
	if len(p) == 0 {
		return false
	}

	dest, ok := p[:len(p)-1].Get(dest)
	if !ok {
		return false
	}

	key := p[len(p)-1]
	switch {
	case dest.IsArray():
		index, ok := key.(int)
		if !ok || index < 0 || index >= len(dest.ArrayValue()) {
			return false
		}
		dest.ArrayValue()[index] = PropertyValue{}
	case dest.IsObject():
		k, ok := key.(string)
		if !ok {
			return false
		}
		delete(dest.ObjectValue(), PropertyKey(k))
	default:
		return false
	}
	return true
}

// Contains returns true if the receiver property path contains the other property path.
// For example, the path `foo["bar"][1]` contains the path `foo.bar[1].baz`.  The key `"*"`
// is a wildcard which matches any string or int index at that same nesting level.  So for example,
// the path `foo.*.baz` contains `foo.bar.baz.bam`, and the path `*` contains any path.
func (p PropertyPath) Contains(other PropertyPath) bool {
	if len(other) < len(p) {
		return false
	}

	for i := range p {
		pp := p[i]
		otherp := other[i]

		switch pp := pp.(type) {
		case int:
			if otherpi, ok := otherp.(int); !ok || otherpi != pp {
				return false
			}
		case string:
			if pp == "*" {
				continue
			}
			if otherps, ok := otherp.(string); !ok || otherps != pp {
				return false
			}
		default:
			// Invalid path, return false
			return false
		}
	}

	return true
}

// unwrapSecrets recursively unwraps any secrets from the given PropertyValue returning true if any secrets were
// unwrapped.
func unwrapSecrets(v PropertyValue) (PropertyValue, bool) {
	if v.IsSecret() {
		inner, _ := unwrapSecrets(v.SecretValue().Element)
		return inner, true
	}
	return v, false
}

func (p PropertyPath) reset(old, new PropertyValue, oldIsSecret, newIsSecret bool) bool {
	if len(p) == 0 {
		return false
	}

	// Unwrap any secrets from old & new, we can just go through them for this traversal.
	old, isSecret := unwrapSecrets(old)
	oldIsSecret = oldIsSecret || isSecret
	new, isSecret = unwrapSecrets(new)
	newIsSecret = newIsSecret || isSecret

	// If this is the last component we want to do the reset, else we want to search for the next component.
	key := p[0]
	switch key := key.(type) {
	case int:
		// An index < 0 is always a path error, even for empty arrays or objects
		if key < 0 {
			return false
		}

		// This is a leaf path element, so we want to reset the value at this index in new to the value at this index from old
		if len(p) == 1 {
			if !old.IsArray() && !new.IsArray() {
				// Neither old nor new are arrays, so we can't reset this index
				return true
			} else if !old.IsArray() || !new.IsArray() {
				// One of old or new is an array but the other isn't, so this is a path error
				return false
			}

			// If neither array contains this index then this is a _same_ and so ok, e.g. given old:[1, 2] and
			// new:[1] and a path of [3] we can return true because new at [3] is the same as old at [3], it
			// doesn't exist.
			if key >= len(old.ArrayValue()) && key >= len(new.ArrayValue()) {
				return true
			}
			// If one array has this index but the other doesn't this is a path failure because we can't
			// remove a location from an array.
			if key >= len(old.ArrayValue()) || key >= len(new.ArrayValue()) {
				return false
			}
			// Otherwise both arrays contain this index and we can reset the value of it in new to what is in
			// old.
			v := old.ArrayValue()[key]
			// If this was a secret value in old, but new isn't currently a secret context then we need to mark this
			// reset value as secret.
			if oldIsSecret && !newIsSecret {
				v = MakeSecret(v)
			}
			new.ArrayValue()[key] = v
			return true
		}

		if !old.IsArray() || !new.IsArray() {
			// At least one of old or new is not an array, so we can't keep searching along this path but
			// we only return an error if both are not arrays.
			return !old.IsArray() && !new.IsArray()
		}

		// If this index is out of bounds in either array then this is a path failure because we can't
		// continue the search of this path down each PropertyValue.
		if key >= len(old.ArrayValue()) || key >= len(new.ArrayValue()) {
			return false
		}
		old = old.ArrayValue()[key]
		new = new.ArrayValue()[key]
		return p[1:].reset(old, new, oldIsSecret, newIsSecret)

	case string:
		if key == "*" {
			if len(p) == 1 {
				if new.IsObject() {
					if old.IsObject() {
						for k := range old.ObjectValue() {
							v := old.ObjectValue()[k]
							// If this was a secret value in old, but new isn't currently a secret context then we need
							// to mark this reset value as secret.
							if oldIsSecret && !newIsSecret {
								v = MakeSecret(v)
							}
							new.ObjectValue()[k] = v
						}
						for k := range new.ObjectValue() {
							if _, has := old.ObjectValue()[k]; !has {
								delete(new.ObjectValue(), k)
							}
						}
					}
					return true
				} else if new.IsArray() {
					if old.IsArray() {
						oldArray := old.ArrayValue()
						newArray := new.ArrayValue()
						// If arrays are of different length then this is a path failure because we can't
						// synchronise the two values.
						if len(oldArray) != len(newArray) {
							return false
						}

						for i := range oldArray {
							v := oldArray[i]
							// If this was a secret value in old, but new isn't currently a secret context then we need
							// to mark this reset value as secret.
							if oldIsSecret && !newIsSecret {
								v = MakeSecret(v)
							}
							newArray[i] = v
						}
					}
					return true
				}
				return false
			}

			if old.IsObject() && new.IsObject() {
				oldObject := old.ObjectValue()
				newObject := new.ObjectValue()

				for k := range oldObject {
					var hasOld, hasNew bool
					oldValue, hasOld := oldObject[k]
					newValue, hasNew := newObject[k]
					if !hasOld || !hasNew {
						return false
					}

					if !p[1:].reset(oldValue, newValue, oldIsSecret, newIsSecret) {
						return false
					}
				}
				return true
			} else if old.IsArray() && new.IsArray() {
				oldArray := old.ArrayValue()
				newArray := new.ArrayValue()
				// If arrays are of different length then this is a path failure because we can't
				// continue the search of this path down each PropertyValue.
				if len(oldArray) != len(newArray) {
					return false
				}

				for i := range oldArray {
					if !p[1:].reset(oldArray[i], newArray[i], oldIsSecret, newIsSecret) {
						return false
					}
				}
				return true
			}
			return false
		}
		pkey := PropertyKey(key)

		if len(p) == 1 {
			// This is the leaf path entry, so we want to reset this property in new to it's value in old.

			// Firstly if old doesn't have this key (either because it isn't an object or because it
			// doesn't have the property) then we want to delete this from new.
			var v PropertyValue
			var has bool
			if old.IsObject() {
				v, has = old.ObjectValue()[pkey]
			}

			if has {
				// If this path exists in old but new isn't an object than return a path error
				if !new.IsObject() {
					return false
				}
				// Else simply overwrite the value in new with the value from old, if this was a secret value in
				// old, but new isn't currently a secret context then we need to mark this reset value as secret.
				if oldIsSecret && !newIsSecret {
					v = MakeSecret(v)
				}
				new.ObjectValue()[pkey] = v
			} else {
				// If the path doesn't exist in old then we want to delete it from new, but if new isn't
				// an object then we can just do nothing we don't consider this a path error. e.g. given
				// old:{} and new:1 and a path of "a" we can return true because ["a"] in both is the
				// same (it doesn't exist).
				if new.IsObject() {
					delete(new.ObjectValue(), pkey)
				}
			}
			return true
		}

		if !old.IsObject() || !new.IsObject() {
			// At least one of old or new is not an object, so we can't keep searching along this path but
			// we only return an error if both are not objects.
			return !old.IsObject() && !new.IsObject()
		}

		new, hasNew := new.ObjectValue()[pkey]
		old, hasOld := old.ObjectValue()[pkey]

		if hasOld && !hasNew {
			// Old has this key but new doesn't, but we still searching for the leaf item to set so this
			// is a path error.
			return false
		}
		if !hasOld && !hasNew {
			// Neither value contain this path, so we're done.
			return true
		}

		return p[1:].reset(old, new, oldIsSecret, newIsSecret)
	}

	contract.Failf("Invalid property path component type: %T", key)
	return true
}

// Reset attempts to reset the values located by the PropertyPath inside the given new PropertyMap to the
// values from the same location in the old PropertyMap. Reset behaves likes Set in that it will not create
// intermediate locations, it also won't create or delete array locations (because that would change the size
// of the array).
func (p PropertyPath) Reset(old, new PropertyMap) bool {
	return p.reset(NewObjectProperty(old), NewObjectProperty(new), false, false)
}

func requiresQuote(c rune) bool {
	return !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_')
}

func (p PropertyPath) String() string {
	var buf bytes.Buffer
	for i, k := range p {
		switch k := k.(type) {
		case string:
			var keyBuf bytes.Buffer
			quoted := false
			for _, c := range k {
				if requiresQuote(c) {
					quoted = true
					if c == '"' {
						keyBuf.WriteByte('\\')
					}
				}
				keyBuf.WriteRune(c)
			}
			if !quoted {
				if i == 0 {
					fmt.Fprintf(&buf, "%s", keyBuf.String())
				} else {
					fmt.Fprintf(&buf, ".%s", keyBuf.String())
				}
			} else {
				fmt.Fprintf(&buf, `["%s"]`, keyBuf.String())
			}
		case int:
			fmt.Fprintf(&buf, "[%d]", k)
		}
	}
	return buf.String()
}
