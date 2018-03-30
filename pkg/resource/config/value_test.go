// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestMarshallNormalValueYAML(t *testing.T) {
	v := NewValue("value")

	b, err := yaml.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("value\n"), b)

	newV, err := roundtripValueYAML(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallSecureValueYAML(t *testing.T) {
	v := NewSecureValue("value")

	b, err := yaml.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("secure: value\n"), b)

	newV, err := roundtripValueYAML(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallNormalValueJSON(t *testing.T) {
	v := NewValue("value")

	b, err := json.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("\"value\""), b)

	newV, err := roundtripValueJSON(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallSecureValueJSON(t *testing.T) {
	v := NewSecureValue("value")

	b, err := json.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("{\"secure\":\"value\"}"), b)

	newV, err := roundtripValueJSON(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func roundtripValueYAML(v Value) (Value, error) {
	return roundtripValue(v, yaml.Marshal, yaml.Unmarshal)
}

func roundtripValueJSON(v Value) (Value, error) {
	return roundtripValue(v, json.Marshal, json.Unmarshal)
}

func roundtripValue(v Value, marshal func(v interface{}) ([]byte, error),
	unmarshal func([]byte, interface{}) error) (Value, error) {
	b, err := marshal(v)
	if err != nil {
		return Value{}, err
	}

	var newV Value
	err = unmarshal(b, &newV)
	return newV, err
}
