// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package config

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestMarshalMapJSON(t *testing.T) {
	m := Map{
		Key{namespace: "my", name: "testKey"}:        NewValue("testValue"),
		Key{namespace: "my", name: "anotherTestKey"}: NewValue("anotherTestValue"),
	}

	b, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(t,
		[]byte("{\"my:config:anotherTestKey\":\"anotherTestValue\",\"my:config:testKey\":\"testValue\"}"),
		b)

	newM, err := roundtripMapJSON(m)
	assert.NoError(t, err)
	assert.Equal(t, m, newM)

}

func TestMarshalMapYAML(t *testing.T) {
	m := Map{
		Key{namespace: "my", name: "testKey"}:        NewValue("testValue"),
		Key{namespace: "my", name: "anotherTestKey"}: NewValue("anotherTestValue"),
	}

	b, err := yaml.Marshal(m)
	assert.NoError(t, err)

	s1 := string(b)
	contract.Ignore(s1)
	assert.Equal(t, []byte("my:config:anotherTestKey: anotherTestValue\nmy:config:testKey: testValue\n"), b)

	newM, err := roundtripMapYAML(m)
	assert.NoError(t, err)
	assert.Equal(t, m, newM)
}

func roundtripMapYAML(m Map) (Map, error) {
	return roundtripMap(m, yaml.Marshal, yaml.Unmarshal)
}

func roundtripMapJSON(m Map) (Map, error) {
	return roundtripMap(m, json.Marshal, json.Unmarshal)
}

func roundtripMap(m Map, marshal func(v interface{}) ([]byte, error),
	unmarshal func([]byte, interface{}) error) (Map, error) {
	b, err := marshal(m)
	if err != nil {
		return nil, err
	}

	var newM Map
	err = unmarshal(b, &newM)
	return newM, err
}
