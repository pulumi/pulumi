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
	"encoding/json"
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
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
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
				t.Context(), pkgWorkspace.Instance, diagtest.LogSink(t), server.URL, nil, false, false, colors.Never)
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
					return lm.Login(ctx, pkgWorkspace.Instance, diagtest.LogSink(t), url, nil, false, false, colors.Never)
				}
			}
			diy, err := IsDIYBackend(ctx, pkgWorkspace.Instance, lm)
			require.NoError(t, err)
			assert.True(t, diy)
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
		})
	}
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
			url, setCurrent, err := PrepareCurrentBackend(t.Context(), session, ws, store, project)
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, url)
			assert.Equal(t, tt.setCurrent, setCurrent)
			assert.Equal(t, []string{tt.selectedURL}, selections)
		})
	}
}
