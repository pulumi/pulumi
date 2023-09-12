// Copyright 2023, Pulumi Corporation.  All rights reserved.

package schema

import (
	"encoding/json"
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
}
