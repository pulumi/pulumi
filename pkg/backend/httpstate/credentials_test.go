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

package httpstate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestNewWithCredentialsUsesSelectedAccount(t *testing.T) {
	for _, fileState := range []string{"other account", "invalid file", "no file"} {
		t.Run(fileState, func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "selected-org")
			var userRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "token memory-token", r.Header.Get("Authorization"))
				switch r.URL.Path {
				case "/api/capabilities":
					require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
				case "/api/user":
					userRequests.Add(1)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"githubLogin":   "remote-user",
						"organizations": []map[string]string{{"githubLogin": "remote-org"}},
					}))
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)

			path := filepath.Join(dirs.Home, "credentials.json")
			switch fileState {
			case "other account":
				require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
					AccessToken: "stored-token", Username: "stored-user",
				}, true))
			case "invalid file":
				require.NoError(t, os.WriteFile(path, []byte("invalid credentials JSON"), 0o600))
			}
			var before []byte
			if fileState != "no file" {
				var err error
				before, err = os.ReadFile(path)
				require.NoError(t, err)
			}

			credentials := auth.Credentials{
				BackendURL: server.URL,
				Account:    workspace.Account{AccessToken: "memory-token"},
			}
			if fileState == "other account" {
				credentials.Account.Username = "selected-user"
				credentials.Account.Organizations = []string{"selected-org"}
				credentials.Account.TokenInformation = &workspace.TokenInformation{Name: "selected-token"}
			}
			be, err := NewWithCredentials(t.Context(), diagtest.LogSink(t), credentials, nil, false)
			require.NoError(t, err)
			be.Capabilities(t.Context())
			username, orgs, tokenInfo, err := be.CurrentUser()
			require.NoError(t, err)
			if fileState == "other account" {
				assert.Equal(t, "selected-user", username)
				assert.Equal(t, []string{"selected-org"}, orgs)
				assert.Equal(t, credentials.Account.TokenInformation, tokenInfo)
				assert.Zero(t, userRequests.Load())
			} else {
				assert.Equal(t, "remote-user", username)
				assert.Equal(t, []string{"remote-org"}, orgs)
				assert.EqualValues(t, 1, userRequests.Load())
			}
			username, _, _, err = be.(*cloudBackend).escClient.GetPulumiAccountDetails(t.Context())
			require.NoError(t, err)
			assert.Equal(t, "remote-user", username)
			if fileState == "no file" {
				assert.NoFileExists(t, path)
			} else {
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, before, after)
			}
			assert.NoFileExists(t, filepath.Join(dirs.AgentDir, "credentials.json"))
		})
	}
}

func TestNewWithCredentialsRefreshPersistence(t *testing.T) {
	for _, tt := range []struct {
		name          string
		source        string
		failSave      bool
		allowFallback bool
	}{
		{name: "memory", source: "memory"},
		{name: "default", source: "default"},
		{name: "agent", source: "agent"},
		{name: "default save failure", source: "default", failSave: true},
		{name: "default save failure in agent mode", source: "default", failSave: true, allowFallback: true},
		{name: "agent save failure", source: "agent", failSave: true, allowFallback: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "selected-org")
			t.Setenv("CODEX_SANDBOX", "1")
			if tt.allowFallback {
				t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
			}
			var refreshes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/oauth/token" {
					refreshes.Add(1)
					require.NoError(t, r.ParseForm())
					assert.Equal(t, "initial-refresh-token", r.Form.Get("refresh_token"))
					require.NoError(t, json.NewEncoder(w).Encode(apitype.TokenExchangeGrantResponse{
						AccessToken: "new-access-token", RefreshToken: "new-refresh-token", ExpiresIn: 3600,
					}))
					return
				}
				if r.Header.Get("Authorization") != "token new-access-token" {
					w.WriteHeader(http.StatusUnauthorized)
					require.NoError(t, json.NewEncoder(w).Encode(apitype.ErrorResponse{Code: 401}))
					return
				}
				assert.Equal(t, "/api/capabilities", r.URL.Path)
				require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{}))
			}))
			t.Cleanup(server.Close)

			expiry := time.Now().Add(time.Hour)
			credentials := &auth.Credentials{
				BackendURL: server.URL,
				Account: workspace.Account{
					AccessToken: "stale-access-token", RefreshToken: "initial-refresh-token",
					Username: "selected-user", LastValidatedAt: time.Now(),
					TokenInformation: &workspace.TokenInformation{ExpiresAt: &expiry},
				},
			}
			var err error
			const currentURL = "https://api.other.example.com"
			switch tt.source {
			case "default":
				require.NoError(t, workspace.StoreAccount(currentURL, workspace.Account{AccessToken: "other-token"}, true))
				require.NoError(t, workspace.StoreAccount(server.URL, credentials.Account, false))
				credentials, err = NewLoginManager().Current(t.Context(), server.URL, false, false)
			case "agent":
				require.NoError(t, workspace.StoreAgentAccount(currentURL, workspace.Account{AccessToken: "other-token"}, true))
				require.NoError(t, workspace.StoreAgentAccount(server.URL, credentials.Account, false))
				credentials, err = defaultLoginManager{}.currentOrSignupAgentAccount(
					t.Context(), server.URL, false, false, "codex", nil)
			}
			require.NoError(t, err)
			require.NotNil(t, credentials)
			if tt.failSave {
				dir := dirs.Home
				if tt.source == "agent" {
					dir = dirs.AgentDir
				}
				path := filepath.Join(dir, "credentials.json")
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.Mkdir(path, 0o700))
			}
			// Persistence keeps the policy and destination chosen at login.
			if tt.allowFallback {
				t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "false")
			} else {
				t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
			}
			otherHome, otherAgent := t.TempDir(), t.TempDir()
			t.Setenv("PULUMI_HOME", otherHome)
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", otherAgent)

			var saveErr error
			if save := credentials.Persist; save != nil {
				credentials.Persist = func(updated workspace.Account) error {
					saveErr = save(updated)
					return saveErr
				}
			}
			be, err := NewWithCredentials(t.Context(), diagtest.LogSink(t), *credentials, nil, false)
			require.NoError(t, err)
			_, err = be.(*cloudBackend).capabilities.Result(t.Context())
			require.NoError(t, err)
			if tt.failSave && (tt.source == "agent" || !tt.allowFallback) {
				require.Error(t, saveErr)
			} else {
				require.NoError(t, saveErr)
			}
			_, _, _, err = be.CurrentUser()
			require.NoError(t, err)
			assert.EqualValues(t, 1, refreshes.Load())
			assert.True(t, expiry.Equal(*credentials.Account.TokenInformation.ExpiresAt))
			assert.NoFileExists(t, filepath.Join(otherHome, "credentials.json"))
			assert.NoFileExists(t, filepath.Join(otherAgent, "credentials.json"))
			for name, dir := range map[string]string{"default": dirs.Home, "agent": dirs.AgentDir} {
				path := filepath.Join(dir, "credentials.json")
				if name != tt.source {
					assert.NoFileExists(t, path)
					continue
				}
				if tt.failSave {
					assert.DirExists(t, path)
					continue
				}
				raw, err := os.ReadFile(path)
				require.NoError(t, err)
				var saved workspace.Credentials
				require.NoError(t, json.Unmarshal(raw, &saved))
				assert.Equal(t, currentURL, saved.Current)
				account := saved.Accounts[server.URL]
				assert.Equal(t, "new-access-token", account.AccessToken)
				assert.Equal(t, "new-refresh-token", account.RefreshToken)
				require.NotNil(t, account.TokenInformation)
				require.NotNil(t, account.TokenInformation.ExpiresAt)
				assert.True(t, account.TokenInformation.ExpiresAt.After(time.Now()))
			}
		})
	}
}
