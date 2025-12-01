// Copyright 2016-2023, Pulumi Corporation.
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

package afero

import (
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyDirWorksWithFilters(t *testing.T) {
	t.Parallel()
	content := []byte("hello world")
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "/src/file.txt", content, 0o644)
	require.NoError(t, err)
	// filter always returns false so nothing is copied
	err = CopyDir(fs, "/src", "/dst", func(f os.FileInfo) bool {
		return false
	})

	require.NoError(t, err)
	// assert that the file was not copied and thus should not exist
	_, err = fs.Open("/dst/file.txt")
	assert.True(t, os.IsNotExist(err))

	// now copy the file with a filter that always returns true
	err = CopyDir(fs, "/src", "/dst", func(f os.FileInfo) bool {
		return true
	})

	require.NoError(t, err)
	// check that the file was copied
	file, err := fs.Open("/dst/file.txt")
	require.NoError(t, err)
	defer file.Close()
	fileContent, err := afero.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, string(content), string(fileContent))
}
