// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"errors"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

type Value struct {
	value  string
	secure bool
}

func (c Value) Value(decrypter ValueDecrypter) (string, error) {
	contract.Require(decrypter != nil, "decrypter")

	if !c.secure {
		return c.value, nil
	}

	return decrypter.DecryptValue(c.value)
}

func (c Value) Secure() bool {
	return c.secure
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

func (c Value) MarshalYAML() (interface{}, error) {
	if !c.secure {
		return c.value, nil
	}

	m := make(map[string]string)
	m["secure"] = c.value

	return m, nil
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

func (c Value) MarshalJSON() ([]byte, error) {
	if !c.secure {
		return json.Marshal(c.value)
	}

	m := make(map[string]string)
	m["secure"] = c.value

	return json.Marshal(m)
}

func NewSecureValue(v string) Value {
	return Value{value: v, secure: true}
}

func NewValue(v string) Value {
	return Value{value: v, secure: false}
}

type ValueDecrypter interface {
	DecryptValue(cypertext string) (string, error)
}

type ValueEncrypter interface {
	EncryptValue(plaintext string) (string, error)
}

type ValueEncrypterDecrypter interface {
	ValueEncrypter
	ValueDecrypter
}
