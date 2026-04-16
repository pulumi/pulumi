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
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), nil, 0o600))
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

	outside := t.TempDir()
	target := filepath.Join(outside, "passwd")
	require.NoError(t, os.WriteFile(target, nil, 0o600))

	_, err = fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, target)))
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

func TestFilesystem_RejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "passwd"), nil, 0o600))

	link := filepath.Join(root, "escape")
	require.NoError(t, os.Symlink(outside, link))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, filepath.Join(root, "escape", "passwd"))))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestFilesystem_ReadOffsetBeyondFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "small.txt")
	require.NoError(t, os.WriteFile(target, []byte("line1\nline2\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q,"offset":999}`, target)))
	require.NoError(t, err)
	assert.Equal(t, "", res.(map[string]any)["content"])
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

func TestNewFilesystem_RejectsMissingRoot(t *testing.T) {
	t.Parallel()

	_, err := NewFilesystem(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestNewFilesystem_RejectsFileRoot(t *testing.T) {
	t.Parallel()

	// The root must be a directory — pointing it at a regular file should be caught
	// before any tool call runs, otherwise callers would leak undefined behavior.
	file := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	_, err := NewFilesystem(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestFilesystem_UnknownMethodReturnsError(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "delete", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown filesystem method")
}

func TestFilesystem_InvokeRejectsMalformedArgs(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	for _, method := range []string{"read", "write", "directory_tree"} {
		_, err := fs.Invoke(t.Context(), method, json.RawMessage(`{`))
		require.Error(t, err, method)
		assert.Contains(t, err.Error(), "decoding", method)
	}
}

func TestFilesystem_InvokeRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	// `resolve` rejects empty paths; this exercises the guard through Invoke so the
	// user-visible error mentions the missing path rather than some lower-level
	// syscall failure.
	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "read", json.RawMessage(`{"file_path":""}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")
}

func TestFilesystem_ReadWithLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "big.txt")
	require.NoError(t, os.WriteFile(target, []byte("a\nb\nc\nd\ne\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q,"limit":2}`, target)))
	require.NoError(t, err)
	assert.Equal(t, "a\nb", res.(map[string]any)["content"])
}

func TestFilesystem_ReadWithOffsetAndLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "big.txt")
	require.NoError(t, os.WriteFile(target, []byte("a\nb\nc\nd\ne\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q,"offset":1,"limit":2}`, target)))
	require.NoError(t, err)
	assert.Equal(t, "b\nc", res.(map[string]any)["content"])
}

func TestFilesystem_ReadMissingFileReturnsError(t *testing.T) {
	t.Parallel()

	// Inside the sandbox, but the file itself doesn't exist — os.ReadFile should
	// surface a clear error (not a silent empty-content result).
	root := t.TempDir()
	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "read",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, filepath.Join(root, "missing.txt"))))
	require.Error(t, err)
}

func TestFilesystem_WriteRejectsPathOutsideRoot(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	outside := filepath.Join(t.TempDir(), "evil.txt")
	_, err = fs.Invoke(t.Context(), "write",
		json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"x"}`, outside)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")

	_, statErr := os.Stat(outside)
	assert.True(t, os.IsNotExist(statErr), "rejected write must not touch the filesystem")
}

func TestFilesystem_DirectoryTreeDepthTwo(t *testing.T) {
	t.Parallel()

	// depth=2 should descend one level below the root entry but stop before
	// grandchildren so the agent can iterate the tree incrementally.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "file.txt"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "b", "deep.txt"), nil, 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "directory_tree",
		json.RawMessage(fmt.Sprintf(`{"path":%q,"depth":2}`, root)))
	require.NoError(t, err)
	entries := res.(map[string]any)["entries"].([]map[string]any)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e["name"].(string))
	}
	assert.Contains(t, names, "a")
	assert.Contains(t, names, filepath.Join("a", "b"))
	assert.Contains(t, names, filepath.Join("a", "file.txt"))
	assert.NotContains(t, names, filepath.Join("a", "b", "deep.txt"))
}

func TestFilesystem_DirectoryTreeMissingPathReturnsError(t *testing.T) {
	t.Parallel()

	// An absent path inside the sandbox still has to surface an error rather than
	// an empty tree, which would look like "this directory is empty" to the agent.
	root := t.TempDir()
	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "directory_tree",
		json.RawMessage(fmt.Sprintf(`{"path":%q}`, filepath.Join(root, "nope"))))
	require.Error(t, err)
}

func TestFilesystem_WriteRelativePathResolvesAgainstRoot(t *testing.T) {
	t.Parallel()

	// The agent may send paths that aren't absolute (they are, in its sandbox, but
	// not on this host). These must land inside Root, not wherever the CLI was
	// launched from.
	root := t.TempDir()
	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "write",
		json.RawMessage(`{"file_path":"nested/child.txt","content":"hi"}`))
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(root, "nested", "child.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hi", string(got))
}
