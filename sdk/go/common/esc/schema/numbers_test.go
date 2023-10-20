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

func TestNumbers(t *testing.T) {
	t.Run("ref", func(t *testing.T) {
		ref := "example"
		s := Number().
			Ref(ref).
			Schema()
		require.Equal(t, s.Ref, ref)
	})

	t.Run("anyOf", func(t *testing.T) {
		anyOf := []Builder{
			Boolean(),
			String(),
		}
		s := Number().
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
		s := Number().
			OneOf(oneOf...).
			Schema()
		require.Equal(t, len(oneOf), len(s.OneOf))
		for i, v := range oneOf {
			require.Equal(t, v.Schema(), s.OneOf[i])
		}
	})

	t.Run("const", func(t *testing.T) {
		constV := json.Number("5")
		s := Number().
			Const(constV).
			Schema()
		require.Equal(t, constV, s.Const)
	})

	t.Run("enum", func(t *testing.T) {
		enum := []json.Number{
			json.Number("1"),
			json.Number("2"),
		}
		s := Number().
			Enum(enum...).
			Schema()
		require.Equal(t, len(enum), len(s.Enum))
		for i, v := range enum {
			require.Equal(t, v, s.Enum[i])
		}
	})

	t.Run("multipleOf", func(t *testing.T) {
		multipleOf := json.Number("5")
		s := Number().
			MultipleOf(multipleOf).
			Schema()
		require.Equal(t, multipleOf, s.MultipleOf)
	})

	t.Run("maximum", func(t *testing.T) {
		maximum := json.Number("5")
		s := Number().
			Maximum(maximum).
			Schema()
		require.Equal(t, maximum, s.Maximum)
	})

	t.Run("exclusiveMaximum", func(t *testing.T) {
		exclusiveMaximum := json.Number("5")
		s := Number().
			ExclusiveMaximum(exclusiveMaximum).
			Schema()
		require.Equal(t, exclusiveMaximum, s.ExclusiveMaximum)
	})

	t.Run("minimum", func(t *testing.T) {
		minimum := json.Number("5")
		s := Number().
			Minimum(minimum).
			Schema()
		require.Equal(t, minimum, s.Minimum)
	})

	t.Run("exclusiveMinimum", func(t *testing.T) {
		exclusiveMinimum := json.Number("5")
		s := Number().
			ExclusiveMinimum(exclusiveMinimum).
			Schema()
		require.Equal(t, exclusiveMinimum, s.ExclusiveMinimum)
	})

	t.Run("title", func(t *testing.T) {
		title := "example"
		s := Number().
			Title(title).
			Schema()
		require.Equal(t, title, s.Title)
	})

	t.Run("description", func(t *testing.T) {
		description := "example"
		s := Number().
			Description(description).
			Schema()
		require.Equal(t, description, s.Description)
	})

	t.Run("default", func(t *testing.T) {
		defaultV := json.Number("5")
		s := Number().
			Default(defaultV).
			Schema()
		require.Equal(t, defaultV, s.Default)
	})

	t.Run("deprecated", func(t *testing.T) {
		deprecated := false
		s := Number().
			Deprecated(deprecated).
			Schema()
		require.Equal(t, deprecated, s.Deprecated)
	})

	t.Run("examples", func(t *testing.T) {
		examples := []json.Number{
			json.Number("1"),
			json.Number("2"),
		}
		s := Number().
			Examples(examples...).
			Schema()
		require.Equal(t, len(examples), len(s.Examples))
		for i, v := range examples {
			require.Equal(t, v, s.Examples[i])
		}
	})

	t.Run("schema", func(t *testing.T) {
		s := Number().
			Schema()
		require.Equal(t, "number", s.Type)
	})
}
