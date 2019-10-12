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

func TestMarshallNormalValueYAML(t *testing.T) {
	t.Parallel()
	v := NewValue("value")

	b, err := yaml.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("value\n"), b)

	newV, err := roundtripValueYAML(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallSecureValueYAML(t *testing.T) {
	t.Parallel()
	v := NewSecureValue("value")

	b, err := yaml.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("secure: value\n"), b)

	newV, err := roundtripValueYAML(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallNormalValueJSON(t *testing.T) {
	t.Parallel()
	v := NewValue("value")

	b, err := json.Marshal(v)
	assert.NoError(t, err)
	assert.Equal(t, []byte("\"value\""), b)

	newV, err := roundtripValueJSON(v)
	assert.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallSecureValueJSON(t *testing.T) {
	t.Parallel()
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
