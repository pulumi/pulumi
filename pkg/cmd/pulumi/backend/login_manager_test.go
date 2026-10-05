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

package backend

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/secrets/service"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestLoginManagerUsesReturnedCredentialsWhenPersistenceIsSkipped(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		dirs := ptesting.IsolateCredentials(t)
		t.Setenv("CODEX_SANDBOX", "1")
		t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
		t.Setenv("PULUMI_ACCESS_TOKEN", "user-token")
		t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "user-org")
		badHome := filepath.Join(dirs.Home, "not-a-directory")
		require.NoError(t, os.WriteFile(badHome, []byte("not a directory"), 0o600))
		t.Setenv("PULUMI_HOME", badHome)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token user-token", r.Header.Get("Authorization"))
			switch r.URL.Path {
			case "/api/user":
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"githubLogin": "user", "organizations": []map[string]string{},
				}))
			case "/api/capabilities":
				require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
			default:
				t.Errorf("unexpected request: %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(server.Close)
		require.NoError(t, workspace.StoreAgentAccount(server.URL, workspace.Account{
			AccessToken: "agent-token", Username: "agent-user",
		}, true))

		manager := NewLoginManager(auth.NewSessionWithHelperFunc(nil))
		var be pkgBackend.Backend
		var err error
		if interactive {
			be, err = manager.Login(
				t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL, nil, LoginOptions{Color: colors.Never})
		} else {
			be, err = manager.Current(t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL, nil, false)
		}
		require.NoError(t, err)
		require.NotNil(t, be)
		be.(httpstate.Backend).Capabilities(t.Context())
		username, _, _, err := be.CurrentUser()
		require.NoError(t, err)
		assert.Equal(t, "user", username)
		agent, err := workspace.GetAgentAccount(server.URL)
		require.NoError(t, err)
		assert.Equal(t, "agent-token", agent.AccessToken)
		credentials := PluginCredentials(be)
		assert.Equal(t, server.URL, credentials.BackendURL)
		assert.Equal(t, "user-token", credentials.HTTPAuth.AccessToken())
		resolved, err := manager.ResolveCredentials(t.Context(), pkgWorkspace.Instance, server.URL, nil)
		require.NoError(t, err)
		assert.Same(t, credentials.HTTPAuth, resolved.HTTPAuth)
	}
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestLoginManagerPreparesHelperSelectedDIYBackend(t *testing.T) {
	for _, mode := range []string{"interactive", "noninteractive"} {
		t.Run(mode, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Chdir(t.TempDir())
			backendURL := "file://" + filepath.ToSlash(t.TempDir())
			ptesting.Unsetenv(t, "HELPER_BACKEND_READY")
			calls := 0
			session := auth.NewSessionWithHelperFunc(func(
				context.Context, credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				calls++
				return &credentialhelper.Response{
					BackendURL: backendURL, Env: map[string]string{"HELPER_BACKEND_READY": "ready"},
				}, nil
			})
			ctx, lm := t.Context(), NewLoginManager(session)
			open := func(url string) (pkgBackend.Backend, error) {
				return lm.Current(ctx, pkgWorkspace.Instance, diagtest.LogSink(t), url, nil, false)
			}
			if mode == "interactive" {
				open = func(url string) (pkgBackend.Backend, error) {
					return lm.Login(ctx, pkgWorkspace.Instance, diagtest.LogSink(t), url, nil, LoginOptions{Color: colors.Never})
				}
			}
			url, err := PrepareBackend(ctx, pkgWorkspace.Instance, lm)
			require.NoError(t, err)
			assert.Equal(t, backendURL, url)
			assert.Equal(t, "ready", os.Getenv("HELPER_BACKEND_READY"))
			stored, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Empty(t, stored.Current)
			be, err := open("")
			require.NoError(t, err)
			assert.Equal(t, backendURL, be.URL())
			assert.Equal(t, 1, calls)
			stored, err = workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, backendURL, stored.Current)

			otherURL := "file://" + filepath.ToSlash(t.TempDir())
			be, err = open(otherURL)
			require.NoError(t, err)
			assert.Equal(t, otherURL, be.URL())
			stored, err = workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, backendURL, stored.Current)
			stored.Current = otherURL
			require.NoError(t, workspace.StoreCredentials(stored))
			_, err = open(backendURL)
			require.NoError(t, err)
			stored, err = workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, otherURL, stored.Current, "reopening the helper-selected URL must respect setCurrent=false")
		})
	}
}

func TestLoginManagerPreparesEachBackendOnce(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "migration-user")
	servers := make([]*httptest.Server, 2)
	tokens := map[string]string{}
	for i := range servers {
		token := fmt.Sprintf("backend-%d-token", i)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "token "+token, r.Header.Get("Authorization"))
			switch r.URL.Path {
			case "/api/user":
				require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"githubLogin": "migration-user"}))
			case "/api/capabilities":
				require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
			default:
				t.Errorf("unexpected request: %s", r.URL.Path)
			}
		}))
		defer server.Close()
		servers[i] = server
		tokens[server.URL] = token
	}
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{Current: servers[1].URL}))
	var requests []credentialhelper.Request
	session := auth.NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		requests = append(requests, request)
		return &credentialhelper.Response{AccessToken: tokens[request.SelectedBackendURL]}, nil
	})
	ctx, lm := t.Context(), NewLoginManager(session)
	for range 2 {
		source, err := lm.Login(ctx, pkgWorkspace.Instance, diagtest.LogSink(t),
			servers[0].URL, nil, LoginOptions{Color: colors.Never})
		require.NoError(t, err)
		source.(httpstate.Backend).Capabilities(ctx)
		_, _, _, err = source.CurrentUser()
		require.NoError(t, err)
		target, err := CurrentBackend(ctx, pkgWorkspace.Instance, lm, nil, display.Options{Color: colors.Never})
		require.NoError(t, err)
		target.(httpstate.Backend).Capabilities(ctx)
		_, _, _, err = target.CurrentUser()
		require.NoError(t, err)
		assert.Equal(t, servers[1].URL, target.(httpstate.Backend).CloudURL())
	}
	assert.Equal(t, []credentialhelper.Request{
		{Reason: credentialhelper.Initial, SelectedBackendURL: servers[0].URL},
		{Reason: credentialhelper.Initial, SelectedBackendURL: servers[1].URL},
	}, requests)
	assert.Equal(t, "backend-1-token", session.HelperHTTPAuth(servers[1].URL).AccessToken())
	stored, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, servers[1].URL, stored.Current)
	assert.Empty(t, stored.Accounts)
}

func TestPrepareCurrentBackendSelectionPrecedence(t *testing.T) {
	t.Parallel()
	const (
		environmentURL = "https://environment.example.com"
		projectURL     = "https://project.example.com"
		storedURL      = "https://stored.example.com"
		overrideURL    = "https://override.example.com"
		helperURL      = "https://helper.example.com"
	)
	for _, tt := range []struct {
		name        string
		backendURL  string
		projectURL  string
		currentURL  string
		apiURL      string
		decline     bool
		selectedURL string
		wantURL     string
		setCurrent  bool
	}{
		{
			name:       "environment overrides project, stored backend, and PULUMI_API",
			backendURL: environmentURL, projectURL: projectURL, currentURL: storedURL, apiURL: overrideURL,
			selectedURL: environmentURL, wantURL: environmentURL,
		},
		{
			name:       "project overrides stored backend and PULUMI_API",
			projectURL: projectURL, currentURL: storedURL, apiURL: overrideURL,
			selectedURL: projectURL, wantURL: projectURL,
		},
		{
			name: "stored backend overrides PULUMI_API", currentURL: storedURL, apiURL: overrideURL,
			selectedURL: storedURL, wantURL: storedURL,
		},
		{
			name: "PULUMI_API selects a fixed backend", apiURL: overrideURL,
			selectedURL: overrideURL, wantURL: overrideURL, setCurrent: true,
		},
		{name: "helper replaces implicit fallback", wantURL: helperURL, setCurrent: true},
		{name: "helper declines implicit fallback", decline: true, wantURL: "https://api.pulumi.com", setCurrent: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := env.NewEnv(env.MapStore{
				"PULUMI_BACKEND_URL": tt.backendURL,
				"PULUMI_API":         tt.apiURL,
			})
			ws := &pkgWorkspace.MockContext{
				GetStoredCredentialsF: func() (workspace.Credentials, error) {
					return workspace.Credentials{Current: tt.currentURL}, nil
				},
			}
			project := &workspace.Project{Backend: &workspace.ProjectBackend{URL: tt.projectURL}}
			var selections []string
			session := auth.NewSessionWithHelperFunc(func(
				_ context.Context, request credentialhelper.Request,
			) (*credentialhelper.Response, error) {
				selections = append(selections, request.SelectedBackendURL)
				if request.SelectedBackendURL != "" || tt.decline {
					return nil, nil
				}
				return &credentialhelper.Response{BackendURL: helperURL}, nil
			})
			url, setCurrent, err := PrepareCurrentBackend(t.Context(), NewLoginManager(session), ws, store, project)
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, url)
			assert.Equal(t, tt.setCurrent, setCurrent)
			assert.Equal(t, []string{tt.selectedURL}, selections)
		})
	}
}

func TestHelperLoginPreservesAccounts(t *testing.T) {
	for _, mode := range []string{"token", "headers only", "rejected", "insecure"} {
		t.Run(mode, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_ACCESS_TOKEN", "environment-token")
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "helper-user")
			token, authorization := "helper-token", "token helper-token"
			if mode == "headers only" {
				token, authorization = "", ""
			}
			serve := httptest.NewServer
			if mode == "insecure" {
				serve = httptest.NewTLSServer
			}
			server := serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, authorization, r.Header.Get("Authorization"))
				assert.Equal(t, "helper-header", r.Header.Get("X-Helper"))
				if mode == "rejected" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/api/user":
					require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"githubLogin": "helper-user"}))
				case "/api/capabilities":
					require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			oldAccount := workspace.Account{AccessToken: "stored-token", Username: "stored-user"}
			require.NoError(t, workspace.StoreAccount("https://old.example.com", oldAccount, true))
			require.NoError(t, workspace.StoreAccount(server.URL, oldAccount, false))
			run := func(context.Context, credentialhelper.Request) (*credentialhelper.Response, error) {
				return &credentialhelper.Response{AccessToken: token, Headers: http.Header{"X-Helper": {"helper-header"}}}, nil
			}
			manager := NewLoginManager(auth.NewSessionWithHelperFunc(run))
			be, err := manager.Login(t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL,
				nil, LoginOptions{SetCurrent: true, Insecure: mode == "insecure", Color: colors.Never})
			expectedCurrent := server.URL
			if mode == "rejected" {
				require.Error(t, err)
				expectedCurrent = "https://old.example.com"
			} else {
				require.NoError(t, err)
				be.(httpstate.Backend).Capabilities(t.Context())
				username, _, _, err := be.CurrentUser()
				require.NoError(t, err)
				assert.Equal(t, "helper-user", username)
				pluginCredentials := PluginCredentials(be)
				t.Setenv("PULUMI_API", "https://unrelated.example.com")
				assert.Equal(t, server.URL, pluginCredentials.BackendURL)
				assert.Equal(t, token, pluginCredentials.HTTPAuth.AccessToken())
			}
			stored, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, expectedCurrent, stored.Current)
			assert.Equal(t, oldAccount, stored.Accounts["https://old.example.com"])
			oldAccount.Insecure = mode == "insecure"
			assert.Equal(t, oldAccount, stored.Accounts[server.URL])
			if mode == "insecure" {
				// A fresh login manager must use the saved TLS setting.
				manager = NewLoginManager(auth.NewSessionWithHelperFunc(run))
				be, err = manager.Current(t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL, nil, false)
				require.NoError(t, err)
				be.(httpstate.Backend).Capabilities(t.Context())
			}
		})
	}
}

func TestResolveCredentialsWithoutLogin(t *testing.T) {
	for _, tt := range []struct {
		name, envToken, wantToken, wantSource string
		missing                               bool
		helper                                bool
		response                              *credentialhelper.Response
	}{
		{name: "missing", missing: true, wantSource: "none"},
		{name: "stored", wantToken: "stored-token", wantSource: "account credentials"},
		{name: "environment", envToken: "env-token", wantToken: "env-token", wantSource: "PULUMI_ACCESS_TOKEN"},
		{
			name: "helper", helper: true, envToken: "env-token",
			response:  &credentialhelper.Response{AccessToken: "helper-token"},
			wantToken: "helper-token", wantSource: "credential helper",
		},
		{
			name: "header only", helper: true, envToken: "env-token",
			response: &credentialhelper.Response{Headers: http.Header{"X-Gate": {"secret"}}}, wantSource: "none",
		},
		{name: "declined", helper: true, envToken: "env-token", wantToken: "env-token", wantSource: "PULUMI_ACCESS_TOKEN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_ACCESS_TOKEN", tt.envToken)
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Error("resolving credentials must not contact the backend")
			}))
			defer server.Close()
			if !tt.missing {
				require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{AccessToken: "stored-token"}, true))
			}
			before, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			session := auth.NewSessionWithHelperFunc(nil)
			if tt.helper {
				session = auth.NewSessionWithHelperFunc(func(
					_ context.Context, request credentialhelper.Request,
				) (*credentialhelper.Response, error) {
					assert.Equal(t, server.URL, request.SelectedBackendURL)
					return tt.response, nil
				})
			}
			manager := NewLoginManager(session)
			credentials, err := manager.ResolveCredentials(t.Context(), pkgWorkspace.Instance, server.URL, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, credentials.HTTPAuth.AccessToken())
			assert.Equal(t, tt.wantSource, credentials.Info.AccessTokenSource)
			again, err := manager.ResolveCredentials(t.Context(), pkgWorkspace.Instance, server.URL, nil)
			require.NoError(t, err)
			assert.Same(t, credentials.HTTPAuth, again.HTTPAuth)
			after, err := workspace.GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}
}

func TestServiceSecretsResolveTheirOwnBackend(t *testing.T) {
	ptesting.IsolateCredentials(t)
	const currentURL = "https://current.example.com"
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{Current: currentURL}))
	t.Setenv("PULUMI_ACCESS_TOKEN", "unused-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Equal(t, "secret", r.Header.Get("X-Gate"))
		switch r.URL.Path {
		case "/api/capabilities":
			fmt.Fprint(w, `{}`)
		case "/api/stacks/owner/project/stack/decrypt":
			fmt.Fprintf(w, `{"plaintext":%q}`, base64.StdEncoding.EncodeToString([]byte("decrypted")))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	var requests []credentialhelper.Request
	manager := NewLoginManager(auth.NewSessionWithHelperFunc(func(
		_ context.Context, request credentialhelper.Request,
	) (*credentialhelper.Response, error) {
		requests = append(requests, request)
		return &credentialhelper.Response{Headers: http.Header{"X-Gate": {"secret"}}}, nil
	}))
	state := json.RawMessage(fmt.Sprintf(`{"url":%q,"owner":"owner","project":"project","stack":"stack"}`, server.URL))
	sm, err := SecretsProvider(manager).OfType(t.Context(), service.Type, state)
	require.NoError(t, err)
	plaintext, err := sm.Decrypter().DecryptValue(t.Context(), base64.StdEncoding.EncodeToString([]byte("ciphertext")))
	require.NoError(t, err)
	assert.Equal(t, "decrypted", plaintext)
	assert.JSONEq(t, string(state), string(sm.State()))
	assert.Equal(t, []credentialhelper.Request{
		{Reason: credentialhelper.Initial, SelectedBackendURL: server.URL},
	}, requests)
	stored, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, currentURL, stored.Current)
	assert.Empty(t, stored.Accounts)
}
