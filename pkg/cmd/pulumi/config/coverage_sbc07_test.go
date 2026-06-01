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
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestParseEjectedEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("extracts config, imports, and other values", func(t *testing.T) {
		t.Parallel()
		def := []byte(`imports:
  - shared
  - structured:
      merge: false
values:
  pulumiConfig:
    proj:key: v
  environmentVariables:
    FOO: bar
`)
		pulumiConfig, imports, structured, otherValues, err := parseEjectedEnvironment(def)
		require.NoError(t, err)
		require.Equal(t, "v", pulumiConfig["proj:key"])
		require.Equal(t, []string{"shared", "structured"}, imports)
		require.Equal(t, []string{"structured"}, structured)
		require.Equal(t, []string{"environmentVariables"}, otherValues)
	})

	t.Run("rejects an unparseable definition", func(t *testing.T) {
		t.Parallel()
		_, _, _, _, err := parseEjectedEnvironment([]byte("\tnot: [valid"))
		require.ErrorContains(t, err, "parsing environment definition")
	})

	t.Run("rejects a non-map pulumiConfig", func(t *testing.T) {
		t.Parallel()
		_, _, _, _, err := parseEjectedEnvironment([]byte("values:\n  pulumiConfig: scalar\n"))
		require.ErrorContains(t, err, "parsing values.pulumiConfig")
	})
}

func TestEscValueToPlaintext(t *testing.T) {
	t.Parallel()

	t.Run("null is a non-secret empty plaintext", func(t *testing.T) {
		t.Parallel()
		pt, err := escValueToPlaintext(nil)
		require.NoError(t, err)
		require.False(t, pt.Secure())
	})

	t.Run("fn::secret marker becomes a secure plaintext", func(t *testing.T) {
		t.Parallel()
		pt, err := escValueToPlaintext(map[string]any{"fn::secret": "shh"})
		require.NoError(t, err)
		require.True(t, pt.Secure())
		require.Equal(t, config.PlaintextSecret("shh"), pt.Value())
	})

	t.Run("scalars preserve their type", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			in   any
			want any
		}{
			{true, true},
			{int(5), int64(5)},
			{int64(6), int64(6)},
			{uint64(7), uint64(7)},
			{1.5, 1.5},
			{"s", "s"},
			{uint(9), "9"}, // default branch renders unrecognized types
		}
		for _, c := range cases {
			pt, err := escValueToPlaintext(c.in)
			require.NoError(t, err)
			require.Equal(t, c.want, pt.Value())
		}
	})

	t.Run("nested map and array recurse", func(t *testing.T) {
		t.Parallel()
		pt, err := escValueToPlaintext(map[string]any{"a": "x", "b": []any{1, 2}})
		require.NoError(t, err)
		require.IsType(t, map[string]config.Plaintext{}, pt.Value())

		arr, err := escValueToPlaintext([]any{"a", "b"})
		require.NoError(t, err)
		require.IsType(t, []config.Plaintext{}, arr.Value())
	})
}

func TestSecretInnerString(t *testing.T) {
	t.Parallel()

	s, err := secretInnerString("plain")
	require.NoError(t, err)
	require.Equal(t, "plain", s)

	s, err = secretInnerString(42)
	require.NoError(t, err)
	require.Equal(t, "42", s)

	s, err = secretInnerString(map[string]any{"a": 1})
	require.NoError(t, err)
	require.JSONEq(t, `{"a":1}`, s)
}

func TestBuildPlaintextMap(t *testing.T) {
	t.Parallel()

	m, err := buildPlaintextMap(map[string]any{
		"proj:plain":  "v",
		"proj:secret": map[string]any{"fn::secret": "shh"},
	})
	require.NoError(t, err)
	require.False(t, m[config.MustMakeKey("proj", "plain")].Secure())
	require.True(t, m[config.MustMakeKey("proj", "secret")].Secure())
}

func TestAtomicWriteProjectStack(t *testing.T) {
	t.Parallel()

	t.Run("writes the file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "Pulumi.stack.yaml")
		require.NoError(t, atomicWriteProjectStack(&workspace.ProjectStack{}, path))
		_, err := os.Stat(path)
		require.NoError(t, err)
	})

	t.Run("rejects an unknown file extension", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "stack.unknown")
		require.ErrorContains(t, atomicWriteProjectStack(&workspace.ProjectStack{}, path), "no marshaler found")
	})
}

func TestEjectIsNotFound(t *testing.T) {
	t.Parallel()
	require.True(t, isNotFound(&apitype.ErrorResponse{Code: http.StatusNotFound}))
	require.False(t, isNotFound(&apitype.ErrorResponse{Code: http.StatusBadRequest}))
	require.False(t, isNotFound(errors.New("plain")))
}
