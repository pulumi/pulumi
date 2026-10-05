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
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestMissingPulumiAccessToken(t *testing.T) {
	ptesting.IsolateCredentials(t)

	{ // Disable interactive mode
		disableInteractive := cmdutil.DisableInteractive
		cmdutil.DisableInteractive = true
		t.Cleanup(func() {
			cmdutil.DisableInteractive = disableInteractive
		})
	}

	ctx := t.Context()

	_, err := NewLoginManager().Login(ctx, "https://api.example.com", false, "", "", nil, true, display.Options{})
	var expectedErr backenderr.MissingEnvVarForNonInteractiveError
	if assert.ErrorAs(t, err, &expectedErr) {
		assert.Equal(t, env.AccessToken.Var(), expectedErr.Var)
	}
}

func TestGetBackendAccountDoesNotFallbackToAgentCredentialsWithExplicitPath(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")

	badCredentialsDir := t.TempDir()
	badCredentialsPath := badCredentialsDir + "/not-a-directory"
	require.NoError(t, os.WriteFile(badCredentialsPath, []byte("not a directory"), 0o600))
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, badCredentialsPath)
	t.Setenv("CODEX_SANDBOX", "1")

	err := workspace.StoreAgentAccount("https://api.example.com", workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	account, err := getBackendAccount(t.Context(), "https://api.example.com")
	require.Error(t, err)
	assert.Empty(t, account.AccessToken)
}

func TestCurrentEnvTokenFailsWithInaccessibleExplicitPath(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")

	badCredentialsDir := t.TempDir()
	badCredentialsPath := badCredentialsDir + "/not-a-directory"
	require.NoError(t, os.WriteFile(badCredentialsPath, []byte("not a directory"), 0o600))
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, badCredentialsPath)
	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv("PULUMI_ACCESS_TOKEN", "env-token")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal(t, "/api/user", req.URL.Path)
		err := json.NewEncoder(rw).Encode(map[string]any{
			"githubLogin":   "agent-user",
			"organizations": []map[string]string{},
		})
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.Error(t, err)
	assert.Nil(t, account)

	agentAccount, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, agentAccount.AccessToken)
}

func TestCurrentEnvTokenStoresInDefaultPathWhenWritable(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")

	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv("PULUMI_ACCESS_TOKEN", "env-token")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		require.Equal(t, "/api/user", req.URL.Path)
		err := json.NewEncoder(rw).Encode(map[string]any{
			"githubLogin":   "agent-user",
			"organizations": []map[string]string{},
		})
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "env-token", account.Account.AccessToken)

	defaultAccount, err := workspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "env-token", defaultAccount.AccessToken)
	agentAccount, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, agentAccount.AccessToken)
}

func TestValidateStoredAccountSkipsNetworkWhenNoCredential(t *testing.T) {
	t.Parallel()
	// An account with neither an access nor a refresh token can't authenticate and must short-
	// circuit before any network attempt — the cloudURL here intentionally points nowhere.
	account, valid, err := validateStoredAccount(t.Context(), nil, "http://127.0.0.1:0", false, workspace.Account{})
	require.NoError(t, err)
	assert.False(t, valid)
	assert.Empty(t, account.AccessToken)
}

func TestGetAccountDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		accessToken  string
		setupServer  func() *httptest.Server
		wantErr      bool
		wantUsername string
		wantOrgs     []string
		checkErr     func(*testing.T, error)
	}{
		{
			name:        "successful account details fetch",
			accessToken: "pul-valid-token",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/user" {
						// Create a response matching the serviceUser structure
						resp := map[string]any{
							"githubLogin": "testuser",
							"organizations": []map[string]any{
								{"githubLogin": "org1"},
								{"githubLogin": "org2"},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(resp)
					}
				}))
			},
			wantErr:      false,
			wantUsername: "testuser",
			wantOrgs:     []string{"org1", "org2"},
		},
		{
			name:        "unauthorized access",
			accessToken: "pul-invalid-token",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/user" {
						w.WriteHeader(http.StatusUnauthorized)
						_ = json.NewEncoder(w).Encode(apitype.ErrorResponse{
							Code:    401,
							Message: "Unauthorized",
						})
					}
				}))
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, ErrUnauthorized))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cloudURL := ""
			if tt.setupServer != nil {
				server := tt.setupServer()
				defer server.Close()
				cloudURL = server.URL
			}

			username, orgs, tokenInfo, err := getAccountDetails(
				t.Context(), nil, cloudURL, false, tt.accessToken, "", nil,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantUsername, username)
				assert.Equal(t, tt.wantOrgs, orgs)
				// tokenInfo might be nil for old services
				_ = tokenInfo
			}
		})
	}
}

// Regression test: an undecryptable credentials file must surface its
// actionable error from Current instead of degrading into "not logged in".
// Mirrors the report: PULUMI_BACKEND_URL set (so no earlier read fails),
// explicit credentials path, no env token, no agent environment.
//
//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentSurfacesUndecryptableCredentials(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	// An envelope recording a backend that exists on no platform is
	// undecryptable everywhere, which is what a lost key looks like.
	unreachable := securestore.Backend("not-a-real-backend")
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	cloudURL := "https://api.undecryptable-current.example.com"
	payload, err := json.Marshal(workspace.Credentials{
		Current:  cloudURL,
		Accounts: map[string]workspace.Account{cloudURL: {AccessToken: "pul-lost"}},
	})
	require.NoError(t, err)
	envelope, err := securestore.Seal(key, unreachable, payload)
	require.NoError(t, err)
	credsFile := filepath.Join(credsDir, "credentials.json")
	require.NoError(t, os.WriteFile(credsFile, envelope, 0o600))

	_, err = defaultLoginManager{}.Current(t.Context(), cloudURL, false, false)
	require.Error(t, err, "Current must not treat an undecryptable file as logged-out")
	assert.True(t, workspace.IsUndecryptableCredentials(err))
	assert.Contains(t, err.Error(), "pulumi login")
}

func TestCurrentEnvTokenDoesNotBypassUndecryptableCredentials(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home
	// While PULUMI_ACCESS_TOKEN is persisted into the credentials file,
	// proceeding despite an undecryptable file would end in a write over an
	// envelope that may only be temporarily unreadable. Surface the
	// actionable error instead; revisit once the env token is no longer
	// written to disk.
	t.Setenv("PULUMI_ACCESS_TOKEN", "env-token")

	key := make([]byte, 32)
	envelope, err := securestore.Seal(key, securestore.Backend("test-unsupported"), []byte(`{}`))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(credsDir, "credentials.json"), envelope, 0o600))

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		err := json.NewEncoder(rw).Encode(map[string]any{
			"githubLogin":   "env-user",
			"organizations": []map[string]string{},
		})
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.Error(t, err)
	assert.Nil(t, account)
	assert.True(t, workspace.IsUndecryptableCredentials(err),
		"the typed error must surface so callers can point at recovery, got: %v", err)

	raw, err := os.ReadFile(filepath.Join(credsDir, "credentials.json"))
	require.NoError(t, err)
	assert.Equal(t, envelope, raw, "the unreadable file must be left untouched")
}
