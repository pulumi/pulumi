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
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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
		obj, err := c.unmarshalObjectJSON()
		if err != nil {
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

func (c Value) Copy(decrypter Decrypter, encrypter Encrypter) (Value, error) {
	var val Value
	raw, err := c.Value(decrypter)
	if err != nil {
		return Value{}, err
	}
	if c.Secure() {
		if c.Object() {
			objVal, err := c.ToObject()
			if err != nil {
				return Value{}, err
			}
			encryptedObj, err := reencryptObject(objVal, decrypter, encrypter)
			if err != nil {
				return Value{}, err
			}
			json, err := json.Marshal(encryptedObj)
			if err != nil {
				return Value{}, err
			}

			val = NewSecureObjectValue(string(json))
		} else {
			enc, eerr := encrypter.EncryptValue(raw)
			if eerr != nil {
				return Value{}, eerr
			}
			val = NewSecureValue(enc)
		}
	} else {
		if c.Object() {
			val = NewObjectValue(raw)
		} else {
			val = NewValue(raw)
		}
	}

	return val, nil
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
	return c.unmarshalObjectJSON()
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
		return c.unmarshalObjectJSON()
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

func reencryptObject(v interface{}, decrypter Decrypter, encrypter Encrypter) (interface{}, error) {
	reencryptIt := func(val interface{}) (interface{}, error) {
		if isSecure, secureVal := isSecureValue(val); isSecure {
			newVal := NewSecureValue(secureVal)
			raw, err := newVal.Value(decrypter)
			if err != nil {
				return nil, err
			}

			encVal, err := encrypter.EncryptValue(raw)
			if err != nil {
				return nil, err
			}

			m := make(map[string]string)
			m["secure"] = encVal

			return m, nil
		}
		return reencryptObject(val, decrypter, encrypter)
	}

	switch t := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{})
		for key, val := range t {
			encrypted, err := reencryptIt(val)
			if err != nil {
				return nil, err
			}
			m[key] = encrypted
		}
		return m, nil
	case []interface{}:
		a := make([]interface{}, len(t))
		for i, val := range t {
			encrypted, err := reencryptIt(val)
			if err != nil {
				return nil, err
			}
			a[i] = encrypted
		}
		return a, nil
	}
	return v, nil
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

func (c Value) unmarshalObjectJSON() (interface{}, error) {
	contract.Assertf(c.object, "expected value to be an object")
	var v interface{}
	dec := json.NewDecoder(strings.NewReader(c.value))
	// By default, the JSON decoder will unmarshal numbers as float64, but we want to keep integers as integers
	// if possible. We use the decoder's UseNumber option so that numbers are unmarshalled as json.Number, and
	// then iterate through the object and try to convert any values of json.Number to an int64, otherwise falling
	// back to float64.
	dec.UseNumber()
	err := dec.Decode(&v)
	if err != nil {
		return nil, err
	}
	v, err = replaceNumberWithIntOrFloat(v)
	if err != nil {
		return nil, err
	}
	return v, err
}

func replaceNumberWithIntOrFloat(v interface{}) (interface{}, error) {
	switch t := v.(type) {
	case map[string]interface{}:
		for key, val := range t {
			f, err := replaceNumberWithIntOrFloat(val)
			if err != nil {
				return nil, err
			}
			t[key] = f
		}
	case []interface{}:
		for i, val := range t {
			f, err := replaceNumberWithIntOrFloat(val)
			if err != nil {
				return nil, err
			}
			t[i] = f
		}
	case json.Number:
		// Try to return the number as an int64, otherwise fall back to float64.
		i, err := t.Int64()
		if err == nil {
			return i, nil
		}
		return t.Float64()
	}
	return v, nil
}
