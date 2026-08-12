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

package npm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadPackageManifest(t *testing.T) {
	t.Parallel()

	t.Run("reads package.json", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"),
			[]byte(`{"name":"foo","version":"1.2.3"}`), 0o600))

		data, path, err := ReadPackageManifest(dir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "package.json"), path)
		require.Equal(t, "foo", data["name"])
		require.Equal(t, "1.2.3", data["version"])
	})

	t.Run("reads package.yaml", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"),
			[]byte("name: foo\nversion: 1.2.3\n"), 0o600))

		data, path, err := ReadPackageManifest(dir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "package.yaml"), path)
		require.Equal(t, "foo", data["name"])
		require.Equal(t, "1.2.3", data["version"])
	})

	t.Run("prefers package.json when both exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"),
			[]byte(`{"name":"from-json"}`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"),
			[]byte("name: from-yaml\n"), 0o600))

		data, path, err := ReadPackageManifest(dir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "package.json"), path)
		require.Equal(t, "from-json", data["name"])
	})

	t.Run("returns ErrNotExist when neither exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		_, _, err := ReadPackageManifest(dir)
		require.Error(t, err)
		require.True(t, errors.Is(err, os.ErrNotExist))
	})

	t.Run("errors on malformed package.json", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"),
			[]byte(`{not json`), 0o600))

		_, _, err := ReadPackageManifest(dir)
		require.Error(t, err)
	})

	t.Run("errors on malformed package.yaml", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"),
			[]byte("not: : yaml: ::\n"), 0o600))

		_, _, err := ReadPackageManifest(dir)
		require.Error(t, err)
	})
}

func TestSearchupPackageManifest(t *testing.T) {
	t.Parallel()

	t.Run("finds package.json in cwd", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o600))

		path, err := SearchupPackageManifest(dir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "package.json"), path)
	})

	t.Run("finds package.yaml in cwd", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"), []byte("{}\n"), 0o600))

		path, err := SearchupPackageManifest(dir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "package.yaml"), path)
	})

	t.Run("finds manifest in a parent directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.yaml"), []byte("{}\n"), 0o600))
		sub := filepath.Join(dir, "sub", "deeper")
		require.NoError(t, os.MkdirAll(sub, 0o700))

		path, err := SearchupPackageManifest(sub)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "package.yaml"), path)
	})
}
