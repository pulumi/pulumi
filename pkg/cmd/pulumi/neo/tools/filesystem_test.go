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

package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesystem_WriteThenReadAbsolutePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	target := filepath.Join(root, "sub", "a.txt")

	_, err = fs.Invoke(t.Context(), "write",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"hello"}`, target)))
	require.NoError(t, err)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))

	res, err := fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, target)))
	require.NoError(t, err)
	assert.Equal(t, "hello", res.(map[string]any)["content"])
}

func TestFilesystem_DirectoryTreeShallow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), nil, 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0o755))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "directory_tree",
		json.RawMessage(fmt.Sprintf(`{"path":%q}`, root)))
	require.NoError(t, err)
	entries := res.(map[string]any)["entries"].([]map[string]any)
	require.Len(t, entries, 2)
	assert.Equal(t, "a.txt", entries[0]["name"])
	assert.Equal(t, "sub", entries[1]["name"])
}

func TestFilesystem_RejectsAbsolutePathOutsideRoot(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "read", json.RawMessage(`{"file_path":"/etc/passwd"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestFilesystem_RejectsRelativeEscape(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "read", json.RawMessage(`{"file_path":"../etc/passwd"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestFilesystem_UnimplementedMethodsReturnClearError(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	for _, method := range []string{"edit", "multi_edit", "grep", "grep_ast", "content_replace"} {
		_, err := fs.Invoke(t.Context(), method, json.RawMessage(`{}`))
		require.Error(t, err, method)
		assert.Contains(t, err.Error(), "not yet implemented", method)
	}
}
