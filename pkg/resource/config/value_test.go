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
	"fmt"
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

func TestHasSecureValue(t *testing.T) {
	tests := []struct {
		Value    interface{}
		Expected bool
	}{
		{
			Value:    []interface{}{"a", "b", "c"},
			Expected: false,
		},
		{
			Value: map[string]interface{}{
				"foo": "bar",
				"hi":  map[string]interface{}{"secure": "securevalue", "but": "not"},
			},
			Expected: false,
		},
		{
			Value: map[string]interface{}{
				"foo": "bar",
				"hi":  map[string]interface{}{"secure": 1},
			},
			Expected: false,
		},
		{
			Value:    []interface{}{"a", "b", map[string]interface{}{"secure": "securevalue"}},
			Expected: true,
		},
		{
			Value: map[string]interface{}{
				"foo": "bar",
				"hi":  map[string]interface{}{"secure": "securevalue"},
			},
			Expected: true,
		},
		{
			Value: map[string]interface{}{
				"foo":   "bar",
				"array": []interface{}{"a", "b", map[string]interface{}{"secure": "securevalue"}},
			},
			Expected: true,
		},
		{
			Value: map[string]interface{}{
				"foo": "bar",
				"map": map[string]interface{}{
					"nest": "blah",
					"hi":   map[string]interface{}{"secure": "securevalue"},
				},
			},
			Expected: true,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Value), func(t *testing.T) {
			jsonBytes, err := json.Marshal(test.Value)
			assert.NoError(t, err)

			var val interface{}
			err = json.Unmarshal(jsonBytes, &val)
			assert.NoError(t, err)

			assert.Equal(t, test.Expected, hasSecureValue(val))
		})
	}
}

func TestDecryptingValue(t *testing.T) {
	tests := []struct {
		Value    Value
		Expected string
	}{
		{
			Value:    NewValue("value"),
			Expected: "value",
		},
		{
			Value:    NewValue(`{"foo":"bar"}`),
			Expected: `{"foo":"bar"}`,
		},
		{
			Value:    NewValue(`["a","b"]`),
			Expected: `["a","b"]`,
		},
		{
			Value:    NewObjectValue(`{"foo":"bar"}`),
			Expected: `{"foo":"bar"}`,
		},
		{
			Value:    NewObjectValue(`["a","b"]`),
			Expected: `["a","b"]`,
		},
		{
			Value:    NewSecureValue("securevalue"),
			Expected: "[secret]",
		},
		{
			Value:    NewSecureObjectValue(`{"foo":{"secure":"securevalue"}}`),
			Expected: `{"foo":"[secret]"}`,
		},
		{
			Value:    NewSecureObjectValue(`["a",{"secure":"securevalue"}]`),
			Expected: `["a","[secret]"]`,
		},
	}

	decrypter := NewBlindingDecrypter()

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Value), func(t *testing.T) {
			actual, err := test.Value.Value(decrypter)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, actual)

			// Ensure the same value is returned when the NopDecrypter is used.
			actualNop, err := test.Value.Value(NopDecrypter)
			assert.NoError(t, err)
			assert.Equal(t, test.Value.value, actualNop)
		})
	}
}

type passThroughDecrypter struct{}

func (d passThroughDecrypter) DecryptValue(ciphertext string) (string, error) {
	return ciphertext, nil
}

func TestSecureValues(t *testing.T) {
	tests := []struct {
		Value    Value
		Expected []string
	}{
		{
			Value:    NewValue("value"),
			Expected: nil,
		},
		{
			Value:    NewObjectValue(`{"foo":"bar"}`),
			Expected: nil,
		},
		{
			Value:    NewObjectValue(`["a","b"]`),
			Expected: nil,
		},
		{
			Value:    NewSecureValue("securevalue"),
			Expected: []string{"securevalue"},
		},
		{
			Value:    NewSecureObjectValue(`{"foo":{"secure":"securevalue"}}`),
			Expected: []string{"securevalue"},
		},
		{
			Value:    NewSecureObjectValue(`["a",{"secure":"securevalue"}]`),
			Expected: []string{"securevalue"},
		},
		{
			Value:    NewSecureObjectValue(`["a",{"secure":"alpha"},{"test":{"secure":"beta"}}]`),
			Expected: []string{"alpha", "beta"},
		},
	}

	decrypter := passThroughDecrypter{}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Value), func(t *testing.T) {
			actual, err := test.Value.SecureValues(decrypter)
			assert.NoError(t, err)
			assert.Equal(t, test.Expected, actual)
		})
	}
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
