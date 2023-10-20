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

func TestObjects(t *testing.T) {
	t.Run("record", func(t *testing.T) {
		record := map[string]Builder{
			"bool":   Boolean(),
			"string": String(),
		}
		s := Record(record).
			Schema()
		require.Equal(t, len(record), len(s.Properties))
		require.Equal(t, len(record), len(s.Required))
		for key, property := range record {
			require.Contains(t, s.Properties, key)
			require.Equal(t, property.Schema(), s.Properties[key].Schema())
			require.Contains(t, s.Required, key)
		}
	})

	t.Run("defs", func(t *testing.T) {
		defs := map[string]Builder{
			"bool": Boolean(),
		}
		s := Object().
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
		s := Object().
			Ref(ref).
			Schema()
		require.Equal(t, s.Ref, ref)
	})

	t.Run("anyOf", func(t *testing.T) {
		anyOf := []Builder{
			Boolean(),
			String(),
		}
		s := Object().
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
		s := Object().
			OneOf(oneOf...).
			Schema()
		require.Equal(t, len(oneOf), len(s.OneOf))
		for i, v := range oneOf {
			require.Equal(t, v.Schema(), s.OneOf[i])
		}
	})

	t.Run("properties", func(t *testing.T) {
		properties := map[string]Builder{
			"bool":   Boolean(),
			"string": String(),
		}
		s := Object().
			Properties(properties).
			Schema()
		require.Equal(t, len(properties), len(s.Properties))
		for i, v := range properties {
			require.Equal(t, v.Schema(), s.Properties[i])
		}
	})

	t.Run("additionalProperties", func(t *testing.T) {
		additionalProperties := Boolean()
		s := Object().
			AdditionalProperties(additionalProperties).
			Schema()
		require.Equal(t, additionalProperties.Schema(), s.AdditionalProperties)
	})

	t.Run("minProperties", func(t *testing.T) {
		minProperties := json.Number("5")
		s := Object().
			MinProperties(5).
			Schema()
		require.Equal(t, minProperties, s.MinProperties)
	})

	t.Run("maxProperties", func(t *testing.T) {
		maxProperties := json.Number("5")
		s := Object().
			MaxProperties(5).
			Schema()
		require.Equal(t, maxProperties, s.MaxProperties)
	})

	t.Run("required", func(t *testing.T) {
		required := []string{"a", "b"}
		s := Object().
			Required(required...).
			Schema()
		require.EqualValues(t, required, s.Required)
	})

	t.Run("dependentRequired", func(t *testing.T) {
		dependentRequired := map[string][]string{
			"a": {"A", "B"},
			"b": {"C", "D"},
		}
		s := Object().
			DependentRequired(dependentRequired).
			Schema()
		require.EqualValues(t, dependentRequired, s.DependentRequired)
	})

	t.Run("title", func(t *testing.T) {
		title := "example"
		s := Object().
			Title(title).
			Schema()
		require.Equal(t, title, s.Title)
	})

	t.Run("description", func(t *testing.T) {
		description := "example"
		s := Object().
			Description(description).
			Schema()
		require.Equal(t, description, s.Description)
	})

	t.Run("default", func(t *testing.T) {
		defaultV := map[string]any{
			"a": 1,
			"b": 2,
		}
		s := Object().
			Default(defaultV).
			Schema()
		require.Equal(t, defaultV, s.Default)
	})

	t.Run("deprecated", func(t *testing.T) {
		deprecated := false
		s := Object().
			Deprecated(deprecated).
			Schema()
		require.Equal(t, deprecated, s.Deprecated)
	})

	t.Run("examples", func(t *testing.T) {
		examples := []map[string]any{
			{"a": 1, "b": 2},
			{"c": 3, "d": 4},
		}
		s := Object().
			Examples(examples...).
			Schema()
		require.Equal(t, len(examples), len(s.Examples))
		for i, v := range examples {
			require.Equal(t, v, s.Examples[i])
		}
	})

	t.Run("schema", func(t *testing.T) {
		s := Object().
			Schema()
		require.Equal(t, "object", s.Type)
	})
}
