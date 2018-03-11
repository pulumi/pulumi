// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"errors"
)

// Value is a single config value.
type Value struct {
	value  string
	secure bool
}

func NewSecureValue(v string) Value {
	return Value{value: v, secure: true}
}

func NewValue(v string) Value {
	return Value{value: v, secure: false}
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

	return decrypter.DecryptValue(c.value)
}

func (c Value) Secure() bool {
	return c.secure
}

func (c Value) MarshalJSON() ([]byte, error) {
	if !c.secure {
		return json.Marshal(c.value)
	}

	m := make(map[string]string)
	m["secure"] = c.value

	return json.Marshal(m)
}

func (c *Value) UnmarshalJSON(b []byte) error {
	var m map[string]string
	err := json.Unmarshal(b, &m)
	if err == nil {
		if len(m) != 1 {
			return errors.New("malformed secure data")
		}

		val, has := m["secure"]
		if !has {
			return errors.New("malformed secure data")
		}

		c.value = val
		c.secure = true
		return nil
	}

	return json.Unmarshal(b, &c.value)
}

func (c Value) MarshalYAML() (interface{}, error) {
	if !c.secure {
		return c.value, nil
	}

	m := make(map[string]string)
	m["secure"] = c.value

	return m, nil
}

func (c *Value) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]string
	err := unmarshal(&m)
	if err == nil {
		if len(m) != 1 {
			return errors.New("malformed secure data")
		}

		val, has := m["secure"]
		if !has {
			return errors.New("malformed secure data")
		}

		c.value = val
		c.secure = true
		return nil
	}

	c.secure = false
	return unmarshal(&c.value)
}
