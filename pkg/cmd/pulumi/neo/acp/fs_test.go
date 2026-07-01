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

package acp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
)

// recordingCaller captures the single outbound request a test makes so the
// method name and decoded params can be asserted. err is returned from Call;
// readContent is served back for fs/read_text_file responses.
type recordingCaller struct {
	calls       int
	method      string
	params      any
	err         error
	readContent string
}

func (c *recordingCaller) Call(_ context.Context, method string, params, result any) error {
	c.calls++
	c.method = method
	c.params = params
	if c.err != nil {
		return c.err
	}
	if r, ok := result.(*readTextFileResult); ok {
		r.Content = c.readContent
	}
	return nil
}

func TestClientFSWriteTextFile(t *testing.T) {
	t.Parallel()

	caller := &recordingCaller{}
	fs := &ClientFS{Caller: caller, SessionID: "sess_123"}

	require.NoError(t, fs.WriteTextFile(t.Context(), "/abs/path/main.go", "package main"))

	assert.Equal(t, 1, caller.calls)
	assert.Equal(t, "fs/write_text_file", caller.method)
	require.IsType(t, writeTextFileParams{}, caller.params)
	got := caller.params.(writeTextFileParams)
	assert.Equal(t, writeTextFileParams{
		SessionID: "sess_123",
		Path:      "/abs/path/main.go",
		Content:   "package main",
	}, got)
}

func TestClientFSReadTextFile(t *testing.T) {
	t.Parallel()

	caller := &recordingCaller{readContent: "package main"}
	fs := &ClientFS{Caller: caller, SessionID: "sess_123"}

	got, err := fs.ReadTextFile(t.Context(), "/abs/path/main.go")
	require.NoError(t, err)
	assert.Equal(t, "package main", got)

	assert.Equal(t, "fs/read_text_file", caller.method)
	require.IsType(t, readTextFileParams{}, caller.params)
	assert.Equal(t,
		readTextFileParams{SessionID: "sess_123", Path: "/abs/path/main.go"},
		caller.params.(readTextFileParams))
}

func TestClientFSReadTextFilePropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("not found")
	fs := &ClientFS{Caller: &recordingCaller{err: wantErr}, SessionID: "sess_123"}

	_, err := fs.ReadTextFile(t.Context(), "/abs/path/main.go")
	assert.ErrorIs(t, err, wantErr)
}

// TestFilesystemReadRoutesThroughACP is the read-side counterpart to the edit
// test: a filesystem read served by the ACP client returns the editor's content
// (and the tool still applies its offset/limit slicing), ignoring whatever is on
// local disk.
func TestFilesystemReadRoutesThroughACP(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	require.NoError(t, os.WriteFile(target, []byte("ON DISK\n"), 0o600))

	fs, err := tools.NewFilesystem(root)
	require.NoError(t, err)

	caller := &recordingCaller{readContent: "line1\nline2\nline3\n"}
	client := &ClientFS{Caller: caller, SessionID: "sess_abc"}
	fs.OnRead = client.ReadTextFile

	res, err := fs.Invoke(t.Context(), "read",
		json.RawMessage(`{"file_path":"`+target+`","offset":1}`))
	require.NoError(t, err)
	assert.Equal(t, "fs/read_text_file", caller.method)

	// The result reflects the editor's content (sliced from offset 1), not disk.
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "line2")
	assert.NotContains(t, string(raw), "ON DISK")
	assert.NotContains(t, string(raw), "line1", "offset slicing should still apply to editor content")
}

// TestFilesystemEditUsesEditorBufferOverDisk guards that an ACP-backed edit
// matches and writes against the editor's buffer (via OnRead), not stale disk:
// the on-disk content here would not contain old_string at all.
func TestFilesystemEditUsesEditorBufferOverDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	require.NoError(t, os.WriteFile(target, []byte("STALE DISK CONTENT\n"), 0o600))

	fs, err := tools.NewFilesystem(root)
	require.NoError(t, err)
	caller := &recordingCaller{readContent: "func target() {}\n"}
	cfs := &ClientFS{Caller: caller, SessionID: "sess_abc"}
	fs.OnRead = cfs.ReadTextFile
	fs.OnWrite = cfs.WriteTextFile

	res, err := fs.Invoke(t.Context(), "edit", json.RawMessage(
		`{"file_path":"`+target+`","old_string":"func target()","new_string":"func renamed()"}`))
	require.NoError(t, err)

	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Successfully edited", "edit should match against the editor buffer")

	// The write carried the buffer-derived content, and disk was left untouched.
	require.IsType(t, writeTextFileParams{}, caller.params)
	assert.Equal(t, "func renamed() {}\n", caller.params.(writeTextFileParams).Content)
	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "STALE DISK CONTENT\n", string(onDisk))
}

func TestClientFSWriteTextFilePropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("permission denied")
	fs := &ClientFS{Caller: &recordingCaller{err: wantErr}, SessionID: "sess_123"}

	err := fs.WriteTextFile(t.Context(), "/abs/path/main.go", "x")
	assert.ErrorIs(t, err, wantErr)
}

// TestFilesystemEditRoutesThroughACPWrite is the end-to-end check for the
// edit-as-ACP-write path: a filesystem edit computes the modified content
// locally but commits it through the ACP client's fs/write_text_file, leaving
// the on-disk file untouched (the editor owns the write).
func TestFilesystemEditRoutesThroughACPWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	const original = "package main\n\nfunc foo() {}\n"
	require.NoError(t, os.WriteFile(target, []byte(original), 0o600))

	fs, err := tools.NewFilesystem(root)
	require.NoError(t, err)

	caller := &recordingCaller{}
	client := &ClientFS{Caller: caller, SessionID: "sess_abc"}
	fs.OnWrite = client.WriteTextFile

	args, err := json.Marshal(map[string]string{
		"file_path":  target,
		"old_string": "func foo()",
		"new_string": "func bar()",
	})
	require.NoError(t, err)

	_, err = fs.Invoke(t.Context(), "edit", args)
	require.NoError(t, err)

	// The write was diverted to the ACP client with the fully-modified content.
	require.Equal(t, 1, caller.calls)
	require.Equal(t, "fs/write_text_file", caller.method)
	require.IsType(t, writeTextFileParams{}, caller.params)
	got := caller.params.(writeTextFileParams)
	// The filesystem tool canonicalizes the path before writing, so the editor
	// receives the symlink-evaluated absolute path.
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realRoot, "main.go"), got.Path)
	assert.Equal(t, "sess_abc", got.SessionID)
	assert.Equal(t, "package main\n\nfunc bar() {}\n", got.Content)

	// Disk is untouched: the CLI did not perform the write itself.
	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, original, string(onDisk))
}
