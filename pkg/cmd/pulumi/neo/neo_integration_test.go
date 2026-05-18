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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// neoFakeServer fakes the four Pulumi Cloud HTTP endpoints runNeo touches
// (account details, create-task, SSE event stream, and post-user-event) over
// httptest. The handlers record the requests they observe so the test can
// assert on the wire path, and expose a streamSend channel for pushing SSE
// events on demand.
type neoFakeServer struct {
	server *httptest.Server

	mu        sync.Mutex
	posts     []recordedPost
	streamHit bool

	// streamSend pushes raw event payloads (one JSON object per send) to the
	// SSE handler, which frames them as `data: ...\n\n` and flushes. Closing
	// it via endStream ends the stream cleanly.
	streamSend chan []byte
	endOnce    sync.Once
}

type recordedPost struct {
	path string
	body []byte
}

func newNeoFakeServer(t *testing.T) *neoFakeServer {
	t.Helper()

	s := &neoFakeServer{streamSend: make(chan []byte, 16)}
	mux := http.NewServeMux()

	// runNeo discards the result and error of GetPulumiAccountDetails, but a
	// 404 here would log noise. Return a minimal valid serviceUser shape.
	mux.HandleFunc("/api/user", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"githubLogin":   "test-user",
			"organizations": []map[string]any{{"githubLogin": "test-org"}},
		})
	})

	// POST /api/preview/agents/{org}/tasks → CreateNeoTask. Record the body so
	// the test can assert the prompt was forwarded, then return a fixed task ID.
	mux.HandleFunc("/api/preview/agents/test-org/tasks",
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.posts = append(s.posts, recordedPost{path: r.URL.Path, body: body})
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(client.NeoTaskResponse{TaskID: "task-1"})
		})

	// GET /api/preview/agents/{org}/tasks/{id}/events/stream → SSE handler.
	// Mirrors the framing in client_test.go:TestStreamNeoTaskEvents — flush
	// initial headers, then emit one event per streamSend value, each as a
	// `data:` frame followed by a blank line. Exits when the request context
	// is cancelled (the test triggers this by quitting the TUI).
	mux.HandleFunc("/api/preview/agents/test-org/tasks/task-1/events/stream",
		func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			s.streamHit = true
			s.mu.Unlock()

			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				return
			}
			flusher.Flush()

			for {
				select {
				case data, ok := <-s.streamSend:
					if !ok {
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		})

	// POST /api/preview/agents/{org}/tasks/{id} → PostNeoTaskUserEvent. Record
	// the wrapped {"event": ...} body. The dispatcher posts here when the user
	// types into the TUI; the bug-fix scenario doesn't strictly need it, but
	// recording lets the test assert the wiring is reachable.
	mux.HandleFunc("/api/preview/agents/test-org/tasks/task-1",
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.posts = append(s.posts, recordedPost{path: r.URL.Path, body: body})
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		})

	s.server = httptest.NewServer(mux)
	t.Cleanup(func() {
		// Closing streamSend lets any blocked SSE handler return promptly so
		// httptest.Server.Close (also called below) doesn't have to wait on
		// the request context cancellation path.
		s.endStream()
		s.server.Close()
	})
	return s
}

// endStream closes streamSend so the SSE handler returns. Idempotent so tests
// can call it as part of their flow without conflicting with the cleanup.
func (s *neoFakeServer) endStream() {
	s.endOnce.Do(func() { close(s.streamSend) })
}

// sendFinalAssistantMessage pushes a final-turn assistant_message event to the
// SSE stream. Session.Run treats a final message with no CLI tool calls as the
// natural end of an agent turn — used by the non-interactive happy-path test
// to drive the session to a clean shutdown after sendFinalAssistantMessage +
// endStream.
func (s *neoFakeServer) sendFinalAssistantMessage(t *testing.T) {
	t.Helper()
	body, err := json.Marshal(apitype.AgentBackendEventAssistantMessage{
		Type:    backendEventAssistantMessage,
		IsFinal: true,
	})
	require.NoError(t, err)
	env, err := json.Marshal(apitype.AgentConsoleEvent{
		Type:      consoleEventAgentResponse,
		ID:        "evt-1",
		EventBody: body,
	})
	require.NoError(t, err)
	s.streamSend <- env
}

// awaitStreamConnect polls until the SSE handler sees a request or the
// deadline elapses. Returns whether the connect was observed.
func (s *neoFakeServer) awaitStreamConnect(t *testing.T, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.sawStreamConnect() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func (s *neoFakeServer) recordedPosts() []recordedPost {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]recordedPost, len(s.posts))
	copy(out, s.posts)
	return out
}

func (s *neoFakeServer) sawStreamConnect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamHit
}

// TestRunNeoIntegration_DoubleCtrlCExits is the high-fidelity regression test
// for the Ctrl+C double-press hang. It runs the full runNeo entrypoint
// (interactive path) against an in-process httptest.Server with a real
// *client.Client — exercising HTTP marshalling, SSE parsing, and the real
// Session.Run loop — then triggers the TUI to quit and asserts runNeo
// returns within 5s. Pre-fix, p.Run returned nil from tea.Quit, errgroup
// kept gctx alive, Session.Run blocked on `<-ctx.Done` forever, and g.Wait
// hung — this test would time out instead of completing.
//
//nolint:paralleltest // mutates package globals (BackendInstance, pkgWorkspace.Instance, newTeaProgram, isInteractive)
func TestRunNeoIntegration_DoubleCtrlCExits(t *testing.T) {
	isolateWorkspace(t) // PULUMI_STACK="", PULUMI_HOME=t.TempDir()

	srv := newNeoFakeServer(t)

	// Real *client.Client pointed at the fake server. NewClient is the public
	// constructor; it reads no global state and performs no I/O at construction.
	pc := client.NewClient(srv.server.URL, "", false, nil)

	// Fake backend wired with the real client. CurrentBackend short-circuits
	// to BackendInstance when set, bypassing all login / cloud-URL resolution.
	be := newFakeBackend()
	be.ClientV = pc
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "test-org", nil }
	be.CurrentUserF = func() (string, []string, *workspace.TokenInformation, error) {
		return "test-user", []string{"test-org"}, nil, nil
	}

	prevBackend := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = be
	t.Cleanup(func() { cmdBackend.BackendInstance = prevBackend })

	prevWorkspace := pkgWorkspace.Instance
	pkgWorkspace.Instance = &pkgWorkspace.MockContext{}
	t.Cleanup(func() { pkgWorkspace.Instance = prevWorkspace })

	// Force the interactive path. cmdutil.Interactive() reads stdout-is-a-tty,
	// which is false under `go test`, so without this override runNeo would
	// take the non-interactive branch and we'd never hit the bug.
	prevInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = prevInteractive })

	// Headless tea.Program, plus capture so the test can drive Quit.
	var (
		programMu sync.Mutex
		program   *tea.Program
	)
	prevProgram := newTeaProgram
	newTeaProgram = func(m tea.Model) *tea.Program {
		p := tea.NewProgram(
			m,
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithoutSignals(),
			tea.WithoutSignalHandler(),
			tea.WithoutRenderer(),
		)
		programMu.Lock()
		program = p
		programMu.Unlock()
		return p
	}
	t.Cleanup(func() { newTeaProgram = prevProgram })

	// Drive shutdown once the SSE stream is established. This is the moment
	// equivalent to "user pressed Ctrl+C twice and the TUI called tea.Quit".
	go func() {
		// Wait briefly for runNeo to call CreateNeoTask and open the SSE
		// stream. 100ms is generous on local hardware; if the test ever
		// flakes here, switch to polling srv.sawStreamConnect().
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if srv.sawStreamConnect() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		programMu.Lock()
		p := program
		programMu.Unlock()
		if p != nil {
			p.Quit()
		}
	}()

	// runNeo on its own goroutine so the test can enforce a hard timeout
	// rather than relying on `go test -timeout` to catch a hang.
	done := make(chan error, 1)
	go func() {
		done <- runNeo(t.Context(), "do a thing", "" /*stack*/, "test-org", t.TempDir(),
			client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	}()

	select {
	case err := <-done:
		// Session.Run returns ctx.Err() on cancellation; createTask propagates
		// that through the errgroup. Either context.Canceled or nil is an
		// acceptable surface — the load-bearing assertion is "we got here at
		// all". Pre-fix, this branch never fires and the timeout below does.
		if err != nil {
			require.ErrorIs(t, err, context.Canceled,
				"unexpected error from runNeo: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runNeo did not return within 5s — the cancel-on-TUI-exit fix has regressed " +
			"(real *client.Client + real Session.Run hung after tea.Quit)")
	}

	// The fake server should have seen at least the CreateNeoTask call. The
	// SSE stream should also have been opened — otherwise the test wasn't
	// actually exercising the dispatcher / Session.Run lifecycle the bug
	// lives in.
	posts := srv.recordedPosts()
	require.NotEmpty(t, posts, "server saw no requests — runNeo never reached CreateNeoTask")
	assert.Equal(t, "/api/preview/agents/test-org/tasks", posts[0].path,
		"first request must be CreateNeoTask")
	assert.Contains(t, string(posts[0].body), "do a thing",
		"CreateNeoTask body must include the prompt")
	assert.True(t, srv.sawStreamConnect(),
		"SSE stream was never opened — test did not exercise Session.Run")
}

// installNeoTestEnv wires the fake backend and workspace globals for an
// integration test and registers cleanup to restore the originals. The
// `interactive` flag selects which branch of runNeo the test exercises;
// only the interactive path needs the tea.NewProgram override (the caller
// installs that separately when needed).
func installNeoTestEnv(t *testing.T, srv *neoFakeServer, interactive bool) {
	t.Helper()

	pc := client.NewClient(srv.server.URL, "", false, nil)

	be := newFakeBackend()
	be.ClientV = pc
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "test-org", nil }

	prevBackend := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = be
	t.Cleanup(func() { cmdBackend.BackendInstance = prevBackend })

	prevWorkspace := pkgWorkspace.Instance
	pkgWorkspace.Instance = &pkgWorkspace.MockContext{}
	t.Cleanup(func() { pkgWorkspace.Instance = prevWorkspace })

	prevInteractive := isInteractive
	isInteractive = func() bool { return interactive }
	t.Cleanup(func() { isInteractive = prevInteractive })
}

// TestRunNeoIntegration_NonInteractiveHappyPath drives the non-interactive
// branch of runNeo: a prompt is supplied, no TUI is started, and the session
// loop runs against the SSE stream until the server closes the connection.
// Pre-this test, lines 161-186 of neo.go (the entire non-interactive block —
// CreateNeoTask, console URL print, Session construction, session.Run) had no
// coverage.
//
//nolint:paralleltest // mutates package globals (BackendInstance, pkgWorkspace.Instance, isInteractive)
func TestRunNeoIntegration_NonInteractiveHappyPath(t *testing.T) {
	isolateWorkspace(t)
	srv := newNeoFakeServer(t)
	installNeoTestEnv(t, srv, false /*interactive*/)

	// Once Session.Run opens the SSE stream, push a final assistant_message
	// and close the stream so Session.Run sees the channel close and returns
	// nil. Without this nudge the handler would block forever on streamSend.
	go func() {
		if !srv.awaitStreamConnect(t, 2*time.Second) {
			return
		}
		srv.sendFinalAssistantMessage(t)
		srv.endStream()
	}()

	done := make(chan error, 1)
	go func() {
		done <- runNeo(t.Context(), "do a thing", "" /*stack*/, "test-org", t.TempDir(),
			client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("non-interactive runNeo did not return within 5s")
	}

	posts := srv.recordedPosts()
	require.NotEmpty(t, posts, "server saw no requests")
	assert.Equal(t, "/api/preview/agents/test-org/tasks", posts[0].path)
	assert.Contains(t, string(posts[0].body), "do a thing",
		"CreateNeoTask body must include the prompt")
}

// TestRunNeoIntegration_NonInteractiveRequiresPrompt covers the early-return
// guard that rejects an empty prompt in non-interactive mode (there's no input
// mechanism, so the agent has nothing to react to).
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_NonInteractiveRequiresPrompt(t *testing.T) {
	isolateWorkspace(t)
	srv := newNeoFakeServer(t)
	installNeoTestEnv(t, srv, false /*interactive*/)

	err := runNeo(t.Context(), "" /*prompt*/, "", "test-org", t.TempDir(),
		client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt argument is required")

	// The guard fires before any HTTP call — server must be untouched.
	assert.Empty(t, srv.recordedPosts(), "no API calls should be made when the prompt guard fires")
}

// TestRunNeoIntegration_RequiresCloudBackend covers the type-assertion guard
// that rejects backends not implementing httpstate.Backend (filestate, DIY,
// etc. — only the Pulumi Cloud backend exposes the Neo API).
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_RequiresCloudBackend(t *testing.T) {
	isolateWorkspace(t)

	// A bare MockBackend deliberately doesn't implement httpstate.Backend.
	prevBackend := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = &backend.MockBackend{}
	t.Cleanup(func() { cmdBackend.BackendInstance = prevBackend })

	prevWorkspace := pkgWorkspace.Instance
	pkgWorkspace.Instance = &pkgWorkspace.MockContext{}
	t.Cleanup(func() { pkgWorkspace.Instance = prevWorkspace })

	err := runNeo(t.Context(), "do a thing", "", "test-org", t.TempDir(),
		client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Pulumi Cloud backend",
		"non-cloud backends must surface a clear error rather than panic on the type assertion")
}

// TestRunNeoIntegration_ResolvesCwdWhenEmpty covers the os.Getwd() fallback at
// the top of runNeo: when the caller passes an empty cwdFlag, runNeo must
// resolve the process working directory rather than handing the empty string
// to the tools constructors (which would otherwise reject it).
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_ResolvesCwdWhenEmpty(t *testing.T) {
	isolateWorkspace(t)
	srv := newNeoFakeServer(t)
	installNeoTestEnv(t, srv, false /*interactive*/)

	go func() {
		if !srv.awaitStreamConnect(t, 2*time.Second) {
			return
		}
		srv.sendFinalAssistantMessage(t)
		srv.endStream()
	}()

	done := make(chan error, 1)
	go func() {
		// cwdFlag empty → runNeo calls os.Getwd. The test's own working
		// directory is always a real, readable path, so the tools constructors
		// accept it and runNeo proceeds.
		done <- runNeo(t.Context(), "do a thing", "", "test-org", "", /*cwd*/
			client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("runNeo with empty cwd did not return within 5s")
	}
}

// TestRunNeoIntegration_RejectsNonexistentCwd covers the tools.NewFilesystem
// error path: when cwdFlag points at a directory that doesn't exist, runNeo
// must surface the underlying os.Stat error rather than continuing to a half-
// initialized session.
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_RejectsNonexistentCwd(t *testing.T) {
	isolateWorkspace(t)
	srv := newNeoFakeServer(t)
	installNeoTestEnv(t, srv, false /*interactive*/)

	missing := t.TempDir() + "/does-not-exist"
	err := runNeo(t.Context(), "do a thing", "", "test-org", missing,
		client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	require.Error(t, err)
	// The exact wrapping is internal to tools.NewFilesystem, but the missing
	// path should be referenced so the user can see what went wrong.
	assert.Contains(t, err.Error(), "does-not-exist")

	assert.Empty(t, srv.recordedPosts(),
		"no API calls should be made when filesystem setup fails")
}

// TestRunNeoIntegration_PropagatesReadProjectError covers the ReadProject
// error branch — any error other than ErrProjectNotFound must abort runNeo
// before it touches the backend (otherwise we'd create a Neo task against a
// half-loaded project).
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_PropagatesReadProjectError(t *testing.T) {
	isolateWorkspace(t)
	srv := newNeoFakeServer(t)
	installNeoTestEnv(t, srv, false /*interactive*/)

	// Override the workspace to surface a non-NotFound error from ReadProject.
	pkgWorkspace.Instance = &pkgWorkspace.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return nil, "", errors.New("synthetic ReadProject failure")
		},
	}

	err := runNeo(t.Context(), "do a thing", "", "test-org", t.TempDir(),
		client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "synthetic ReadProject failure")
	assert.Empty(t, srv.recordedPosts(),
		"no API calls should be made when project resolution fails")
}

// TestRunNeoIntegration_PropagatesCreateNeoTaskError covers the non-interactive
// CreateNeoTask error branch: a server-side failure during task creation must
// surface as the runNeo return value, with no session.Run started.
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_PropagatesCreateNeoTaskError(t *testing.T) {
	isolateWorkspace(t)

	// Bespoke server: CreateNeoTask returns 500. We don't reuse newNeoFakeServer
	// because here we want CreateNeoTask itself to fail.
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/preview/agents/test-org/tasks" {
				http.Error(w, "synthetic 500", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
	t.Cleanup(server.Close)

	pc := client.NewClient(server.URL, "", false, nil)
	be := newFakeBackend()
	be.ClientV = pc
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "test-org", nil }

	prevBackend := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = be
	t.Cleanup(func() { cmdBackend.BackendInstance = prevBackend })

	prevWorkspace := pkgWorkspace.Instance
	pkgWorkspace.Instance = &pkgWorkspace.MockContext{}
	t.Cleanup(func() { pkgWorkspace.Instance = prevWorkspace })

	prevInteractive := isInteractive
	isInteractive = func() bool { return false }
	t.Cleanup(func() { isInteractive = prevInteractive })

	err := runNeo(t.Context(), "do a thing", "", "test-org", t.TempDir(),
		client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating Neo task")
}

// TestRunNeoIntegration_InteractiveCreateNeoTaskFailureExits covers the
// interactive path's CreateNeoTask error handling. Two coupled behaviors must
// hold:
//
//  1. createTask emits a UIError on uiCh so the user sees the failure in the
//     TUI before it tears down (vs. an unexplained "Thinking…" hang).
//  2. The errgroup goroutine that watches gctx.Done fires p.Quit() once the
//     createTask worker returns the error, so runNeo returns instead of
//     blocking on p.Run forever.
//
// Pre-fix, the test would time out: with no manual p.Quit and no auto-quit
// goroutine, p.Run would block until the test deadline even though createTask
// had already errored.
//
//nolint:paralleltest // mutates package globals (BackendInstance, pkgWorkspace.Instance, newTeaProgram, isInteractive)
func TestRunNeoIntegration_InteractiveCreateNeoTaskFailureExits(t *testing.T) {
	isolateWorkspace(t)

	// Server returns 500 on CreateNeoTask. Other endpoints 404 — they should
	// never be hit, since the failure must abort before any session starts.
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/preview/agents/test-org/tasks" {
				http.Error(w, "synthetic 500", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
	t.Cleanup(server.Close)

	pc := client.NewClient(server.URL, "", false, nil)
	be := newFakeBackend()
	be.ClientV = pc
	be.GetDefaultOrgF = func(context.Context) (string, error) { return "test-org", nil }

	prevBackend := cmdBackend.BackendInstance
	cmdBackend.BackendInstance = be
	t.Cleanup(func() { cmdBackend.BackendInstance = prevBackend })

	prevWorkspace := pkgWorkspace.Instance
	pkgWorkspace.Instance = &pkgWorkspace.MockContext{}
	t.Cleanup(func() { pkgWorkspace.Instance = prevWorkspace })

	// Force the interactive branch — createTask only runs on the interactive
	// path, where the new UIError + p.Quit goroutine live.
	prevInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = prevInteractive })

	// Headless tea.Program. Notably, the test does NOT call p.Quit anywhere —
	// the only path to a clean exit is the new auto-quit goroutine reacting
	// to the createTask worker's non-nil return.
	prevProgram := newTeaProgram
	newTeaProgram = func(m tea.Model) *tea.Program {
		return tea.NewProgram(
			m,
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithoutSignals(),
			tea.WithoutSignalHandler(),
			tea.WithoutRenderer(),
		)
	}
	t.Cleanup(func() { newTeaProgram = prevProgram })

	done := make(chan error, 1)
	go func() {
		done <- runNeo(t.Context(), "do a thing", "" /*stack*/, "test-org", t.TempDir(),
			client.NeoApprovalModeManual, client.NeoPermissionModeDefault, false /*printMode*/)
	}()

	select {
	case err := <-done:
		// CreateNeoTask's wrapped error must surface to the caller; the TUI
		// tear-down path should not swallow it.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating Neo task",
			"CreateNeoTask error must propagate, not be replaced by tear-down state")
	case <-time.After(5 * time.Second):
		t.Fatal("runNeo did not return within 5s after CreateNeoTask failure — " +
			"the gctx.Done → p.Quit goroutine must tear the TUI down on worker error")
	}
}

// TestRunNeoIntegration_NewNeoCmdRunE covers the cobra RunE closure in
// NewNeoCmd: it pulls the prompt from positional args, reads the flag values,
// and forwards everything to runNeo. Without this test the entire RunE body
// (lines 83-90 in neo.go) is uncovered — the flag-registration test in
// neo_test.go only inspects cmd.Args, never invokes RunE.
//
//nolint:paralleltest // mutates package globals
func TestRunNeoIntegration_NewNeoCmdRunE(t *testing.T) {
	isolateWorkspace(t)

	srv := newNeoFakeServer(t)
	installNeoTestEnv(t, srv, false /*interactive*/)

	go func() {
		if !srv.awaitStreamConnect(t, 2*time.Second) {
			return
		}
		srv.sendFinalAssistantMessage(t)
		srv.endStream()
	}()

	cmd := NewNeoCmd()
	cmd.SetContext(t.Context())
	require.NoError(t, cmd.Flags().Set("org", "test-org"))
	require.NoError(t, cmd.Flags().Set("cwd", t.TempDir()))

	done := make(chan error, 1)
	go func() {
		done <- cmd.RunE(cmd, []string{"my prompt"})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("cmd.RunE did not return within 5s")
	}

	posts := srv.recordedPosts()
	require.NotEmpty(t, posts, "RunE did not reach CreateNeoTask")
	assert.Contains(t, string(posts[0].body), "my prompt",
		"prompt from positional args must reach CreateNeoTask")
}
