// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestParseKey(t *testing.T) {
	k, err := ParseKey("test:config:key")
	assert.NoError(t, err)
	assert.Equal(t, "test", k.namespace)
	assert.Equal(t, "key", k.name)

	k, err = ParseKey("test:key")
	assert.NoError(t, err)
	assert.Equal(t, "test", k.namespace)
	assert.Equal(t, "key", k.name)

	_, err = ParseKey("foo")
	assert.Error(t, err)
}

func TestFromModuleMember(t *testing.T) {
	mm := tokens.ModuleMember("test:config:key")
	k, err := fromModuleMember(mm)
	assert.NoError(t, err)
	assert.Equal(t, "test", k.namespace)
	assert.Equal(t, "key", k.name)

	mm = tokens.ModuleMember("test:data:key")
	_, err = fromModuleMember(mm)
	assert.Error(t, err)
}

func TestMarshalKeyJSON(t *testing.T) {
	k := Key{namespace: "test", name: "key"}

	b, err := json.Marshal(k)
	assert.NoError(t, err)
	assert.Equal(t, []byte("\"test:key\""), b)

	newK, err := roundtripKeyJSON(k)
	assert.NoError(t, err)
	assert.Equal(t, k, newK)
}

func TestMarshalKeyYAML(t *testing.T) {
	k := Key{namespace: "test", name: "key"}

	b, err := yaml.Marshal(k)
	assert.NoError(t, err)
	assert.Equal(t, []byte("test:key\n"), b)

	newK, err := roundtripKeyYAML(k)
	assert.NoError(t, err)
	assert.Equal(t, k, newK)
}

func roundtripKeyYAML(k Key) (Key, error) {
	return roundtripKey(k, yaml.Marshal, yaml.Unmarshal)
}

func roundtripKeyJSON(k Key) (Key, error) {
	return roundtripKey(k, json.Marshal, json.Unmarshal)
}

func roundtripKey(m Key, marshal func(v interface{}) ([]byte, error),
	unmarshal func([]byte, interface{}) error) (Key, error) {
	b, err := marshal(m)
	if err != nil {
		return Key{}, err
	}

	var newM Key
	err = unmarshal(b, &newM)
	return newM, err
}
