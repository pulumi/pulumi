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

package packagecmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests mutate the process working directory, so they must not run in parallel.
func TestEffectiveLang(t *testing.T) {
	t.Run("explicit flag wins", func(t *testing.T) {
		assert.Equal(t, "python", effectiveLang("python"))
	})

	t.Run("defaults to go outside a project", func(t *testing.T) {
		tmp := t.TempDir()
		orig, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmp))
		t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck

		assert.Equal(t, "go", effectiveLang(""))
	})

	t.Run("detects language from Pulumi.yaml", func(t *testing.T) {
		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "Pulumi.yaml"),
			[]byte("name: test\nruntime: python\n"),
			0o644,
		))
		orig, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmp))
		t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck

		assert.Equal(t, "python", effectiveLang(""))
	})

	t.Run("explicit flag overrides Pulumi.yaml", func(t *testing.T) {
		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(tmp, "Pulumi.yaml"),
			[]byte("name: test\nruntime: python\n"),
			0o644,
		))
		orig, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmp))
		t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck

		assert.Equal(t, "java", effectiveLang("java"))
	})
}

func TestRuntimeToLang(t *testing.T) {
	t.Parallel()

	tests := []struct {
		runtime  string
		expected string
	}{
		{"nodejs", "typescript"},
		{"dotnet", "csharp"},
		{"go", "go"},
		{"python", "python"},
		{"yaml", "yaml"},
		{"java", "java"},
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			t.Parallel()
			lang, ok := runtimeToLang[tt.runtime]
			assert.True(t, ok, "runtime %q should be in runtimeToLang", tt.runtime)
			assert.Equal(t, tt.expected, lang)
		})
	}

	t.Run("unknown runtime not in map", func(t *testing.T) {
		t.Parallel()
		_, ok := runtimeToLang["rust"]
		assert.False(t, ok)
	})
}

func TestDocsOpts(t *testing.T) {
	t.Run("with explicit lang", func(t *testing.T) {
		t.Parallel()
		opts := docsOpts("python", "linux", "Bucket")
		assert.Equal(t, "python", opts.Lang)
		assert.Equal(t, "linux", opts.OS)
		assert.Equal(t, "Bucket", opts.Query)
	})

	t.Run("lang is never empty", func(t *testing.T) {
		tmp := t.TempDir()
		orig, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmp))
		t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck

		opts := docsOpts("", "", "")
		assert.Equal(t, "go", opts.Lang, "lang should default to 'go' outside a project")
	})
}
