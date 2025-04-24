// Copyright 2016-2024, Pulumi Corporation.
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

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	t.Parallel()
	t.Run("File", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello, world"), 0o600)
		require.NoError(t, err)

		err = CopyFile(dst, src, nil)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dst, "file.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello, world", string(data))
	})
	t.Run("Folder", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		err := os.MkdirAll(filepath.Join(src, "folder"), 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(src, "folder", "file.txt"), []byte("hello, world"), 0o600)
		require.NoError(t, err)

		err = CopyFile(dst, src, nil)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dst, "folder", "file.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello, world", string(data))
	})
	t.Run("File link", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello, world"), 0o600)
		require.NoError(t, err)

		err = os.Symlink(filepath.Join(src, "file.txt"), filepath.Join(src, "new.txt"))
		require.NoError(t, err)

		err = CopyFile(dst, src, nil)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dst, "new.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello, world", string(data))
	})
	t.Run("Folder link", func(t *testing.T) {
		t.Parallel()

		src := t.TempDir()
		dst := t.TempDir()

		err := os.MkdirAll(filepath.Join(src, "folder"), 0o755)
		require.NoError(t, err)

		err = os.Symlink(filepath.Join(src, "folder"), filepath.Join(src, "new"))
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(src, "folder", "file.txt"), []byte("hello, world"), 0o600)
		require.NoError(t, err)

		err = CopyFile(dst, src, nil)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dst, "new", "file.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello, world", string(data))
	})
}
