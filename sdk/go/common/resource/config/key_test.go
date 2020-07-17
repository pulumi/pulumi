// Copyright 2016-2018, Pulumi Corporation.
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
	"encoding/json"
	"testing"

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

	_, err = ParseKey("test:data:key")
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
