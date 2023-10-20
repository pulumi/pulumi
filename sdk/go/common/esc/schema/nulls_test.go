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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNulls(t *testing.T) {
	t.Run("ref", func(t *testing.T) {
		ref := "example"
		s := Null().
			Ref(ref).
			Schema()
		require.Equal(t, s.Ref, ref)
	})

	t.Run("anyOf", func(t *testing.T) {
		anyOf := []Builder{
			Boolean(),
			String(),
		}
		s := Null().
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
		s := Null().
			OneOf(oneOf...).
			Schema()
		require.Equal(t, len(oneOf), len(s.OneOf))
		for i, v := range oneOf {
			require.Equal(t, v.Schema(), s.OneOf[i])
		}
	})

	t.Run("title", func(t *testing.T) {
		title := "example"
		s := Null().
			Title(title).
			Schema()
		require.Equal(t, title, s.Title)
	})

	t.Run("description", func(t *testing.T) {
		description := "example"
		s := Null().
			Description(description).
			Schema()
		require.Equal(t, description, s.Description)
	})

	t.Run("deprecated", func(t *testing.T) {
		deprecated := false
		s := Null().
			Deprecated(deprecated).
			Schema()
		require.Equal(t, deprecated, s.Deprecated)
	})

	t.Run("schema", func(t *testing.T) {
		s := Null().
			Schema()
		require.Equal(t, "null", s.Type)
	})
}
