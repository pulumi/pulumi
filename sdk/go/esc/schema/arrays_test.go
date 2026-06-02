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

func TestArray(t *testing.T) {
	t.Run("defs", func(t *testing.T) {
		defs := map[string]Builder{
			"bool": Boolean(),
		}
		s := Array().
			Defs(defs).
			Schema()
		require.Equal(t, len(defs), len(s.Defs))
		for key, def := range defs {
			require.Contains(t, s.Defs, key)
			require.Equal(t, def.Schema(), s.Defs[key].Schema())
		}
	})

	t.Run("ref", func(t *testing.T) {
		ref := "example"
		s := Array().
			Ref(ref).
			Schema()
		require.Equal(t, s.Ref, ref)
	})

	t.Run("anyOf", func(t *testing.T) {
		anyOf := []Builder{
			Boolean(),
			String(),
		}
		s := Array().
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
		s := Array().
			OneOf(oneOf...).
			Schema()
		require.Equal(t, len(oneOf), len(s.OneOf))
		for i, v := range oneOf {
			require.Equal(t, v.Schema(), s.OneOf[i])
		}
	})

	t.Run("prefixItems", func(t *testing.T) {
		items := []Builder{
			Boolean(),
			String(),
		}
		s := Array().
			PrefixItems(items...).
			Schema()
		require.Equal(t, len(items), len(s.PrefixItems))
		for i, v := range items {
			require.Equal(t, v.Schema(), s.PrefixItems[i])
		}
	})

	t.Run("minItems", func(t *testing.T) {
		minItems := json.Number("5")
		s := Array().
			MinItems(5).
			Schema()
		require.Equal(t, minItems, s.MinItems)
	})

	t.Run("maxItems", func(t *testing.T) {
		maxItems := json.Number("5")
		s := Array().
			MaxItems(5).
			Schema()
		require.Equal(t, maxItems, s.MaxItems)
	})

	t.Run("uniqueItems", func(t *testing.T) {
		uniqueItems := true
		s := Array().
			UniqueItems(uniqueItems).
			Schema()
		require.Equal(t, uniqueItems, s.UniqueItems)
	})

	t.Run("title", func(t *testing.T) {
		title := "example"
		s := Array().
			Title(title).
			Schema()
		require.Equal(t, title, s.Title)
	})

	t.Run("description", func(t *testing.T) {
		description := "example"
		s := Array().
			Description(description).
			Schema()
		require.Equal(t, description, s.Description)
	})

	t.Run("default", func(t *testing.T) {
		defaultV := []any{1, 2, 3}
		s := Array().
			Default(defaultV).
			Schema()
		require.Equal(t, defaultV, s.Default)
	})

	t.Run("deprecated", func(t *testing.T) {
		deprecated := false
		s := Array().
			Deprecated(deprecated).
			Schema()
		require.Equal(t, deprecated, s.Deprecated)
	})

	t.Run("examples", func(t *testing.T) {
		examples := [][]any{
			{1, 2, 3},
			{4, 5, 6},
		}
		s := Array().
			Examples(examples...).
			Schema()
		require.Equal(t, len(examples), len(s.Examples))
		for i, v := range examples {
			require.Equal(t, v, s.Examples[i])
		}
	})

	t.Run("schema", func(t *testing.T) {
		s := Array().
			Schema()
		require.Equal(t, "array", s.Type)
	})
}
