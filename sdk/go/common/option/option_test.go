// Copyright 2016-2024, Pulumi Corporation.
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

package option

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOption(t *testing.T) {
	var null Option[int]
	value, has := Value(null)
	assert.False(t, has)
	assert.Zero(t, value)

	none := None[int]()
	value, has = Value(none)
	assert.False(t, has)
	assert.Zero(t, value)

	some := Some(42)
	value, has = Value(some)
	assert.True(t, has)
	assert.Equal(t, 42, value)
}

func TestZero(t *testing.T) {
	var zero option[int]
	assert.PanicsWithError(t, "Option must be initialized", func() {
		Value(&zero)
	})
}

func TestSomeNil(t *testing.T) {
	t.Run("pointer", func(t *testing.T) {
		assert.PanicsWithError(t, "Option must not be nil", func() {
			Some[*float32](nil)
		})
	})

	t.Run("any", func(t *testing.T) {
		assert.PanicsWithError(t, "Option must not be nil", func() {
			Some[any](nil)
		})
	})

	t.Run("slice", func(t *testing.T) {
		assert.PanicsWithError(t, "Option must not be nil", func() {
			Some[[]int](nil)
		})
	})

	t.Run("map", func(t *testing.T) {
		assert.PanicsWithError(t, "Option must not be nil", func() {
			Some[map[string]int](nil)
		})
	})

	t.Run("interface", func(t *testing.T) {
		assert.PanicsWithError(t, "Option must not be nil", func() {
			Some[io.Reader](nil)
		})
	})

	t.Run("channel", func(t *testing.T) {
		assert.PanicsWithError(t, "Option must not be nil", func() {
			Some[chan<- int](nil)
		})
	})
}

func TestMarshalJSON(t *testing.T) {
	none := None[int]()
	data, err := json.Marshal(none)
	require.NoError(t, err)
	assert.Equal(t, "null", string(data))

	some := Some(42)
	data, err = json.Marshal(some)
	require.NoError(t, err)
	assert.Equal(t, "42", string(data))
}

func TestUnmarshalJSON(t *testing.T) {
	var none Option[int]
	err := json.Unmarshal([]byte("null"), &none)
	require.NoError(t, err)
	value, has := Value(none)
	assert.False(t, has)
	assert.Zero(t, value)

	var some Option[int]
	err = json.Unmarshal([]byte("42"), &some)
	require.NoError(t, err)
	value, has = Value(some)
	assert.True(t, has)
	assert.Equal(t, 42, value)
}

func TestUnmarshalJSONIntoStruct(t *testing.T) {
	type Foo struct {
		Bar Option[int]
		Baz Option[int]
	}

	var foo Foo
	err := json.Unmarshal([]byte("{\"Bar\":null,\"Baz\":42}"), &foo)
	require.NoError(t, err)
}

func TestOmitempty(t *testing.T) {
	type Foo struct {
		Bar Option[int] `json:""`
		Baz Option[int] `json:",omitempty"`
	}

	var foo Foo
	data, err := json.Marshal(foo)
	require.NoError(t, err)
	assert.Equal(t, "{\"Bar\":null}", string(data))

	foo.Bar = Some(12)
	foo.Baz = Some(42)
	data, err = json.Marshal(foo)
	require.NoError(t, err)
	assert.Equal(t, "{\"Bar\":12,\"Baz\":42}", string(data))

	t.Run("null", func(t *testing.T) {
		var foo Foo
		err := json.Unmarshal([]byte("{\"Bar\":null,\"Baz\":null}"), &foo)
		require.NoError(t, err)
		value, has := Value(foo.Bar)
		assert.False(t, has)
		assert.Zero(t, value)
		value, has = Value(foo.Baz)
		assert.False(t, has)
		assert.Zero(t, value)
	})

	t.Run("missing", func(t *testing.T) {
		var foo Foo
		err := json.Unmarshal([]byte("{}"), &foo)
		require.NoError(t, err)
		value, has := Value(foo.Bar)
		assert.False(t, has)
		assert.Zero(t, value)
		value, has = Value(foo.Baz)
		assert.False(t, has)
		assert.Zero(t, value)
	})
}
