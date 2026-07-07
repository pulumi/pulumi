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
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/tools"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// fakeACPCaller is a no-op acp.Caller; buildACPHandlers only needs a non-nil
// value to wire the editor-backed write path.
type fakeACPCaller struct{}

func (fakeACPCaller) Call(context.Context, string, any, any) error { return nil }

func TestBuildACPHandlersRoutesWritesToEditor(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	handlers, err := buildACPHandlers(cwd, "sess_1",
		acp.ClientCapabilities{FS: acp.FileSystemCapability{WriteTextFile: true}},
		fakeACPCaller{}, pkgWorkspace.Instance)
	require.NoError(t, err)

	for _, name := range []string{"filesystem", "shell", "pulumi"} {
		assert.Contains(t, handlers, name)
	}

	fs, ok := handlers["filesystem"].(*tools.Filesystem)
	require.True(t, ok)
	require.NotNil(t, fs.OnWrite, "fs.writeTextFile capability should route writes through the editor")
	assert.Nil(t, fs.OnRead, "fs.readTextFile was not advertised, so reads stay local")
}

func TestBuildACPHandlersRoutesReadsToEditor(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	handlers, err := buildACPHandlers(cwd, "sess_1",
		acp.ClientCapabilities{FS: acp.FileSystemCapability{ReadTextFile: true}},
		fakeACPCaller{}, pkgWorkspace.Instance)
	require.NoError(t, err)

	fs, ok := handlers["filesystem"].(*tools.Filesystem)
	require.True(t, ok)
	require.NotNil(t, fs.OnRead, "fs.readTextFile capability should route reads through the editor")
	assert.Nil(t, fs.OnWrite, "fs.writeTextFile was not advertised, so writes stay local")
}

func TestBuildACPHandlersLocalWhenNoFSCapability(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	handlers, err := buildACPHandlers(cwd, "sess_1",
		acp.ClientCapabilities{}, fakeACPCaller{}, pkgWorkspace.Instance)
	require.NoError(t, err)

	fs, ok := handlers["filesystem"].(*tools.Filesystem)
	require.True(t, ok)
	assert.Nil(t, fs.OnWrite, "without the capability, writes stay local")
	assert.Nil(t, fs.OnRead, "without the capability, reads stay local")

	sh, ok := handlers["shell"].(*tools.Shell)
	require.True(t, ok)
	assert.Nil(t, sh.OnExec, "without the terminal capability, shell stays local")
}

func TestBuildACPHandlersRoutesShellToTerminal(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	handlers, err := buildACPHandlers(cwd, "sess_1",
		acp.ClientCapabilities{Terminal: true}, fakeACPCaller{}, pkgWorkspace.Instance)
	require.NoError(t, err)

	sh, ok := handlers["shell"].(*tools.Shell)
	require.True(t, ok)
	require.NotNil(t, sh.OnExec, "terminal capability should route shell commands through the editor")
}

// scriptedTermCaller records the terminal/* methods invoked and can fail the
// wait step to exercise the timeout path, or report a signal exit to exercise
// the signal-mapping path. It otherwise leaves results at their zero value.
type scriptedTermCaller struct {
	mu      sync.Mutex
	methods []string
	waitErr error
	signal  string
}

func (c *scriptedTermCaller) Call(_ context.Context, method string, _, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.methods = append(c.methods, method)
	if method == "terminal/wait_for_exit" {
		if c.waitErr != nil {
			return c.waitErr
		}
		// A signal-terminated process reports a null exit code plus the signal.
		// Decode into the caller-provided result so we don't name acp's
		// unexported exitStatus type.
		if c.signal != "" {
			_ = json.Unmarshal([]byte(`{"exitCode":null,"signal":`+strconv.Quote(c.signal)+`}`), result)
		}
	}
	return nil
}

func TestRunInEditorTerminalSuccess(t *testing.T) {
	t.Parallel()

	sc := &scriptedTermCaller{}
	ct := &acp.ClientTerminal{Caller: sc, SessionID: "sess_1"}

	out, err := runInEditorTerminal(t.Context(), ct, "echo hi", "/work", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, out.ExitCode)
	assert.Equal(t, "", out.Stderr)
	assert.Equal(t,
		[]string{"terminal/create", "terminal/wait_for_exit", "terminal/output", "terminal/release"},
		sc.methods)
}

func TestRunInEditorTerminalTimeout(t *testing.T) {
	t.Parallel()

	sc := &scriptedTermCaller{waitErr: context.DeadlineExceeded}
	ct := &acp.ClientTerminal{Caller: sc, SessionID: "sess_1"}

	out, err := runInEditorTerminal(t.Context(), ct, "sleep 100", "/work", time.Minute)
	require.Error(t, err)
	assert.True(t, out.TimedOut)
	assert.Contains(t, sc.methods, "terminal/kill")
}

func TestRunInEditorTerminalSignalMapsToNonZeroExit(t *testing.T) {
	t.Parallel()

	// A signal-terminated command has a null exit code over the wire; it must
	// surface as a non-zero exit (-1, matching the local shell) rather than a
	// clean exit 0.
	sc := &scriptedTermCaller{signal: "SIGKILL"}
	ct := &acp.ClientTerminal{Caller: sc, SessionID: "sess_1"}

	out, err := runInEditorTerminal(t.Context(), ct, "sleep 100", "/work", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, -1, out.ExitCode)
	assert.False(t, out.TimedOut)
}

func TestACPDelegateSessionLookup(t *testing.T) {
	t.Parallel()

	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["sess_1"] = &acpSession{
		orgName:      "acme",
		projectName:  "proj",
		stackRefName: "dev",
		cwd:          "/work",
		handlers:     map[string]ToolHandler{},
	}

	got, ok := d.session("sess_1")
	require.True(t, ok)
	assert.Equal(t, "acme", got.orgName)
	assert.Equal(t, "proj", got.projectName)
	assert.Equal(t, "dev", got.stackRefName)
	assert.Equal(t, "/work", got.cwd)
	assert.Nil(t, got.pc)
	require.NotNil(t, got.handlers)

	_, ok = d.session("missing")
	assert.False(t, ok)
}

func TestPromptRejectsOverlappingTurn(t *testing.T) {
	t.Parallel()

	api := &fakeTaskAPI{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, baseCtx: t.Context(), sessions: map[string]*acpSession{}}
	d.sessions["sess_1"] = &acpSession{
		acpID:   "sess_1",
		api:     api,
		orgName: "acme",
		started: true,
		taskID:  "task_1",
		// A turn is already registered; a second prompt must be rejected rather
		// than overwrite it and orphan the prior waiter.
		activeTurn: make(chan turnResult, 1),
	}

	_, err := d.Prompt(t.Context(), acp.PromptParams{
		SessionID: "sess_1",
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "hi"}},
	})
	require.ErrorContains(t, err, "already in progress")
	assert.Empty(t, api.posted, "overlapping prompt must not post to the backend")
}

func TestPromptRejectedAfterSessionEnded(t *testing.T) {
	t.Parallel()

	api := &fakeTaskAPI{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, baseCtx: t.Context(), sessions: map[string]*acpSession{}}
	d.sessions["sess_1"] = &acpSession{
		acpID:   "sess_1",
		api:     api,
		orgName: "acme",
		started: true,
		taskID:  "task_1",
		// The event loop exited with an error; a new turn could never complete.
		ended:  true,
		runErr: errors.New("event stream lost"),
	}

	_, err := d.Prompt(t.Context(), acp.PromptParams{
		SessionID: "sess_1",
		Prompt:    []acp.ContentBlock{{Type: "text", Text: "hi"}},
	})
	require.ErrorContains(t, err, "event stream lost")
	assert.Empty(t, api.posted, "a prompt on an ended session must not post to the backend")
}

// fakeTaskAPI records the user events and mode PATCHes a session sends to the
// (fake) Neo backend.
type fakeTaskAPI struct {
	mu      sync.Mutex
	posted  []any
	patches []client.UpdateNeoTaskOptions
}

func (p *fakeTaskAPI) PostNeoTaskUserEvent(_ context.Context, _, _ string, body any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.posted = append(p.posted, body)
	return nil
}

func (p *fakeTaskAPI) UpdateNeoTask(
	_ context.Context, _, _ string, opts client.UpdateNeoTaskOptions,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.patches = append(p.patches, opts)
	return nil
}

func TestCancelPostsUserCancel(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["sess_x"] = &acpSession{acpID: "sess_x", api: fp, orgName: "acme", taskID: "task_1", started: true}

	require.NoError(t, d.Cancel(t.Context(), acp.CancelParams{SessionID: "sess_x"}))

	fp.mu.Lock()
	defer fp.mu.Unlock()
	require.Len(t, fp.posted, 1)
	_, ok := fp.posted[0].(apitype.AgentUserEventCancel)
	assert.True(t, ok)
}

func TestCancelNoopWhenUnknownOrNotStarted(t *testing.T) {
	t.Parallel()

	fp := &fakeTaskAPI{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["not_started"] = &acpSession{api: fp}

	require.NoError(t, d.Cancel(t.Context(), acp.CancelParams{SessionID: "missing"}))
	require.NoError(t, d.Cancel(t.Context(), acp.CancelParams{SessionID: "not_started"}))

	fp.mu.Lock()
	defer fp.mu.Unlock()
	assert.Empty(t, fp.posted)
}
