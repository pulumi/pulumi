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
	if decrypter == NopDecrypter {
		return c.value, nil
	}

	obj, err := c.unmarshalObject()
	if err != nil {
		return "", err
	}
	plaintext, err := obj.Decrypt(context.TODO(), decrypter)
	if err != nil {
		return "", err
	}
	return plaintext.marshalText()
}

func (c Value) Decrypt(ctx context.Context, decrypter Decrypter) (Plaintext, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return Plaintext{}, err
	}
	return obj.Decrypt(ctx, decrypter)
}

func (c Value) Merge(base Value) (Value, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return Value{}, err
	}
	baseObj, err := base.unmarshalObject()
	if err != nil {
		return Value{}, err
	}
	return obj.Merge(baseObj).marshalValue()
}

func (c Value) Copy(decrypter Decrypter, encrypter Encrypter) (Value, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return Value{}, err
	}
	plaintext, err := obj.Decrypt(context.TODO(), decrypter)
	if err != nil {
		return Value{}, err
	}
	return plaintext.Encrypt(context.TODO(), encrypter)
}

func (c Value) SecureValues(decrypter Decrypter) ([]string, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return nil, err
	}
	return obj.SecureValues(decrypter)
}

func (c Value) Secure() bool {
	return c.secure
}

func (c Value) Object() bool {
	return c.object
}

func (c Value) unmarshalObject() (object, error) {
	var obj object
	err := obj.UnmarshalString(c.value, c.secure, c.object)
	return obj, err
}

// ToObject returns the string value (if not an object), or the unmarshalled JSON object (if an object).
func (c Value) ToObject() (any, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return nil, err
	}
	return obj.marshalObjectValue(true), nil
}

func (c Value) MarshalJSON() ([]byte, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return nil, err
	}
	return obj.MarshalJSON()
}

func (c *Value) UnmarshalJSON(b []byte) (err error) {
	var obj object
	if err = obj.UnmarshalJSON(b); err != nil {
		return err
	}
	c.value, c.secure, c.object, err = obj.MarshalString()
	return err
}

func (c Value) MarshalYAML() (interface{}, error) {
	obj, err := c.unmarshalObject()
	if err != nil {
		return "", err
	}
	return obj.MarshalYAML()
}

func (c *Value) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {
	var obj object
	if err = obj.UnmarshalYAML(unmarshal); err != nil {
		return err
	}
	c.value, c.secure, c.object, err = obj.MarshalString()
	return err
}
