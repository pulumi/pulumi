// Copyright 2024-2025, Pulumi Corporation.
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

package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestUvVirtualenvPath(t *testing.T) {
	t.Parallel()

	t.Run("no virtualenv specified", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		uv, err := newUv(root, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".venv"), uv.virtualenvPath, "virtualenv is in the project root")
	})

	t.Run("no virtualenv specified, in a subfolder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte{}, 0o600))
		require.NoError(t, os.Mkdir(pulumiRoot, 0o755))

		uv, err := newUv(pulumiRoot, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".venv"), uv.virtualenvPath, "virtualenv is next to uv.lock")
	})

	t.Run("no virtualenv specified, in a subfolder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.Mkdir(pulumiRoot, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "uv.lock"), []byte{}, 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(pulumiRoot, "uv.lock"), []byte{}, 0o600))

		uv, err := newUv(pulumiRoot, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(pulumiRoot, ".venv"), uv.virtualenvPath,
			"virtualenv is next to the uv.lock closest to the project root")
	})

	t.Run("virtualenv option is provided", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		uv, err := newUv(root, "banana")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, "banana"), uv.virtualenvPath, "virtualenv is in the project root")
	})

	t.Run("virtualenv option is provided, in  subfolder", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		pulumiRoot := filepath.Join(root, "subfolder")
		require.NoError(t, os.Mkdir(pulumiRoot, 0o755))

		uv, err := newUv(pulumiRoot, "banana")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(pulumiRoot, "banana"), uv.virtualenvPath, "virtualenv is in the project root")
	})
}

func TestUvVersion(t *testing.T) {
	t.Parallel()

	for _, versionString := range []string{
		"uv 0.4.26",
		"uv 0.4.26 (Homebrew 2024-10-23)",
		"uv 0.4.26 (d2cd09bbd 2024-10-25)",
	} {
		v, err := ParseUvVersion(versionString)
		require.NoError(t, err)
		require.Equal(t, semver.MustParse("0.4.26"), v)
	}

	_, err := ParseUvVersion("uv 0.4.25")
	require.ErrorContains(t, err, "less than the minimum required version")
}

func TestUvCommandSyncsEnvironment(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject, err := os.ReadFile(filepath.Join("testdata", "project", "pyproject.toml"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(root, "pyproject.toml"), pyproject, 0o600)
	require.NoError(t, err)

	uv, err := newUv(root, "")
	require.NoError(t, err)

	// Run a python command, this should run `uv sync` as side effect
	cmd, err := uv.Command(context.Background(), "-c", "print('hello')")
	require.NoError(t, err)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, "hello", strings.TrimSpace(string(out)))

	// check that .venv exists
	require.DirExists(t, filepath.Join(root, ".venv"))

	// `wheel`, the project's dependency, should be installed
	cmd, err = uv.ModuleCommand(context.Background(), "wheel", "version")
	require.NoError(t, err)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err)
	require.True(t, strings.Contains(string(out), "wheel"), "unexpected output: %s", out)
}

func TestUvCommandSyncsEnvironmentCustomVenv(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pyproject, err := os.ReadFile(filepath.Join("testdata", "project", "pyproject.toml"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(root, "pyproject.toml"), pyproject, 0o600)
	require.NoError(t, err)

	uv, err := newUv(root, "my_venv")
	require.NoError(t, err)

	// Run a python command, this should run `uv sync` as side effect
	cmd, err := uv.Command(context.Background(), "-c", "print('hello')")
	require.NoError(t, err)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, "hello", strings.TrimSpace(string(out)))

	// check that my_venv exists
	require.DirExists(t, filepath.Join(root, "my_venv"))

	// `wheel`, the project's dependency, should be installed
	cmd, err = uv.ModuleCommand(context.Background(), "wheel", "version")
	require.NoError(t, err)
	out, err = cmd.CombinedOutput()
	require.NoError(t, err)
	require.True(t, strings.Contains(string(out), "wheel"), "unexpected output: %s", out)
}
