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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrings(t *testing.T) {
	t.Run("ref", func(t *testing.T) {
		ref := "example"
		s := String().
			Ref(ref).
			Schema()
		require.Equal(t, s.Ref, ref)
	})

	t.Run("anyOf", func(t *testing.T) {
		anyOf := []Builder{
			Boolean(),
			String(),
		}
		s := String().
			AnyOf(anyOf...).
			Schema()
		require.Equal(t, len(anyOf), len(s.AnyOf))
		for i, v := range anyOf {
			require.Equal(t, v.Schema(), s.AnyOf[i])
		}
	})

	t.Run("oneOf", func(t *testing.T) {
		oneOf := []Builder{
			Boolean(),
			String(),
		}
		s := String().
			OneOf(oneOf...).
			Schema()
		require.Equal(t, len(oneOf), len(s.OneOf))
		for i, v := range oneOf {
			require.Equal(t, v.Schema(), s.OneOf[i])
		}
	})

	t.Run("const", func(t *testing.T) {
		constV := "example"
		s := String().
			Const(constV).
			Schema()
		require.Equal(t, constV, s.Const)
	})

	t.Run("enum", func(t *testing.T) {
		enum := []string{"a", "b", "c"}
		s := String().
			Enum(enum...).
			Schema()
		require.Equal(t, len(enum), len(s.Enum))
		for i, v := range enum {
			require.Equal(t, v, s.Enum[i])
		}
	})

	t.Run("maxLength", func(t *testing.T) {
		maxLength := json.Number("5")
		s := String().
			MaxLength(5).
			Schema()
		require.Equal(t, maxLength, s.MaxLength)
	})

	t.Run("minLength", func(t *testing.T) {
		minLength := json.Number("5")
		s := String().
			MinLength(5).
			Schema()
		require.Equal(t, minLength, s.MinLength)
	})

	t.Run("pattern", func(t *testing.T) {
		pattern := "example"
		s := String().
			Pattern(pattern).
			Schema()
		require.Equal(t, pattern, s.Pattern)
	})

	t.Run("title", func(t *testing.T) {
		title := "example"
		s := String().
			Title(title).
			Schema()
		require.Equal(t, title, s.Title)
	})

	t.Run("description", func(t *testing.T) {
		description := "example"
		s := String().
			Description(description).
			Schema()
		require.Equal(t, description, s.Description)
	})

	t.Run("default", func(t *testing.T) {
		defaultV := "example"
		s := String().
			Default(defaultV).
			Schema()
		require.Equal(t, defaultV, s.Default)
	})

	t.Run("deprecated", func(t *testing.T) {
		deprecated := false
		s := String().
			Deprecated(deprecated).
			Schema()
		require.Equal(t, deprecated, s.Deprecated)
	})

	t.Run("examples", func(t *testing.T) {
		examples := []string{"a", "b", "c"}
		s := String().
			Examples(examples...).
			Schema()
		require.Equal(t, len(examples), len(s.Examples))
		for i, v := range examples {
			require.Equal(t, v, s.Examples[i])
		}
	})

	t.Run("schema", func(t *testing.T) {
		s := String().
			Schema()
		require.Equal(t, "string", s.Type)
	})
}
