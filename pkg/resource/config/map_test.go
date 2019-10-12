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

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestMarshalMapJSON(t *testing.T) {
	t.Parallel()
	m := Map{
		Key{namespace: "my", name: "testKey"}:        NewValue("testValue"),
		Key{namespace: "my", name: "anotherTestKey"}: NewValue("anotherTestValue"),
	}

	b, err := json.Marshal(m)
	assert.NoError(t, err)
	assert.Equal(t,
		[]byte("{\"my:anotherTestKey\":\"anotherTestValue\",\"my:testKey\":\"testValue\"}"),
		b)

	newM, err := roundtripMapJSON(m)
	assert.NoError(t, err)
	assert.Equal(t, m, newM)

}

func TestMarshalMapYAML(t *testing.T) {
	t.Parallel()
	m := Map{
		Key{namespace: "my", name: "testKey"}:        NewValue("testValue"),
		Key{namespace: "my", name: "anotherTestKey"}: NewValue("anotherTestValue"),
	}

	b, err := yaml.Marshal(m)
	assert.NoError(t, err)

	s1 := string(b)
	contract.Ignore(s1)
	assert.Equal(t, []byte("my:anotherTestKey: anotherTestValue\nmy:testKey: testValue\n"), b)

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
