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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

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

// AsDecryptedPropertyMap returns the config as a property map, with secret values decrypted.
func (m Map) AsDecryptedPropertyMap(ctx context.Context, decrypter Decrypter) (resource.PropertyMap, error) {
	pm := resource.PropertyMap{}

	for k, v := range m {
		newV, err := adjustObjectValue(v)
		if err != nil {
			return resource.PropertyMap{}, err
		}
		plaintext, err := newV.toDecryptedPropertyValue(ctx, decrypter)
		if err != nil {
			return resource.PropertyMap{}, err
		}
		pm[resource.PropertyKey(k.String())] = plaintext
	}
	return pm, nil
}

// Get gets the value for a given key. If path is true, the key's name portion is treated as a path.
func (m Map) Get(k Key, path bool) (_ Value, ok bool, err error) {
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
	root, ok := m[configKey]
	if len(p) == 1 {
		return root, ok, nil
	}

	obj, err := root.unmarshalObject()
	if err != nil {
		return Value{}, false, err
	}
	objValue, ok, err := obj.Get(p[1:])
	if !ok || err != nil {
		return Value{}, ok, err
	}
	v, err := objValue.marshalValue()
	if err != nil {
		return v, false, err
	}
	return v, true, nil
}

// Remove removes the value for a given key. If path is true, the key's name portion is treated as a path.
func (m Map) Remove(k Key, path bool) error {
	// If the key isn't a path, go ahead and delete it and return.
	if !path {
		delete(m, k)
		return nil
	}

	// Otherwise, parse the path and get the new config key.
	p, configKey, err := parseKeyPath(k)
	if err != nil {
		return err
	}

	// If we only have a single path segment, delete the key and return.
	root, ok := m[configKey]
	if len(p) == 1 {
		delete(m, configKey)
		return nil
	}
	if !ok {
		return nil
	}

	obj, err := root.unmarshalObject()
	if err != nil {
		return err
	}
	err = obj.Delete(p[1:], p[1:])
	if err != nil {
		return err
	}
	root, err = obj.marshalValue()
	if err != nil {
		return err
	}
	m[configKey] = root
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

	newV, err := adjustObjectValue(v)
	if err != nil {
		return err
	}

	var obj object
	if root, ok := m[configKey]; ok {
		obj, err = root.unmarshalObject()
		if err != nil {
			return err
		}
	} else {
		obj = object{value: newContainer(p[1])}
	}
	err = obj.Set(p[:1], p[1:], newV)
	if err != nil {
		return err
	}
	root, err := obj.marshalValue()
	if err != nil {
		return err
	}
	m[configKey] = root
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
		return fmt.Errorf("could not unmarshal map: %w", err)
	}

	newMap := make(Map, len(rawMap))

	for k, v := range rawMap {
		pk, err := ParseKey(k)
		if err != nil {
			return fmt.Errorf("could not unmarshal map: %w", err)
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
		return fmt.Errorf("could not unmarshal map: %w", err)
	}

	newMap := make(Map, len(rawMap))

	for k, v := range rawMap {
		pk, err := ParseKey(k)
		if err != nil {
			return fmt.Errorf("could not unmarshal map: %w", err)
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
		return nil, Key{}, fmt.Errorf("invalid config key path: %w", err)
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

// adjustObjectValue returns a more suitable value for objects:
func adjustObjectValue(v Value) (object, error) {
	// If it's a secure value or an object, return as-is.
	if v.Secure() || v.Object() {
		return v.unmarshalObject()
	}

	// If "false" or "true", return the boolean value.
	if v.value == "false" {
		return newObject(false), nil
	} else if v.value == "true" {
		return newObject(true), nil
	}

	// If the value has more than one character and starts with "0", return the value as-is
	// so values like "0123456" are saved as a string (without stripping any leading zeros)
	// rather than as the integer 123456.
	if len(v.value) > 1 && v.value[0] == '0' {
		return v.unmarshalObject()
	}

	// If it's convertible to an int, return the int.
	if i, err := strconv.Atoi(v.value); err == nil {
		return newObject(int64(i)), nil
	}

	// Otherwise, just return the string value.
	return v.unmarshalObject()
}
