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

package config

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var errSecureKeyReserved = errors.New(`"secure" key in maps of length 1 are reserved`)

// Map is a bag of config stored in the settings file.
type Map map[Key]Value

// Decrypt returns the configuration as a map from module member to decrypted value.
func (m Map) Decrypt(decrypter Decrypter) (map[Key]string, error) {
	r := map[Key]string{}
	for k, c := range m {
		v, err := c.Value(decrypter)
		if err != nil {
			return nil, err
		}
		r[k] = v
	}
	return r, nil
}

func (m Map) Copy(decrypter Decrypter, encrypter Encrypter) (Map, error) {
	newConfig := make(Map)
	for k, c := range m {
		val, err := c.Copy(decrypter, encrypter)
		if err != nil {
			return nil, err
		}

		newConfig[k] = val
	}

	return newConfig, nil
}

// SecureKeys returns a list of keys that have secure values.
func (m Map) SecureKeys() []Key {
	var keys []Key
	for k, v := range m {
		if v.Secure() {
			keys = append(keys, k)
		}
	}
	return keys
}

// HasSecureValue returns true if the config map contains a secure (encrypted) value.
func (m Map) HasSecureValue() bool {
	for _, v := range m {
		if v.Secure() {
			return true
		}
	}

	return false
}

// Get gets the value for a given key. If path is true, the key's name portion is treated as a path.
func (m Map) Get(k Key, path bool) (Value, bool, error) {
	// If the key isn't a path, go ahead and lookup the value.
	if !path {
		v, ok := m[k]
		return v, ok, nil
	}

	// Otherwise, parse the path and get the new config key.
	p, configKey, err := parseKeyPath(k)
	if err != nil {
		return Value{}, false, err
	}

	// If we only have a single path segment, go ahead and lookup the value.
	if len(p) == 1 {
		v, ok := m[configKey]
		return v, ok, nil
	}

	// Otherwise, lookup the current root value and save it into a temporary map.
	root := make(map[string]interface{})
	if val, ok := m[configKey]; ok {
		obj, err := val.ToObject()
		if err != nil {
			return Value{}, false, err
		}
		root[configKey.Name()] = obj
	}

	// Get the value within the object.
	_, v, ok := getValueForPath(root, p)
	if !ok {
		return Value{}, false, nil
	}

	// If the value is a secure value, return it as one.
	if is, s := isSecureValue(v); is {
		return NewSecureValue(s), true, nil
	}

	// If it's a simple type, return it as a regular value.
	switch t := v.(type) {
	case string:
		return NewValue(t), true, nil
	case bool, int, uint, int32, uint32, int64, uint64, float32, float64:
		return NewValue(fmt.Sprintf("%v", v)), true, nil
	}

	// Otherwise, return it as an object value.
	json, err := json.Marshal(v)
	if err != nil {
		return Value{}, false, err
	}
	if hasSecureValue(v) {
		return NewSecureObjectValue(string(json)), true, nil
	}
	return NewObjectValue(string(json)), true, nil
}

// Remove removes the value for a given key. If path is true, the key's name portion is treated as a path.
func (m Map) Remove(k Key, path bool) error {
	// If the key isn't a path, go ahead and delete it and return.
	if !path {
		delete(m, k)
		return nil
	}

	// Parse the path.
	p, err := resource.ParsePropertyPath(k.Name())
	if err != nil {
		return errors.Wrap(err, "invalid config key path")
	}
	if len(p) == 0 {
		return nil
	}
	firstKey, ok := p[0].(string)
	if !ok || firstKey == "" {
		return nil
	}
	configKey := MustMakeKey(k.Namespace(), firstKey)

	// If we only have a single path segment, delete the key and return.
	if len(p) == 1 {
		delete(m, configKey)
		return nil
	}

	// Otherwise, lookup the current root value and save it into a temporary map.
	root := make(map[string]interface{})
	if val, ok := m[configKey]; ok {
		obj, err := val.ToObject()
		if err != nil {
			return err
		}
		root[configKey.Name()] = obj
	}

	// Get the value within the object up to the second-to-last path segment.
	// If not found, exit early.
	parent, dest, ok := getValueForPath(root, p[:len(p)-1])
	if !ok {
		return nil
	}

	// Remove the last path segment.
	key := p[len(p)-1]
	switch t := dest.(type) {
	case []interface{}:
		index, ok := key.(int)
		if !ok || index < 0 || index >= len(t) {
			return nil
		}
		t = append(t[:index], t[index+1:]...)
		// Since we changed the array, we need to update the parent.
		if parent != nil {
			pkey := p[len(p)-2]
			if _, err := setValue(parent, pkey, t, nil, nil); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		k, ok := key.(string)
		if !ok {
			return nil
		}
		delete(t, k)

		// Secure values are reserved, so return an error when attempting to add one.
		if isSecure, _ := isSecureValue(t); isSecure {
			return errSecureKeyReserved
		}
	}

	// Now, marshal then unmarshal the value, which will handle detecting
	// whether it's a secure object or not.
	jsonBytes, err := json.Marshal(root[configKey.Name()])
	if err != nil {
		return err
	}
	var v Value
	if err = json.Unmarshal(jsonBytes, &v); err != nil {
		return err
	}

	m[configKey] = v
	return nil
}

// Set sets the value for a given key. If path is true, the key's name portion is treated as a path.
func (m Map) Set(k Key, v Value, path bool) error {
	// If the key isn't a path, go ahead and set the value and return.
	if !path {
		m[k] = v
		return nil
	}

	// Otherwise, parse the path and get the new config key.
	p, configKey, err := parseKeyPath(k)
	if err != nil {
		return err
	}

	// If we only have a single path segment, set the value and return.
	if len(p) == 1 {
		m[configKey] = v
		return nil
	}

	// Otherwise, lookup the current value and save it into a temporary map.
	root := make(map[string]interface{})
	if val, ok := m[configKey]; ok {
		obj, err := val.ToObject()
		if err != nil {
			return err
		}

		// If obj is a string, set it to nil, which allows overwriting the existing
		// top-level string value in the first iteration of the loop below.
		if _, ok := obj.(string); ok {
			obj = nil
		}

		root[configKey.Name()] = obj
	}

	// Now, loop through the path segments, and walk the object tree.
	// If the value for a given segment is nil, create a new array/map.
	// The root map is the initial cursor value, and parent is nil.
	var parent interface{}
	var parentKey interface{}
	var cursor interface{}
	var cursorKey interface{}
	cursor = root
	cursorKey = p[0]
	for _, pkey := range p[1:] {
		pvalue, err := getValue(cursor, cursorKey)
		if err != nil {
			return err
		}

		// If the value is nil, create a new array/map.
		// Otherwise, return an error due to the type mismatch.
		var newValue interface{}
		switch pkey.(type) {
		case int:
			if pvalue == nil {
				newValue = make([]interface{}, 0)
			} else if _, ok := pvalue.([]interface{}); !ok {
				return errors.Errorf("an array was expected for index %v", pkey)
			}
		case string:
			if pvalue == nil {
				newValue = make(map[string]interface{})
			} else if _, ok := pvalue.(map[string]interface{}); !ok {
				return errors.Errorf("a map was expected for key %q", pkey)
			}
		default:
			contract.Failf("unexpected path type")
		}
		if newValue != nil {
			pvalue = newValue
			cursor, err = setValue(cursor, cursorKey, pvalue, parent, parentKey)
			if err != nil {
				return err
			}
		}

		parent = cursor
		parentKey = cursorKey
		cursor = pvalue
		cursorKey = pkey
	}

	// Adjust the value (e.g. convert "true"/"false" to booleans and integers to ints) and set it.
	adjustedValue := adjustObjectValue(v, path)
	if _, err = setValue(cursor, cursorKey, adjustedValue, parent, parentKey); err != nil {
		return err
	}

	// Secure values are reserved, so return an error when attempting to add one.
	if isSecure, _ := isSecureValue(cursor); isSecure {
		return errSecureKeyReserved
	}

	// Serialize the updated object as JSON, and save it in the config map.
	json, err := json.Marshal(root[configKey.Name()])
	if err != nil {
		return err
	}
	if v.Secure() {
		m[configKey] = NewSecureObjectValue(string(json))
	} else {
		m[configKey] = NewObjectValue(string(json))
	}

	return nil
}

func (m Map) MarshalJSON() ([]byte, error) {
	rawMap := make(map[string]Value, len(m))
	for k, v := range m {
		rawMap[k.String()] = v
	}

	return json.Marshal(rawMap)
}

func (m *Map) UnmarshalJSON(b []byte) error {
	rawMap := make(map[string]Value)
	if err := json.Unmarshal(b, &rawMap); err != nil {
		return errors.Wrap(err, "could not unmarshal map")
	}

	newMap := make(Map, len(rawMap))

	for k, v := range rawMap {
		pk, err := ParseKey(k)
		if err != nil {
			return errors.Wrap(err, "could not unmarshal map")
		}
		newMap[pk] = v
	}

	*m = newMap
	return nil
}

func (m Map) MarshalYAML() (interface{}, error) {
	rawMap := make(map[string]Value, len(m))
	for k, v := range m {
		rawMap[k.String()] = v
	}

	return rawMap, nil
}

func (m *Map) UnmarshalYAML(unmarshal func(interface{}) error) error {
	rawMap := make(map[string]Value)
	if err := unmarshal(&rawMap); err != nil {
		return errors.Wrap(err, "could not unmarshal map")
	}

	newMap := make(Map, len(rawMap))

	for k, v := range rawMap {
		pk, err := ParseKey(k)
		if err != nil {
			return errors.Wrap(err, "could not unmarshal map")
		}
		newMap[pk] = v
	}

	*m = newMap
	return nil
}

// parseKeyPath returns the property paths in the key and a new config key with the first
// path segment as the name.
func parseKeyPath(k Key) (resource.PropertyPath, Key, error) {
	// Parse the path, which will be in the name portion of the key.
	p, err := resource.ParsePropertyPath(k.Name())
	if err != nil {
		return nil, Key{}, errors.Wrap(err, "invalid config key path")
	}
	if len(p) == 0 {
		return nil, Key{}, errors.New("empty config key path")
	}

	// Create a new key that has the first path segment as the name.
	firstKey, ok := p[0].(string)
	if !ok {
		return nil, Key{}, errors.New("first path segement of config key must be a string")
	}
	if firstKey == "" {
		return nil, Key{}, errors.New("config key is empty")
	}

	configKey := MustMakeKey(k.Namespace(), firstKey)

	return p, configKey, nil
}

// getValueForPath returns the parent, value, and true if the value is found in source given the path segments in p.
func getValueForPath(source interface{}, p resource.PropertyPath) (interface{}, interface{}, bool) {
	// If the source is nil, exit early.
	if source == nil {
		return nil, nil, false
	}

	// Lookup the value by each path segment.
	var parent interface{}
	v := source
	for _, key := range p {
		parent = v
		switch t := v.(type) {
		case []interface{}:
			index, ok := key.(int)
			if !ok || index < 0 || index >= len(t) {
				return nil, nil, false
			}
			v = t[index]
		case map[string]interface{}:
			k, ok := key.(string)
			if !ok {
				return nil, nil, false
			}
			v, ok = t[k]
			if !ok {
				return nil, nil, false
			}
		default:
			return nil, nil, false
		}
	}
	return parent, v, true
}

// getValue returns the value in the container for the given key.
func getValue(container, key interface{}) (interface{}, error) {
	switch t := container.(type) {
	case []interface{}:
		i, ok := key.(int)
		contract.Assertf(ok, "key for an array must be an int")
		// We explicitly allow i == len(t) here, which indicates a
		// value that will be appended to the end of the array.
		if i < 0 || i > len(t) {
			return nil, errors.New("array index out of range")
		}
		if i == len(t) {
			return nil, nil
		}
		return t[i], nil
	case map[string]interface{}:
		k, ok := key.(string)
		contract.Assertf(ok, "key for a map must be a string")
		return t[k], nil
	}

	contract.Failf("should not reach here")
	return nil, nil
}

// Set value sets the value in the container for the given key, and returns the container.
// If the container is an array, and a value is being appended, containerParent and containerParentKey must
// not be nil since a new slice will be created to append the value, which needs to be saved in the parent.
// In this case, the new slice will be returned.
func setValue(container, key, value, containerParent, containerParentKey interface{}) (interface{}, error) {
	switch t := container.(type) {
	case []interface{}:
		i, ok := key.(int)
		contract.Assertf(ok, "key for an array must be an int")
		// We allow i == len(t), which indicates the value should be appended to the end of the array.
		if i < 0 || i > len(t) {
			return nil, errors.New("array index out of range")
		}
		// If i == len(t), we need to append to the end of the array, which involves creating a new slice
		// and saving it in the parent container.
		if i == len(t) {
			t = append(t, value)
			contract.Assertf(containerParent != nil, "parent must not be nil")
			contract.Assertf(containerParentKey != nil, "parentKey must not be nil")
			if _, err := setValue(containerParent, containerParentKey, t, nil, nil); err != nil {
				return nil, err
			}
			return t, nil
		}
		t[i] = value
	case map[string]interface{}:
		k, ok := key.(string)
		contract.Assertf(ok, "key for a map must be a string")
		t[k] = value
	}
	return container, nil
}

// adjustObjectValue returns a more suitable value for objects:
func adjustObjectValue(v Value, path bool) interface{} {
	contract.Assertf(!v.Object(), "v must not be an Object")

	// If the path flag isn't set, just return the value as-is.
	if !path {
		return v
	}

	// If it's a secure value, return as-is.
	if v.Secure() {
		return v
	}

	// If "false" or "true", return the boolean value.
	if v.value == "false" {
		return false
	} else if v.value == "true" {
		return true
	}

	// If the value has more than one character and starts with "0", return the value as-is
	// so values like "0123456" are saved as a string (without stripping any leading zeros)
	// rather than as the integer 123456.
	if len(v.value) > 1 && v.value[0] == '0' {
		return v.value
	}

	// If it's convertible to an int, return the int.
	i, err := strconv.Atoi(v.value)
	if err == nil {
		return i
	}

	// Otherwise, just return the string value.
	return v.value
}
