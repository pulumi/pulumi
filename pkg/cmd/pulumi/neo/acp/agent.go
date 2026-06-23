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
	"sync"

	"github.com/sourcegraph/jsonrpc2"
)

// ErrAuthRequired is returned by a Delegate when there is no usable Pulumi Cloud
// session. The agent maps it to the ACP "Authentication required" error
// (code -32000) so the editor can prompt the user to authenticate — for Neo,
// by running `pulumi login`.
var ErrAuthRequired = errors.New("not authenticated with Pulumi Cloud; run `pulumi login`")

// codeAuthRequired is the JSON-RPC error code ACP uses for "Authentication
// required". It lies in the implementation-defined server-error range.
const codeAuthRequired int64 = -32000

// Delegate supplies the Pulumi-specific behavior behind the ACP session methods.
// The neo package implements it over the Pulumi Cloud backend and the Neo
// Session event loop, keeping this package free of Pulumi dependencies. A nil
// Delegate makes the session methods report "not implemented".
type Delegate interface {
	// NewSession resolves Pulumi Cloud auth from existing CLI credentials and
	// the org/stack target for a session rooted at params.Cwd, builds the
	// session's tool handlers — routing filesystem writes through client (as an
	// editor fs/write_text_file request) when caps advertises fs.writeTextFile —
	// and returns the new session id. It returns ErrAuthRequired when no CLI
	// login is available. client is retained for the session's lifetime to push
	// session/update notifications and request permissions.
	NewSession(
		ctx context.Context, params NewSessionParams, caps ClientCapabilities, client Client,
	) (NewSessionResult, error)

	// Prompt runs one prompt turn for params.SessionID and returns the reason it
	// ended. It streams session/update notifications through the Client retained
	// at NewSession, and blocks until the turn completes (or ctx is done).
	Prompt(ctx context.Context, params PromptParams) (PromptResult, error)

	// Cancel asks the agent to stop the in-flight turn for params.SessionID. It
	// corresponds to the session/cancel notification, so it returns promptly; the
	// turn itself ends with StopCancelled once the backend acknowledges.
	Cancel(ctx context.Context, params CancelParams) error
}

// Agent is the ACP agent side of the Pulumi Neo adapter. It holds the state that
// spans the connection's lifetime — the negotiated client capabilities and the
// Caller used to reach back to the editor — and implements the client→agent
// methods dispatched by handle.
//
// Agent owns only the protocol handshake (initialize/authenticate) and request
// routing; the Pulumi-specific session behavior (Cloud client, org/stack
// resolution, the Neo Session event loop) lives behind the Delegate, which the
// neo package installs via SetDelegate.
type Agent struct {
	// version is reported as agentInfo.version on initialize.
	version string

	mu sync.Mutex
	// clientCaps is captured from the initialize request and consulted when
	// building per-session tool handlers (filesystem/shell routing).
	clientCaps ClientCapabilities
	// client reaches the editor for outbound requests and notifications (fs/*,
	// terminal/*, session/update, session/request_permission). Set once by Serve.
	client Client
	// delegate supplies the Pulumi-specific session behavior. nil until
	// SetDelegate is called; the session methods report "not implemented" while
	// nil.
	delegate Delegate
}

// NewAgent constructs an Agent. version is surfaced to the editor as
// agentInfo.version during initialize.
func NewAgent(version string) *Agent {
	return &Agent{version: version}
}

// setClient records the connection-backed Client. Called once by Serve before
// any request is dispatched.
func (a *Agent) setClient(c Client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = c
}

// SetDelegate installs the Pulumi-specific session behavior. Call before Serve.
func (a *Agent) SetDelegate(d Delegate) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.delegate = d
}

// ClientCapabilities returns the capabilities the editor advertised on
// initialize. Safe to call from any goroutine.
func (a *Agent) ClientCapabilities() ClientCapabilities {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.clientCaps
}

// Client returns the connection-backed Client used to issue outbound requests
// and notifications to the editor. It is nil until Serve has wired the
// connection. session/new hands it to the Delegate to build editor-backed tool
// handlers (e.g. ClientFS) and to stream session/update notifications.
func (a *Agent) Client() Client {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.client
}

// initialize handles the `initialize` request: it records the client's
// capabilities and reports the agent's. Per the spec we echo the client's
// protocol version when we support it, otherwise our latest; since v1 is the
// only version we implement, we always answer ProtocolVersion.
func (a *Agent) initialize(req *jsonrpc2.Request) (any, error) {
	params, err := decodeParams[InitializeParams](req)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.clientCaps = params.ClientCapabilities
	a.mu.Unlock()

	return InitializeResult{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{
			// session/load and non-text prompt content are not yet supported.
			LoadSession:        false,
			PromptCapabilities: PromptCapabilities{},
		},
		AgentInfo: &Implementation{
			Name:    "pulumi-neo",
			Title:   "Pulumi Neo",
			Version: a.version,
		},
		AuthMethods: []AuthMethod{{
			ID:   authMethodPulumiLogin,
			Name: "Pulumi login",
			Description: "Authenticate by running `pulumi login` in a terminal. " +
				"Neo uses your existing Pulumi Cloud session.",
		}},
	}, nil
}

// authenticate handles the `authenticate` request. Interactive browser login
// cannot run over the stdio JSON-RPC channel, so the real Pulumi Cloud session
// check is performed by the Delegate when the first session is created (it
// returns ErrAuthRequired when no CLI login is present); here we only
// acknowledge the chosen method. Returns null on success.
func (a *Agent) authenticate(_ context.Context, req *jsonrpc2.Request) (any, error) {
	if _, err := decodeParams[AuthenticateParams](req); err != nil {
		return nil, err
	}
	return nil, nil
}

// errNotImplemented is the response a session method returns when no Delegate is
// installed. In normal operation the neo package always sets one before Serve;
// this guards the standalone-handshake case (and tests).
func errNotImplemented(method string) error {
	return &jsonrpc2.Error{
		Code:    jsonrpc2.CodeInternalError,
		Message: method + " is not implemented yet",
	}
}

func (a *Agent) newSession(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	a.mu.Lock()
	d, client, caps := a.delegate, a.client, a.clientCaps
	a.mu.Unlock()
	if d == nil {
		return nil, errNotImplemented("session/new")
	}
	params, err := decodeParams[NewSessionParams](req)
	if err != nil {
		return nil, err
	}
	res, err := d.NewSession(ctx, params, caps, client)
	if err != nil {
		if errors.Is(err, ErrAuthRequired) {
			return nil, &jsonrpc2.Error{Code: codeAuthRequired, Message: err.Error()}
		}
		return nil, err
	}
	return res, nil
}

func (a *Agent) prompt(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	a.mu.Lock()
	d := a.delegate
	a.mu.Unlock()
	if d == nil {
		return nil, errNotImplemented("session/prompt")
	}
	params, err := decodeParams[PromptParams](req)
	if err != nil {
		return nil, err
	}
	return d.Prompt(ctx, params)
}

// cancel handles the session/cancel notification. As a notification it has no
// response; HandlerWithError logs any returned error.
func (a *Agent) cancel(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	a.mu.Lock()
	d := a.delegate
	a.mu.Unlock()
	if d == nil {
		return nil, nil
	}
	params, err := decodeParams[CancelParams](req)
	if err != nil {
		return nil, err
	}
	return nil, d.Cancel(ctx, params)
}
