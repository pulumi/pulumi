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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestClientFSWriteTextFilePropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("permission denied")
	fs := &ClientFS{Caller: &recordingCaller{err: wantErr}, SessionID: "sess_123"}

	err := fs.WriteTextFile(t.Context(), "/abs/path/main.go", "x")
	assert.ErrorIs(t, err, wantErr)
}
