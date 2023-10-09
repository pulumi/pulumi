// Copyright 2023, Pulumi Corporation.
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

package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	var s Schema
	err := json.Unmarshal([]byte(`{"type":"string"}`), &s)
	require.NoError(t, err)
	assert.Equal(t, "string", s.Type)
}

func TestUnmarshalNever(t *testing.T) {
	var s Schema
	err := json.Unmarshal([]byte("false"), &s)
	require.NoError(t, err)
	assert.True(t, s.Never)
	assert.False(t, s.Always)
}

func TestUnmarshalAlways(t *testing.T) {
	var s Schema
	err := json.Unmarshal([]byte("true"), &s)
	require.NoError(t, err)
	assert.True(t, s.Always)
	assert.False(t, s.Never)
}

func TestMarshal(t *testing.T) {
	bytes, err := json.Marshal(String().Schema())
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"type":"string"}`), bytes)
}

func TestMarshalNever(t *testing.T) {
	bytes, err := json.Marshal(Never())
	require.NoError(t, err)
	assert.Equal(t, []byte("false"), bytes)
}

func TestMarshalAlways(t *testing.T) {
	bytes, err := json.Marshal(Always())
	require.NoError(t, err)
	assert.Equal(t, []byte("true"), bytes)
}

func TestItem(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		s := String().Schema().Item(0)
		assert.True(t, s.Never)
	})

	t.Run("valid", func(t *testing.T) {
		s := Array().PrefixItems(String(), Number()).Items(Boolean()).Schema()
		assert.Equal(t, s.PrefixItems[0], s.Item(0))
		assert.Equal(t, s.PrefixItems[1], s.Item(1))
		assert.Equal(t, s.Items, s.Item(2))
	})

	t.Run("anyOf", func(t *testing.T) {
		s := AnyOf(
			Array().PrefixItems(String(), Number()).Items(Boolean()),
			Array().PrefixItems(Number(), Boolean()),
			String(),
		).Schema()

		assert.Equal(t, OneOf(String(), Number()).Schema(), s.Item(0))
		assert.Equal(t, OneOf(Number(), Boolean()).Schema(), s.Item(1))
		assert.Equal(t, Boolean().Schema(), s.Item(2))
	})

	t.Run("oneOf", func(t *testing.T) {
		s := OneOf(
			Array().PrefixItems(String(), Number()).Items(Boolean()),
			Array().PrefixItems(Number(), Boolean()),
			String(),
		).Schema()

		assert.Equal(t, OneOf(String(), Number()).Schema(), s.Item(0))
		assert.Equal(t, OneOf(Number(), Boolean()).Schema(), s.Item(1))
		assert.Equal(t, Boolean().Schema(), s.Item(2))
	})
}

func TestProperty(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		s := String().Schema().Property("foo")
		assert.True(t, s.Never)
	})

	t.Run("valid", func(t *testing.T) {
		s := Object().
			Properties(map[string]Builder{"a": String(), "b": Number()}).
			AdditionalProperties(Boolean()).
			Schema()

		assert.Equal(t, s.Properties["a"], s.Property("a"))
		assert.Equal(t, s.Properties["b"], s.Property("b"))
		assert.Equal(t, s.AdditionalProperties, s.Property("c"))
	})

	t.Run("anyOf", func(t *testing.T) {
		a := Object().
			Properties(map[string]Builder{"a": String(), "b": Number()}).
			AdditionalProperties(Never()).
			Schema()

		b := Object().
			Properties(map[string]Builder{"a": Number(), "b": Boolean()}).
			AdditionalProperties(Number()).
			Schema()

		c := String().Schema()

		s := AnyOf(a, b, c).Schema()
		assert.Equal(t, OneOf(String(), Number()).Schema(), s.Property("a"))
		assert.Equal(t, OneOf(Number(), Boolean()).Schema(), s.Property("b"))
		assert.Equal(t, Number().Schema(), s.Property("c"))
	})

	t.Run("oneOf", func(t *testing.T) {
		a := Object().
			Properties(map[string]Builder{"a": String(), "b": Number()}).
			AdditionalProperties(Never()).
			Schema()

		b := Object().
			Properties(map[string]Builder{"a": Number(), "b": Boolean()}).
			AdditionalProperties(Number()).
			Schema()

		c := String().Schema()

		s := OneOf(a, b, c).Schema()
		assert.Equal(t, OneOf(String(), Number()).Schema(), s.Property("a"))
		assert.Equal(t, OneOf(Number(), Boolean()).Schema(), s.Property("b"))
		assert.Equal(t, Number().Schema(), s.Property("c"))
	})
}

func TestCompile(t *testing.T) {
	path := "testdata"
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			bytes, err := os.ReadFile(filepath.Join(path, e.Name()))
			require.NoError(t, err)

			var schema Schema
			err = json.Unmarshal(bytes, &schema)
			require.NoError(t, err)

			err = schema.Compile()
			assert.NoError(t, err)
		})
	}
}
