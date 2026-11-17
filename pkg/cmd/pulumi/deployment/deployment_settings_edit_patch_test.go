// Copyright 2026, Pulumi Corporation.
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

package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetNested(t *testing.T) {
	t.Parallel()

	t.Run("creates intermediate maps", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{}
		setNested(m, []string{"a", "b", "c"}, "v")
		assert.Equal(t, map[string]any{
			"a": map[string]any{"b": map[string]any{"c": "v"}},
		}, m)
	})

	t.Run("single-element path", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{}
		setNested(m, []string{"k"}, 42)
		assert.Equal(t, map[string]any{"k": 42}, m)
	})

	t.Run("preserves siblings", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{
			"a": map[string]any{
				"existing": "leaf",
				"nested":   map[string]any{"k1": "v1"},
			},
			"top": "scalar",
		}
		setNested(m, []string{"a", "nested", "k2"}, "v2")
		assert.Equal(t, map[string]any{
			"a": map[string]any{
				"existing": "leaf",
				"nested":   map[string]any{"k1": "v1", "k2": "v2"},
			},
			"top": "scalar",
		}, m)
	})

	t.Run("overwrites leaf", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{"k": "old"}
		setNested(m, []string{"k"}, "new")
		assert.Equal(t, map[string]any{"k": "new"}, m)
	})

	t.Run("replaces non-map intermediate", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{"a": "scalar"}
		setNested(m, []string{"a", "b"}, "v")
		assert.Equal(t, map[string]any{"a": map[string]any{"b": "v"}}, m)
	})

	t.Run("accepts nil leaf value", func(t *testing.T) {
		t.Parallel()
		m := map[string]any{}
		setNested(m, []string{"operationContext", "environmentVariables", "STALE"}, nil)
		assert.Equal(t, map[string]any{
			"operationContext": map[string]any{
				"environmentVariables": map[string]any{"STALE": nil},
			},
		}, m)
	})
}
