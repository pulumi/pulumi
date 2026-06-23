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
	"fmt"
	"io"

	"github.com/sourcegraph/jsonrpc2"
)

// Serve runs the ACP agent over the given stdio streams, speaking newline-framed
// JSON-RPC 2.0, until the peer disconnects or ctx is cancelled. The `pulumi neo
// acp` command calls this with os.Stdin/os.Stdout; nothing else may write to the
// output stream or it will corrupt the protocol.
func Serve(ctx context.Context, a *Agent, in io.Reader, out io.Writer) error {
	stream := jsonrpc2.NewPlainObjectStream(stdio{Reader: in, Writer: out})
	// AsyncHandler runs each inbound request in its own goroutine. This is
	// required, not just an optimization: a session/prompt turn issues outbound
	// requests back to the editor (fs/read_text_file, terminal/*,
	// session/request_permission) and blocks on their responses. Those responses
	// arrive on this same connection and are dispatched by jsonrpc2's single read
	// loop — which, with a synchronous handler, would still be parked inside the
	// prompt handler and could never deliver them, deadlocking on the first such
	// call. Async handling also lets a session/cancel be processed while its
	// session/prompt is still in flight.
	handler := jsonrpc2.AsyncHandler(jsonrpc2.HandlerWithError(a.handle))
	conn := jsonrpc2.NewConn(ctx, stream, handler)
	// Give the agent a Client so its method handlers can issue outbound requests
	// to the editor (fs/*, terminal/*, session/update, session/request_permission).
	a.setClient(connCaller{conn: conn})

	select {
	case <-ctx.Done():
		_ = conn.Close()
		return ctx.Err()
	case <-conn.DisconnectNotify():
		return nil
	}
}

// handle dispatches a single client→agent request by method name. It is wrapped
// in jsonrpc2.HandlerWithError, which turns the (result, error) return into a
// JSON-RPC response: a *jsonrpc2.Error becomes a structured error response,
// any other error becomes a generic one, and notifications drop the result.
func (a *Agent) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
	switch req.Method {
	case "initialize":
		return a.initialize(req)
	case "authenticate":
		return a.authenticate(ctx, req)
	case "session/new":
		return a.newSession(ctx, req)
	case "session/prompt":
		return a.prompt(ctx, req)
	case "session/cancel":
		return a.cancel(ctx, req)
	case "session/set_config_option":
		return a.setConfigOption(ctx, req)
	default:
		return nil, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: fmt.Sprintf("method %q is not supported", req.Method),
		}
	}
}

// decodeParams unmarshals a request's params into T, returning a CodeInvalidParams
// JSON-RPC error on failure. Absent params decode to the zero value of T.
func decodeParams[T any](req *jsonrpc2.Request) (T, error) {
	var v T
	if req.Params == nil {
		return v, nil
	}
	if err := json.Unmarshal(*req.Params, &v); err != nil {
		return v, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: fmt.Sprintf("decoding %s params: %v", req.Method, err),
		}
	}
	return v, nil
}

// Client is the agent's handle to the editor for the connection: it adds
// fire-and-forget notifications (session/update) on top of the request/response
// Caller (session/request_permission, fs/*, terminal/*).
type Client interface {
	Caller
	// Notify sends a JSON-RPC notification (no response expected).
	Notify(ctx context.Context, method string, params any) error
}

// connCaller adapts a *jsonrpc2.Conn to the Client (and Caller) interface so
// outbound helpers (e.g. ClientFS) and the session translator can issue requests
// and notifications without depending on jsonrpc2 directly.
type connCaller struct {
	conn *jsonrpc2.Conn
}

func (c connCaller) Call(ctx context.Context, method string, params, result any) error {
	return c.conn.Call(ctx, method, params, result)
}

func (c connCaller) Notify(ctx context.Context, method string, params any) error {
	return c.conn.Notify(ctx, method, params)
}

// stdio adapts a separate reader and writer (stdin/stdout) into the single
// io.ReadWriteCloser that jsonrpc2.NewPlainObjectStream expects. Close is a no-op:
// the process owns the underlying file descriptors and closing them here would
// race the runtime's own teardown.
type stdio struct {
	io.Reader
	io.Writer
}

func (stdio) Close() error { return nil }
