// Copyright 2016-2025, Pulumi Corporation.
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_WalkUpDirs(t *testing.T) {
	d1, err := filepath.Abs(t.TempDir())
	assert.NoError(t, err)
	d2 := filepath.Join(d1, "a")
	d3 := filepath.Join(d2, "b")
	d4 := filepath.Join(d3, "c")

	err = os.MkdirAll(d4, 0o700)
	require.NoError(t, err)

	walkUpDirsCollect := func(path string, checkDir func(dir string) bool) (string, []string) {
		var result []string
		found, err := WalkUpDirs(path, func(dir string) bool {
			result = append(result, dir)
			if checkDir == nil {
				return false
			}
			return checkDir(dir)
		})
		assert.NoError(t, err)
		return found, result
	}

	t.Run("walks from a sub-directory", func(t *testing.T) {
		_, d4Up := walkUpDirsCollect(d4, nil)
		assert.Contains(t, d4Up, d4)
		assert.Contains(t, d4Up, d3)
		assert.Contains(t, d4Up, d2)
		assert.Contains(t, d4Up, d1)
	})

	t.Run("walks from a file path", func(t *testing.T) {
		f1 := filepath.Join(d4, "hello.txt")
		err = os.WriteFile(f1, []byte("hello world"), 0o600)
		require.NoError(t, err)

		_, f1Up := walkUpDirsCollect(f1, nil)
		assert.Contains(t, f1Up, d4)
		assert.Contains(t, f1Up, d3)
		assert.Contains(t, f1Up, d2)
		assert.Contains(t, f1Up, d1)
	})

	t.Run("terminates the walk early if requested", func(t *testing.T) {
		f1 := filepath.Join(d4, "hello.txt")
		err = os.WriteFile(f1, []byte("hello world"), 0o600)
		require.NoError(t, err)

		found, visited := walkUpDirsCollect(f1, func(dir string) bool {
			return dir == d2
		})
		assert.Equal(t, d2, found)
		assert.Equal(t, visited, []string{d4, d3, d2})
	})
}
