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

package neo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func approveAll(_ ApprovalRequest) (bool, error) { return true, nil }
func denyAll(_ ApprovalRequest) (bool, error)    { return false, nil }

func newTestExecutor(t *testing.T) (*ToolExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	// On macOS, t.TempDir() returns /var/... but /var is a symlink to /private/var.
	// Resolve symlinks so path comparisons work correctly.
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	executor := NewToolExecutor(realDir, approveAll, nil)
	return executor, realDir
}

// --- resolvePath tests ---

func TestResolvePath_RelativePath(t *testing.T) {
	executor, dir := newTestExecutor(t)
	// Create a file so resolvePath can resolve symlinks.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))

	resolved, err := executor.resolvePath("test.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "test.txt"), resolved)
}

func TestResolvePath_AbsolutePathInsideWorkDir(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))

	resolved, err := executor.resolvePath(filepath.Join(dir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "test.txt"), resolved)
}

func TestResolvePath_TraversalBlocked(t *testing.T) {
	executor, _ := newTestExecutor(t)
	_, err := executor.resolvePath("../../../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestResolvePath_SymlinkTraversalBlocked(t *testing.T) {
	executor, dir := newTestExecutor(t)

	// Create a symlink that points outside the working directory.
	outsideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o644))
	require.NoError(t, os.Symlink(outsideDir, filepath.Join(dir, "escape")))

	_, err := executor.resolvePath("escape/secret.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")
}

func TestResolvePath_NewFileInExistingDir(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	// File doesn't exist yet, but parent does -- should succeed.
	resolved, err := executor.resolvePath("subdir/newfile.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "subdir", "newfile.txt"), resolved)
}

// --- read_file tests ---

func TestReadFile_Success(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world"), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "read_file",
		json.RawMessage(`{"path":"hello.txt"}`))

	assert.False(t, result.IsError)
	assert.Equal(t, "hello world", result.Content)
	assert.Equal(t, "tc_1", result.ToolCallID)
	assert.Equal(t, "read_file", result.Name)
}

func TestReadFile_NotFound(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "read_file",
		json.RawMessage(`{"path":"nonexistent.txt"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "no such file")
}

func TestReadFile_EmptyPath(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "read_file",
		json.RawMessage(`{"path":""}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "path is required")
}

func TestReadFile_TooLarge(t *testing.T) {
	executor, dir := newTestExecutor(t)
	// Create a file just over the limit.
	data := make([]byte, maxFileReadSize+1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "large.bin"), data, 0o644))

	result := executor.Execute(context.Background(), "tc_1", "read_file",
		json.RawMessage(`{"path":"large.bin"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "too large")
}

func TestReadFile_PathTraversal(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "read_file",
		json.RawMessage(`{"path":"../../../etc/passwd"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "outside the working directory")
}

// --- write_file tests ---

func TestWriteFile_Success(t *testing.T) {
	executor, dir := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "write_file",
		json.RawMessage(`{"path":"output.txt","content":"written content"}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "wrote")

	data, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	require.NoError(t, err)
	assert.Equal(t, "written content", string(data))
}

func TestWriteFile_CreatesSubdirectories(t *testing.T) {
	executor, dir := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "write_file",
		json.RawMessage(`{"path":"deep/nested/dir/file.txt","content":"deep content"}`))

	assert.False(t, result.IsError)

	data, err := os.ReadFile(filepath.Join(dir, "deep", "nested", "dir", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "deep content", string(data))
}

func TestWriteFile_EmptyPath(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "write_file",
		json.RawMessage(`{"path":"","content":"stuff"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "path is required")
}

func TestWriteFile_PathTraversal(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "write_file",
		json.RawMessage(`{"path":"../../evil.txt","content":"malicious"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "outside the working directory")
}

// --- execute_command tests ---

func TestExecuteCommand_Success(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "execute_command",
		json.RawMessage(`{"command":"echo hello"}`))

	assert.False(t, result.IsError)
	assert.Equal(t, "hello\n", result.Content)
}

func TestExecuteCommand_WithArgs(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "execute_command",
		json.RawMessage(`{"command":"echo","args":["hello","world"]}`))

	assert.False(t, result.IsError)
	assert.Equal(t, "hello world\n", result.Content)
}

func TestExecuteCommand_EmptyCommand(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "execute_command",
		json.RawMessage(`{"command":""}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "command is required")
}

func TestExecuteCommand_ContextCancelled(t *testing.T) {
	executor, _ := newTestExecutor(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := executor.Execute(ctx, "tc_1", "execute_command",
		json.RawMessage(`{"command":"sleep 10"}`))

	assert.True(t, result.IsError)
}

func TestExecuteCommand_OutputTruncation(t *testing.T) {
	executor, dir := newTestExecutor(t)

	// Create a file larger than maxCommandOutputSize, then cat it.
	bigFile := filepath.Join(dir, "big.txt")
	data := make([]byte, maxCommandOutputSize+1000)
	for i := range data {
		data[i] = 'A'
	}
	require.NoError(t, os.WriteFile(bigFile, data, 0o644))

	result := executor.Execute(context.Background(), "tc_1", "execute_command",
		json.RawMessage(fmt.Sprintf(`{"command":"cat %s"}`, bigFile)))

	// Output should be truncated to maxCommandOutputSize.
	assert.True(t, len(result.Content) <= maxCommandOutputSize+100, // allow for truncation message
		"output should be bounded; got %d bytes", len(result.Content))
	assert.Contains(t, result.Content, "truncated")
}

// --- search_files tests ---

func TestSearchFiles_Success(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "foo.txt"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bar.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "baz.txt"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":"*.txt"}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "foo.txt")
	assert.Contains(t, result.Content, "baz.txt")
	assert.NotContains(t, result.Content, "bar.go")
}

func TestSearchFiles_NoMatches(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":"*.xyz"}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "no matching files found")
}

func TestSearchFiles_WithIncludeFilter(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":"*.go","include":"*.go"}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "main.go")
	assert.NotContains(t, result.Content, "readme.md")
}

func TestSearchFiles_SkipsHiddenDirs(t *testing.T) {
	executor, dir := newTestExecutor(t)
	hiddenDir := filepath.Join(dir, ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "secret.txt"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible.txt"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":"*.txt"}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "visible.txt")
	assert.NotContains(t, result.Content, "secret.txt")
}

func TestSearchFiles_SkipsNodeModules(t *testing.T) {
	executor, dir := newTestExecutor(t)
	nmDir := filepath.Join(dir, "node_modules")
	require.NoError(t, os.MkdirAll(nmDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nmDir, "package.json"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":"*.json"}`))

	assert.False(t, result.IsError)
	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	assert.Len(t, lines, 1) // only the root package.json
}

func TestSearchFiles_EmptyPattern(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":""}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "pattern is required")
}

func TestSearchFiles_InvalidPattern(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "search_files",
		json.RawMessage(`{"pattern":"[invalid"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid pattern")
}

// --- directory_tree tests ---

func TestDirectoryTree_Success(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "directory_tree",
		json.RawMessage(`{}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "src/")
	assert.Contains(t, result.Content, "main.go")
	assert.Contains(t, result.Content, "README.md")
}

func TestDirectoryTree_SkipsHiddenDirs(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "visible.txt"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "directory_tree",
		json.RawMessage(`{}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "visible.txt")
	assert.NotContains(t, result.Content, ".git")
}

func TestDirectoryTree_DepthLimit(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b", "c", "d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "c", "d", "deep.txt"), []byte(""), 0o644))

	result := executor.Execute(context.Background(), "tc_1", "directory_tree",
		json.RawMessage(`{"depth":2}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "a/")
	assert.Contains(t, result.Content, "b/")
	assert.NotContains(t, result.Content, "deep.txt")
}

func TestDirectoryTree_EmptyDir(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "directory_tree",
		json.RawMessage(`{}`))

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "empty directory")
}

func TestDirectoryTree_NilArgs(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0o644))

	// Server may send empty/nil args — should default to cwd.
	result := executor.Execute(context.Background(), "tc_1", "directory_tree", nil)

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "file.txt")
}

// --- unknown tool ---

func TestUnknownTool(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "nonexistent_tool",
		json.RawMessage(`{}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "unknown tool")
}

// --- invalid JSON ---

func TestInvalidJSON(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "read_file",
		json.RawMessage(`not valid json`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid")
}

// --- git commands ---

func TestGitStatus_InNonGitDir(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "tc_1", "git_status",
		json.RawMessage(`{}`))

	// Should fail since temp dir is not a git repo.
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "git")
}

// --- Execute return structure ---

func TestExecute_ResponseFields(t *testing.T) {
	executor, dir := newTestExecutor(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644))

	result := executor.Execute(context.Background(), "call-123", "read_file",
		json.RawMessage(`{"path":"f.txt"}`))

	assert.Equal(t, "tool_response", result.Type)
	assert.Equal(t, "call-123", result.ToolCallID)
	assert.Equal(t, "read_file", result.Name)
	assert.False(t, result.IsError)
	assert.Equal(t, "x", result.Content)
}

func TestExecute_ErrorResponseFields(t *testing.T) {
	executor, _ := newTestExecutor(t)

	result := executor.Execute(context.Background(), "call-456", "unknown_tool",
		json.RawMessage(`{}`))

	assert.Equal(t, "tool_response", result.Type)
	assert.Equal(t, "call-456", result.ToolCallID)
	assert.Equal(t, "unknown_tool", result.Name)
	assert.True(t, result.IsError)
}
