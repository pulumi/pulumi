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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// PlaintextType describes the allowed types for a Plaintext.
type PlaintextType interface {
	bool | int64 | uint64 | float64 | string | PlaintextSecret | []Plaintext | map[string]Plaintext
}

// Plaintext is a single plaintext config value.
type Plaintext struct {
	value any
}

// NewPlaintext creates a new plaintext config value.
func NewPlaintext[T PlaintextType](v T) Plaintext {
	if m, ok := any(v).(map[string]Plaintext); ok && len(m) == 1 {
		if _, ok := m["secure"].Value().(string); ok {
			contract.Failf("%s", errSecureReprReserved.Error())
		}
	}

	return Plaintext{value: v}
}

// Secure returns true if the receiver is a secure string or a composite value that contains a secure string.
func (c Plaintext) Secure() bool {
	switch v := c.Value().(type) {
	case []Plaintext:
		for _, v := range v {
			if v.Secure() {
				return true
			}
		}
		return false
	case map[string]Plaintext:
		for _, v := range v {
			if v.Secure() {
				return true
			}
		}
		return false
	case PlaintextSecret:
		return true
	default:
		return false
	}
}

// Value returns the inner plaintext value.
//
// The returned value satisfies the PlaintextType constraint.
func (c Plaintext) Value() any {
	return c.value
}

// GoValue returns the inner plaintext value as a plain Go value:
//
//   - secure strings are mapped to their plaintext
//   - []Plaintext values are mapped to []any values
//   - map[string]Plaintext values are mapped to map[string]any values
func (c Plaintext) GoValue() any {
	switch v := c.Value().(type) {
	case []Plaintext:
		vs := make([]any, len(v))
		for i, v := range v {
			vs[i] = v.GoValue()
		}
		return vs
	case map[string]Plaintext:
		vs := make(map[string]any, len(v))
		for k, v := range v {
			vs[k] = v.GoValue()
		}
		return vs
	case PlaintextSecret:
		return string(v)
	default:
		return v
	}
}

// PropertyValue converts a plaintext value into a property.Value.
func (c Plaintext) PropertyValue() property.Value {
	var prop property.Value
	switch v := c.Value().(type) {
	case bool:
		prop = property.New(v)
	case int64:
		prop = property.New(float64(v))
	case uint64:
		prop = property.New(float64(v))
	case float64:
		prop = property.New(v)
	case string:
		prop = property.New(v)
	case PlaintextSecret:
		prop = property.New(string(v)).WithSecret(true)
	case []Plaintext:
		vs := make([]property.Value, len(v))
		for i, v := range v {
			vs[i] = v.PropertyValue()
		}
		prop = property.New(vs)
	case map[string]Plaintext:
		vs := make(map[string]property.Value, len(v))
		for k, v := range v {
			vs[k] = v.PropertyValue()
		}
		prop = property.New(vs)
	case nil:
		prop = property.New(property.Null)
	default:
		contract.Failf("unexpected value of type %T", v)
		return property.Value{}
	}
	return prop
}

func encryptMap(ctx context.Context, plaintextMap map[Key]Plaintext, encrypter Encrypter) (map[Key]object, error) {
	// Collect all secure values
	var refs []containerRef
	var ptChunks [][]string
	collectPlaintextSecrets(plaintextMap, &refs, &ptChunks)

	// Encrypt objects in batches
	offset := 0
	for _, ptChunk := range ptChunks {
		if len(ptChunk) == 0 {
			continue
		}
		encryptedChunk, err := encrypter.BatchEncrypt(ctx, ptChunk)
		if err != nil {
			return nil, err
		}
		// Assign encrypted values back into original structure
		// We are accepting that a Plaintext Secret now has a ciphertext value
		// TODO: https://github.com/pulumi/pulumi/issues/20663
		// Prefer using a caching encrypter to avoid mixing plaintext and ciphertext
		for i, encrypted := range encryptedChunk {
			refs[offset+i].setPlaintext(NewPlaintext(PlaintextSecret(encrypted)))
		}
		offset += len(ptChunk)
	}

	// Marshal each top-level object back into an object value.
	// Note that at this point, all Plaintext Secrets have been encrypted.
	// So we can use the NopEncrypter here.
	result := map[Key]object{}
	for k, pt := range plaintextMap {
		obj, err := pt.encrypt(ctx, nil, NopEncrypter)
		if err != nil {
			return nil, err
		}
		result[k] = obj
	}
	return result, nil
}

func EncryptMap(ctx context.Context, plaintextMap map[Key]Plaintext, encrypter Encrypter) (map[Key]Value, error) {
	objectMap, err := encryptMap(ctx, plaintextMap, encrypter)
	if err != nil {
		return nil, err
	}

	result := Map{}
	for k, obj := range objectMap {
		v, err := obj.marshalValue()
		if err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, nil
}

// Encrypt converts the receiver as a Value. All secure strings in the result are encrypted using encrypter.
func (c Plaintext) Encrypt(ctx context.Context, encrypter Encrypter) (Value, error) {
	obj, err := c.encrypt(ctx, nil, encrypter)
	if err != nil {
		return Value{}, err
	}
	return obj.marshalValue()
}

// encrypt converts the receiver to an object. All secure strings in the result are encrypted using encrypter.
func (c Plaintext) encrypt(ctx context.Context, path resource.PropertyPath, encrypter Encrypter) (object, error) {
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
		return newObject(v), nil
	case PlaintextSecret:
		ciphertext, err := v.Encrypt(ctx, encrypter)
		if err != nil {
			return object{}, fmt.Errorf("%v: %w", path, err)
		}
		return newObject(ciphertext), nil
	case []Plaintext:
		vs := make([]object, len(v))
		for i, v := range v {
			ev, err := v.encrypt(ctx, append(path, i), encrypter)
			if err != nil {
				return object{}, err
			}
			vs[i] = ev
		}
		return newObject(vs), nil
	case map[string]Plaintext:
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
func (c Plaintext) marshalText() (string, error) {
	switch v := c.Value().(type) {
	case string:
		return v, nil
	case PlaintextSecret:
		return string(v), nil
	}
	bytes, err := json.Marshal(c.GoValue())
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (c Plaintext) MarshalJSON() ([]byte, error) {
	contract.Failf("plaintext must be encrypted before marshaling")
	return nil, nil
}

func (c *Plaintext) UnmarshalJSON(b []byte) error {
	contract.Failf("plaintext cannot be unmarshaled")
	return nil
}

func (c Plaintext) MarshalYAML() (any, error) {
	contract.Failf("plaintext must be encrypted before marshaling")
	return nil, nil
}

func (c *Plaintext) UnmarshalYAML(unmarshal func(any) error) error {
	contract.Failf("plaintext cannot be unmarshaled")
	return nil
}
