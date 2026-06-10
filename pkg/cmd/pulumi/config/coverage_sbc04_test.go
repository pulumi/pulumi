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

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestESCConfigEditorImports(t *testing.T) {
	t.Parallel()

	t.Run("absent imports", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "values:\n  pulumiConfig: {}\n", nil)
		require.Empty(t, e.Imports())
	})

	t.Run("plain and structured entries", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "imports:\n  - shared\n  - other:\n      merge: false\nvalues: {}\n", nil)
		require.Equal(t, []string{"shared", "other"}, e.Imports())
	})

	t.Run("non-name entries are skipped", func(t *testing.T) {
		t.Parallel()
		// A multi-key mapping and a nested sequence are not valid single-name import entries, so they
		// are reported as no entries rather than guessed at.
		e := newESCEditor(t, "imports:\n  - a: 1\n    b: 2\n  - - nested\n", nil)
		require.Empty(t, e.Imports())
	})

	t.Run("imports key present but not a sequence", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "imports: scalar\n", nil)
		require.Empty(t, e.Imports())
	})
}

func TestESCConfigEditorAddImports(t *testing.T) {
	t.Parallel()

	t.Run("creates the sequence when absent", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "values:\n  pulumiConfig: {}\n", nil)
		require.NoError(t, e.AddImports("env1", "env2"))
		require.Equal(t, []string{"env1", "env2"}, e.Imports())
	})

	t.Run("appends to an existing sequence in order, no dedup", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "imports:\n  - existing\n", nil)
		require.NoError(t, e.AddImports("existing", "new"))
		require.Equal(t, []string{"existing", "existing", "new"}, e.Imports())
	})

	t.Run("refuses a malformed non-sequence imports", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "imports: notasequence\n", nil)
		require.ErrorContains(t, e.AddImports("env1"), "not a sequence")
	})
}

func TestESCConfigEditorRemoveImport(t *testing.T) {
	t.Parallel()

	t.Run("removes the last matching entry", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "imports:\n  - a\n  - b\n  - a\n", nil)
		require.NoError(t, e.RemoveImport("a"))
		require.Equal(t, []string{"a", "b"}, e.Imports())
	})

	t.Run("absent entry is a no-op", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "imports:\n  - a\n", nil)
		require.NoError(t, e.RemoveImport("missing"))
		require.Equal(t, []string{"a"}, e.Imports())
	})

	t.Run("no imports sequence is a no-op", func(t *testing.T) {
		t.Parallel()
		e := newESCEditor(t, "values: {}\n", nil)
		require.NoError(t, e.RemoveImport("a"))
	})
}
