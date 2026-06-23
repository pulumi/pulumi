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
	"net"
	"testing"
	"time"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestPeers wires an agent connection to a client connection over an
// in-memory pipe, returning the client side to drive requests against. Both
// connections are closed via t.Cleanup.
func newTestPeers(t *testing.T, a *Agent) *jsonrpc2.Conn {
	return newTestPeersWithClient(t, a, noopHandler{})
}

// newTestPeersWithClient is like newTestPeers but lets the caller supply the
// client-side handler so a test can answer the agent's outbound requests (e.g.
// fs/read_text_file). The agent side is wrapped in AsyncHandler exactly as Serve
// does, so the wiring under test matches production — in particular, an inbound
// request handler may block on an outbound request without deadlocking the read
// loop.
func newTestPeersWithClient(t *testing.T, a *Agent, clientHandler jsonrpc2.Handler) *jsonrpc2.Conn {
	t.Helper()
	agentEnd, clientEnd := net.Pipe()

	agentConn := jsonrpc2.NewConn(t.Context(),
		jsonrpc2.NewPlainObjectStream(agentEnd), jsonrpc2.AsyncHandler(jsonrpc2.HandlerWithError(a.handle)))
	a.setClient(connCaller{conn: agentConn})

	clientConn := jsonrpc2.NewConn(t.Context(),
		jsonrpc2.NewPlainObjectStream(clientEnd), clientHandler)

	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = agentConn.Close()
	})
	return clientConn
}

// noopHandler is the client-side handler for tests that only make outbound
// calls and expect no agent→client requests.
type noopHandler struct{}

func (noopHandler) Handle(context.Context, *jsonrpc2.Conn, *jsonrpc2.Request) {}

func TestInitializeNegotiatesAndCapturesCapabilities(t *testing.T) {
	t.Parallel()

	agent := NewAgent("v3.999.0")
	client := newTestPeers(t, agent)

	var res InitializeResult
	err := client.Call(t.Context(), "initialize", InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientCapabilities: ClientCapabilities{
			FS:       FileSystemCapability{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
		},
		ClientInfo: &Implementation{Name: "test-editor", Version: "1.0"},
	}, &res)
	require.NoError(t, err)

	assert.Equal(t, ProtocolVersion, res.ProtocolVersion)
	require.NotNil(t, res.AgentInfo)
	assert.Equal(t, "v3.999.0", res.AgentInfo.Version)
	require.Len(t, res.AuthMethods, 1)
	assert.Equal(t, authMethodPulumiLogin, res.AuthMethods[0].ID)

	// The editor's capabilities are captured for later tool routing.
	caps := agent.ClientCapabilities()
	assert.True(t, caps.FS.WriteTextFile)
	assert.True(t, caps.FS.ReadTextFile)
	assert.True(t, caps.Terminal)
}

func TestAuthenticateAcknowledges(t *testing.T) {
	t.Parallel()

	client := newTestPeers(t, NewAgent("v3.0.0"))

	err := client.Call(t.Context(), "authenticate", AuthenticateParams{MethodID: authMethodPulumiLogin}, nil)
	require.NoError(t, err)
}

func TestUnknownMethodIsMethodNotFound(t *testing.T) {
	t.Parallel()

	client := newTestPeers(t, NewAgent("v3.0.0"))

	err := client.Call(t.Context(), "does/not/exist", nil, nil)
	var rpcErr *jsonrpc2.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.EqualValues(t, jsonrpc2.CodeMethodNotFound, rpcErr.Code)
}

func TestCallerWiredBySetup(t *testing.T) {
	t.Parallel()

	agent := NewAgent("v3.0.0")
	newTestPeers(t, agent)
	require.NotNil(t, agent.Client())
}

// fakeDelegate records what the session methods were called with and returns
// canned results, so the agent's delegation and error mapping can be tested
// without a Pulumi Cloud backend.
type fakeDelegate struct {
	result       NewSessionResult
	err          error
	gotParams    NewSessionParams
	gotCaps      ClientCapabilities
	gotClient    Client
	promptResult PromptResult
	promptErr    error
	gotPrompt    PromptParams
	gotCancel    CancelParams
}

func (f *fakeDelegate) NewSession(
	_ context.Context, params NewSessionParams, caps ClientCapabilities, client Client,
) (NewSessionResult, error) {
	f.gotParams, f.gotCaps, f.gotClient = params, caps, client
	return f.result, f.err
}

func (f *fakeDelegate) Prompt(_ context.Context, params PromptParams) (PromptResult, error) {
	f.gotPrompt = params
	return f.promptResult, f.promptErr
}

func (f *fakeDelegate) Cancel(_ context.Context, params CancelParams) error {
	f.gotCancel = params
	return nil
}

func TestNewSessionDelegatesWithCapturedCapabilities(t *testing.T) {
	t.Parallel()

	agent := NewAgent("v3.0.0")
	fd := &fakeDelegate{result: NewSessionResult{SessionID: "sess_42"}}
	agent.SetDelegate(fd)
	client := newTestPeers(t, agent)

	// initialize first so the agent captures the client's fs capability and
	// forwards it to the delegate.
	require.NoError(t, client.Call(t.Context(), "initialize", InitializeParams{
		ProtocolVersion:    ProtocolVersion,
		ClientCapabilities: ClientCapabilities{FS: FileSystemCapability{WriteTextFile: true}},
	}, &InitializeResult{}))

	var res NewSessionResult
	err := client.Call(t.Context(), "session/new", NewSessionParams{Cwd: "/work"}, &res)
	require.NoError(t, err)

	assert.Equal(t, "sess_42", res.SessionID)
	assert.Equal(t, "/work", fd.gotParams.Cwd)
	assert.True(t, fd.gotCaps.FS.WriteTextFile, "client fs capability should reach the delegate")
	require.NotNil(t, fd.gotClient, "the delegate should receive a Client for editor-backed writes")
}

// readingDelegate's Prompt reads a file back through the editor, exercising the
// outbound-request-during-an-inbound-request path that previously deadlocked.
type readingDelegate struct {
	client    Client
	readPath  string
	gotResult string
}

func (d *readingDelegate) NewSession(
	_ context.Context, _ NewSessionParams, _ ClientCapabilities, client Client,
) (NewSessionResult, error) {
	d.client = client
	return NewSessionResult{SessionID: "sess_read"}, nil
}

func (d *readingDelegate) Prompt(ctx context.Context, _ PromptParams) (PromptResult, error) {
	cfs := &ClientFS{Caller: d.client, SessionID: "sess_read"}
	content, err := cfs.ReadTextFile(ctx, d.readPath)
	if err != nil {
		return PromptResult{}, err
	}
	d.gotResult = content
	return PromptResult{StopReason: StopEndTurn}, nil
}

func (d *readingDelegate) Cancel(context.Context, CancelParams) error { return nil }

// replyingClient answers fs/read_text_file with canned content. It models the
// editor satisfying the agent's read while the agent's prompt handler is blocked
// waiting for it.
type replyingClient struct{ content string }

func (c replyingClient) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if req.Method == "fs/read_text_file" {
		_ = conn.Reply(ctx, req.ID, readTextFileResult{Content: c.content})
	}
}

// TestPromptCanReadFileBackThroughClient is the regression test for the deadlock
// where a prompt turn that issued an outbound fs/read_text_file hung forever: the
// agent's single JSON-RPC read loop was parked inside the synchronous prompt
// handler and could never deliver the editor's response. With AsyncHandler the
// turn completes. The test uses its own timeout so a regression shows up as a
// failure, not a hung test.
func TestPromptCanReadFileBackThroughClient(t *testing.T) {
	t.Parallel()

	agent := NewAgent("v3.0.0")
	fd := &readingDelegate{readPath: "/abs/file.txt"}
	agent.SetDelegate(fd)
	client := newTestPeersWithClient(t, agent, replyingClient{content: "hello"})

	require.NoError(t, client.Call(t.Context(), "session/new", NewSessionParams{Cwd: "/work"}, &NewSessionResult{}))

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var res PromptResult
	err := client.Call(ctx, "session/prompt", PromptParams{SessionID: "sess_read"}, &res)
	require.NoError(t, err, "prompt that reads a file back through the editor must not deadlock")
	assert.Equal(t, StopEndTurn, res.StopReason)
	assert.Equal(t, "hello", fd.gotResult)
}

func TestNewSessionAuthRequiredMapsToCode(t *testing.T) {
	t.Parallel()

	agent := NewAgent("v3.0.0")
	agent.SetDelegate(&fakeDelegate{err: ErrAuthRequired})
	client := newTestPeers(t, agent)

	err := client.Call(t.Context(), "session/new", NewSessionParams{Cwd: "/work"}, nil)
	var rpcErr *jsonrpc2.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.EqualValues(t, codeAuthRequired, rpcErr.Code)
}

func TestNewSessionWithoutDelegateIsNotImplemented(t *testing.T) {
	t.Parallel()

	client := newTestPeers(t, NewAgent("v3.0.0"))

	err := client.Call(t.Context(), "session/new", NewSessionParams{Cwd: "/work"}, nil)
	var rpcErr *jsonrpc2.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.EqualValues(t, jsonrpc2.CodeInternalError, rpcErr.Code)
}
