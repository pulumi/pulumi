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

	"github.com/pkg/errors"
)

// Value is a single config value.
type Value struct {
	value  string
	secure bool
	object bool
}

func NewSecureValue(v string) Value {
	return Value{value: v, secure: true}
}

func NewValue(v string) Value {
	return Value{value: v, secure: false}
}

func NewSecureObjectValue(v string) Value {
	return Value{value: v, secure: true, object: true}
}

func NewObjectValue(v string) Value {
	return Value{value: v, secure: false, object: true}
}

// Value fetches the value of this configuration entry, using decrypter to decrypt if necessary.  If the value
// is a secret and decrypter is nil, or if decryption fails for any reason, a non-nil error is returned.
func (c Value) Value(decrypter Decrypter) (string, error) {
	if !c.secure {
		return c.value, nil
	}
	if decrypter == nil {
		return "", errors.New("non-nil decrypter required for secret")
	}
	if c.object && decrypter != NopDecrypter {
		var obj interface{}
		if err := json.Unmarshal([]byte(c.value), &obj); err != nil {
			return "", err
		}
		decryptedObj, err := decryptObject(obj, decrypter)
		if err != nil {
			return "", err
		}
		json, err := json.Marshal(decryptedObj)
		if err != nil {
			return "", err
		}
		return string(json), nil
	}

	return decrypter.DecryptValue(c.value)
}

func (c Value) SecureValues(decrypter Decrypter) ([]string, error) {
	d := NewTrackingDecrypter(decrypter)
	if _, err := c.Value(d); err != nil {
		return nil, err
	}
	return d.SecureValues(), nil
}

func (c Value) Secure() bool {
	return c.secure
}

func (c Value) Object() bool {
	return c.object
}

// ToObject returns the string value (if not an object), or the unmarshalled JSON object (if an object).
func (c Value) ToObject() (interface{}, error) {
	if !c.object {
		return c.value, nil
	}

	var v interface{}
	err := json.Unmarshal([]byte(c.value), &v)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (c Value) MarshalJSON() ([]byte, error) {
	v, err := c.marshalValue()
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func (c *Value) UnmarshalJSON(b []byte) error {
	return c.unmarshalValue(
		func(v interface{}) error {
			return json.Unmarshal(b, v)
		},
		func(v interface{}) interface{} {
			return v
		})
}

func (c Value) MarshalYAML() (interface{}, error) {
	return c.marshalValue()
}

func (c *Value) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return c.unmarshalValue(func(v interface{}) error {
		return unmarshal(v)
	}, interfaceMapToStringMap)
}

func (c *Value) unmarshalValue(unmarshal func(interface{}) error, fix func(interface{}) interface{}) error {
	// First, try to unmarshal as a string.
	err := unmarshal(&c.value)
	if err == nil {
		c.secure = false
		c.object = false
		return nil
	}

	// Otherwise, try to unmarshal as an object.
	var obj interface{}
	if err = unmarshal(&obj); err != nil {
		return errors.Wrapf(err, "malformed config value")
	}

	// Fix-up the object (e.g. convert `map[interface{}]interface{}` to `map[string]interface{}`).
	obj = fix(obj)

	if is, val := isSecureValue(obj); is {
		c.value = val
		c.secure = true
		c.object = false
		return nil
	}

	json, err := json.Marshal(obj)
	if err != nil {
		return errors.Wrapf(err, "marshalling obj")
	}
	c.value = string(json)
	c.secure = hasSecureValue(obj)
	c.object = true
	return nil
}

func (c Value) marshalValue() (interface{}, error) {
	if c.object {
		var obj interface{}
		err := json.Unmarshal([]byte(c.value), &obj)
		return obj, err
	}

	if !c.secure {
		return c.value, nil
	}

	m := make(map[string]string)
	m["secure"] = c.value

	return m, nil
}

// The unserialized value from YAML needs to be serializable as JSON, but YAML will unmarshal maps as
// `map[interface{}]interface{}` (because it supports bools as keys), which isn't supported by the JSON
// marshaller. To address, when unserializing YAML, we convert `map[interface{}]interface{}` to
// `map[string]interface{}`.
func interfaceMapToStringMap(v interface{}) interface{} {
	switch t := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for key, val := range t {
			m[fmt.Sprintf("%v", key)] = interfaceMapToStringMap(val)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(t))
		for i, val := range t {
			a[i] = interfaceMapToStringMap(val)
		}
		return a
	}
	return v
}

// hasSecureValue returns true if the object contains a value that's a `map[string]string` of
// length one with a "secure" key.
func hasSecureValue(v interface{}) bool {
	switch t := v.(type) {
	case map[string]interface{}:
		if is, _ := isSecureValue(t); is {
			return true
		}
		for _, val := range t {
			if hasSecureValue(val) {
				return true
			}
		}
	case []interface{}:
		for _, val := range t {
			if hasSecureValue(val) {
				return true
			}
		}
	}
	return false
}

// isSecureValue returns true if the object is a `map[string]string` of length one with a "secure" key.
func isSecureValue(v interface{}) (bool, string) {
	if m, isMap := v.(map[string]interface{}); isMap && len(m) == 1 {
		if val, hasSecureKey := m["secure"]; hasSecureKey {
			if valString, isString := val.(string); isString {
				return true, valString
			}
		}
	}
	return false, ""
}

// decryptObject returns a new object with all secure values in the object converted to decrypted strings.
func decryptObject(v interface{}, decrypter Decrypter) (interface{}, error) {
	decryptIt := func(val interface{}) (interface{}, error) {
		if isSecure, secureVal := isSecureValue(val); isSecure {
			return decrypter.DecryptValue(secureVal)
		}
		return decryptObject(val, decrypter)
	}

	switch t := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{})
		for key, val := range t {
			decrypted, err := decryptIt(val)
			if err != nil {
				return nil, err
			}
			m[key] = decrypted
		}
		return m, nil
	case []interface{}:
		a := make([]interface{}, len(t))
		for i, val := range t {
			decrypted, err := decryptIt(val)
			if err != nil {
				return nil, err
			}
			a[i] = decrypted
		}
		return a, nil
	}
	return v, nil
}
