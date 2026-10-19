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

package credentialhelper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain also runs this test executable as a credential helper subprocess.
// Tests use os.Executable() with -credential-helper-test, a request log path, and JSON responses keyed by reason.
// An optional release-file path holds refresh responses until the file exists. Other invocations run tests normally.
func TestMain(m *testing.M) {
	index := -1
	for i, arg := range os.Args {
		if arg == "-credential-helper-test" {
			index = i
			break
		}
	}
	if index < 0 {
		os.Exit(m.Run())
	}
	args := os.Args[index+1:]
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		os.Exit(2)
	}
	if args[0] != "" {
		f, err := os.OpenFile(args[0], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(3)
		}
		_ = json.NewEncoder(f).Encode(req)
		_ = f.Close()
	}
	if req.Reason == "wait" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if len(args) > 2 && req.Reason != "initial" {
		for {
			if _, err := os.Stat(args[2]); err == nil {
				break
			} else if !os.IsNotExist(err) {
				os.Exit(5)
			}
			time.Sleep(time.Millisecond)
		}
	}
	var responses map[string]string
	if err := json.Unmarshal([]byte(args[1]), &responses); err != nil {
		os.Exit(4)
	}
	fmt.Print(responses[req.Reason])
	os.Exit(0)
}

func helperConfig(t *testing.T, responses map[string]string) (*workspace.CredentialHelper, string) {
	t.Helper()
	path, err := os.Executable()
	require.NoError(t, err)
	log := filepath.Join(t.TempDir(), "requests.jsonl")
	payload, err := json.Marshal(responses)
	require.NoError(t, err)
	return &workspace.CredentialHelper{
		Path: path,
		Args: []string{"--", "-credential-helper-test", log, string(payload)},
	}, log
}

func testSession(t *testing.T, responses map[string]string) (context.Context, *Session, string) {
	t.Helper()
	config, log := helperConfig(t, responses)
	ctx := t.Context()
	s := NewSession()
	require.NoError(t, s.Configure(config.Path, config.Args))
	return ctx, s, log
}

func readRequests(t *testing.T, path string) []request {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var requests []request
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var r request
		err := decoder.Decode(&r)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		requests = append(requests, r)
	}
	return requests
}

func TestProtocolValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, response string
		valid          bool
	}{
		{"decline", "{}", true},
		{"versioned decline", `{"version":1}`, true},
		{"headers only", `{"version":1,"headers":{"X-Session":["secret"]}}`, true},
		{"unknown field", `{"version":1,"token":"secret"}`, false},
		{"missing version", `{"accessToken":"secret"}`, false},
		{"unsupported version", `{"version":2}`, false},
		{"wrong type", `{"version":1,"accessToken":42}`, false},
		{"null environment value", `{"version":1,"env":{"AWS_PROFILE":null}}`, false},
		{"null header value", `{"version":1,"headers":{"X-Session":[null]}}`, false},
		{"null header array", `{"version":1,"headers":{"X-Session":null}}`, false},
		{"null object", "null", false},
		{"null field", `{"version":null}`, false},
		{"extra output", "{}\n{}", false},
		{"array", "[]", false},
		{"forbidden header", `{"version":1,"headers":{"aUtHoRiZaTiOn":["bad"]}}`, false},
		{"header injection", `{"version":1,"headers":{"X-Session":["bad\r\ninjection"]}}`, false},
		{"token injection", `{"version":1,"accessToken":"bad\r\ninjection"}`, false},
		{"token control character", `{"version":1,"accessToken":"bad\u0001token"}`, false},
		{"reserved env", `{"version":1,"env":{"PULUMI_ACCESS_TOKEN":"bad"}}`, false},
		{"invalid expiry", `{"version":1,"expiresAt":"tomorrow"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			c, _ := helperConfig(t, map[string]string{"initial": test.response})
			_, err := invoke(t.Context(), c, request{Version: 1, Reason: "initial"}, nil)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "secret")
			}
		})
	}
}

func TestInvokeContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("timeout", func(t *testing.T) {
		t.Parallel()

		c, _ := helperConfig(t, nil)
		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
		defer cancel()
		_, err := invoke(ctx, c, request{Version: 1, Reason: "wait"}, nil)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.ErrorContains(t, err, "timed out")
		assert.NotContains(t, err.Error(), "signal: killed")
	})
	t.Run("canceled", func(t *testing.T) {
		t.Parallel()

		c, _ := helperConfig(t, nil)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := invoke(ctx, c, request{Version: 1, Reason: "wait"}, nil)
		require.ErrorIs(t, err, context.Canceled)
		assert.NotContains(t, err.Error(), "timed out")
	})
}

func TestBackendSelectionAndCaching(t *testing.T) {
	t.Parallel()

	ctx, s, log := testSession(t, map[string]string{
		"initial": `{"version":1,"backendUrl":"https://api.example.com","accessToken":"secret"}`,
		"expired": `{"version":1,"accessToken":"fresh"}`,
	})
	// Let the helper propose a backend, then explicitly select it for the command.
	r, err := s.Resolve(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com", r.BackendURL())
	assert.True(t, r.SelectedBackend())
	assert.Nil(t, s.Current())
	require.NoError(t, s.SelectBackend(r, r.BackendURL()))
	assert.Same(t, r, s.Current())

	// Resolving the proposed URL reuses the result without invoking the helper again.
	again, err := s.Resolve(ctx, r.BackendURL())
	require.NoError(t, err)
	assert.Same(t, r, again)
	require.Len(t, readRequests(t, log), 1)

	// Refreshing credentials preserves the fact that the helper chose the backend.
	err = r.refresh(ctx, r.snapshot(), "expired")
	require.NoError(t, err)
	assert.True(t, r.SelectedBackend())
}

func TestHelperCannotOverrideSelectedBackend(t *testing.T) {
	t.Parallel()

	ctx, s, _ := testSession(t, map[string]string{
		"initial": `{"version":1,"backendUrl":"https://api.example.com"}`,
	})
	_, err := s.Resolve(ctx, "https://other.example.com")
	require.ErrorContains(t, err, "cannot return backendUrl")
}

func TestResolvePreservesSelectedBackend(t *testing.T) {
	t.Parallel()

	ctx, session, log := testSession(t, map[string]string{"initial": `{"version":1,"accessToken":"secret"}`})
	// Select the target backend for the command.
	selected, err := session.Resolve(ctx, "https://target.example.com")
	require.NoError(t, err)
	assert.Nil(t, session.Current())
	require.NoError(t, session.SelectBackend(selected, selected.BackendURL()))

	// Looking up a source backend's credentials must not switch the target or its plugin credentials.
	source, err := session.Resolve(ctx, "https://source.example.com")
	require.NoError(t, err)
	assert.Equal(t, "https://source.example.com", source.BackendURL())
	assert.Same(t, selected, session.Current())
	assert.Equal(t, "https://target.example.com", session.Current().PluginEnv()["PULUMI_API"])

	// A trailing slash still hits the source's cached result and leaves the target selected.
	again, err := session.Resolve(ctx, "https://source.example.com/")
	require.NoError(t, err)
	assert.Same(t, source, again)
	assert.Same(t, selected, session.Current())

	// Only explicit selection switches the backend; neither cached lookup nor selection invokes the helper.
	require.NoError(t, session.SelectBackend(source, source.BackendURL()))
	assert.Same(t, source, session.Current())
	requests := readRequests(t, log)
	require.Len(t, requests, 2)
	assert.Equal(t, "https://source.example.com", requests[1].SelectedBackendURL)
}

func TestEnvironmentMerge(t *testing.T) {
	const inherited = "PULUMI_HELPER_TEST_INHERITED"
	const applied = "PULUMI_HELPER_TEST_APPLIED"
	t.Setenv(inherited, "original")
	t.Setenv(applied, "")
	ptesting.Unsetenv(t, applied)
	ctx, s, _ := testSession(t, map[string]string{
		"initial": `{"version":1,"env":{"PULUMI_HELPER_TEST_INHERITED":"replacement","PULUMI_HELPER_TEST_APPLIED":"first"}}`,
	})
	_, err := s.Resolve(ctx, "s3://bucket-one")
	require.NoError(t, err)
	assert.Equal(t, "original", os.Getenv(inherited))
	assert.Equal(t, "first", os.Getenv(applied))
	c, _ := helperConfig(t, map[string]string{"initial": `{"version":1,"env":{"PULUMI_HELPER_TEST_APPLIED":"second"}}`})
	require.NoError(t, s.Configure(c.Path, c.Args))
	_, err = s.Resolve(ctx, "s3://bucket-two")
	require.ErrorContains(t, err, "conflicting values")
	assert.Equal(t, "first", os.Getenv(applied))
}

func TestRefreshAndLeaseTokens(t *testing.T) {
	t.Parallel()

	for _, reason := range []string{"expired", "rejected"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("X-Session") != "fresh" {
					w.WriteHeader(401)
					return
				}
				assert.Equal(t, "update-token lease", r.Header.Get("Authorization"))
				w.WriteHeader(200)
			}))
			defer server.Close()
			expiry := ""
			if reason == "expired" {
				expiry = `,"expiresAt":"2000-01-01T00:00:00Z"`
			}
			ctx, s, log := testSession(t, map[string]string{
				"initial": `{"version":1,"headers":{"X-Session":["stale"]}` + expiry + `}`,
				reason:    `{"version":1,"headers":{"X-Session":["fresh"]}}`,
			})
			helper, err := s.Resolve(ctx, server.URL)
			require.NoError(t, err)
			req, err := http.NewRequestWithContext(WithOperation(ctx), http.MethodPost, server.URL, strings.NewReader("payload"))
			require.NoError(t, err)
			req.Header.Set("Authorization", "update-token lease")
			resp, err := helper.HTTPClient(server.Client()).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, 200, resp.StatusCode)
			calls := readRequests(t, log)
			require.Len(t, calls, 2)
			assert.Equal(t, reason, calls[1].Reason)
			if reason == "expired" {
				assert.EqualValues(t, 1, requests.Load())
			} else {
				assert.EqualValues(t, 2, requests.Load())
			}
		})
	}
}

func TestHTTPRetriesShareOneRefreshAttempt(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer server.Close()
	ctx, s, log := testSession(t, map[string]string{
		"initial": `{"version":1,"accessToken":"stale","expiresAt":"2000-01-01T00:00:00Z"}`,
		"expired": `{"version":1,"accessToken":"fresh"}`,
	})
	helper, err := s.Resolve(ctx, server.URL)
	require.NoError(t, err)
	op := WithOperation(ctx)
	// Simulate HTTP retries of one backend API call, all sharing the same refresh allowance.
	for range 5 {
		req, err := http.NewRequestWithContext(op, http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := helper.HTTPClient(server.Client()).Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, 401, resp.StatusCode)
	}
	calls := readRequests(t, log)
	require.Len(t, calls, 2)
	assert.Equal(t, "initial", calls[0].Reason)
	assert.Equal(t, "expired", calls[1].Reason)
}

func TestConcurrentRefresh(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token fresh" {
			w.WriteHeader(401)
		}
	}))
	defer server.Close()
	ctx, s, log := testSession(t, map[string]string{
		"initial":  `{"version":1,"accessToken":"stale"}`,
		"rejected": `{"version":1,"accessToken":"fresh"}`,
	})
	helper, err := s.Resolve(ctx, server.URL)
	require.NoError(t, err)
	c := helper.HTTPClient(server.Client())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			req, err := http.NewRequestWithContext(WithOperation(ctx), http.MethodGet, server.URL, nil)
			if err != nil {
				t.Errorf("creating request: %v", err)
				return
			}
			resp, err := c.Do(req)
			if err == nil {
				assert.Equal(t, 200, resp.StatusCode)
				resp.Body.Close()
			} else {
				t.Errorf("performing request: %v", err)
			}
		})
	}
	wg.Wait()
	require.Len(t, readRequests(t, log), 2)
}

func TestConcurrentRefreshFailure(t *testing.T) {
	t.Parallel()

	const concurrent = 8
	var requests atomic.Int32
	allRequests := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == concurrent {
			close(allRequests)
		}
		select {
		case <-allRequests:
			w.WriteHeader(http.StatusUnauthorized)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	ctx, s, log := testSession(t, map[string]string{
		"initial":  `{"version":1,"accessToken":"stale"}`,
		"rejected": `{"version":1}`,
	})
	helper, err := s.Resolve(ctx, server.URL)
	require.NoError(t, err)
	c := helper.HTTPClient(server.Client())
	errors := make(chan error, concurrent)
	for range concurrent {
		go func() {
			req, err := http.NewRequestWithContext(WithOperation(ctx), http.MethodGet, server.URL, nil)
			if err == nil {
				var resp *http.Response
				resp, err = c.Do(req)
				if resp != nil {
					resp.Body.Close()
				}
			}
			errors <- err
		}()
	}
	for range concurrent {
		require.ErrorContains(t, <-errors, "credentials helper refresh requires HTTP credentials")
	}
	require.Len(t, readRequests(t, log), 2, "concurrent requests share one initial helper lookup and one failed refresh")

	// A later independent request can try refreshing again.
	req, err := http.NewRequestWithContext(WithOperation(ctx), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	_, err = c.Do(req)
	require.ErrorContains(t, err, "credentials helper refresh requires HTTP credentials")
	require.Len(t, readRequests(t, log), 3)
}

func TestRefreshWaiterCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token fresh", r.Header.Get("Authorization"))
	}))
	defer server.Close()
	config, log := helperConfig(t, map[string]string{
		"initial": `{"version":1,"accessToken":"stale","expiresAt":"2000-01-01T00:00:00Z"}`,
		"expired": `{"version":1,"accessToken":"fresh"}`,
	})
	// The helper waits for this file before returning refreshed credentials.
	release := filepath.Join(t.TempDir(), "release")
	config.Args = append(config.Args, release)
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0o600) })
	s := NewSession()
	require.NoError(t, s.Configure(config.Path, config.Args))
	helper, err := s.Resolve(t.Context(), server.URL)
	require.NoError(t, err)
	c := helper.HTTPClient(server.Client())
	startRequest := func(ctx context.Context) <-chan error {
		done := make(chan error, 1)
		go func() {
			req, err := http.NewRequestWithContext(WithOperation(ctx), http.MethodGet, server.URL, nil)
			if err == nil {
				var resp *http.Response
				resp, err = c.Do(req)
				if resp != nil {
					resp.Body.Close()
				}
			}
			done <- err
		}()
		return done
	}
	// Start a request with expired credentials and wait until its refresh has invoked the helper.
	firstCtx, cancelFirst := context.WithCancel(t.Context())
	defer cancelFirst()
	first := startRequest(firstCtx)
	// Waits until the helper log contains two requests: the initial lookup and the refresh.
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(log)
		return err == nil && strings.Count(string(data), "\n") == 2
	}, 30*time.Second, 10*time.Millisecond)

	// A second request waits for the same refresh, but its deadline must still take effect.
	waitCtx, cancelWaiter := context.WithTimeout(t.Context(), time.Second)
	defer cancelWaiter()
	select {
	case err := <-startRequest(waitCtx):
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(30 * time.Second):
		t.Fatal("canceled waiter remained blocked on the helper")
	}
	// Canceling the request that started the refresh must also stop its wait promptly.
	cancelFirst()
	select {
	case err := <-first:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(30 * time.Second):
		t.Fatal("initiating request remained blocked on the helper")
	}

	// The shared refresh survives both cancellations; release it and use its credentials in a new request.
	require.NoError(t, os.WriteFile(release, nil, 0o600))
	require.NoError(t, <-startRequest(t.Context()))
	// Only the initial lookup and the original refresh invoked the helper.
	require.Len(t, readRequests(t, log), 2)
}

func TestConcurrentResolutionAndSelection(t *testing.T) {
	t.Parallel()

	ctx, s, log := testSession(t, map[string]string{"initial": `{"version":1,"accessToken":"secret"}`})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			r, err := s.Resolve(ctx, "")
			if err != nil {
				t.Errorf("resolving helper: %v", err)
				return
			}
			_ = r.BackendURL()
			if err := s.SelectBackend(r, "https://api.example.com"); err != nil {
				t.Errorf("selecting backend: %v", err)
				return
			}
			assert.Equal(t, "https://api.example.com", r.BackendURL())
			assert.Equal(t, map[string]string{
				"PULUMI_API": "https://api.example.com", "PULUMI_ACCESS_TOKEN": "secret",
			}, r.PluginEnv())
		})
	}
	wg.Wait()
	require.Len(t, readRequests(t, log), 1)
}

func TestRedirectScope(t *testing.T) {
	t.Parallel()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("X-Session"))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token secret", r.Header.Get("Authorization"))
		assert.Equal(t, "secret", r.Header.Get("X-Session"))
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer origin.Close()
	ctx, s, log := testSession(t, map[string]string{
		"initial": `{"version":1,"accessToken":"secret","headers":{"X-Session":["secret"]}}`,
	})
	helper, err := s.Resolve(ctx, origin.URL)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin.URL, nil)
	require.NoError(t, err)
	resp, err := helper.HTTPClient(origin.Client()).Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Len(t, readRequests(t, log), 1)
}

func TestRefreshResponseRules(t *testing.T) {
	t.Parallel()

	for _, response := range []string{
		`{"version":1}`,
		`{"version":1,"headers":{"X-Session":["fresh"]}}`,
		`{"version":1,"accessToken":"fresh","env":{}}`,
		`{"version":1,"accessToken":"fresh","backendUrl":"https://api.example.com"}`,
	} {
		ctx, s, _ := testSession(t, map[string]string{"initial": `{"version":1,"accessToken":"stale"}`, "expired": response})
		r, err := s.Resolve(ctx, "https://api.example.com")
		require.NoError(t, err)
		err = r.refresh(ctx, r.snapshot(), "expired")
		require.Error(t, err)
	}
}

func TestPersistenceDoesNotSaveToken(t *testing.T) {
	t.Setenv("PULUMI_CREDENTIALS_PATH", t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
	ctx, s, _ := testSession(t, map[string]string{"initial": `{"version":1,"accessToken":"secret"}`})
	r, err := s.Resolve(ctx, "https://api.example.com")
	require.NoError(t, err)
	require.NoError(t, SetCurrent(r.BackendURL()))
	require.NoError(t, s.SaveConfiguration())
	creds, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, 1, creds.Version)
	assert.Equal(t, r.BackendURL(), creds.Current)
	require.NotNil(t, creds.CredentialHelper)
	assert.Empty(t, creds.Accounts)
	require.NoError(t, ClearConfiguration())
	creds, err = workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Nil(t, creds.CredentialHelper)
}

func TestDisabledHelper(t *testing.T) {
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
	t.Setenv("PULUMI_CREDENTIAL_HELPER_ARGS", "invalid")
	s := NewSession()
	config, _, err := s.Configuration()
	require.NoError(t, err)
	assert.Nil(t, config)
}

func TestDiscoveryWithUnreadableCredentials(t *testing.T) {
	for _, agent := range []bool{false, true} {
		t.Run(fmt.Sprintf("agent=%v", agent), func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_CREDENTIAL_HELPER", "")
			t.Setenv("PATH", t.TempDir())
			if agent {
				t.Setenv("CODEX_SANDBOX", "1")
				t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
				require.NoError(t, workspace.StoreAgentAccount(
					"https://api.agent.example", workspace.Account{AccessToken: "agent-token"}, true))
			}
			require.NoError(t, os.Mkdir(filepath.Join(dirs.Home, "credentials.json"), 0o700))
			r, err := NewSession().Resolve(t.Context(), "https://api.agent.example")
			if agent {
				require.NoError(t, err)
				assert.Nil(t, r.Helper)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestDiagnosticsStreamLines(t *testing.T) {
	t.Parallel()

	s := NewSession()
	var messages []string
	stop := s.DisplayDiagnostics(func(message string) {
		messages = append(messages, message)
	})
	defer stop()
	d := newDiagnostics(s)
	_, err := d.Write([]byte("Approve the authentication prompt\npartial"))
	require.NoError(t, err)
	assert.Equal(t, []string{"Approve the authentication prompt"}, messages)
	_, err = d.Write([]byte(" line\r\nlast line"))
	require.NoError(t, err)
	assert.Equal(t, []string{"Approve the authentication prompt", "partial line"}, messages)
	d.close()
	assert.Equal(t, []string{"Approve the authentication prompt", "partial line", "last line"}, messages)
}

func TestDiagnosticsSlowSink(t *testing.T) {
	t.Parallel()

	s := NewSession()
	ch := make(chan string)
	started := make(chan struct{})
	var once sync.Once
	stop := s.DisplayDiagnostics(func(message string) {
		once.Do(func() { close(started) })
		select {
		case ch <- message:
		case <-t.Context().Done():
		}
	})
	t.Cleanup(stop)
	done := make(chan error, 1)
	longLine := strings.Repeat("x", 8192)
	go func() {
		d := newDiagnostics(s)
		_, err := d.Write([]byte(longLine + "\nline\nlast line"))
		d.close()
		done <- err
	}()
	<-started
	select {
	case <-done:
		t.Fatal("diagnostics did not wait for the display")
	default:
	}
	assert.Equal(t, longLine, <-ch)
	assert.Equal(t, "line", <-ch)
	assert.Equal(t, "last line", <-ch)
	require.NoError(t, <-done)
}

func TestExpiryClearedOnRefresh(t *testing.T) {
	t.Parallel()

	ctx, s, _ := testSession(t, map[string]string{
		"initial": `{"version":1,"accessToken":"stale","expiresAt":"2000-01-01T00:00:00Z"}`,
		"expired": `{"version":1,"accessToken":"fresh"}`,
	})
	r, err := s.Resolve(ctx, "https://api.example.com")
	require.NoError(t, err)
	require.True(t, r.snapshot().credentials.ExpiresAt.Before(time.Now()))
	err = r.refresh(ctx, r.snapshot(), "expired")
	assert.Nil(t, r.snapshot().credentials.ExpiresAt)
	require.NoError(t, err)
}

func TestHTTPFallbackRejectsDIY(t *testing.T) {
	t.Parallel()

	ctx, s, _ := testSession(t, map[string]string{"initial": `{"version":1,"accessToken":"token"}`})
	result, err := s.Resolve(ctx, "")
	require.NoError(t, err)
	require.NoError(t, s.SelectBackend(result, "https://api.example.com"))
	assert.False(t, result.SelectedBackend())
	require.ErrorContains(t, s.SelectBackend(result, "s3://bucket"), "HTTP credentials require an HTTP backend")
}
