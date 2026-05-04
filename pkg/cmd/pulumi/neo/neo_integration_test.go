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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
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
	// it ends the stream cleanly.
	streamSend chan []byte
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
		close(s.streamSend)
		s.server.Close()
	})
	return s
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
		done <- runNeo(t.Context(), "do a thing", "" /*stack*/, "test-org", t.TempDir())
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
