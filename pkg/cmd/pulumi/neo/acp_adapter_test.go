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
	"os"
	"path/filepath"
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

func TestNeoACPIdentityAdvertisesTerminalLogin(t *testing.T) {
	t.Parallel()

	// The single advertised auth method is terminal-typed: editors run our
	// launch binary with these args (`pulumi login`) in a real terminal. The id
	// is wire-visible and must stay stable for editors that persist it.
	identity := neoACPIdentity()
	require.Len(t, identity.AuthMethods, 1)
	m := identity.AuthMethods[0]
	assert.Equal(t, "pulumi-login", m.ID)
	assert.Equal(t, acp.AuthMethodTypeTerminal, m.Type)
	assert.Equal(t, []string{"login"}, m.Args)
	assert.Empty(t, m.Env)
	assert.NotEmpty(t, m.Description, "description doubles as the degraded-method guidance")

	// The pre-stabilization terminal-auth meta mirrors the typed fields with an
	// explicit command (our own binary) for editors that only execute the meta
	// form (e.g. current stable Zed).
	meta, ok := m.Meta[acp.MetaKeyTerminalAuth].(acp.TerminalAuthMeta)
	require.True(t, ok, "terminal-auth meta must be advertised")
	assert.Equal(t, "pulumi login", meta.Label)
	assert.NotEmpty(t, meta.Command)
	assert.Equal(t, []string{"login"}, meta.Args)
}

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

// recordingFSCaller answers the agent's outbound fs/* requests: it records the
// last method and params (re-encoded as JSON so assertions don't need acp's
// unexported wire structs) and serves readContent for fs/read_text_file.
type recordingFSCaller struct {
	calls       int
	method      string
	params      json.RawMessage
	readContent string
}

func (c *recordingFSCaller) Call(_ context.Context, method string, params, result any) error {
	c.calls++
	c.method = method
	c.params, _ = json.Marshal(params)
	if method == "fs/read_text_file" && result != nil {
		return json.Unmarshal([]byte(`{"content":`+strconv.Quote(c.readContent)+`}`), result)
	}
	return nil
}

// fsParams is the wire shape of the fs/read_text_file and fs/write_text_file
// params, decoded from the recorded JSON for assertions.
type fsParams struct {
	SessionID string `json:"sessionId"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

// TestACPFilesystemReadUsesEditorContent: with fs.readTextFile advertised, a
// filesystem read is served by the editor — returning its (possibly unsaved)
// buffer, with the tool's offset/limit slicing still applied — ignoring
// whatever is on local disk.
func TestACPFilesystemReadUsesEditorContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	require.NoError(t, os.WriteFile(target, []byte("ON DISK\n"), 0o600))

	caller := &recordingFSCaller{readContent: "line1\nline2\nline3\n"}
	handlers, err := buildACPHandlers(root, "sess_abc",
		acp.ClientCapabilities{FS: acp.FileSystemCapability{ReadTextFile: true}},
		caller, pkgWorkspace.Instance)
	require.NoError(t, err)

	res, err := handlers["filesystem"].Invoke(t.Context(), "read",
		mustJSON(t, map[string]any{"file_path": target, "offset": 1}))
	require.NoError(t, err)
	assert.Equal(t, "fs/read_text_file", caller.method)

	// The result reflects the editor's content (sliced from offset 1), not disk.
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "line2")
	assert.NotContains(t, string(raw), "ON DISK")
	assert.NotContains(t, string(raw), "line1", "offset slicing should still apply to editor content")
}

// TestACPFilesystemEditUsesEditorBufferOverDisk guards that an ACP-backed edit
// matches and writes against the editor's buffer (via the read routing), not
// stale disk: the on-disk content here would not contain old_string at all.
func TestACPFilesystemEditUsesEditorBufferOverDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	require.NoError(t, os.WriteFile(target, []byte("STALE DISK CONTENT\n"), 0o600))

	caller := &recordingFSCaller{readContent: "func target() {}\n"}
	handlers, err := buildACPHandlers(root, "sess_abc",
		acp.ClientCapabilities{FS: acp.FileSystemCapability{ReadTextFile: true, WriteTextFile: true}},
		caller, pkgWorkspace.Instance)
	require.NoError(t, err)

	res, err := handlers["filesystem"].Invoke(t.Context(), "edit", mustJSON(t, map[string]string{
		"file_path":  target,
		"old_string": "func target()",
		"new_string": "func renamed()",
	}))
	require.NoError(t, err)

	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Successfully edited", "edit should match against the editor buffer")

	// The write carried the buffer-derived content, and disk was left untouched.
	var wrote fsParams
	require.NoError(t, json.Unmarshal(caller.params, &wrote))
	assert.Equal(t, "func renamed() {}\n", wrote.Content)
	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "STALE DISK CONTENT\n", string(onDisk))
}

// TestACPFilesystemEditRoutesThroughEditorWrite is the end-to-end check for the
// edit-as-ACP-write path: with only fs.writeTextFile advertised, a filesystem
// edit reads and computes the modified content locally but commits it through
// the editor's fs/write_text_file, leaving the on-disk file untouched (the
// editor owns the write).
func TestACPFilesystemEditRoutesThroughEditorWrite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "main.go")
	const original = "package main\n\nfunc foo() {}\n"
	require.NoError(t, os.WriteFile(target, []byte(original), 0o600))

	caller := &recordingFSCaller{}
	handlers, err := buildACPHandlers(root, "sess_abc",
		acp.ClientCapabilities{FS: acp.FileSystemCapability{WriteTextFile: true}},
		caller, pkgWorkspace.Instance)
	require.NoError(t, err)

	_, err = handlers["filesystem"].Invoke(t.Context(), "edit", mustJSON(t, map[string]string{
		"file_path":  target,
		"old_string": "func foo()",
		"new_string": "func bar()",
	}))
	require.NoError(t, err)

	// The write was diverted to the editor with the fully-modified content.
	require.Equal(t, 1, caller.calls)
	require.Equal(t, "fs/write_text_file", caller.method)
	var wrote fsParams
	require.NoError(t, json.Unmarshal(caller.params, &wrote))
	// The filesystem tool canonicalizes the path before writing, so the editor
	// receives the symlink-evaluated absolute path.
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realRoot, "main.go"), wrote.Path)
	assert.Equal(t, "sess_abc", wrote.SessionID)
	assert.Equal(t, "package main\n\nfunc bar() {}\n", wrote.Content)

	// Disk is untouched: the CLI did not perform the write itself.
	onDisk, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, original, string(onDisk))
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

// createdTask records one CreateNeoTask call a session made against the fake
// backend.
type createdTask struct {
	content     string
	stackName   string
	projectName string
	opts        client.CreateNeoTaskOptions
}

// fakeTaskAPI is a fake neoSessionAPI: it records the task creations, user
// events, and mode PATCHes a session sends to the (fake) Neo backend, and
// serves stream (which may be nil: reads then block until ctx ends) as the
// task's event stream. The *Err fields, when set, fail the corresponding call.
type fakeTaskAPI struct {
	stream    chan client.NeoStreamEvent
	createErr error
	postErr   error
	updateErr error
	// rejectStack, when true, fails any CreateNeoTask that names a stack with
	// the backend's "invalid entities" rejection, driving the retry-without-
	// stack fallback in createNeoTaskWithEntityRetry.
	rejectStack bool

	mu      sync.Mutex
	created []createdTask
	posted  []any
	patches []client.UpdateNeoTaskOptions
}

func (p *fakeTaskAPI) CreateNeoTask(
	_ context.Context, _, content, stackName, projectName string, opts client.CreateNeoTaskOptions,
) (*client.NeoTaskResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.createErr != nil {
		return nil, p.createErr
	}
	if p.rejectStack && stackName != "" {
		return nil, &apitype.ErrorResponse{Code: 400, Message: "invalid entities for task"}
	}
	p.created = append(p.created, createdTask{
		content: content, stackName: stackName, projectName: projectName, opts: opts,
	})
	return &client.NeoTaskResponse{TaskID: "task_1"}, nil
}

func (p *fakeTaskAPI) StreamNeoTaskEvents(
	context.Context, string, string, string,
) (<-chan client.NeoStreamEvent, error) {
	return p.stream, nil
}

func (p *fakeTaskAPI) PostNeoTaskUserEvent(_ context.Context, _, _ string, body any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.postErr != nil {
		return p.postErr
	}
	p.posted = append(p.posted, body)
	return nil
}

func (p *fakeTaskAPI) UpdateNeoTask(
	_ context.Context, _, _ string, opts client.UpdateNeoTaskOptions,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.updateErr != nil {
		return p.updateErr
	}
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
