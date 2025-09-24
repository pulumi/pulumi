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
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// plaintextType describes the allowed types for a plaintext.
type plaintextType interface {
	bool | int64 | uint64 | float64 | string | []plaintext | map[string]plaintext
}

// plaintext is a single plaintext config value.
type plaintext struct {
	value  any
	secure bool
}

// newPlaintext creates a new plaintext config value.
func newPlaintext[T plaintextType](v T) plaintext {
	if m, ok := any(v).(map[string]plaintext); ok && len(m) == 1 {
		if _, ok := m["secure"].Value().(string); ok {
			contract.Failf("%s", errSecureReprReserved.Error())
		}
	}

	return plaintext{value: v}
}

// newSecurePlaintext creates a new secure string with the given plaintext.
func newSecurePlaintext(value string) plaintext {
	return plaintext{value: value, secure: true}
}

// Secure returns true if the receiver is a secure string or a composite value that contains a secure string.
func (c plaintext) Secure() bool {
	switch v := c.Value().(type) {
	case []plaintext:
		for _, v := range v {
			if v.Secure() {
				return true
			}
		}
		return false
	case map[string]plaintext:
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

// Value returns the inner plaintext value.
//
// The returned value satisfies the plaintextType constraint.
func (c plaintext) Value() any {
	return c.value
}

// GoValue returns the inner plaintext value as a plain Go value:
//
//   - secure strings are mapped to their plaintext
//   - []plaintext values are mapped to []any values
//   - map[string]plaintext values are mapped to map[string]any values
func (c plaintext) GoValue() any {
	switch v := c.Value().(type) {
	case []plaintext:
		vs := make([]any, len(v))
		for i, v := range v {
			vs[i] = v.GoValue()
		}
		return vs
	case map[string]plaintext:
		vs := make(map[string]any, len(v))
		for k, v := range v {
			vs[k] = v.GoValue()
		}
		return vs
	default:
		return v
	}
}

// PropertyValue converts a plaintext value into a resource.PropertyValue.
func (c plaintext) PropertyValue() resource.PropertyValue {
	var prop resource.PropertyValue
	switch v := c.Value().(type) {
	case bool:
		prop = resource.NewProperty(v)
	case int64:
		prop = resource.NewProperty(float64(v))
	case uint64:
		prop = resource.NewProperty(float64(v))
	case float64:
		prop = resource.NewProperty(v)
	case string:
		prop = resource.NewProperty(v)
	case []plaintext:
		vs := make([]resource.PropertyValue, len(v))
		for i, v := range v {
			vs[i] = v.PropertyValue()
		}
		prop = resource.NewProperty(vs)
	case map[string]plaintext:
		vs := make(resource.PropertyMap, len(v))
		for k, v := range v {
			vs[resource.PropertyKey(k)] = v.PropertyValue()
		}
		prop = resource.NewProperty(vs)
	case nil:
		prop = resource.NewNullProperty()
	default:
		contract.Failf("unexpected value of type %T", v)
		return resource.PropertyValue{}
	}
	if c.secure {
		prop = resource.MakeSecret(prop)
	}
	return prop
}

func encryptMap(ctx context.Context, plaintextMap map[Key]plaintext, encrypter Encrypter) (map[Key]object, error) {
	// Collect all secure values
	var locationRefs []secureLocationRef
	var valuesChunks [][]string
	collectSecureFromPlaintextKeyMap(plaintextMap, &locationRefs, &valuesChunks)

	// Encrypt objects in batches
	offset := 0
	for _, valuesChunk := range valuesChunks {
		if len(valuesChunk) == 0 {
			continue
		}
		encryptedChunk, err := encrypter.BatchEncrypt(ctx, valuesChunk)
		if err != nil {
			return nil, err
		}
		// Assign encrypted values back into original structure
		// We are accepting that a secure plaintext now has a ciphertext value
		for i, encrypted := range encryptedChunk {
			locationRef := locationRefs[offset+i]
			switch container := locationRef.container.(type) {
			case map[Key]plaintext:
				container[locationRef.key.(Key)] = newSecurePlaintext(encrypted)
			case map[string]plaintext:
				container[locationRef.key.(string)] = newSecurePlaintext(encrypted)
			case []plaintext:
				container[locationRef.key.(int)] = newSecurePlaintext(encrypted)
			}
		}
		offset += len(valuesChunk)
	}

	// Marshal each top-level object back into an object value.
	// Note that at this point, all secure values have been encrypted.
	// So we can use the NopEncrypter here.
	result := map[Key]object{}
	for k, obj := range plaintextMap {
		obj, err := obj.Encrypt(ctx, NopEncrypter)
		if err != nil {
			return nil, err
		}
		result[k] = obj
	}
	return result, nil
}

// Encrypt converts the receiver as an object. All secure strings in the result are encrypted using encrypter.
func (c plaintext) Encrypt(ctx context.Context, encrypter Encrypter) (object, error) {
	return c.encrypt(ctx, nil, encrypter)
}

// encrypt converts the receiver to an object. All secure strings in the result are encrypted using encrypter.
func (c plaintext) encrypt(ctx context.Context, path resource.PropertyPath, encrypter Encrypter) (object, error) {
	switch v := c.Value().(type) {
	case nil:
		return object{}, nil
	case bool:
		return newObject(v), nil
	case int64:
		return newObject(v), nil
	case uint64:
		return newObject(v), nil
	case float64:
		return newObject(v), nil
	case string:
		if !c.secure {
			return newObject(v), nil
		}
		ciphertext, err := encrypter.EncryptValue(ctx, v)
		if err != nil {
			return object{}, fmt.Errorf("%v: %w", path, err)
		}
		return newSecureObject(ciphertext), nil
	case []plaintext:
		vs := make([]object, len(v))
		for i, v := range v {
			ev, err := v.encrypt(ctx, append(path, i), encrypter)
			if err != nil {
				return object{}, err
			}
			vs[i] = ev
		}
		return newObject(vs), nil
	case map[string]plaintext:
		vs := make(map[string]object, len(v))
		for k, v := range v {
			ev, err := v.encrypt(ctx, append(path, k), encrypter)
			if err != nil {
				return object{}, err
			}
			vs[k] = ev
		}
		return newObject(vs), nil
	default:
		contract.Failf("unexpected plaintext of type %T", v)
		return object{}, nil
	}
}

// marshalText returns the text representation of the plaintext.
func (c plaintext) marshalText() (string, error) {
	if str, ok := c.Value().(string); ok {
		return str, nil
	}
	bytes, err := json.Marshal(c.GoValue())
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (c plaintext) MarshalJSON() ([]byte, error) {
	contract.Failf("plaintext must be encrypted before marshaling")
	return nil, nil
}

func (c *plaintext) UnmarshalJSON(b []byte) error {
	contract.Failf("plaintext cannot be unmarshaled")
	return nil
}

func (c plaintext) MarshalYAML() (any, error) {
	contract.Failf("plaintext must be encrypted before marshaling")
	return nil, nil
}

func (c *plaintext) UnmarshalYAML(unmarshal func(any) error) error {
	contract.Failf("plaintext cannot be unmarshaled")
	return nil
}
