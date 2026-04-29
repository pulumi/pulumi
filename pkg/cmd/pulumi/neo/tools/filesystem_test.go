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
	"strings"
	"testing"
	"time"

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
	assert.Equal(t, "hello", res.(readResult).Content)
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
	entries := res.(directoryTreeResult).Entries
	require.Len(t, entries, 2)
	assert.Equal(t, "a.txt", entries[0].Name)
	assert.Equal(t, "sub", entries[1].Name)
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
	assert.Equal(t, "", res.(readResult).Content)
}

func TestFilesystem_UnimplementedMethodsReturnClearError(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	for _, method := range []string{"grep_ast"} {
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

	for _, method := range []string{"read", "write", "directory_tree", "edit", "grep", "content_replace"} {
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
	assert.Equal(t, "a\nb", res.(readResult).Content)
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
	assert.Equal(t, "b\nc", res.(readResult).Content)
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
	entries := res.(directoryTreeResult).Entries

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
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

// editResult wraps the result returned by an edit invocation so tests can assert against
// the human-readable response string the agent receives.
func editResult(t *testing.T, fs *Filesystem, method string, args string) string {
	t.Helper()
	res, err := fs.Invoke(t.Context(), method, json.RawMessage(args))
	require.NoError(t, err)
	s, ok := res.(string)
	require.True(t, ok, "expected string result, got %T", res)
	return s
}

func TestFilesystem_EditReplaceSingleMatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "hello.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello world\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"world","new_string":"there"}`, target))
	assert.Contains(t, res, "Successfully edited file")
	assert.Contains(t, res, "1 replacements applied")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "hello there\n", string(got))
}

func TestFilesystem_EditReplaceAllWithExplicitCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "triples.txt")
	require.NoError(t, os.WriteFile(target, []byte("a\na\na\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"a","new_string":"b","expected_replacements":3}`, target))
	assert.Contains(t, res, "3 replacements applied")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "b\nb\nb\n", string(got))
}

func TestFilesystem_EditOccurrenceMismatch(t *testing.T) {
	t.Parallel()

	// Two occurrences, agent expected one. The error must surface both counts plus the
	// "set expected_replacements=<actual>" suggestion so the agent can self-correct.
	root := t.TempDir()
	target := filepath.Join(root, "doubles.txt")
	require.NoError(t, os.WriteFile(target, []byte("x x"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"x","new_string":"y"}`, target))
	assert.Contains(t, res, "Found 2 occurrences")
	assert.Contains(t, res, "expected 1")
	assert.Contains(t, res, "expected_replacements=2")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "x x", string(got), "mismatched edit must not write the file")
}

func TestFilesystem_EditOldStringNotFound(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, []byte("nothing here"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"missing","new_string":"x"}`, target))
	assert.Contains(t, res, "was not found in the file content")
}

func TestFilesystem_EditEmptyOldStringOnExistingFile(t *testing.T) {
	t.Parallel()

	// A whitespace-only old_string against an existing file would match every run of
	// whitespace, which is almost never what the agent intends — reject it.
	root := t.TempDir()
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, []byte("hi"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	for _, old := range []string{"", "   ", "\t\n"} {
		res := editResult(t, fs, "edit", fmt.Sprintf(
			`{"file_path":%q,"old_string":%q,"new_string":"x"}`, target, old))
		assert.Contains(t, res, "cannot be empty for existing files")
	}
}

func TestFilesystem_EditNegativeExpectedReplacements(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, []byte("hi"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"h","new_string":"j","expected_replacements":-1}`, target))
	assert.Contains(t, res, "non-negative")
}

func TestFilesystem_EditCreatesFileWhenMissingAndOldEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "nested", "new.txt")

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"","new_string":"seeded"}`, target))
	assert.Contains(t, res, "Successfully created file")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "seeded", string(got))
}

func TestFilesystem_EditMissingFileWithNonEmptyOldString(t *testing.T) {
	t.Parallel()

	// old_string != "" + file missing must be an error, not silent creation.
	root := t.TempDir()
	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "edit", json.RawMessage(fmt.Sprintf(
		`{"file_path":%q,"old_string":"x","new_string":"y"}`,
		filepath.Join(root, "nope.txt"))))
	require.Error(t, err)
}

func TestFilesystem_EditRejectsBinaryFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "bin.dat")
	require.NoError(t, os.WriteFile(target, []byte{0xff, 0xfe, 0x00, 0x01}, 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"x","new_string":"y"}`, target))
	assert.Contains(t, res, "Cannot edit binary file")
}

func TestFilesystem_EditNoOpReturnsNoChanges(t *testing.T) {
	t.Parallel()

	// old == new is a legal no-op (matches upstream semantics). The file's mtime must
	// not change because the guard skips the write entirely when the diff is empty.
	root := t.TempDir()
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello"), 0o600))

	before, err := os.Stat(target)
	require.NoError(t, err)

	// Roll mtime back so even a same-second rewrite would be detectable.
	oldTime := before.ModTime().Add(-time.Hour)
	require.NoError(t, os.Chtimes(target, oldTime, oldTime))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"hello","new_string":"hello"}`, target))
	assert.Contains(t, res, "No changes made")

	after, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, oldTime.Unix(), after.ModTime().Unix(), "no-op edit must not rewrite the file")
}

func TestFilesystem_EditRejectsPathOutsideRoot(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	outside := filepath.Join(t.TempDir(), "evil.txt")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o600))

	_, err = fs.Invoke(t.Context(), "edit", json.RawMessage(fmt.Sprintf(
		`{"file_path":%q,"old_string":"x","new_string":"y"}`, outside)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the working directory")

	// Sanity-check the untouched file so we know the sandbox caught it before the write.
	got, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
}

func TestFilesystem_EditReturnsUnifiedDiff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, []byte("alpha\nbeta\ngamma\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res := editResult(t, fs, "edit", fmt.Sprintf(
		`{"file_path":%q,"old_string":"beta","new_string":"BETA"}`, target))
	assert.Contains(t, res, "--- "+target+" (original)")
	assert.Contains(t, res, "+++ "+target+" (modified)")
	assert.Contains(t, res, "@@")
	assert.Contains(t, res, "```diff")
}

func TestFilesystem_Grep_FindsMatchesWithLineNumbers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := filepath.Join(root, "a.go")
	b := filepath.Join(root, "b.go")
	require.NoError(t, os.WriteFile(a, []byte("alpha\nbeta\nTODO: one\n"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("TODO: two\nunrelated\nTODO: three\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "grep",
		json.RawMessage(fmt.Sprintf(`{"pattern":"TODO","path":%q}`, root)))
	require.NoError(t, err)
	content := res.(grepResult).Content
	assert.Contains(t, content, "Found 3 matches in 2 file(s)")
	assert.Contains(t, content, a+":3: TODO: one")
	assert.Contains(t, content, b+":1: TODO: two")
	assert.Contains(t, content, b+":3: TODO: three")
}

func TestFilesystem_Grep_RespectsIncludeGlob(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "match.go"), []byte("needle\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "skip.txt"), []byte("needle\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "grep",
		json.RawMessage(fmt.Sprintf(`{"pattern":"needle","path":%q,"include":"*.go"}`, root)))
	require.NoError(t, err)
	content := res.(grepResult).Content
	assert.Contains(t, content, "match.go")
	assert.NotContains(t, content, "skip.txt")
}

func TestFilesystem_Grep_DefaultPathUsesRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "f.txt"), []byte("hit\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	// Omitting path should default to Root.
	res, err := fs.Invoke(t.Context(), "grep", json.RawMessage(`{"pattern":"hit"}`))
	require.NoError(t, err)
	assert.Contains(t, res.(grepResult).Content, "f.txt:1: hit")
}

func TestFilesystem_Grep_EmptyPatternRejected(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "grep", json.RawMessage(`{"pattern":""}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pattern is required")
}

func TestFilesystem_Grep_InvalidRegexRejected(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "grep", json.RawMessage(`{"pattern":"["}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex")
}

func TestFilesystem_Grep_InvalidIncludeGlobRejected(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "grep",
		json.RawMessage(`{"pattern":"x","include":"["}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid include glob")
}

func TestFilesystem_Grep_SkipsBinaryFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Text file has a match; binary has the literal bytes but contains a NUL.
	require.NoError(t, os.WriteFile(filepath.Join(root, "text.txt"), []byte("secret\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin"), []byte("secret\x00data"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "grep",
		json.RawMessage(fmt.Sprintf(`{"pattern":"secret","path":%q}`, root)))
	require.NoError(t, err)
	content := res.(grepResult).Content
	assert.Contains(t, content, "text.txt")
	assert.NotContains(t, content, filepath.Join(root, "bin"))
	assert.Contains(t, content, "Found 1 matches in 1 file(s)")
}

func TestFilesystem_Grep_SkipsHiddenDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("secret\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "visible.txt"), []byte("secret\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "grep",
		json.RawMessage(fmt.Sprintf(`{"pattern":"secret","path":%q}`, root)))
	require.NoError(t, err)
	content := res.(grepResult).Content
	assert.Contains(t, content, "visible.txt")
	assert.NotContains(t, content, ".git")
	assert.Contains(t, content, "Found 1 matches in 1 file(s)")
}

func TestFilesystem_Grep_NoMatchesReturnsFriendlyText(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("nothing here\n"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "grep",
		json.RawMessage(fmt.Sprintf(`{"pattern":"absent","path":%q}`, root)))
	require.NoError(t, err)
	assert.Equal(t, "No matches found.", res.(grepResult).Content)
}

func TestFilesystem_Grep_RejectsPathOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "grep", json.RawMessage(`{"pattern":"x","path":"/etc"}`))
	require.Error(t, err)
}

func TestFilesystem_ContentReplace_ReplacesAcrossFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("foo foo bar"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("bar foo baz"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(fmt.Sprintf(`{"pattern":"foo","replacement":"qux","path":%q}`, root)))
	require.NoError(t, err)
	m := res.(contentReplaceResult)
	assert.Equal(t, 3, m.ReplacementsMade)
	assert.Equal(t, 2, m.FilesModified)
	assert.False(t, m.DryRun)
	assert.Contains(t, m.Content, "Made 3 replacements")

	aBytes, _ := os.ReadFile(a)
	bBytes, _ := os.ReadFile(b)
	assert.Equal(t, "qux qux bar", string(aBytes))
	assert.Equal(t, "bar qux baz", string(bBytes))
}

func TestFilesystem_ContentReplace_DryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(a, []byte("foo foo"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(fmt.Sprintf(`{"pattern":"foo","replacement":"bar","path":%q,"dry_run":true}`, root)))
	require.NoError(t, err)
	m := res.(contentReplaceResult)
	assert.True(t, m.DryRun)
	assert.True(t, strings.HasPrefix(m.Content, "Dry run:"))

	got, _ := os.ReadFile(a)
	assert.Equal(t, "foo foo", string(got), "dry run must not modify files")
}

func TestFilesystem_ContentReplace_RespectsFilePattern(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	keep := filepath.Join(root, "a.go")
	skip := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(keep, []byte("foo"), 0o600))
	require.NoError(t, os.WriteFile(skip, []byte("foo"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(fmt.Sprintf(
			`{"pattern":"foo","replacement":"bar","path":%q,"file_pattern":"*.go"}`, root)))
	require.NoError(t, err)

	goBytes, _ := os.ReadFile(keep)
	txtBytes, _ := os.ReadFile(skip)
	assert.Equal(t, "bar", string(goBytes))
	assert.Equal(t, "foo", string(txtBytes))
}

func TestFilesystem_ContentReplace_SinglePathArgument(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	other := filepath.Join(root, "b.txt")
	require.NoError(t, os.WriteFile(a, []byte("xx"), 0o600))
	require.NoError(t, os.WriteFile(other, []byte("xx"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(fmt.Sprintf(`{"pattern":"xx","replacement":"yy","path":%q}`, a)))
	require.NoError(t, err)

	aBytes, _ := os.ReadFile(a)
	otherBytes, _ := os.ReadFile(other)
	assert.Equal(t, "yy", string(aBytes))
	assert.Equal(t, "xx", string(otherBytes), "only the single passed file should change")
}

func TestFilesystem_ContentReplace_NoMatchesReturnsError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.txt"), []byte("unrelated"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(fmt.Sprintf(`{"pattern":"missing","replacement":"x","path":%q}`, root)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFilesystem_ContentReplace_SkipsBinaryFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	txt := filepath.Join(root, "a.txt")
	bin := filepath.Join(root, "b.bin")
	require.NoError(t, os.WriteFile(txt, []byte("foo"), 0o600))
	require.NoError(t, os.WriteFile(bin, []byte("foo\x00"), 0o600))

	fs, err := NewFilesystem(root)
	require.NoError(t, err)

	res, err := fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(fmt.Sprintf(`{"pattern":"foo","replacement":"bar","path":%q}`, root)))
	require.NoError(t, err)
	assert.Equal(t, 1, res.(contentReplaceResult).FilesModified)

	binBytes, _ := os.ReadFile(bin)
	assert.Equal(t, "foo\x00", string(binBytes), "binary files must not be modified")
}

func TestFilesystem_ContentReplace_EmptyPatternRejected(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(`{"pattern":"","replacement":"x","path":"."}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pattern is required")
}

func TestFilesystem_ContentReplace_RejectsPathOutsideRoot(t *testing.T) {
	t.Parallel()

	fs, err := NewFilesystem(t.TempDir())
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "content_replace",
		json.RawMessage(`{"pattern":"x","replacement":"y","path":"/etc"}`))
	require.Error(t, err)
}
