// Copyright 2016-2022, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var errSecureReprReserved = errors.New(`maps with the single key "secure" are reserved`)

// object is the internal object representation of a single config value. All operations on Value first decode the
// Value's string representation into its object representation. Secure strings are stored in objects as ciphertext.
type object struct {
	value  any
	secure bool
}

// objectType describes the types of values that may be stored in the value field of an object
type objectType interface {
	bool | int64 | float64 | string | []object | map[string]object
}

// newObject creates a new object with the given representation.
func newObject[T objectType](v T) object {
	return object{value: v}
}

// newSecureObject creates a new secure object with the given ciphertext.
func newSecureObject(ciphertext string) object {
	return object{value: ciphertext, secure: true}
}

// Secure returns true if the receiver is a secure string or a composite value that contains a secure string.
func (c object) Secure() bool {
	switch v := c.value.(type) {
	case []object:
		for _, v := range v {
			if v.Secure() {
				return true
			}
		}
		return false
	case map[string]object:
		for _, v := range v {
			if v.Secure() {
				return true
			}
		}
		return false
	case string:
		return c.secure
	default:
		return false
	}
}

// Decrypt decrypts any ciphertexts within the object and returns appropriately-shaped Plaintext values.
func (c object) Decrypt(ctx context.Context, decrypter Decrypter) (Plaintext, error) {
	return c.decrypt(ctx, nil, decrypter)
}

func (c object) decrypt(ctx context.Context, path resource.PropertyPath, decrypter Decrypter) (Plaintext, error) {
	switch v := c.value.(type) {
	case bool:
		return NewPlaintext(v), nil
	case int64:
		return NewPlaintext(v), nil
	case float64:
		return NewPlaintext(v), nil
	case string:
		if !c.secure {
			return NewPlaintext(v), nil
		}
		plaintext, err := decrypter.DecryptValue(ctx, v)
		if err != nil {
			return Plaintext{}, fmt.Errorf("%v: %w", path, err)
		}
		return NewSecurePlaintext(plaintext), nil
	case []object:
		vs := make([]Plaintext, len(v))
		for i, v := range v {
			pv, err := v.decrypt(ctx, append(path, i), decrypter)
			if err != nil {
				return Plaintext{}, err
			}
			vs[i] = pv
		}
		return NewPlaintext(vs), nil
	case map[string]object:
		vs := make(map[string]Plaintext, len(v))
		for k, v := range v {
			pv, err := v.decrypt(ctx, append(path, k), decrypter)
			if err != nil {
				return Plaintext{}, err
			}
			vs[k] = pv
		}
		return NewPlaintext(vs), nil
	case nil:
		return Plaintext{}, nil
	default:
		contract.Failf("unexpected value of type %T", v)
		return Plaintext{}, nil
	}
}

// Merge merges the receiver onto the given base using JSON merge patch semantics. Merge does not modify the receiver or
// the base.
func (c object) Merge(base object) object {
	if co, ok := c.value.(map[string]object); ok {
		if bo, ok := base.value.(map[string]object); ok {
			mo := make(map[string]object, len(co))
			for k, v := range bo {
				mo[k] = v
			}
			for k, v := range co {
				mo[k] = v.Merge(mo[k])
			}
			return newObject(mo)
		}
	}
	return c
}

// Get gets the member value at path. The path to the receiver is prefix.
func (c object) Get(path resource.PropertyPath) (_ object, ok bool, err error) {
	if len(path) == 0 {
		return c, true, nil
	}

	switch v := c.value.(type) {
	case []object:
		index, ok := path[0].(int)
		if !ok || index < 0 || index >= len(v) {
			return object{}, false, nil
		}
		elem := v[index]
		return elem.Get(path[1:])
	case map[string]object:
		key, ok := path[0].(string)
		if !ok {
			return object{}, false, nil
		}
		elem, ok := v[key]
		if !ok {
			return object{}, false, nil
		}
		return elem.Get(path[1:])
	default:
		return object{}, false, nil
	}
}

// Delete deletes the member value at path. The path to the receiver is prefix.
func (c *object) Delete(prefix, path resource.PropertyPath) error {
	if len(path) == 0 {
		return nil
	}

	prefix = append(prefix, path[0])
	switch v := c.value.(type) {
	case []object:
		index, ok := path[0].(int)
		if !ok || index < 0 || index >= len(v) {
			return nil
		}
		if len(path) == 1 {
			c.value = append(v[:index], v[index+1:]...)
			return nil
		}
		elem := &v[index]
		return elem.Delete(prefix, path[1:])
	case map[string]object:
		key, ok := path[0].(string)
		if !ok {
			return nil
		}

		// If we're deleting a property from this object, make sure that the result won't be mistaken for a secure
		// value when it is encoded. Secure values are encoded as `{"secure": "ciphertext"}`.
		if len(path) == 1 {
			if len(v) == 2 {
				keys := make([]string, 0, 2)
				for k := range v {
					if k != key {
						keys = append(keys, k)
					}
				}
				if len(keys) == 1 && keys[0] == "secure" {
					if _, ok := v["secure"].value.(string); ok {
						return fmt.Errorf("%v: %w", prefix, errSecureReprReserved)
					}
				}
			}

			delete(v, key)
			return nil
		}
		elem, ok := v[key]
		if !ok {
			return nil
		}
		err := elem.Delete(prefix, path[1:])
		v[key] = elem
		return err
	default:
		return nil
	}
}

func newContainer(accessor any) any {
	switch accessor := accessor.(type) {
	case int:
		return make([]object, accessor+1)
	case string:
		return make(map[string]object)
	default:
		contract.Failf("unexpected accessor kind %T", accessor)
		return nil
	}
}

// Set sets the member value at path to new. The path to the receiver is prefix.
func (c *object) Set(prefix, path resource.PropertyPath, new object) error {
	if len(path) == 0 {
		*c = new
		return nil
	}

	// Check the type of the receiver and create a new container if allowed.
	switch c.value.(type) {
	case []object, map[string]object:
		// OK
	case nil:
		// This value is nil. Create a new container ny inferring the container type (i.e. array or object) from the
		// accessor at the head of the path.
		c.value = newContainer(path[0])
	default:
		// COMPAT: If this is the first level, we create a new container and overwrite the old value rather than issuing
		// a type error.
		if len(prefix) == 1 {
			c.value, c.secure = newContainer(path[0]), false
		} else {
			switch path[0].(type) {
			case int:
				return fmt.Errorf("%v: expected an array", prefix)
			case string:
				return fmt.Errorf("%v: expected a map", prefix)
			default:
				contract.Failf("unreachable")
				return nil
			}
		}
	}

	prefix = append(prefix, path[0])
	switch v := c.value.(type) {
	case []object:
		index, ok := path[0].(int)
		if !ok {
			return fmt.Errorf("%v: key for an array must be an int", prefix)
		}
		if index < 0 || index > len(v) {
			return fmt.Errorf("%v: array index out of range", prefix)
		}
		if index == len(v) {
			v = append(v, object{})
			c.value = v
		}
		elem := &v[index]
		return elem.Set(prefix, path[1:], new)
	case map[string]object:
		key, ok := path[0].(string)
		if !ok {
			return fmt.Errorf("%v: key for a map must be a string", prefix)
		}

		// If we're adding a property tothis object, make sure that the result won't be mistaken for a secure
		// value when it is encoded. Secure values are encoded as `{"secure": "ciphertext"}`.
		if len(path) == 1 && len(v) == 0 && key == "secure" {
			if _, ok := new.value.(string); ok {
				return errSecureReprReserved
			}
		}

		elem := v[key]
		err := elem.Set(prefix, path[1:], new)
		v[key] = elem
		return err
	default:
		contract.Failf("unreachable")
		return nil
	}
}

// SecureValues returns the plaintext values for any secure strings contained in the receiver.
func (c object) SecureValues(dec Decrypter) ([]string, error) {
	switch v := c.value.(type) {
	case []object:
		var values []string
		for _, v := range v {
			vs, err := v.SecureValues(dec)
			if err != nil {
				return nil, err
			}
			values = append(values, vs...)
		}
		return values, nil
	case map[string]object:
		var values []string
		for _, v := range v {
			vs, err := v.SecureValues(dec)
			if err != nil {
				return nil, err
			}
			values = append(values, vs...)
		}
		return values, nil
	case string:
		if c.secure {
			plaintext, err := dec.DecryptValue(context.TODO(), v)
			if err != nil {
				return nil, err
			}
			return []string{plaintext}, nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

// marshalValue converts the receiver into a Value.
func (c object) marshalValue() (v Value, err error) {
	v.value, v.secure, v.object, err = c.MarshalString()
	return
}

// marshalObjectValue converts the receiver into a shape that is compatible with Value.ToObject().
func (c object) marshalObjectValue(root bool) any {
	switch v := c.value.(type) {
	case []object:
		vs := make([]any, len(v))
		for i, v := range v {
			vs[i] = v.marshalObjectValue(false)
		}
		return vs
	case map[string]object:
		vs := make(map[string]any, len(v))
		for k, v := range v {
			vs[k] = v.marshalObjectValue(false)
		}
		return vs
	case string:
		if !root && c.secure {
			return map[string]any{"secure": c.value}
		}
		return c.value
	default:
		return c.value
	}
}

// MarshalString returns the receiver's string representation. The string representation is accompanied by bools that
// indicate whether the receiver is secure and whether it is an object.
func (c object) MarshalString() (text string, secure, object bool, err error) {
	switch v := c.value.(type) {
	case bool, int64, float64:
		bytes, err := c.MarshalJSON()
		return string(bytes), false, false, err
	case string:
		return v, c.secure, false, nil
	default:
		bytes, err := c.MarshalJSON()
		if err != nil {
			return "", false, false, err
		}
		return string(bytes), c.Secure(), true, nil
	}
}

// UnmarshalString unmarshals the string representation accompanied by secure and object metadata into the receiver.
func (c *object) UnmarshalString(text string, secure, object bool) error {
	if !object {
		c.value, c.secure = text, secure
		return nil
	}
	return c.UnmarshalJSON([]byte(text))
}

func (c object) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.marshalObject())
}

func (c *object) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	var v any
	err := dec.Decode(&v)
	if err != nil {
		return err
	}
	*c, err = unmarshalObject(v)
	return err
}

func (c object) MarshalYAML() (any, error) {
	return c.marshalObject(), nil
}

func (c *object) UnmarshalYAML(unmarshal func(any) error) error {
	var v any
	err := unmarshal(&v)
	if err != nil {
		return err
	}
	*c, err = unmarshalObject(v)
	return err
}

// unmarshalObject unmarshals a raw JSON or YAML value into an object. json.Number values are converted to int64 if
// possible and float64 otherwise.
func unmarshalObject(v any) (object, error) {
	switch v := v.(type) {
	case bool:
		return newObject(v), nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return newObject(i), nil
		}
		f, err := v.Float64()
		if err == nil {
			return newObject(f), nil
		}
		return object{}, fmt.Errorf("unrepresentable number %v: %w", v, err)
	case int:
		return newObject(int64(v)), nil
	case int64:
		return newObject(v), nil
	case float64:
		return newObject(v), nil
	case string:
		return newObject(v), nil
	case time.Time:
		return newObject(v.String()), nil
	case map[string]any:
		if ok, ciphertext := isSecureValue(v); ok {
			return newSecureObject(ciphertext), nil
		}
		m := make(map[string]object, len(v))
		for k, v := range v {
			sv, err := unmarshalObject(v)
			if err != nil {
				return object{}, err
			}
			m[k] = sv
		}
		return newObject(m), nil
	case map[any]any:
		m := make(map[string]any, len(v))
		for k, v := range v {
			m[fmt.Sprintf("%v", k)] = v
		}
		return unmarshalObject(m)
	case []any:
		a := make([]object, len(v))
		for i, v := range v {
			sv, err := unmarshalObject(v)
			if err != nil {
				return object{}, err
			}
			a[i] = sv
		}
		return newObject(a), nil
	case nil:
		return object{}, nil
	default:
		contract.Failf("unexpected wire type %T", v)
		return object{}, nil
	}
}

// marshalObject returns the value that should be passed to the JSON or YAML packages when marshaling the receiver.
func (c object) marshalObject() any {
	if str, ok := c.value.(string); ok && c.secure {
		type secureValue struct {
			Secure string `json:"secure" yaml:"secure"`
		}
		return secureValue{Secure: str}
	}
	return c.value
}

// isSecureValue returns true if the object is a `map[string]any` of length one with a "secure" property of type string.
func isSecureValue(v any) (bool, string) {
	if m, isMap := v.(map[string]any); isMap && len(m) == 1 {
		if val, hasSecureKey := m["secure"]; hasSecureKey {
			if valString, isString := val.(string); isString {
				return true, valString
			}
		}
	}
	return false, ""
}

func (c object) toDecryptedPropertyValue(ctx context.Context, decrypter Decrypter) (resource.PropertyValue, error) {
	plaintext, err := c.Decrypt(ctx, decrypter)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	return plaintext.PropertyValue(), nil
}
