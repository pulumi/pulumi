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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
	dirs := ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "selected-org")
	path := filepath.Join(dirs.Home, "credentials.json")
	contents := []byte("invalid credentials JSON") // we should never read this
	require.NoError(t, os.WriteFile(path, contents, 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token memory-token", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/capabilities":
			require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{
				Capabilities: []apitype.APICapabilityConfig{{Capability: apitype.BatchEncrypt}},
			}))
		case "/api/user":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"githubLogin": "remote-user"}))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	credentials := auth.Credentials{
		BackendURL: server.URL,
		Account: workspace.Account{
			AccessToken: "memory-token", Username: "selected-user", Organizations: []string{"selected-org"},
			TokenInformation: &workspace.TokenInformation{Name: "selected-token"},
		},
	}
	be, err := NewWithCredentials(t.Context(), diagtest.LogSink(t), credentials, nil, false)
	require.NoError(t, err)
	assert.True(t, be.Capabilities(t.Context()).BatchEncryption)
	username, orgs, tokenInfo, err := be.CurrentUser()
	require.NoError(t, err)
	assert.Equal(t, "selected-user", username)
	assert.Equal(t, []string{"selected-org"}, orgs)
	assert.Equal(t, credentials.Account.TokenInformation, tokenInfo)
	// ESC should use the passed credeentials
	username, _, _, err = be.(*cloudBackend).escClient.GetPulumiAccountDetails(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "remote-user", username)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, contents, after)
	assert.NoFileExists(t, filepath.Join(dirs.AgentDir, "credentials.json"))
}

func TestNewWithCredentialsRefreshPersistence(t *testing.T) {
	for _, mode := range []string{"callback", "memory only"} {
		t.Run(mode, func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			t.Setenv("PULUMI_DEFAULT_ORGANIZATION", "selected-org")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/oauth/token" {
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
				require.NoError(t, json.NewEncoder(w).Encode(apitype.CapabilitiesResponse{
					Capabilities: []apitype.APICapabilityConfig{{Capability: apitype.BatchEncrypt}},
				}))
			}))
			t.Cleanup(server.Close)

			expiry := time.Now().Add(24 * time.Hour)
			credentials := auth.Credentials{
				BackendURL: server.URL,
				Account: workspace.Account{
					AccessToken: "stale-access-token", RefreshToken: "initial-refresh-token",
					Username:         "selected-user",
					TokenInformation: &workspace.TokenInformation{ExpiresAt: &expiry},
				},
			}
			var saved *workspace.Account
			if mode == "callback" {
				credentials.Persist = func(updated workspace.Account) error {
					saved = &updated
					return nil
				}
			}
			be, err := NewWithCredentials(t.Context(), diagtest.LogSink(t), credentials, nil, false)
			require.NoError(t, err)
			assert.True(t, be.Capabilities(t.Context()).BatchEncryption)
			if mode == "callback" {
				require.NotNil(t, saved)
				assert.Equal(t, "new-access-token", saved.AccessToken)
				assert.Equal(t, "new-refresh-token", saved.RefreshToken)
				assert.Equal(t, "selected-user", saved.Username)
				require.NotNil(t, saved.TokenInformation)
				require.NotNil(t, saved.TokenInformation.ExpiresAt)
				assert.WithinDuration(t, time.Now().Add(time.Hour), *saved.TokenInformation.ExpiresAt, time.Minute)
			}
			assert.True(t, expiry.Equal(*credentials.Account.TokenInformation.ExpiresAt))
			assert.NoFileExists(t, filepath.Join(dirs.Home, "credentials.json"))
			assert.NoFileExists(t, filepath.Join(dirs.AgentDir, "credentials.json"))
		})
	}
}

func TestCurrentCredentialsPersistToOriginalFile(t *testing.T) {
	for _, source := range []string{"user", "agent"} {
		t.Run(source, func(t *testing.T) {
			dirs := ptesting.IsolateCredentials(t)
			store, dir, unusedDir := workspace.StoreAccount, dirs.Home, dirs.AgentDir
			if source == "agent" {
				store, dir, unusedDir = workspace.StoreAgentAccount, dirs.AgentDir, dirs.Home
			}
			const cloudURL = "https://api.example.com"
			expiry := time.Now().Add(time.Hour)
			require.NoError(t, store(cloudURL, workspace.Account{
				AccessToken: "initial-token", Username: "selected-user", LastValidatedAt: time.Now(),
				TokenInformation: &workspace.TokenInformation{ExpiresAt: &expiry},
			}, false))

			var credentials *auth.Credentials
			var err error
			if source == "user" {
				credentials, err = NewLoginManager().Current(t.Context(), cloudURL, false, true)
			} else {
				credentials, err = defaultLoginManager{}.currentOrSignupAgentAccount(
					t.Context(), cloudURL, false, true, "codex", nil)
			}
			require.NoError(t, err)
			require.NotNil(t, credentials)
			require.NotNil(t, credentials.Persist)

			// A later refresh must preserve the current backend and the original file location.
			const currentURL = "https://api.other.example.com"
			require.NoError(t, store(currentURL, workspace.Account{AccessToken: "other-token"}, true))
			otherHome, otherAgent := t.TempDir(), t.TempDir()
			t.Setenv("PULUMI_HOME", otherHome)
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", otherAgent)
			updated := credentials.Account
			updated.AccessToken = "new-access-token"
			require.NoError(t, credentials.Persist(updated))

			raw, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
			require.NoError(t, err)
			var saved workspace.Credentials
			require.NoError(t, json.Unmarshal(raw, &saved))
			assert.Equal(t, currentURL, saved.Current)
			assert.Equal(t, "new-access-token", saved.Accounts[cloudURL].AccessToken)
			assert.Equal(t, "other-token", saved.Accounts[currentURL].AccessToken)
			assert.NoFileExists(t, filepath.Join(unusedDir, "credentials.json"))
			assert.NoFileExists(t, filepath.Join(otherHome, "credentials.json"))
			assert.NoFileExists(t, filepath.Join(otherAgent, "credentials.json"))
		})
	}
}

func TestLoginPersistenceFailurePolicy(t *testing.T) {
	for _, tt := range []struct {
		name          string
		agentAccount  bool
		allowFallback bool
		wantError     bool
	}{
		{name: "user", wantError: true},
		{name: "user with agent fallback", allowFallback: true},
		{name: "agent", agentAccount: true, allowFallback: true, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ptesting.IsolateCredentials(t)
			t.Setenv("CODEX_SANDBOX", "1")
			t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", strconv.FormatBool(tt.allowFallback))
			// Allow the initial save, then simulate a failed save during refresh.
			var nextSaveErr error
			save := func(string, workspace.Account, bool) error { return nextSaveErr }
			account := workspace.Account{AccessToken: "initial-token"}
			const cloudURL = "https://api.example.com"
			var credentials *auth.Credentials
			var err error
			if tt.agentAccount {
				credentials, err = finishLogin(cloudURL, account, true, save)
			} else {
				credentials, err = finishUserLoginWithAgentFallback(cloudURL, account, true, save)
			}
			require.NoError(t, err)
			require.NotNil(t, credentials)
			require.NotNil(t, credentials.Persist)

			// Later refreshes retain the failure policy selected at login.
			t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", strconv.FormatBool(!tt.allowFallback))
			nextSaveErr = errors.New("save failed")
			err = credentials.Persist(account)
			if tt.wantError {
				assert.ErrorIs(t, err, nextSaveErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
