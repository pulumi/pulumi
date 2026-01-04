// Copyright 2016-2025, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestMarshallNormalValueYAML(t *testing.T) {
	t.Parallel()

	v := NewValue("value")

	b, err := yaml.Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, []byte("value\n"), b)

	newV, err := roundtripValueYAML(v)
	require.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallSecureValueYAML(t *testing.T) {
	t.Parallel()

	v := NewSecureValue("value")

	b, err := yaml.Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, []byte("secure: value\n"), b)

	newV, err := roundtripValueYAML(v)
	require.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallNormalValueJSON(t *testing.T) {
	t.Parallel()

	v := NewValue("value")

	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, []byte("\"value\""), b)

	newV, err := roundtripValueJSON(v)
	require.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestMarshallSecureValueJSON(t *testing.T) {
	t.Parallel()

	v := NewSecureValue("value")

	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.Equal(t, []byte("{\"secure\":\"value\"}"), b)

	newV, err := roundtripValueJSON(v)
	require.NoError(t, err)
	assert.Equal(t, v, newV)
}

func TestHasSecureValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Value    any
		Expected bool
	}{
		{
			Value:    []any{"a", "b", "c"},
			Expected: false,
		},
		{
			Value: map[string]any{
				"foo": "bar",
				"hi":  map[string]any{"secure": "securevalue", "but": "not"},
			},
			Expected: false,
		},
		{
			Value:    []any{"a", "b", map[string]any{"secure": "securevalue"}},
			Expected: true,
		},
		{
			Value: map[string]any{
				"foo": "bar",
				"hi":  map[string]any{"secure": "securevalue"},
			},
			Expected: true,
		},
		{
			Value: map[string]any{
				"foo":   "bar",
				"array": []any{"a", "b", map[string]any{"secure": "securevalue"}},
			},
			Expected: true,
		},
		{
			Value: map[string]any{
				"foo": "bar",
				"map": map[string]any{
					"nest": "blah",
					"hi":   map[string]any{"secure": "securevalue"},
				},
			},
			Expected: true,
		},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Value), func(t *testing.T) {
			t.Parallel()

			jsonBytes, err := json.Marshal(test.Value)
			require.NoError(t, err)

			var val object
			err = json.Unmarshal(jsonBytes, &val)
			require.NoError(t, err)

			assert.Equal(t, test.Expected, val.Secure())
		})
	}
}

func TestDecryptingValue(t *testing.T) {
	t.Parallel()

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

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Value), func(t *testing.T) {
			t.Parallel()

			actual, err := test.Value.Value(decrypter)
			require.NoError(t, err)
			assert.Equal(t, test.Expected, actual)

			// Ensure the same value is returned when the NopDecrypter is used.
			actualNop, err := test.Value.Value(NopDecrypter)
			require.NoError(t, err)
			assert.Equal(t, test.Value.value, actualNop)
		})
	}
}

type passThroughDecrypter struct{}

func (d passThroughDecrypter) DecryptValue(
	ctx context.Context, ciphertext string,
) (string, error) {
	return ciphertext, nil
}

func (d passThroughDecrypter) BatchDecrypt(
	ctx context.Context, ciphertexts []string,
) ([]string, error) {
	return DefaultBatchDecrypt(ctx, d, ciphertexts)
}

func TestSecureValues(t *testing.T) {
	t.Parallel()

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

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Value), func(t *testing.T) {
			t.Parallel()

			actual, err := test.Value.SecureValues(decrypter)
			require.NoError(t, err)
			assert.Equal(t, test.Expected, actual)
		})
	}
}

func TestCopyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Val      Value
		Expected Value
	}{
		{
			Val:      NewValue("value"),
			Expected: NewValue("value"),
		},
		{
			Val:      NewObjectValue(`{"foo":"bar"}`),
			Expected: NewObjectValue(`{"foo":"bar"}`),
		},
		{
			Val:      NewSecureObjectValue(`{"foo":{"secure":"stackAsecurevalue"}}`),
			Expected: NewSecureObjectValue(`{"foo":{"secure":"stackBsecurevalue"}}`),
		},
		{
			Val:      NewSecureValue("stackAsecurevalue"),
			Expected: NewSecureValue("stackBsecurevalue"),
		},
		{
			Val:      NewSecureObjectValue(`["a",{"secure":"stackAalpha"},{"test":{"secure":"stackAbeta"}}]`),
			Expected: NewSecureObjectValue(`["a",{"secure":"stackBalpha"},{"test":{"secure":"stackBbeta"}}]`),
		},
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			t.Parallel()

			newConfig, err := test.Val.Copy(newPrefixCrypter("stackA"), newPrefixCrypter("stackB"))
			require.NoError(t, err)

			assert.Equal(t, test.Expected, newConfig)
		})
	}
}

func TestBoolValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Val      Value
		Expected bool
	}{
		{
			Val:      NewTypedValue("true", TypeBool),
			Expected: true,
		},
		{
			Val:      NewTypedValue("false", TypeBool),
			Expected: false,
		},
		{
			Val:      NewTypedValue("TRUE", TypeBool),
			Expected: true,
		},
		{
			Val:      NewTypedValue("True", TypeBool),
			Expected: true,
		},
		{
			Val:      NewTypedValue("invalid", TypeBool),
			Expected: false,
		},
		{
			Val:      NewTypedValue("yes", TypeBool),
			Expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test.Val), func(t *testing.T) {
			t.Parallel()

			actual, err := test.Val.ToObject()
			require.NoError(t, err)
			a := actual.(bool)
			assert.Equal(t, test.Expected, a)
		})
	}
}

func roundtripValueYAML(v Value) (Value, error) {
	return roundtripValue(v, yaml.Marshal, yaml.Unmarshal)
}

func roundtripValueJSON(v Value) (Value, error) {
	return roundtripValue(v, json.Marshal, json.Unmarshal)
}

func roundtripValue(v Value, marshal func(v any) ([]byte, error),
	unmarshal func([]byte, any) error,
) (Value, error) {
	b, err := marshal(v)
	if err != nil {
		return Value{}, err
	}

	var newV Value
	err = unmarshal(b, &newV)
	return newV, err
}
