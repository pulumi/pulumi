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

// ErrAuthRequired is the sentinel a Delegate returns (directly or wrapped) when
// there is no usable session. The agent maps it to the ACP "Authentication
// required" error (code -32000) so the editor can prompt the user to
// authenticate. A Delegate may wrap it with an application-specific message
// (e.g. "run `pulumi login`") that the editor then surfaces.
var ErrAuthRequired = errors.New("authentication required")

// codeAuthRequired is the JSON-RPC error code ACP uses for "Authentication
// required". It lies in the implementation-defined server-error range.
const codeAuthRequired int64 = -32000

// Delegate supplies the application-specific behavior behind the ACP session
// methods, keeping this package free of application dependencies. The neo
// package implements it over the Pulumi Cloud backend and the Neo Session event
// loop. A nil Delegate makes the session methods report "not implemented".
type Delegate interface {
	// CheckAuth reports whether a usable session is available from the existing
	// credentials, returning ErrAuthRequired when none is. It never prompts. The
	// agent calls it from the `authenticate` handler so the editor learns at that
	// point whether the user still needs to authenticate, rather than only when
	// the first session is created. (For Neo this checks the CLI's Pulumi Cloud
	// login.)
	CheckAuth(ctx context.Context) error

	// NewSession resolves auth and the session target for a session rooted at
	// params.Cwd, builds the session's tool handlers — routing filesystem writes
	// through client (as an editor fs/write_text_file request) when caps
	// advertises fs.writeTextFile — and returns the new session id. It returns
	// ErrAuthRequired when no usable credentials are available. client is retained
	// for the session's lifetime to push session/update notifications and request
	// permissions.
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

	// SetConfigOption applies a session config option change (params.ConfigID =
	// params.Value) for params.SessionID and returns the complete, updated option
	// list. The agent may clamp or ignore a change it cannot honor; the returned
	// current values are authoritative.
	SetConfigOption(ctx context.Context, params SetConfigOptionParams) (SetConfigOptionResult, error)
}

// Identity is the agent's self-description, advertised to the editor on
// initialize. The embedding application supplies it so this package carries no
// application-specific branding.
type Identity struct {
	Name        string       // agentInfo.name (e.g. "pulumi-neo")
	Title       string       // agentInfo.title (e.g. "Pulumi Neo")
	AuthMethods []AuthMethod // advertised authenticate methods
}

// Agent is the ACP agent side of the adapter. It holds the state that spans the
// connection's lifetime — the negotiated client capabilities and the installed
// Delegate — and implements the client→agent methods dispatched by handle. The
// Client used to reach back to the editor is not stored here: handle derives it
// from the connection on each dispatch.
//
// Agent owns only the protocol handshake (initialize/authenticate) and request
// routing; the application-specific session behavior (for Neo: the Cloud client,
// org/stack resolution, and the Neo Session event loop) lives behind the
// Delegate, which the embedding package installs via SetDelegate.
type Agent struct {
	// identity is reported as agentInfo and the advertised auth methods on
	// initialize.
	identity Identity
	// version is reported as agentInfo.version on initialize.
	version string

	mu sync.Mutex
	// clientCaps is captured from the initialize request and consulted when
	// building per-session tool handlers (filesystem/shell routing).
	clientCaps ClientCapabilities
	// delegate supplies the Pulumi-specific session behavior. nil until
	// SetDelegate is called; the session methods report "not implemented" while
	// nil.
	delegate Delegate
}

// NewAgent constructs an Agent. identity is advertised to the editor as agentInfo
// and the available auth methods, and version is surfaced as agentInfo.version,
// both during initialize.
func NewAgent(identity Identity, version string) *Agent {
	return &Agent{identity: identity, version: version}
}

// SetDelegate installs the application-specific session behavior. Call before Serve.
func (a *Agent) SetDelegate(d Delegate) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.delegate = d
}

// getDelegate returns the installed Delegate, or nil when SetDelegate has not
// been called yet.
func (a *Agent) getDelegate() Delegate {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.delegate
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
			Name:    a.identity.Name,
			Title:   a.identity.Title,
			Version: a.version,
		},
		AuthMethods: a.identity.AuthMethods,
	}, nil
}

// authenticate handles the `authenticate` request. Interactive browser login
// cannot run over the stdio JSON-RPC channel, so authenticate cannot perform a
// login itself; instead it verifies via the Delegate that a usable session
// already exists, mapping its absence to the ACP "Authentication required" error
// so the editor can tell the user how to authenticate at the point it asked. (For
// Neo this is a prior `pulumi login`.) session/new performs the same check, so
// auth stays gated even if credentials lapse between calls. Returns null on
// success.
func (a *Agent) authenticate(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	if _, err := decodeParams[AuthenticateParams](req); err != nil {
		return nil, err
	}
	d := a.getDelegate()
	if d == nil {
		return nil, errNotImplemented("authenticate")
	}
	if err := d.CheckAuth(ctx); err != nil {
		return nil, mapDelegateErr(err)
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

// mapDelegateErr converts a Delegate error into the response the editor should
// see, translating ErrAuthRequired to the ACP "Authentication required" code so
// the editor can prompt for login. Other errors pass through unchanged.
func mapDelegateErr(err error) error {
	if errors.Is(err, ErrAuthRequired) {
		return &jsonrpc2.Error{Code: codeAuthRequired, Message: err.Error()}
	}
	return err
}

// newSession handles the `session/new` request. client is the connection-backed
// Client handle supplied by handle; the Delegate retains it for the session's
// lifetime to build editor-backed tool handlers (e.g. ClientFS) and to stream
// session/update notifications.
func (a *Agent) newSession(ctx context.Context, client Client, req *jsonrpc2.Request) (any, error) {
	a.mu.Lock()
	d, caps := a.delegate, a.clientCaps
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
		return nil, mapDelegateErr(err)
	}
	return res, nil
}

func (a *Agent) prompt(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	d := a.getDelegate()
	if d == nil {
		return nil, errNotImplemented("session/prompt")
	}
	params, err := decodeParams[PromptParams](req)
	if err != nil {
		return nil, err
	}
	return d.Prompt(ctx, params)
}

func (a *Agent) setConfigOption(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	d := a.getDelegate()
	if d == nil {
		return nil, errNotImplemented("session/set_config_option")
	}
	params, err := decodeParams[SetConfigOptionParams](req)
	if err != nil {
		return nil, err
	}
	return d.SetConfigOption(ctx, params)
}

// cancel handles the session/cancel notification. As a notification it has no
// response; HandlerWithError logs any returned error.
func (a *Agent) cancel(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	d := a.getDelegate()
	if d == nil {
		return nil, nil
	}
	params, err := decodeParams[CancelParams](req)
	if err != nil {
		return nil, err
	}
	return nil, d.Cancel(ctx, params)
}
