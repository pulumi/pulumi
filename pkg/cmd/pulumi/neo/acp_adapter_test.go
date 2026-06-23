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
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	poster := &fakePoster{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, baseCtx: t.Context(), sessions: map[string]*acpSession{}}
	d.sessions["sess_1"] = &acpSession{
		acpID:   "sess_1",
		poster:  poster,
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
	assert.Empty(t, poster.posted, "overlapping prompt must not post to the backend")
}

// capturedNotif is one session/update the fake client received.
type capturedNotif struct {
	method string
	params any
}

// fakeACPClient implements acp.Client, recording notifications and serving a
// canned permission outcome for Call.
type fakeACPClient struct {
	mu            sync.Mutex
	notifications []capturedNotif
	callMethod    string
	callParams    any
	permResult    acp.RequestPermissionResult
	callErr       error
}

func (c *fakeACPClient) Notify(_ context.Context, method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, capturedNotif{method: method, params: params})
	return nil
}

func (c *fakeACPClient) Call(_ context.Context, method string, params, result any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callMethod, c.callParams = method, params
	if c.callErr != nil {
		return c.callErr
	}
	if r, ok := result.(*acp.RequestPermissionResult); ok {
		*r = c.permResult
	}
	return nil
}

// fakePoster records the user events posted back to the (fake) Neo task.
type fakePoster struct {
	mu     sync.Mutex
	posted []any
}

func (p *fakePoster) PostNeoTaskUserEvent(_ context.Context, _, _ string, body any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.posted = append(p.posted, body)
	return nil
}

func TestToolTrackerTranslate(t *testing.T) {
	t.Parallel()

	var tr toolTracker

	u, ok := tr.translate(UIAssistantMessage{Content: "hi"})
	require.True(t, ok)
	require.IsType(t, acp.AgentMessageChunk{}, u)
	assert.Equal(t, "hi\n", u.(acp.AgentMessageChunk).Content.Text,
		"each complete message gets a trailing newline so successive messages don't run together")

	_, ok = tr.translate(UIAssistantMessage{})
	assert.False(t, ok, "empty assistant message produces no update")

	u, ok = tr.translate(UIToolStarted{Name: "filesystem__edit", Args: json.RawMessage(`{"x":1}`)})
	require.True(t, ok)
	start := u.(acp.ToolCallStart)
	assert.Equal(t, "tc_1", start.ToolCallID)
	assert.Equal(t, acp.ToolKindEdit, start.Kind)
	assert.Equal(t, acp.ToolStatusInProgress, start.Status)

	u, ok = tr.translate(UIToolCompleted{Name: "filesystem__edit", Result: json.RawMessage(`{"ok":true}`)})
	require.True(t, ok)
	upd := u.(acp.ToolCallProgress)
	assert.Equal(t, "tc_1", upd.ToolCallID, "completed update reuses the started call id")
	assert.Equal(t, acp.ToolStatusCompleted, upd.Status)

	// After completion the current id is cleared, so a stray progress event no
	// longer correlates to the finished call.
	_, ok = tr.translate(UIToolProgress{Message: "late"})
	assert.False(t, ok, "progress after completion produces no update")

	u, ok = tr.translate(UIToolCompleted{IsError: true})
	require.True(t, ok)
	assert.Equal(t, acp.ToolStatusFailed, u.(acp.ToolCallProgress).Status)

	u, ok = tr.translate(UITodoList{Items: []UITodoItem{{Content: "do", Status: "pending", Priority: "high"}}})
	require.True(t, ok)
	plan := u.(acp.PlanUpdate)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, acp.PlanEntry{Content: "do", Status: "pending", Priority: "high"}, plan.Entries[0])
}

func TestToolStartHasReadableTitleAndLocation(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	main := filepath.Join(cwd, "main.go")

	tr := toolTracker{cwd: cwd}
	u, ok := tr.translate(UIToolStarted{
		Name: "filesystem__read",
		Args: mustJSON(t, map[string]string{"file_path": main}),
	})
	require.True(t, ok)
	start := u.(acp.ToolCallStart)

	assert.Equal(t, "Read ./main.go", start.Title, "title should name the file, relative to cwd")
	assert.Equal(t, acp.ToolKindRead, start.Kind)
	require.Len(t, start.Locations, 1)
	assert.Equal(t, main, start.Locations[0].Path)
}

func TestToolTitleAndLocations(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	fsArgs := func(field, path string) toolArgs { return parseToolArgs(mustJSON(t, map[string]string{field: path})) }

	// Filesystem calls name the file they touch, relative to cwd when possible.
	assert.Equal(t, "Read ./pyproject.toml",
		toolTitle("filesystem__read", fsArgs("file_path", filepath.Join(cwd, "pyproject.toml")), cwd))
	assert.Equal(t, "Content replace ./src/a.go",
		toolTitle("filesystem__content_replace", fsArgs("path", filepath.Join(cwd, "src", "a.go")), cwd))
	outside := filepath.Join(filepath.Dir(cwd), "elsewhere", "hosts")
	assert.Equal(t, "Read "+outside,
		toolTitle("filesystem__read", fsArgs("file_path", outside), cwd),
		"paths outside cwd stay absolute")
	assert.Equal(t, "Read",
		toolTitle("filesystem__read", toolArgs{}, cwd), "no path falls back to the verb")

	// Shell calls render the command itself, flattened to one line.
	assert.Equal(t, "git status",
		toolTitle("shell__shell_execute", parseToolArgs(json.RawMessage(`{"command":"git status"}`)), cwd))
	assert.Equal(t, "go build ./...",
		toolTitle("shell__shell_execute", parseToolArgs(json.RawMessage(`{"command":"go build\n  ./..."}`)), cwd),
		"multi-line commands collapse to a single line")
	assert.Equal(t, "Shell execute",
		toolTitle("shell__shell_execute", toolArgs{}, cwd), "no command falls back to the verb")

	assert.Equal(t, "weird", toolTitle("weird", toolArgs{}, ""), "names without a server prefix pass through")

	assert.Nil(t, toolLocations(toolArgs{}))
	assert.Nil(t, toolLocations(parseToolArgs(json.RawMessage(`{"command":"ls"}`))), "no file target -> no location")
	locs := toolLocations(parseToolArgs(json.RawMessage(`{"path":"/work/src"}`)))
	require.Len(t, locs, 1)
	assert.Equal(t, "/work/src", locs[0].Path)
}

// mustJSON marshals v to a json.RawMessage, failing the test on error. Used to
// build tool-call args with OS-correct (cross-platform) file paths.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestPumpForwardsUpdatesAndEndsTurn(t *testing.T) {
	t.Parallel()

	fc := &fakeACPClient{}
	done := make(chan turnResult, 1)
	s := &acpSession{acpID: "sess_x", client: fc, activeTurn: done}
	uiCh := make(chan UIEvent, 8)
	go s.pump(t.Context(), uiCh)

	uiCh <- UIAssistantMessage{Content: "hello"}
	uiCh <- UIToolStarted{Name: "shell__exec"}
	uiCh <- UITaskIdle{}

	select {
	case tr := <-done:
		require.NoError(t, tr.err)
		assert.Equal(t, acp.StopEndTurn, tr.reason)
	case <-time.After(2 * time.Second):
		t.Fatal("turn did not finish")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	require.Len(t, fc.notifications, 2)
	for _, n := range fc.notifications {
		assert.Equal(t, "session/update", n.method)
	}
}

// gatedClient is an acp.Client whose Notify blocks until gate is closed,
// simulating an editor that is slow to read stdout. It counts completed
// notifications so a test can assert none were lost.
type gatedClient struct {
	gate chan struct{}
	mu   sync.Mutex
	n    int
}

func (c *gatedClient) Notify(_ context.Context, _ string, _ any) error {
	<-c.gate
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil
}

func (c *gatedClient) Call(context.Context, string, any, any) error { return nil }

func (c *gatedClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// TestPumpKeepsDrainingWhileEditorIsSlow is the regression guard for the
// decoupling between draining UIEvents and writing to the editor. A slow editor
// (Notify blocked) must not stall the pump: it keeps accepting events into an
// unbounded queue, so Session.sendUI never fills uiCh and never drops a
// turn-boundary event. The turn still resolves only after the editor drains its
// updates, and no notification is lost.
func TestPumpKeepsDrainingWhileEditorIsSlow(t *testing.T) {
	t.Parallel()

	fc := &gatedClient{gate: make(chan struct{})}
	done := make(chan turnResult, 1)
	s := &acpSession{acpID: "sess_x", client: fc, activeTurn: done}

	// Unbuffered: a successful send proves the pump actually received the event
	// rather than it sitting in a channel buffer.
	uiCh := make(chan UIEvent)
	go s.pump(t.Context(), uiCh)

	const n = 100
	for i := range n {
		select {
		case uiCh <- UIAssistantMessage{Content: "x"}:
		case <-time.After(2 * time.Second):
			t.Fatalf("pump stopped draining at event %d while the editor was slow", i)
		}
	}
	uiCh <- UITaskIdle{}

	// The turn can't end before its updates reach the (still-blocked) editor.
	select {
	case <-done:
		t.Fatal("turn resolved before the editor drained its updates")
	case <-time.After(50 * time.Millisecond):
	}

	close(fc.gate) // editor catches up
	select {
	case tr := <-done:
		require.NoError(t, tr.err)
		assert.Equal(t, acp.StopEndTurn, tr.reason)
	case <-time.After(2 * time.Second):
		t.Fatal("turn did not finish after the editor caught up")
	}
	assert.Equal(t, n, fc.count(), "every notification should be delivered, none dropped")
}

func TestPumpBoundaryReasons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event UIEvent
		check func(t *testing.T, tr turnResult)
	}{
		{"cancelled", UICancelled{}, func(t *testing.T, tr turnResult) {
			require.NoError(t, tr.err)
			assert.Equal(t, acp.StopCancelled, tr.reason)
		}},
		{"error", UIError{Message: "boom"}, func(t *testing.T, tr turnResult) {
			require.Error(t, tr.err)
			assert.Contains(t, tr.err.Error(), "boom")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			done := make(chan turnResult, 1)
			s := &acpSession{acpID: "sess_x", client: &fakeACPClient{}, activeTurn: done}
			uiCh := make(chan UIEvent, 1)
			go s.pump(t.Context(), uiCh)
			uiCh <- tt.event
			select {
			case tr := <-done:
				tt.check(t, tr)
			case <-time.After(2 * time.Second):
				t.Fatal("turn did not finish")
			}
		})
	}
}

func TestRequestPermissionRelaysDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		outcome      acp.PermissionOutcome
		callErr      error
		wantApproved bool
	}{
		{"allow", acp.PermissionOutcome{Outcome: "selected", OptionID: "allow"}, nil, true},
		{"reject", acp.PermissionOutcome{Outcome: "selected", OptionID: "reject"}, nil, false},
		{"cancelled", acp.PermissionOutcome{Outcome: "cancelled"}, nil, false},
		{"call error", acp.PermissionOutcome{}, errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fc := &fakeACPClient{permResult: acp.RequestPermissionResult{Outcome: tt.outcome}, callErr: tt.callErr}
			fp := &fakePoster{}
			s := &acpSession{acpID: "sess_x", client: fc, poster: fp, orgName: "acme", taskID: "task_1"}

			s.requestPermission(t.Context(), UIApprovalRequest{ApprovalID: "appr_1", Message: "run it?"})

			fc.mu.Lock()
			assert.Equal(t, "session/request_permission", fc.callMethod)
			fc.mu.Unlock()

			fp.mu.Lock()
			defer fp.mu.Unlock()
			require.Len(t, fp.posted, 1)
			conf, ok := fp.posted[0].(apitype.AgentUserEventUserConfirmation)
			require.True(t, ok)
			assert.Equal(t, "appr_1", conf.ApprovalID)
			assert.Equal(t, tt.wantApproved, conf.Approved)
		})
	}
}

func TestCancelPostsUserCancel(t *testing.T) {
	t.Parallel()

	fp := &fakePoster{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["sess_x"] = &acpSession{acpID: "sess_x", poster: fp, orgName: "acme", taskID: "task_1", started: true}

	require.NoError(t, d.Cancel(t.Context(), acp.CancelParams{SessionID: "sess_x"}))

	fp.mu.Lock()
	defer fp.mu.Unlock()
	require.Len(t, fp.posted, 1)
	_, ok := fp.posted[0].(apitype.AgentUserEventCancel)
	assert.True(t, ok)
}

func TestCancelNoopWhenUnknownOrNotStarted(t *testing.T) {
	t.Parallel()

	fp := &fakePoster{}
	d := &acpDelegate{ws: pkgWorkspace.Instance, sessions: map[string]*acpSession{}}
	d.sessions["not_started"] = &acpSession{poster: fp}

	require.NoError(t, d.Cancel(t.Context(), acp.CancelParams{SessionID: "missing"}))
	require.NoError(t, d.Cancel(t.Context(), acp.CancelParams{SessionID: "not_started"}))

	fp.mu.Lock()
	defer fp.mu.Unlock()
	assert.Empty(t, fp.posted)
}
