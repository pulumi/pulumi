// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestMarshalKeyJSON(t *testing.T) {
	k := Key("test:config:key")

	b, err := json.Marshal(k)
	assert.NoError(t, err)
	assert.Equal(t, []byte("\"test:config:key\""), b)

	newK, err := roundtripKeyJSON(k)
	assert.NoError(t, err)
	assert.Equal(t, k, newK)

}

func TestMarshalKeyYAML(t *testing.T) {
	k := Key("test:config:key")

	b, err := yaml.Marshal(k)
	assert.NoError(t, err)
	assert.Equal(t, []byte("test:config:key\n"), b)

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
		return "", err
	}

	var newM Key
	err = unmarshal(b, &newM)
	return newM, err
}
