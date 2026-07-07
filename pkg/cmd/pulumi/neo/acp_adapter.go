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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// neoACPIdentity is how the adapter introduces itself to the editor on the ACP
// initialize handshake: the agent name/title and the single auth method we
// advertise. Authentication cannot run an interactive browser login over the
// stdio JSON-RPC channel, so it only verifies that a prior `pulumi login`
// session exists (see acpDelegate.CheckAuth).
var neoACPIdentity = acp.Identity{
	Name:  "pulumi-neo",
	Title: "Pulumi Neo",
	AuthMethods: []acp.AuthMethod{{
		ID:   "pulumi-login",
		Name: "Pulumi login",
		Description: "Authenticate by running `pulumi login` in a terminal. " +
			"Neo uses your existing Pulumi Cloud session.",
	}},
}

// errNeoAuthRequired wraps acp.ErrAuthRequired with the Pulumi-specific message
// the editor surfaces to the user. Wrapping keeps errors.Is(err,
// acp.ErrAuthRequired) true so the agent still maps it to the ACP
// "Authentication required" code, while err.Error() carries the login hint.
var errNeoAuthRequired = fmt.Errorf(
	"not authenticated with Pulumi Cloud; run `pulumi login`: %w", acp.ErrAuthRequired)

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
			agent := acp.NewAgent(neoACPIdentity, version.Version, &acpDelegate{
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

// currentBackend resolves the Pulumi Cloud backend from the stored CLI
// credentials, returning errNeoAuthRequired (which wraps acp.ErrAuthRequired)
// when the user is not logged in. It never prompts, so it is safe to call on the
// JSON-RPC channel. The project (if any) is returned alongside so callers needing
// it for target resolution don't re-read it.
func (d *acpDelegate) currentBackend(ctx context.Context) (backend.Backend, *workspace.Project, error) {
	project, _, err := d.ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, nil, err
	}
	// NonInteractiveCurrentBackend uses the stored CLI credentials and never
	// prompts, so it returns a nil backend when the user is not logged in.
	be, err := cmdBackend.NonInteractiveCurrentBackend(ctx, d.ws, cmdBackend.DefaultLoginManager, project)
	if err != nil {
		return nil, nil, err
	}
	if be == nil {
		return nil, nil, errNeoAuthRequired
	}
	return be, project, nil
}

// CheckAuth reports whether a usable Pulumi Cloud session exists. See
// acp.Delegate; the agent calls it from the `authenticate` handler.
func (d *acpDelegate) CheckAuth(ctx context.Context) error {
	_, _, err := d.currentBackend(ctx)
	return err
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

	be, project, err := d.currentBackend(ctx)
	if err != nil {
		return acp.NewSessionResult{}, err
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

	sess := &acpSession{
		acpID:        sessionID,
		api:          cloudBe.Client(),
		orgName:      orgName,
		projectName:  projectName,
		stackRefName: stackRefName,
		cwd:          cwd,
		handlers:     handlers,
		client:       client,
	}
	d.mu.Lock()
	d.sessions[sessionID] = sess
	d.mu.Unlock()

	// Advertise the read-only and plan-mode config options at their defaults so
	// the editor can change them before (or, for read-only, during) the session.
	return acp.NewSessionResult{
		SessionID:     sessionID,
		ConfigOptions: sess.configOptionsSnapshot(),
	}, nil
}

// Prompt runs one prompt turn for the session, blocking until it ends and
// reporting the stop reason. The turn lifecycle itself lives on acpSession
// (see runTurn). See acp.Delegate.
func (d *acpDelegate) Prompt(ctx context.Context, params acp.PromptParams) (acp.PromptResult, error) {
	s, ok := d.session(params.SessionID)
	if !ok {
		return acp.PromptResult{}, fmt.Errorf("unknown session %q", params.SessionID)
	}
	return s.runTurn(ctx, d.baseCtx, promptText(params.Prompt))
}

// Cancel posts a cancel user event for the session's task, if one is running.
// The turn ends with StopCancelled once the backend acknowledges. See
// acp.Delegate.
func (d *acpDelegate) Cancel(ctx context.Context, params acp.CancelParams) error {
	s, ok := d.session(params.SessionID)
	if !ok {
		return nil
	}
	return s.cancel(ctx)
}

// SetConfigOption applies a `permission` or `plan` config-option change for the
// session and returns the full, updated option list. Read-only takes effect
// immediately (PATCHing a running task); plan mode only applies before the task
// is created and is otherwise clamped. See acp.Delegate.
func (d *acpDelegate) SetConfigOption(
	ctx context.Context, params acp.SetConfigOptionParams,
) (acp.SetConfigOptionResult, error) {
	s, ok := d.session(params.SessionID)
	if !ok {
		return acp.SetConfigOptionResult{}, fmt.Errorf("unknown session %q", params.SessionID)
	}
	if err := s.setConfigOption(ctx, params.ConfigID, params.Value); err != nil {
		return acp.SetConfigOptionResult{}, err
	}
	return acp.SetConfigOptionResult{ConfigOptions: s.configOptionsSnapshot()}, nil
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

// runInEditorTerminal runs a shell command in the editor's terminal and shapes
// the result like the local shell tool, through the shared tools.ShellResult so
// the wire shape stays identical. The editor merges stdout and stderr, so the
// combined stream is reported as stdout and stderr is left empty. It is wired
// into tools.Shell.OnExec by buildACPHandlers.
func runInEditorTerminal(
	ctx context.Context, ct *acp.ClientTerminal, command, dir string, timeout time.Duration,
) (tools.ShellResult, error) {
	program, args := tools.ShellInvocation(command)
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Cap the captured output at the same limit as the local shell tool, so
	// truncation behaves identically on both paths.
	res, err := ct.Run(tctx, program, args, dir, tools.MaxOutputBytes)

	exitCode := res.ExitCode
	if res.Signal != "" {
		// A signal-terminated process carries no exit code (the editor reports
		// null, which decodes to 0). Surface it as -1 to match the local shell
		// tool, so a signal-killed command isn't reported as a clean exit 0.
		exitCode = -1
	}
	out := tools.ShellResult{
		Stdout:    res.Output,
		ExitCode:  exitCode,
		Truncated: res.Truncated,
		TimedOut:  res.TimedOut,
	}
	if res.TimedOut {
		return out, fmt.Errorf("shell command timed out after %s", timeout)
	}
	if err != nil {
		// A non-timeout transport error means we can't trust the result.
		return tools.ShellResult{}, err
	}
	return out, nil
}

// newACPSessionID returns a random, opaque ACP session id.
func newACPSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating session id: %w", err)
	}
	return "sess_" + hex.EncodeToString(b[:]), nil
}
