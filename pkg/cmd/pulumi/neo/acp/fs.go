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

// Package acp implements the agent side of the Agent Client Protocol
// (https://agentclientprotocol.com), letting ACP-capable editors drive a CLI
// agent as a subprocess over JSON-RPC. It is application-neutral: the embedding
// package supplies its identity (see NewAgent) and behavior (see Delegate). The
// Pulumi Neo CLI is its first consumer. This file maps an agent's filesystem
// writes onto the ACP client's fs/write_text_file method so they surface as
// native editor diffs.
package acp

import "context"

// Caller issues an outbound JSON-RPC request to the ACP client (the editor) and
// decodes the response into result, which may be nil when the method returns no
// payload. The session connection (*jsonrpc2.Conn) satisfies this via a thin
// adapter; tests supply a fake.
type Caller interface {
	Call(ctx context.Context, method string, params, result any) error
}

// writeTextFileParams is the request body for the ACP `fs/write_text_file`
// client method. Field names mirror the ACP schema exactly. path must be
// absolute; the client creates the file (and any missing parents) if needed and
// responds with null.
//
// https://agentclientprotocol.com/protocol/file-system
type writeTextFileParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

// readTextFileParams is the request body for the ACP `fs/read_text_file` client
// method. path must be absolute. The response carries the file's full content.
//
// https://agentclientprotocol.com/protocol/file-system
type readTextFileParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
}

// readTextFileResult is the response to fs/read_text_file.
type readTextFileResult struct {
	Content string `json:"content"`
}

// ClientFS routes filesystem reads and writes to the ACP client so the editor
// performs them — surfacing writes as native diffs and serving reads from the
// editor's (possibly unsaved) buffer — rather than the CLI touching disk
// directly. It is wired into a tools.Filesystem via its OnRead/OnWrite hooks;
// gate that wiring on the client having advertised the corresponding fs
// capability during initialize.
type ClientFS struct {
	// Caller issues the outbound fs/* requests to the editor.
	Caller Caller
	// SessionID is the ACP session the operation belongs to.
	SessionID string
}

// WriteTextFile sends the full content of an absolute path to the editor via the
// ACP `fs/write_text_file` request. Its signature matches
// tools.Filesystem.OnWrite, so it can be assigned to that hook directly.
func (c *ClientFS) WriteTextFile(ctx context.Context, path, content string) error {
	return c.Caller.Call(ctx, "fs/write_text_file", writeTextFileParams{
		SessionID: c.SessionID,
		Path:      path,
		Content:   content,
	}, nil)
}

// ReadTextFile fetches the full content of an absolute path from the editor via
// the ACP `fs/read_text_file` request. Its signature matches
// tools.Filesystem.OnRead, so it can be assigned to that hook directly.
func (c *ClientFS) ReadTextFile(ctx context.Context, path string) (string, error) {
	var res readTextFileResult
	if err := c.Caller.Call(ctx, "fs/read_text_file", readTextFileParams{
		SessionID: c.SessionID,
		Path:      path,
	}, &res); err != nil {
		return "", err
	}
	return res.Content, nil
}
