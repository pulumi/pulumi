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

package neo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newNeoACPCmd is the hidden `pulumi neo acp` subcommand. It speaks the Agent
// Client Protocol (https://agentclientprotocol.com) over stdio so ACP-capable
// editors can drive Neo as a subprocess. It is hidden because it is launched by
// editors via their configured command string, not run by hand.
//
// Authentication defaults to the CLI's existing Pulumi Cloud session, exactly
// like `pulumi neo`: the agent never prompts interactively (that would corrupt
// the JSON-RPC stream on stdout) and instead surfaces an auth-required error
// the editor can act on when no login is present.
func newNeoACPCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "acp",
		Short:  "Run Pulumi Neo as an Agent Client Protocol (ACP) agent over stdio",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			agent := acp.NewAgent(version.Version)
			agent.SetDelegate(&acpDelegate{
				ws:       pkgWorkspace.Instance,
				baseCtx:  ctx,
				sessions: map[string]*acpSession{},
			})
			// stdin/stdout carry the JSON-RPC stream; nothing else may write to
			// stdout. cmd.OutOrStdout()/InOrStdin() resolve to the real fds.
			return acp.Serve(ctx, agent, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// acpDelegate implements acp.Delegate over the Pulumi Cloud backend. It owns the
// live ACP sessions for the connection; the per-session runtime lives in
// acp_session.go.
type acpDelegate struct {
	ws pkgWorkspace.Context
	// baseCtx is the connection-lifetime context (cmd.Context()). Per-session
	// background work (the Neo Session event loop) derives from it so it
	// outlives the request that started it and is torn down when the connection
	// closes.
	baseCtx context.Context //nolint:containedctx // session loops outlive requests; see comment

	mu       sync.Mutex
	sessions map[string]*acpSession
}

// NewSession resolves CLI auth and the task target, builds the session's tool
// handlers, registers the session, and returns its id. See acp.Delegate.
func (d *acpDelegate) NewSession(
	ctx context.Context, params acp.NewSessionParams, caps acp.ClientCapabilities, client acp.Client,
) (acp.NewSessionResult, error) {
	cwd := params.Cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return acp.NewSessionResult{}, fmt.Errorf("resolving working directory: %w", err)
		}
	}

	project, _, err := d.ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return acp.NewSessionResult{}, err
	}

	// NonInteractiveCurrentBackend uses the stored CLI credentials and never
	// prompts, so it returns a nil backend when the user is not logged in.
	be, err := cmdBackend.NonInteractiveCurrentBackend(ctx, d.ws, cmdBackend.DefaultLoginManager, project)
	if err != nil {
		return acp.NewSessionResult{}, err
	}
	if be == nil {
		return acp.NewSessionResult{}, acp.ErrAuthRequired
	}
	cloudBe, ok := be.(httpstate.Backend)
	if !ok {
		return acp.NewSessionResult{}, errors.New("`pulumi neo` requires the Pulumi Cloud backend")
	}
	if msg := neoUpgradeMessage(cloudBe.Capabilities(ctx), version.Version); msg != "" {
		return acp.NewSessionResult{}, errors.New(msg)
	}

	orgName, projectName, stackRefName, err := resolveTaskTarget(ctx, d.ws, cloudBe, project, "", "")
	if err != nil {
		return acp.NewSessionResult{}, err
	}

	sessionID, err := newACPSessionID()
	if err != nil {
		return acp.NewSessionResult{}, err
	}

	handlers, err := buildACPHandlers(cwd, sessionID, caps, client, d.ws)
	if err != nil {
		return acp.NewSessionResult{}, err
	}

	pc := cloudBe.Client()
	d.mu.Lock()
	d.sessions[sessionID] = &acpSession{
		acpID:        sessionID,
		pc:           pc,
		poster:       pc,
		orgName:      orgName,
		projectName:  projectName,
		stackRefName: stackRefName,
		cwd:          cwd,
		handlers:     handlers,
		client:       client,
	}
	d.mu.Unlock()

	return acp.NewSessionResult{SessionID: sessionID}, nil
}

// Prompt runs one prompt turn: on the first prompt it creates the Neo task and
// starts the event loop; on later prompts it posts the user's message. Either
// way it blocks until the turn ends (a final assistant message, cancellation, or
// a fatal error) and reports the stop reason. See acp.Delegate.
func (d *acpDelegate) Prompt(ctx context.Context, params acp.PromptParams) (acp.PromptResult, error) {
	s, ok := d.session(params.SessionID)
	if !ok {
		return acp.PromptResult{}, fmt.Errorf("unknown session %q", params.SessionID)
	}
	text := promptText(params.Prompt)

	// Register this turn before any work so the pump can signal it the moment
	// the turn ends, even for a very fast turn. ACP drives one prompt at a time
	// per session; reject an overlapping prompt rather than overwrite activeTurn,
	// which would orphan the prior waiter (the pump only ever signals the latest).
	done := make(chan turnResult, 1)
	s.mu.Lock()
	if s.activeTurn != nil {
		s.mu.Unlock()
		return acp.PromptResult{}, fmt.Errorf("a prompt turn is already in progress for session %q", params.SessionID)
	}
	s.activeTurn = done
	needStart := !s.started
	taskID := s.taskID
	s.mu.Unlock()

	var startErr error
	if needStart {
		startErr = s.start(d.baseCtx, text)
	} else {
		startErr = s.poster.PostNeoTaskUserEvent(ctx, s.orgName, taskID,
			apitype.AgentUserEventUserMessage{Type: "user_message", Content: text})
	}
	if startErr != nil {
		s.mu.Lock()
		s.activeTurn = nil
		s.mu.Unlock()
		return acp.PromptResult{}, startErr
	}

	select {
	case <-ctx.Done():
		return acp.PromptResult{}, ctx.Err()
	case tr := <-done:
		if tr.err != nil {
			return acp.PromptResult{}, tr.err
		}
		return acp.PromptResult{StopReason: tr.reason}, nil
	}
}

// Cancel posts a cancel user event for the session's task, if one is running.
// The turn ends with StopCancelled once the backend acknowledges. See
// acp.Delegate.
func (d *acpDelegate) Cancel(ctx context.Context, params acp.CancelParams) error {
	s, ok := d.session(params.SessionID)
	if !ok {
		return nil
	}
	s.mu.Lock()
	started, taskID := s.started, s.taskID
	s.mu.Unlock()
	if !started || taskID == "" {
		return nil
	}
	return s.poster.PostNeoTaskUserEvent(ctx, s.orgName, taskID, apitype.AgentUserEventCancel{Type: "user_cancel"})
}

// session returns the registered session for id. Used by Prompt and Cancel to
// find the state established by NewSession.
func (d *acpDelegate) session(id string) (*acpSession, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.sessions[id]
	return s, ok
}

// buildACPHandlers constructs the CLI-local tool handlers for a session rooted
// at cwd. When the editor advertised the matching fs/terminal capability, the
// filesystem and shell tools are routed through the editor (so writes appear as
// native diffs, reads see unsaved buffers, and commands run in the editor's
// terminal); otherwise they run locally. The pulumi tool always runs locally.
func buildACPHandlers(
	cwd, sessionID string, caps acp.ClientCapabilities, caller acp.Caller, ws pkgWorkspace.Context,
) (map[string]ToolHandler, error) {
	lt, err := buildLocalToolHandlers(cwd, ws)
	if err != nil {
		return nil, err
	}
	if caller == nil {
		return lt.handlers, nil
	}
	if caps.FS.WriteTextFile || caps.FS.ReadTextFile {
		cfs := &acp.ClientFS{Caller: caller, SessionID: sessionID}
		if caps.FS.WriteTextFile {
			lt.fs.OnWrite = cfs.WriteTextFile
		}
		if caps.FS.ReadTextFile {
			lt.fs.OnRead = cfs.ReadTextFile
		}
	}
	if caps.Terminal {
		ct := &acp.ClientTerminal{Caller: caller, SessionID: sessionID}
		lt.sh.OnExec = func(
			ctx context.Context, command, dir string, timeout time.Duration,
		) (tools.ShellResult, error) {
			return runInEditorTerminal(ctx, ct, command, dir, timeout)
		}
	}
	return lt.handlers, nil
}

// newACPSessionID returns a random, opaque ACP session id.
func newACPSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating session id: %w", err)
	}
	return "sess_" + hex.EncodeToString(b[:]), nil
}
