// Copyright 2016, Pulumi Corporation.
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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJWT is a test JWT token used in tests.
//
//nolint:lll // JWT token is long
const testJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

//nolint:paralleltest // mutates global configuration
func TestEnabledFullyQualifiedStackNames(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := t.Context()

	_, err := NewLoginManager().Login(ctx, client.PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), client.PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true /*force*/, false /*removeBackups*/)
		require.NoError(t, err)
	}()

	previous := cmdutil.FullyQualifyStackNames
	expected := s.Ref().FullyQualifiedName().String()

	// Act
	cmdutil.FullyQualifyStackNames = true
	defer func() { cmdutil.FullyQualifyStackNames = previous }()

	actual := s.Ref().String()

	// Assert
	assert.Equal(t, expected, actual)
}

//nolint:paralleltest // mutates env vars and global state
func TestMissingPulumiAccessToken(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("AI_AGENT", "")
	t.Setenv("CODEX_SANDBOX", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CURSOR_TRACE_ID", "")
	t.Setenv("CURSOR_AGENT", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "")

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

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestGetBackendAccountDoesNotFallbackToAgentCredentialsWithExplicitPath(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	badCredentialsDir := t.TempDir()
	badCredentialsPath := badCredentialsDir + "/not-a-directory"
	require.NoError(t, os.WriteFile(badCredentialsPath, []byte("not a directory"), 0o600))
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, badCredentialsPath)
	t.Setenv("CODEX_SANDBOX", "1")

	err = pkgWorkspace.StoreAgentAccount("https://api.example.com", pkgWorkspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	account, _, err := getBackendAccount(t.Context(), "https://api.example.com")
	require.Error(t, err)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestCurrentEnvTokenFailsWithInaccessibleExplicitPath(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	badCredentialsDir := t.TempDir()
	badCredentialsPath := badCredentialsDir + "/not-a-directory"
	require.NoError(t, os.WriteFile(badCredentialsPath, []byte("not a directory"), 0o600))
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, badCredentialsPath)
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

	agentAccount, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, agentAccount.AccessToken)
}

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestCurrentEnvTokenStoresInDefaultPathWhenWritable(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
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
	assert.Equal(t, "env-token", account.AccessToken)

	defaultAccount, err := pkgWorkspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "env-token", defaultAccount.AccessToken)
	agentAccount, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, agentAccount.AccessToken)
}

//nolint:paralleltest // mutates env vars and credentials on disk
func TestCurrentRefreshesAccessTokenOn401WhenRefreshTokenStored(t *testing.T) {
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/oauth/token":
			require.Equal(t, http.MethodPost, req.Method)
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "grant_type=refresh_token")
			assert.Contains(t, string(body), "refresh_token=stored-refresh-token")
			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:  "fresh-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "stored-refresh-token",
			})
			require.NoError(t, err)
		case "/api/user":
			switch req.Header.Get("Authorization") {
			case "token stale-access-token":
				rw.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
				require.NoError(t, err)
			case "token fresh-access-token":
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "alice",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
			default:
				t.Errorf("unexpected Authorization header: %q", req.Header.Get("Authorization"))
				rw.WriteHeader(http.StatusUnauthorized)
			}
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.AccessToken,
		"the stored access token should be refreshed before reporting the account as valid")
	assert.Equal(t, "stored-refresh-token", account.RefreshToken,
		"the refresh token is preserved (Phase 1: server doesn't rotate)")
	assert.Equal(t, "alice", account.Username)

	saved, err := pkgWorkspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken,
		"credentials.json should reflect the refreshed access token")
	assert.Equal(t, "stored-refresh-token", saved.RefreshToken)
	assert.WithinDuration(t, time.Now(), saved.LastValidatedAt, time.Minute,
		"validateStoredAccount must stamp LastValidatedAt when it actually validates")
}

//nolint:paralleltest // mutates environment
func TestCurrentRefreshesFromRefreshOnlyStoredAccount(t *testing.T) {
	// HasCredential opens the gate for accounts with a refresh token but no access token. The
	// wrapper mints the first access token on the initial 401 from an empty bearer.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/oauth/token":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "refresh_token=only-refresh-token")
			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:  "minted-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "only-refresh-token",
			})
			require.NoError(t, err)
		case "/api/user":
			if req.Header.Get("Authorization") == "token minted-access-token" {
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "bob",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
			} else {
				rw.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
				require.NoError(t, err)
			}
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		RefreshToken: "only-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "minted-access-token", account.AccessToken)
	assert.Equal(t, "bob", account.Username)
}

//nolint:paralleltest // mutates environment
func TestCurrentPersistsRotatedRefreshToken(t *testing.T) {
	// True rotation: the refresh-token grant returns a refresh token DIFFERENT from the one we
	// sent. The wrapper updates the in-memory account and the writeback persists the rotated
	// value to credentials.json. Server-side rotation is a Phase 2 behavior — this test pins the
	// CLI side so we don't need a change when it lands.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/oauth/token":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "refresh_token=stored-refresh-token")
			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:  "fresh-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "rotated-refresh-token",
			})
			require.NoError(t, err)
		case "/api/user":
			if req.Header.Get("Authorization") == "token fresh-access-token" {
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "alice",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
				return
			}
			rw.WriteHeader(http.StatusUnauthorized)
			err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.AccessToken)
	assert.Equal(t, "rotated-refresh-token", account.RefreshToken)

	saved, err := pkgWorkspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken)
	assert.Equal(t, "rotated-refresh-token", saved.RefreshToken,
		"credentials.json must reflect the rotated refresh token")
}

//nolint:paralleltest // mutates environment
func TestCurrentPreservesRefreshTokenWhenGrantResponseOmitsIt(t *testing.T) {
	// RFC 6749 §6: omitted (or empty) refresh_token in the grant response means "keep using
	// yours" — the server is not signalling termination. credentials.json must hold onto the
	// existing refresh token so the next 401 can refresh again.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/oauth/token":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "refresh_token=stored-refresh-token")
			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken: "fresh-access-token",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
				// RefreshToken omitted — JSON encoder drops it.
			})
			require.NoError(t, err)
		case "/api/user":
			if req.Header.Get("Authorization") == "token fresh-access-token" {
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "alice",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
				return
			}
			rw.WriteHeader(http.StatusUnauthorized)
			err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.AccessToken)
	assert.Equal(t, "stored-refresh-token", account.RefreshToken,
		"omitted refresh_token in the response must not destroy the existing one")

	saved, err := pkgWorkspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken)
	assert.Equal(t, "stored-refresh-token", saved.RefreshToken,
		"credentials.json must hold onto the existing refresh token")
}

func TestValidateStoredAccountSkipsNetworkWhenNoCredential(t *testing.T) {
	t.Parallel()
	// An account with neither an access nor a refresh token can't authenticate and must short-
	// circuit before any network attempt — the cloudURL here intentionally points nowhere.
	account, valid, err := validateStoredAccount(t.Context(), "http://127.0.0.1:0", false, pkgWorkspace.Account{})
	require.NoError(t, err)
	assert.False(t, valid)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates environment
func TestCurrentRefreshesLocallyExpiredAccessTokenWhenRefreshTokenStored(t *testing.T) {
	// Cold-start with a locally-expired access token: validateStoredAccount must take the refresh
	// path instead of hard-failing, so the next call silently mints a fresh access token and
	// credentials.json is updated in place.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/oauth/token":
			require.Equal(t, http.MethodPost, req.Method)
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "grant_type=refresh_token")
			assert.Contains(t, string(body), "refresh_token=stored-refresh-token")
			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:  "fresh-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "stored-refresh-token",
			})
			require.NoError(t, err)
		case "/api/user":
			switch req.Header.Get("Authorization") {
			case "token stale-access-token":
				rw.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
				require.NoError(t, err)
			case "token fresh-access-token":
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "alice",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
			default:
				t.Errorf("unexpected Authorization header: %q", req.Header.Get("Authorization"))
				rw.WriteHeader(http.StatusUnauthorized)
			}
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.AccessToken,
		"a locally-expired access token must trigger a refresh instead of failing the validate step")
	assert.Equal(t, "stored-refresh-token", account.RefreshToken)
	assert.Equal(t, "alice", account.Username)

	saved, err := pkgWorkspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken,
		"credentials.json should reflect the refreshed access token")
	assert.Equal(t, "stored-refresh-token", saved.RefreshToken)
	require.NotNil(t, saved.TokenInformation, "refresh must update TokenInformation with the new expiry")
	require.NotNil(t, saved.TokenInformation.ExpiresAt,
		"the grant's ExpiresIn must land as the new TokenInformation.ExpiresAt; "+
			"without this the next cold-start can't take the local-expiry refresh path")
	assert.True(t, saved.TokenInformation.ExpiresAt.After(time.Now()),
		"the new ExpiresAt must be in the future (roughly now + ExpiresIn)")
}

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestCurrentPreservesExpiresAtWhenServerAcceptsLocallyExpiredAccessToken(t *testing.T) {
	// Cold-start with a locally-expired access token whose server-side TTL is actually still
	// valid: validateStoredAccount enters the refresh-or-fetch branch and /api/user succeeds
	// without firing a refresh. /api/user never returns ExpiresAt, so the merge must keep the
	// existing (now-past) ExpiresAt instead of nullifying TokenInformation entirely — otherwise
	// every subsequent run forfeits the cold-start refresh path and the agent-auth banner
	// mis-reports the account as unable to authenticate.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/user":
			assert.Equal(t, "token live-access-token", req.Header.Get("Authorization"),
				"the existing access token must reach /api/user — refresh should not fire on 200")
			err := json.NewEncoder(rw).Encode(map[string]any{
				"githubLogin":   "alice",
				"organizations": []map[string]string{},
			})
			require.NoError(t, err)
		case "/api/oauth/token":
			t.Errorf("refresh-token grant must not fire when /api/user returns 200")
			rw.WriteHeader(http.StatusInternalServerError)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "live-access-token",
		RefreshToken: "stored-refresh-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "live-access-token", account.AccessToken, "no refresh, no rotation")
	assert.Equal(t, "alice", account.Username)
	require.NotNil(t, account.TokenInformation,
		"TokenInformation must survive a fetch that returns no token info of its own")
	require.NotNil(t, account.TokenInformation.ExpiresAt,
		"ExpiresAt must survive the merge so the banner and cold-start path keep working")
}

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestCurrentReturnsNoAccountWhenAccessTokenLocallyExpiredAndNoRefreshToken(t *testing.T) {
	// Cold-start with a locally-expired access token but no refresh token must short-circuit
	// before hitting the network — preserves the pre-refresh-token behavior for accounts that
	// were stored without one.
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	// Ensure agent-mode fallback doesn't trigger — we're verifying the no-login path.
	t.Setenv("AI_AGENT", "")
	t.Setenv("CODEX_SANDBOX", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CURSOR_TRACE_ID", "")
	t.Setenv("CURSOR_AGENT", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "")

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		hits++
		t.Errorf("no network call should be made when the access token is locally expired "+
			"and no refresh token is stored: %s", req.URL.Path)
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
		AccessToken: "stale-access-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	assert.Nil(t, account, "no refresh token + locally-expired access token must not produce a logged-in account")
	assert.Equal(t, 0, hits, "validateStoredAccount must short-circuit without any network call")
}

//nolint:paralleltest // makes real HTTP calls to a test server
func TestGetAccountDetailsInstallsRefreshWrapperWhenRefreshTokenSupplied(t *testing.T) {
	var refreshCalls, userCalls int
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/oauth/token":
			refreshCalls++
			err := json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:  "wrapper-minted-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "the-refresh",
			})
			require.NoError(t, err)
		case "/api/user":
			userCalls++
			if req.Header.Get("Authorization") == "token wrapper-minted-token" {
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "carol",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
			} else {
				rw.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
				require.NoError(t, err)
			}
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	var gotAT, gotRT string
	var gotExpiresAt time.Time
	username, _, _, err := getAccountDetails(t.Context(), server.URL, false,
		"stale-access", "the-refresh",
		func(at string, expiresAt time.Time, rt string) error {
			gotAT, gotRT, gotExpiresAt = at, rt, expiresAt
			return nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "carol", username)
	assert.Equal(t, 1, refreshCalls, "refresh should fire exactly once after the initial 401")
	assert.Equal(t, 2, userCalls, "the /api/user call should retry after refresh")
	assert.Equal(t, "wrapper-minted-token", gotAT, "onRefresh receives the new access token")
	assert.Equal(t, "the-refresh", gotRT, "onRefresh receives the (preserved) refresh token")
	assert.False(t, gotExpiresAt.IsZero(),
		"onRefresh receives the new access token's ExpiresAt derived from the grant's ExpiresIn")
	assert.True(t, gotExpiresAt.After(time.Now().Add(50*time.Minute)),
		"ExpiresAt is roughly now+ExpiresIn (3600s in this fixture)")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentInvalidAgentCredentialsWithActiveClaimDoesNotSignup(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/agents/signup" {
			signupCalls++
		}
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	err = pkgWorkspace.StoreAgentAccount(server.URL, pkgWorkspace.Account{
		AccessToken: "expired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true)
	require.NoError(t, err)
	err = pkgWorkspace.StoreAgentClaim(pkgWorkspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(time.Hour),
		CloudURL:   server.URL,
	})
	require.NoError(t, err)

	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(t.Context(), server.URL, false, true, "codex")
	require.ErrorIs(t, err, ErrUnauthorized)
	assert.Nil(t, account)
	assert.Equal(t, 0, signupCalls)
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentRejectedAgentCredentialsWithUnexpiredTokenDoesNotSignup(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/user":
			rw.WriteHeader(http.StatusUnauthorized)
		case "/api/agents/signup":
			signupCalls++
			rw.WriteHeader(http.StatusInternalServerError)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	expiresAt := time.Now().Add(time.Hour)
	err = pkgWorkspace.StoreAgentAccount(server.URL, pkgWorkspace.Account{
		AccessToken: "locally-unexpired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = pkgWorkspace.StoreAgentClaim(pkgWorkspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(-time.Hour),
		CloudURL:   server.URL,
	})
	require.NoError(t, err)

	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(t.Context(), server.URL, false, true, "codex")
	require.ErrorIs(t, err, ErrUnauthorized)
	require.ErrorIs(t, err, backenderr.LoginRequiredError{})
	assert.ErrorContains(t, err, "ask the user to run `pulumi login`")
	assert.Nil(t, account)
	assert.Equal(t, 0, signupCalls)
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentValidAgentCredentialsWithExpiredClaimDoesNotSignup(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/agents/signup" {
			signupCalls++
		}
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiresAt := time.Now().Add(time.Hour)
	err = pkgWorkspace.StoreAgentAccount(server.URL, pkgWorkspace.Account{
		AccessToken:     "valid-agent-token",
		Username:        "agent-user",
		Organizations:   []string{"agent-org"},
		LastValidatedAt: time.Now(),
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = pkgWorkspace.StoreAgentClaim(pkgWorkspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(-time.Hour),
		CloudURL:   server.URL,
	})
	require.NoError(t, err)

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex")
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "valid-agent-token", account.AccessToken)
	assert.True(t, AgentCredentialsUsed(ctx, server.URL))
	assert.Equal(t, 0, signupCalls)

	// Post-validate persistence goes through agentAccount.Save, which writes to the agent file
	// (the account's source) rather than leaking into default credentials.
	fromAgent, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "valid-agent-token", fromAgent.AccessToken)
	fromDefault, err := pkgWorkspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, fromDefault.AccessToken, "agent-sourced account must not be copied into default credentials")
}

//nolint:paralleltest // mutates shared temporary agent credentials and console env
func TestCurrentSignupAgentAccountStoresClaimTokenURL(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})
	t.Setenv(client.ConsoleDomainEnvVar, "app.example.com")

	accessTokenValidUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	claimTokenValidUntil := accessTokenValidUntil.Add(24 * time.Hour)
	var signupMethods []string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/agents/signup":
			signupMethods = append(signupMethods, req.Method)
			switch req.Method {
			case http.MethodGet:
				err := json.NewEncoder(rw).Encode(client.AgentSignupChallenge{
					ChallengeID:   "challenge-1",
					ChallengeData: "v1:abcdef:8",
				})
				require.NoError(t, err)
			case http.MethodPost:
				var signupReq struct {
					ChallengeID     string `json:"challengeID"`
					ChallengeResult string `json:"challengeResult"`
					AgentName       string `json:"agentName"`
				}
				require.NoError(t, json.NewDecoder(req.Body).Decode(&signupReq))
				assert.Equal(t, "challenge-1", signupReq.ChallengeID)
				assert.NotEmpty(t, signupReq.ChallengeResult)
				assert.Equal(t, "codex", signupReq.AgentName)
				err := json.NewEncoder(rw).Encode(client.AgentSignupResponse{
					AccessToken:           "agent-token",
					AccessTokenValidUntil: accessTokenValidUntil,
					ClaimToken:            "claim-token",
					ClaimTokenValidUntil:  claimTokenValidUntil,
				})
				require.NoError(t, err)
			default:
				rw.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/user":
			assert.Equal(t, "token agent-token", req.Header.Get("Authorization"))
			err := json.NewEncoder(rw).Encode(map[string]any{
				"githubLogin": "agent-user",
				"organizations": []map[string]string{
					{"githubLogin": "agent-org"},
				},
			})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex")
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-token", account.AccessToken)
	assert.True(t, AgentCredentialsUsed(ctx, server.URL))
	require.NotNil(t, account.TokenInformation)
	require.NotNil(t, account.TokenInformation.ExpiresAt)
	assert.True(t, account.TokenInformation.ExpiresAt.Equal(accessTokenValidUntil))
	assert.Equal(t, []string{http.MethodGet, http.MethodPost}, signupMethods)

	claim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	assert.Equal(t, "http://app.example.com/claim/claim-token", claim.ClaimURL)
	assert.Equal(t, "claim-token", claim.ClaimToken)
	assert.True(t, claim.ValidUntil.Equal(claimTokenValidUntil))
	assert.Equal(t, server.URL, claim.CloudURL)
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentSignupAgentAccountStoresRefreshToken(t *testing.T) {
	// The refresh token returned by agent signup must land in the stored Account so the
	// auto-refresh wrapper can use it once the access token expires.
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})
	t.Setenv(client.ConsoleDomainEnvVar, "app.example.com")

	accessTokenValidUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	claimTokenValidUntil := accessTokenValidUntil.Add(24 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/agents/signup":
			switch req.Method {
			case http.MethodGet:
				err := json.NewEncoder(rw).Encode(client.AgentSignupChallenge{
					ChallengeID:   "challenge-1",
					ChallengeData: "v1:abcdef:8",
				})
				require.NoError(t, err)
			case http.MethodPost:
				err := json.NewEncoder(rw).Encode(client.AgentSignupResponse{
					AccessToken:           "agent-access-token",
					AccessTokenValidUntil: accessTokenValidUntil,
					RefreshToken:          "agent-refresh-token",
					ClaimToken:            "claim-token",
					ClaimTokenValidUntil:  claimTokenValidUntil,
				})
				require.NoError(t, err)
			default:
				rw.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/user":
			err := json.NewEncoder(rw).Encode(map[string]any{
				"githubLogin":   "agent-user",
				"organizations": []map[string]string{},
			})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex")
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-access-token", account.AccessToken)
	assert.Equal(t, "agent-refresh-token", account.RefreshToken,
		"signup-returned refresh token must be plumbed into the returned Account")

	stored, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "agent-refresh-token", stored.RefreshToken,
		"signup-returned refresh token must be persisted to the agent credentials file")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentSignupAgentAccountWithoutRefreshTokenLeavesAccountEmpty(t *testing.T) {
	// Back-compat with a server that doesn't (yet) issue refresh tokens at signup: the response
	// omits refreshToken and the CLI must not error or invent a value.
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})
	t.Setenv(client.ConsoleDomainEnvVar, "app.example.com")

	accessTokenValidUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	claimTokenValidUntil := accessTokenValidUntil.Add(24 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/agents/signup":
			switch req.Method {
			case http.MethodGet:
				err := json.NewEncoder(rw).Encode(client.AgentSignupChallenge{
					ChallengeID:   "challenge-1",
					ChallengeData: "v1:abcdef:8",
				})
				require.NoError(t, err)
			case http.MethodPost:
				err := json.NewEncoder(rw).Encode(client.AgentSignupResponse{
					AccessToken:           "agent-access-token",
					AccessTokenValidUntil: accessTokenValidUntil,
					ClaimToken:            "claim-token",
					ClaimTokenValidUntil:  claimTokenValidUntil,
				})
				require.NoError(t, err)
			default:
				rw.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/user":
			err := json.NewEncoder(rw).Encode(map[string]any{
				"githubLogin":   "agent-user",
				"organizations": []map[string]string{},
			})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex")
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-access-token", account.AccessToken)
	assert.Empty(t, account.RefreshToken, "no refreshToken in response → none on the Account")

	stored, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, stored.RefreshToken, "no refreshToken in response → none persisted")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentSignupAgentAccountReplacesExistingRefreshTokenOnResignup(t *testing.T) {
	// When existing agent creds are no longer valid AND the stored refresh token is rejected by
	// the server, the CLI falls through to re-signup. The refresh token returned by the new
	// signup replaces the stale one — the prior value must not survive into the rebuilt Account.
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
	t.Setenv(client.ConsoleDomainEnvVar, "app.example.com")

	accessTokenValidUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	claimTokenValidUntil := accessTokenValidUntil.Add(24 * time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/agents/signup":
			switch req.Method {
			case http.MethodGet:
				err := json.NewEncoder(rw).Encode(client.AgentSignupChallenge{
					ChallengeID:   "challenge-1",
					ChallengeData: "v1:abcdef:8",
				})
				require.NoError(t, err)
			case http.MethodPost:
				err := json.NewEncoder(rw).Encode(client.AgentSignupResponse{
					AccessToken:           "new-access-token",
					AccessTokenValidUntil: accessTokenValidUntil,
					RefreshToken:          "new-refresh-token",
					ClaimToken:            "new-claim-token",
					ClaimTokenValidUntil:  claimTokenValidUntil,
				})
				require.NoError(t, err)
			default:
				rw.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/oauth/token":
			// Reject the stale refresh token so validateStoredAccount can't revive the account.
			rw.WriteHeader(http.StatusBadRequest)
			err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 400, Message: "invalid_grant"})
			require.NoError(t, err)
		case "/api/user":
			if req.Header.Get("Authorization") == "token new-access-token" {
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "agent-user",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
				return
			}
			rw.WriteHeader(http.StatusUnauthorized)
			err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	// Stale agent creds: locally-expired access token and a stale refresh token that the server
	// will reject. No claim is stored, so currentOrSignupAgentAccount falls through to re-signup
	// once the refresh attempt fails.
	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, pkgWorkspace.StoreAgentAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex")
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "new-access-token", account.AccessToken)
	assert.Equal(t, "new-refresh-token", account.RefreshToken)

	stored, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "new-refresh-token", stored.RefreshToken,
		"re-signup must replace the stale refresh token, not preserve it")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentAgentAccountRefreshesLocallyExpiredAccessTokenInsteadOfResigning(t *testing.T) {
	// Cold-start in agent mode with a locally-expired access token but a valid refresh token:
	// validateStoredAccount must refresh through /api/oauth/token instead of falling through to
	// re-signup. Re-signup would burn a fresh agent identity and lose the claim association.
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
	t.Setenv(client.ConsoleDomainEnvVar, "app.example.com")

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/agents/signup":
			signupCalls++
			t.Errorf("re-signup must NOT happen when the stored refresh token succeeds: %s %s", req.Method, req.URL.Path)
			rw.WriteHeader(http.StatusInternalServerError)
		case "/api/oauth/token":
			require.Equal(t, http.MethodPost, req.Method)
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "grant_type=refresh_token")
			assert.Contains(t, string(body), "refresh_token=stored-refresh-token")
			err = json.NewEncoder(rw).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken:  "fresh-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "stored-refresh-token",
			})
			require.NoError(t, err)
		case "/api/user":
			switch req.Header.Get("Authorization") {
			case "token old-access-token":
				rw.WriteHeader(http.StatusUnauthorized)
				err := json.NewEncoder(rw).Encode(apitype.ErrorResponse{Code: 401, Message: "Unauthorized"})
				require.NoError(t, err)
			case "token fresh-access-token":
				err := json.NewEncoder(rw).Encode(map[string]any{
					"githubLogin":   "agent-user",
					"organizations": []map[string]string{},
				})
				require.NoError(t, err)
			default:
				t.Errorf("unexpected Authorization header: %q", req.Header.Get("Authorization"))
				rw.WriteHeader(http.StatusUnauthorized)
			}
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, pkgWorkspace.StoreAgentAccount(server.URL, pkgWorkspace.Account{
		AccessToken:  "old-access-token",
		RefreshToken: "stored-refresh-token",
		Username:     "agent-user",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex")
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.AccessToken,
		"locally-expired agent access token must be refreshed in place, not resigned")
	assert.Equal(t, "stored-refresh-token", account.RefreshToken)
	assert.Equal(t, "agent-user", account.Username, "username should survive the refresh path")
	assert.Equal(t, 0, signupCalls, "signup must not be called when refresh succeeds")

	stored, err := pkgWorkspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", stored.AccessToken,
		"agent credentials file should reflect the refreshed access token")
	assert.Equal(t, "stored-refresh-token", stored.RefreshToken)
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestCurrentSignupAgentAccountRequiresResponseFields(t *testing.T) {
	tests := []struct {
		name     string
		response client.AgentSignupResponse
		wantErr  string
	}{
		{
			name: "missing access token",
			response: client.AgentSignupResponse{
				AccessTokenValidUntil: time.Now().UTC().Add(time.Hour),
				ClaimToken:            "claim-token",
				ClaimTokenValidUntil:  time.Now().UTC().Add(2 * time.Hour),
			},
			wantErr: "signup response did not include an access token",
		},
		{
			name: "missing access token expiration",
			response: client.AgentSignupResponse{
				AccessToken:          "agent-token",
				ClaimToken:           "claim-token",
				ClaimTokenValidUntil: time.Now().UTC().Add(2 * time.Hour),
			},
			wantErr: "signup response did not include accessTokenValidUntil",
		},
		{
			name: "missing claim token",
			response: client.AgentSignupResponse{
				AccessToken:           "agent-token",
				AccessTokenValidUntil: time.Now().UTC().Add(time.Hour),
				ClaimTokenValidUntil:  time.Now().UTC().Add(2 * time.Hour),
			},
			wantErr: "signup response did not include a claim token",
		},
		{
			name: "missing claim token expiration",
			response: client.AgentSignupResponse{
				AccessToken:           "agent-token",
				AccessTokenValidUntil: time.Now().UTC().Add(time.Hour),
				ClaimToken:            "claim-token",
			},
			wantErr: "signup response did not include claimTokenValidUntil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				assert.Equal(t, "/api/agents/signup", req.URL.Path)
				switch req.Method {
				case http.MethodGet:
					err := json.NewEncoder(rw).Encode(client.AgentSignupChallenge{
						ChallengeID:   "challenge-1",
						ChallengeData: "v1:abcdef:8",
					})
					require.NoError(t, err)
				case http.MethodPost:
					err := json.NewEncoder(rw).Encode(tt.response)
					require.NoError(t, err)
				default:
					rw.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			t.Cleanup(server.Close)

			account, err := defaultLoginManager{}.currentOrSignupAgentAccount(t.Context(), server.URL, false, true, "codex")
			require.ErrorContains(t, err, tt.wantErr)
			assert.Nil(t, account)
		})
	}
}

//nolint:paralleltest // mutates env vars, global interactive mode, shared temporary agent credentials, and console env
func TestLoginUsesAgentSignupInNonInteractiveAgentMode(t *testing.T) {
	oldAgentCreds, err := pkgWorkspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgWorkspace.DeleteAgentCredentials())
		require.NoError(t, pkgWorkspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, pkgWorkspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	disableInteractive := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = true
	t.Cleanup(func() {
		cmdutil.DisableInteractive = disableInteractive
	})

	t.Setenv("PULUMI_ACCESS_TOKEN", "")
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(client.ConsoleDomainEnvVar, "app.example.com")

	accessTokenValidUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	claimTokenValidUntil := accessTokenValidUntil.Add(24 * time.Hour)
	var signupMethods []string
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/agents/signup":
			signupMethods = append(signupMethods, req.Method)
			switch req.Method {
			case http.MethodGet:
				err := json.NewEncoder(rw).Encode(client.AgentSignupChallenge{
					ChallengeID:   "challenge-1",
					ChallengeData: "v1:abcdef:8",
				})
				require.NoError(t, err)
			case http.MethodPost:
				err := json.NewEncoder(rw).Encode(client.AgentSignupResponse{
					AccessToken:           "agent-token",
					AccessTokenValidUntil: accessTokenValidUntil,
					ClaimToken:            "claim-token",
					ClaimTokenValidUntil:  claimTokenValidUntil,
				})
				require.NoError(t, err)
			default:
				rw.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/api/user":
			assert.Equal(t, "token agent-token", req.Header.Get("Authorization"))
			err := json.NewEncoder(rw).Encode(map[string]any{
				"githubLogin":   "agent-user",
				"organizations": []map[string]string{},
			})
			require.NoError(t, err)
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := NewLoginManager().Login(ctx, server.URL, false, "pulumi", "Pulumi Cloud", nil, true,
		display.Options{})
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-token", account.AccessToken)
	assert.Equal(t, []string{http.MethodGet, http.MethodPost}, signupMethods)
	assert.True(t, AgentCredentialsUsed(ctx, server.URL))
}

//nolint:paralleltest // mutates global configuration
func TestDisabledFullyQualifiedStackNames(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := t.Context()

	_, err := NewLoginManager().Login(ctx, client.PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), client.PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true /*force*/, false /*removeBackups*/)
		require.NoError(t, err)
	}()

	previous := cmdutil.FullyQualifyStackNames
	expected := s.Ref().Name().String()

	// Act
	cmdutil.FullyQualifyStackNames = false
	defer func() { cmdutil.FullyQualifyStackNames = previous }()

	actual := s.Ref().String()

	// Assert
	assert.Equal(t, expected, actual)
}

func TestValueOrDefaultURL(t *testing.T) {
	t.Run("TestValueOrDefault", func(t *testing.T) {
		current := ""
		mock := &pkgWorkspace.MockContext{
			GetStoredCredentialsF: func() (pkgWorkspace.Credentials, error) {
				return pkgWorkspace.Credentials{
					Current: current,
				}, nil
			},
		}

		// Validate trailing slash gets cut
		assert.Equal(t, "https://api-test1.pulumi.com", ValueOrDefaultURL(mock, "https://api-test1.pulumi.com/"))

		// Validate no-op case
		assert.Equal(t, "https://api-test2.pulumi.com", ValueOrDefaultURL(mock, "https://api-test2.pulumi.com"))

		// Validate trailing slash in pre-set env var is unchanged
		t.Setenv("PULUMI_API", "https://api-test3.pulumi.com/")
		assert.Equal(t, "https://api-test3.pulumi.com/", ValueOrDefaultURL(mock, ""))
		t.Setenv("PULUMI_API", "")

		// Validate current credentials URL is used
		current = "https://api-test4.pulumi.com"
		assert.Equal(t, "https://api-test4.pulumi.com", ValueOrDefaultURL(mock, ""))

		// Unless the current credentials URL is a filestate url
		current = "s3://test"
		assert.Equal(t, "https://api.pulumi.com", ValueOrDefaultURL(mock, ""))
	})
}

// TestDefaultOrganizationPriority tests the priority of the default organization.
// The priority is:
// 1. The default organization.
// 2. The user's organization.
func TestDefaultOrganizationPriority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		getDefaultOrg func() (string, error)
		getUserOrg    func() (string, error)
		wantOrg       string
		wantErr       bool
	}{
		{
			name: "default org set",
			getDefaultOrg: func() (string, error) {
				return "default-org", nil
			},
			getUserOrg: func() (string, error) {
				return "", nil
			},
			wantOrg: "default-org",
		},
		{
			name: "user org set",
			getDefaultOrg: func() (string, error) {
				return "", nil
			},
			getUserOrg: func() (string, error) {
				return "user-org", nil
			},
			wantOrg: "user-org",
		},
		{
			name: "no org set",
			getDefaultOrg: func() (string, error) {
				return "", nil
			},
			getUserOrg: func() (string, error) {
				return "", nil
			},
			wantErr: true,
		},
		{
			name: "both orgs set",
			getDefaultOrg: func() (string, error) {
				return "default-org", nil
			},
			getUserOrg: func() (string, error) {
				return "user-org", nil
			},
			wantOrg: "default-org",
		},
		{
			name: "default org set, user org error",
			getDefaultOrg: func() (string, error) {
				return "default-org", nil
			},
			getUserOrg: func() (string, error) {
				return "", errors.New("user org error")
			},
			wantOrg: "default-org",
		},
		{
			name: "user org set, default org error",
			getDefaultOrg: func() (string, error) {
				return "", errors.New("default org error")
			},
			getUserOrg: func() (string, error) {
				return "user-org", nil
			},
			wantOrg: "user-org",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			org, err := inferOrg(t.Context(), tt.getDefaultOrg, tt.getUserOrg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantOrg, org)
		})
	}
}

//nolint:paralleltest // mutates PULUMI_HOME-backed credentials/config
func TestNewDefaultOrgResolution(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name                 string
		configuredOrg        string
		serviceOrg           string
		expectedOrg          string
		expectedDefaultCalls int
	}{
		{
			name:                 "prefers configured default org",
			configuredOrg:        "configured-org",
			serviceOrg:           "service-org",
			expectedOrg:          "configured-org",
			expectedDefaultCalls: 0,
		},
		{
			name:                 "falls back to service default org",
			serviceOrg:           "service-org",
			expectedOrg:          "service-org",
			expectedDefaultCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULUMI_HOME", t.TempDir())

			defaultOrgCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				switch req.URL.Path {
				case "/api/capabilities":
					err := json.NewEncoder(rw).Encode(apitype.CapabilitiesResponse{})
					require.NoError(t, err)
				case "/api/user":
					err := json.NewEncoder(rw).Encode(map[string]any{
						"githubLogin":   "test-user",
						"organizations": []map[string]string{},
					})
					require.NoError(t, err)
				case "/api/user/organizations/default":
					defaultOrgCalls++
					err := json.NewEncoder(rw).Encode(apitype.GetDefaultOrganizationResponse{
						GitHubLogin: tt.serviceOrg,
					})
					require.NoError(t, err)
				default:
					panic(fmt.Sprintf("Path not supported: %v", req.URL.Path))
				}
			}))
			t.Cleanup(server.Close)

			err := pkgWorkspace.StoreAccount(server.URL, pkgWorkspace.Account{
				AccessToken: testJWT,
			}, true)
			require.NoError(t, err)

			if tt.configuredOrg != "" {
				err = pkgWorkspace.SetBackendConfigDefaultOrg(server.URL, tt.configuredOrg)
				require.NoError(t, err)
			}

			b, err := New(ctx, diagtest.LogSink(t), server.URL, nil, false)
			require.NoError(t, err)

			org, err := b.GetDefaultOrg(ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedOrg, org)
			assert.Equal(t, tt.expectedDefaultCalls, defaultOrgCalls)
		})
	}
}

//nolint:paralleltest // mutates global state
func TestDisableIntegrityChecking(t *testing.T) {
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := t.Context()

	_, err := NewLoginManager().Login(ctx, client.PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), client.PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true /*force*/, false /*removeBackups*/)
		require.NoError(t, err)
	}()

	// make up a bad stack
	deployment := apitype.UntypedDeployment{
		Version: 3,
		Deployment: json.RawMessage(`{
			"resources": [
				{
					"urn": "urn:pulumi:stack::proj::type::name1",
					"type": "type",
					"parent": "urn:pulumi:stack::proj::type::name2"
				},
				{
					"urn": "urn:pulumi:stack::proj::type::name2",
					"type": "type"
				}
			]
		}`),
	}

	// Import deployment doesn't verify the deployment
	err = b.ImportDeployment(ctx, s, &deployment)
	require.NoError(t, err)

	backend.DisableIntegrityChecking = false
	snap, err := s.Snapshot(ctx, b64.Base64SecretsProvider)
	require.ErrorContains(t, err,
		"child resource urn:pulumi:stack::proj::type::name1's parent urn:pulumi:stack::proj::type::name2 comes after it")
	assert.Nil(t, snap)

	backend.DisableIntegrityChecking = true
	snap, err = s.Snapshot(ctx, b64.Base64SecretsProvider)
	require.NoError(t, err)
	require.NotNil(t, snap)
}

func TestCloudBackend_GetCloudRegistry(t *testing.T) {
	t.Parallel()
	mockClient := &client.Client{}
	b := &cloudBackend{
		client: mockClient,
		d:      diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
	}

	registry, err := b.GetCloudRegistry()
	require.NoError(t, err)
	require.NotNil(t, registry)

	_, ok := registry.(*cloudRegistry)
	assert.True(t, ok, "expected registry to be a cloudRegistry")
}

// Bit of an integration test.
// That we can render engine events, send them to the backend, and get a summary back.
func TestCopilotExplainer(t *testing.T) {
	t.Parallel()

	copilotResponse, err := json.Marshal(apitype.CopilotResponse{
		ThreadMessages: []apitype.CopilotThreadMessage{
			{
				Role:    "assistant",
				Kind:    "response",
				Content: json.RawMessage(`"Test summary of changes"`),
			},
		},
	})
	require.NoError(t, err)

	// Create a mock transport that
	// 1. captures the request to assert on
	// 2. returns our test response
	var requestBody []byte
	mockTransport := &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			var err error
			requestBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(copilotResponse)),
				Header:     make(http.Header),
			}, nil
		},
	}

	// Create a backend and API client using our mock transport
	apiClient := client.NewClient(client.PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
	apiClient.WithHTTPClient(&http.Client{Transport: mockTransport})
	b := &cloudBackend{
		client: apiClient,
		d:      diagtest.LogSink(t),
	}

	// Call explainer
	stackRef := cloudBackendReference{
		name:    tokens.MustParseStackName("foo"),
		owner:   "test-owner",
		project: "test-project",
	}
	op := backend.UpdateOperation{
		Proj: &workspace.Project{Name: "test-project"},
		Opts: backend.UpdateOptions{
			Display: display.Options{
				Color: colors.Never,
			},
		},
	}
	events := []engine.Event{
		engine.NewEvent(engine.StdoutEventPayload{
			Message: "Hello, world!",
			Color:   colors.Never,
		}),
	}
	summary, err := b.Explain(t.Context(), stackRef, apitype.UpdateUpdate, op, events)

	// Verify results
	require.NoError(t, err)
	assert.Contains(t, summary, "Test summary of changes")
	assert.Contains(t, string(requestBody), "Hello, world!")
}

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req)
}

//nolint:paralleltest // mutates global configuration
func TestListStackNames(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := t.Context()

	_, err := NewLoginManager().Login(ctx, client.PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), client.PulumiCloudURL, &workspace.Project{Name: "testproj-list-stacks"}, false)
	require.NoError(t, err)

	// Create test stacks
	numStacks := 3
	stackNames := make([]string, numStacks)
	stacks := make([]backend.Stack, numStacks)

	for i := 0; i < numStacks; i++ {
		stackName := ptesting.RandomStackName()
		stackNames[i] = stackName
		ref, err := b.ParseStackReference(stackName)
		require.NoError(t, err)

		s, err := b.CreateStack(ctx, ref, "", nil, nil)
		require.NoError(t, err)
		stacks[i] = s
	}

	// Cleanup stacks
	defer func() {
		for _, s := range stacks {
			_, err := b.RemoveStack(ctx, s, true /*force*/, false /*removeBackups*/)
			require.NoError(t, err)
		}
	}()

	// Add a small delay to allow for eventual consistency
	time.Sleep(1 * time.Second)

	// Test ListStackNames with limited pagination to avoid excessive stack accumulation
	projectName := "testproj-list-stacks"
	filter := backend.ListStackNamesFilter{
		Project: &projectName, // Filter to just our test project to reduce scope
	}
	var allStackRefs []backend.StackReference
	var token backend.ContinuationToken
	maxPages := 10 // Increase from 5 to 10 to give more chances to find stacks

	// Fetch limited pages to test pagination functionality
	foundAllTestStacks := false
	for page := 0; page < maxPages; page++ {
		stackRefs, nextToken, err := b.ListStackNames(ctx, filter, token)
		require.NoError(t, err)

		allStackRefs = append(allStackRefs, stackRefs...)

		// Check if we found all our test stacks - compare against both simple and fully qualified names
		foundStacks := make(map[string]bool)
		for _, stackRef := range allStackRefs {
			// Add both the simple name and the fully qualified name
			foundStacks[stackRef.Name().String()] = true
			foundStacks[stackRef.FullyQualifiedName().String()] = true
		}

		foundCount := 0
		for _, expectedName := range stackNames {
			// Check if we can find the stack by either simple name or fully qualified name
			if foundStacks[expectedName] {
				foundCount++
			} else {
				// Also check if the stack reference's simple name matches
				for _, stackRef := range allStackRefs {
					if stackRef.Name().String() == expectedName {
						foundCount++
						break
					}
				}
			}
		}

		if foundCount == numStacks {
			foundAllTestStacks = true
			break
		}

		if nextToken == nil {
			break
		}
		token = nextToken
	}

	// Verify we found at least our test stacks within the limited pages
	assert.True(t, foundAllTestStacks, "Should find all test stacks within first few pages")

	// Add debug information if test fails
	if !foundAllTestStacks {
		t.Logf("Created stacks: %v", stackNames)
		t.Logf("Found %d stacks in total", len(allStackRefs))
		foundStackNames := make([]string, 0, len(allStackRefs))
		for _, stackRef := range allStackRefs {
			foundStackNames = append(foundStackNames, stackRef.Name().String())
		}
		t.Logf("Found stack names: %v", foundStackNames)
	}

	// Verify that ListStackNames returns StackReference objects (not StackSummary)
	assert.IsType(t, []backend.StackReference{}, allStackRefs)

	// Verify basic pagination works (should have at least one page of results)
	assert.Greater(t, len(allStackRefs), 0, "Should return at least some stack references")
}

//nolint:paralleltest // mutates global configuration
func TestListStackNamesVsListStacks(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := t.Context()

	_, err := NewLoginManager().Login(ctx, client.PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), client.PulumiCloudURL, &workspace.Project{Name: "testproj-list-stacks"}, false)
	require.NoError(t, err)

	// Create a test stack
	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true /*force*/, false /*removeBackups*/)
		require.NoError(t, err)
	}()

	// Add a small delay to allow for eventual consistency
	time.Sleep(1 * time.Second)

	// Test both methods with limited pagination to avoid excessive stack accumulation
	projectName := "testproj-list-stacks"
	filter := backend.ListStacksFilter{
		Project: &projectName, // Filter to just our test project to reduce scope
	}
	maxPages := 10

	// Test ListStacks with limited pagination
	var allSummaries []backend.StackSummary
	var token1 backend.ContinuationToken
	foundTestStackInSummaries := false

	for page := 0; page < maxPages; page++ {
		summaries, nextToken, err := b.ListStacks(ctx, filter, token1)
		require.NoError(t, err)

		allSummaries = append(allSummaries, summaries...)

		// Check if we found our test stack - compare against both simple and fully qualified names
		for _, summary := range summaries {
			if summary.Name().Name().String() == stackName ||
				summary.Name().FullyQualifiedName().String() == stackName ||
				summary.Name().String() == stackName {
				foundTestStackInSummaries = true
				break
			}
		}

		if foundTestStackInSummaries || nextToken == nil {
			break
		}
		token1 = nextToken
	}

	// Test ListStackNames with limited pagination
	var allStackRefs []backend.StackReference
	var token2 backend.ContinuationToken
	foundTestStackInRefs := false

	// Convert to ListStackNamesFilter for the ListStackNames call
	namesFilter := backend.ListStackNamesFilter{
		Project:      filter.Project,
		Organization: filter.Organization,
	}

	for page := 0; page < maxPages; page++ {
		stackRefs, nextToken, err := b.ListStackNames(ctx, namesFilter, token2)
		require.NoError(t, err)

		allStackRefs = append(allStackRefs, stackRefs...)

		// Check if we found our test stack - compare against both simple and fully qualified names
		for _, stackRef := range stackRefs {
			if stackRef.Name().String() == stackName ||
				stackRef.FullyQualifiedName().String() == stackName ||
				stackRef.String() == stackName {
				foundTestStackInRefs = true
				break
			}
		}

		if foundTestStackInRefs || nextToken == nil {
			break
		}
		token2 = nextToken
	}

	// Verify both methods found our test stack
	assert.True(t, foundTestStackInSummaries, "Test stack should be found in ListStacks results")
	assert.True(t, foundTestStackInRefs, "Test stack should be found in ListStackNames results")

	// Add debug information if tests fail
	if !foundTestStackInSummaries || !foundTestStackInRefs {
		t.Logf("Created stack: %s", stackName)
		t.Logf("Found %d summaries, %d stack refs", len(allSummaries), len(allStackRefs))

		if !foundTestStackInSummaries && len(allSummaries) > 0 {
			summaryNames := make([]string, 0, len(allSummaries))
			for _, summary := range allSummaries {
				summaryNames = append(summaryNames, summary.Name().Name().String())
			}
			t.Logf("Summary names: %v", summaryNames)
		}

		if !foundTestStackInRefs && len(allStackRefs) > 0 {
			refNames := make([]string, 0, len(allStackRefs))
			for _, stackRef := range allStackRefs {
				refNames = append(refNames, stackRef.Name().String())
			}
			t.Logf("Stack ref names: %v", refNames)
		}
	}

	// Verify both methods return some results
	assert.Greater(t, len(allSummaries), 0, "ListStacks should return at least some results")
	assert.Greater(t, len(allStackRefs), 0, "ListStackNames should return at least some results")

	// Verify that both methods are consistent in their pagination behavior
	// (both should either have more pages or both should be done)
	assert.IsType(t, []backend.StackSummary{}, allSummaries)
	assert.IsType(t, []backend.StackReference{}, allStackRefs)
}

func TestCreateStackDeploymentSchemaVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	var lastRequest *http.Request

	var lastUntypedDeployment *apitype.UntypedDeployment

	handleLastRequest := func() {
		var req apitype.CreateStackRequest
		err := json.NewDecoder(lastRequest.Body).Decode(&req)
		assert.Equal(t, "/api/stacks/owner/project", lastRequest.URL.Path)
		require.NoError(t, err)
		require.NotNil(t, req.State)
		lastUntypedDeployment = req.State
	}

	var v4 bool

	capabilities := func() []apitype.APICapabilityConfig {
		if v4 {
			return []apitype.APICapabilityConfig{{
				Capability:    apitype.DeploymentSchemaVersion,
				Version:       1,
				Configuration: json.RawMessage(`{"version":4}`),
			}}
		}
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/capabilities":
			resp := apitype.CapabilitiesResponse{Capabilities: capabilities()}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		case "/api/user":
			resp := map[string]any{
				"githubLogin":   "test-user",
				"organizations": []map[string]string{},
			}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		case "/api/user/organizations/default":
			resp := apitype.GetDefaultOrganizationResponse{
				GitHubLogin: "owner",
			}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		case "/api/stacks/owner/project":
			lastRequest = req
			rw.WriteHeader(200)
			message := `{}`
			rbytes, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			_, err = rw.Write([]byte(message))
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewBuffer(rbytes))
		default:
			panic(fmt.Sprintf("Path not supported: %v", req.URL.Path))
		}
	}))
	defer server.Client()

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, server.URL, nil, false)
	require.NoError(t, err)

	ref, err := b.ParseStackReference("owner/project/stack")
	require.NoError(t, err)

	// Test 1: v4 not supported: send v3 expect v3.

	_, err = b.CreateStack(ctx, ref, "", &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage("{}"),
	}, nil)
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 3, lastUntypedDeployment.Version)
	assert.Empty(t, lastUntypedDeployment.Features)

	// Test 2: v4 not supported: send v4 expect v3.

	_, err = b.CreateStack(ctx, ref, "", &apitype.UntypedDeployment{
		Version:    4,
		Features:   []string{"refreshBeforeUpdate"},
		Deployment: json.RawMessage("{}"),
	}, nil)
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 3, lastUntypedDeployment.Version)
	assert.Empty(t, lastUntypedDeployment.Features)

	// Test 3: v4 supported: send v3 expect v3.

	v4 = true
	b, err = New(ctx, sink, server.URL, nil, false)
	require.NoError(t, err)

	_, err = b.CreateStack(ctx, ref, "", &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage("{}"),
	}, nil)
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 3, lastUntypedDeployment.Version)
	assert.Empty(t, lastUntypedDeployment.Features)

	// Test 4: v4 supported: send v4 expect v4.

	_, err = b.CreateStack(ctx, ref, "", &apitype.UntypedDeployment{
		Version:    4,
		Features:   []string{"refreshBeforeUpdate"},
		Deployment: json.RawMessage("{}"),
	}, nil)
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 4, lastUntypedDeployment.Version)
	assert.Equal(t, []string{"refreshBeforeUpdate"}, lastUntypedDeployment.Features)
}

// TestCreateStackDisplaysBackendMessages verifies that backend-vended messages on the
// CreateStackResponse (e.g. an expired trial warning) flow through the client into the
// cloudBackend without breaking stack creation, and that displayBackendMessages renders
// them via the diag sink.
func TestCreateStackDisplaysBackendMessages(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const trialWarning = "Your organization's trial has expired. " +
		"Please contact sales@pulumi.com to upgrade."

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/capabilities":
			err := json.NewEncoder(rw).Encode(apitype.CapabilitiesResponse{})
			require.NoError(t, err)
		case "/api/user":
			err := json.NewEncoder(rw).Encode(map[string]any{
				"githubLogin":   "test-user",
				"organizations": []map[string]string{},
			})
			require.NoError(t, err)
		case "/api/user/organizations/default":
			err := json.NewEncoder(rw).Encode(apitype.GetDefaultOrganizationResponse{
				GitHubLogin: "owner",
			})
			require.NoError(t, err)
		case "/api/stacks/owner/project":
			rw.WriteHeader(http.StatusOK)
			err := json.NewEncoder(rw).Encode(apitype.CreateStackResponse{
				Messages: []apitype.Message{{
					Severity: apitype.MessageSeverityWarning,
					Message:  trialWarning,
				}},
			})
			require.NoError(t, err)
		default:
			panic(fmt.Sprintf("Path not supported: %v", req.URL.Path))
		}
	}))
	t.Cleanup(server.Close)

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, server.URL, nil, false)
	require.NoError(t, err)

	ref, err := b.ParseStackReference("owner/project/stack")
	require.NoError(t, err)

	// CreateStack should succeed and the warning message should not interfere with
	// stack creation; it is rendered via cmdutil.Diag() as a side effect.
	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	require.NotNil(t, s)

	// Exercise displayBackendMessages directly with each severity, including an
	// unknown one, so the helper's switch arms are all covered. The expired-trial
	// warning is the primary case driving this test.
	displayBackendMessages([]apitype.Message{
		{Severity: apitype.MessageSeverityWarning, Message: trialWarning},
		{Severity: apitype.MessageSeverityError, Message: "test error"},
		{Severity: apitype.MessageSeverityInfo, Message: "test info"},
		{Severity: "mystery", Message: "unknown severity"},
	})

	// Empty input is a no-op and must not panic.
	displayBackendMessages(nil)
}

func TestImportDeploymentSchemaVersion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	var lastRequest *http.Request

	var lastUntypedDeployment *apitype.UntypedDeployment

	handleLastRequest := func() {
		var req apitype.UntypedDeployment
		err := json.NewDecoder(lastRequest.Body).Decode(&req)
		assert.Equal(t, "/api/stacks/owner/project/stack/import", lastRequest.URL.Path)
		require.NoError(t, err)
		lastUntypedDeployment = &req
	}

	var v4 bool

	capabilities := func() []apitype.APICapabilityConfig {
		if v4 {
			return []apitype.APICapabilityConfig{{
				Capability:    apitype.DeploymentSchemaVersion,
				Version:       1,
				Configuration: json.RawMessage(`{"version":4}`),
			}}
		}
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/api/capabilities":
			resp := apitype.CapabilitiesResponse{Capabilities: capabilities()}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		case "/api/user":
			resp := map[string]any{
				"githubLogin":   "test-user",
				"organizations": []map[string]string{},
			}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		case "/api/user/organizations/default":
			resp := apitype.GetDefaultOrganizationResponse{
				GitHubLogin: "owner",
			}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		case "/api/stacks/owner/project":
			rw.WriteHeader(200)
			_, err := rw.Write([]byte("{}"))
			require.NoError(t, err)
		case "/api/stacks/owner/project/stack/import":
			lastRequest = req
			rw.WriteHeader(200)
			message := `{}`
			reader, err := gzip.NewReader(req.Body)
			require.NoError(t, err)
			defer reader.Close()
			rbytes, err := io.ReadAll(reader)
			require.NoError(t, err)
			_, err = rw.Write([]byte(message))
			require.NoError(t, err)
			req.Body = io.NopCloser(bytes.NewBuffer(rbytes))
		case "/api/stacks/owner/project/stack/update":
			resp := apitype.UpdateResults{
				Status: apitype.StatusSucceeded,
			}
			err := json.NewEncoder(rw).Encode(resp)
			require.NoError(t, err)
		default:
			panic(fmt.Sprintf("Path not supported: %v", req.URL.Path))
		}
	}))
	defer server.Client()

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	b, err := New(ctx, sink, server.URL, nil, false)
	require.NoError(t, err)

	ref, err := b.ParseStackReference("owner/project/stack")
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	// Test 1: v4 not supported: send v3 expect v3.

	err = b.ImportDeployment(ctx, s, &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage("{}"),
	})
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 3, lastUntypedDeployment.Version)
	assert.Empty(t, lastUntypedDeployment.Features)

	// Test 2: v4 not supported: send v4 expect v3.

	err = b.ImportDeployment(ctx, s, &apitype.UntypedDeployment{
		Version:    4,
		Features:   []string{"refreshBeforeUpdate"},
		Deployment: json.RawMessage("{}"),
	})
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 3, lastUntypedDeployment.Version)
	assert.Empty(t, lastUntypedDeployment.Features)

	// Test 3: v4 supported: send v3 expect v3.

	v4 = true
	b, err = New(ctx, sink, server.URL, nil, false)
	require.NoError(t, err)

	err = b.ImportDeployment(ctx, s, &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage("{}"),
	})
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 3, lastUntypedDeployment.Version)
	assert.Empty(t, lastUntypedDeployment.Features)

	// Test 4: v4 supported: send v4 expect v4.

	err = b.ImportDeployment(ctx, s, &apitype.UntypedDeployment{
		Version:    4,
		Features:   []string{"refreshBeforeUpdate"},
		Deployment: json.RawMessage("{}"),
	})
	require.NoError(t, err)

	handleLastRequest()
	assert.Equal(t, 4, lastUntypedDeployment.Version)
	assert.Equal(t, []string{"refreshBeforeUpdate"}, lastUntypedDeployment.Features)
}

func TestIsExplainPreviewEnabled(t *testing.T) {
	t.Parallel()

	enabled := true
	b := &cloudBackend{
		neoEnabledForCurrentProject: &enabled,
		capabilities: promise.Run(func() (apitype.Capabilities, error) {
			return apitype.Capabilities{CopilotExplainPreviewV1: true}, nil
		}),
		d: diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
	}

	result := b.IsExplainPreviewEnabled(t.Context(), display.Options{})
	assert.True(t, result)
}

func TestIsExpectedTokenFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string
		isExpected bool
	}{
		{
			name:       "JWT token",
			token:      testJWT,
			isExpected: true,
		},
		{
			name:       "empty token",
			token:      "",
			isExpected: false,
		},
		{
			name:       "unexpected token",
			token:      "unexpected-token",
			isExpected: false,
		},
		{
			name:       "random string",
			token:      "randomstring123",
			isExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isExpectedTokenFormat(tt.token)
			if tt.isExpected {
				assert.True(t, result)
			} else {
				assert.False(t, result)
			}
		})
	}
}

//nolint:paralleltest // Cannot use t.Parallel() because subtests use t.Setenv
func TestGetTokenValue(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		setupEnv    func(*testing.T)
		setupFile   func(*testing.T) string
		wantValue   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "direct JWT token",
			token:     testJWT,
			wantValue: testJWT,
			wantErr:   false,
		},
		{
			name:  "token from file",
			token: "file://",
			setupFile: func(t *testing.T) string {
				tmpFile, err := os.CreateTemp(t.TempDir(), "token-*.txt")
				require.NoError(t, err)
				t.Cleanup(func() { os.Remove(tmpFile.Name()) })
				_, err = fmt.Fprintf(tmpFile, "  %s  \n", testJWT)
				require.NoError(t, err)
				tmpFile.Close()
				return tmpFile.Name()
			},
			wantValue: testJWT,
			wantErr:   false,
		},
		{
			name:        "token from nonexistent file",
			token:       "file:///nonexistent/path/to/token.txt",
			wantErr:     true,
			errContains: "reading token from file",
		},
		{
			name:  "empty file",
			token: "file://",
			setupFile: func(t *testing.T) string {
				tmpFile, err := os.CreateTemp(t.TempDir(), "token-*.txt")
				require.NoError(t, err)
				t.Cleanup(func() { os.Remove(tmpFile.Name()) })
				tmpFile.Close()
				return tmpFile.Name()
			},
			wantErr:     true,
			errContains: "is empty",
		},
		{
			name:  "file with unexpected token format",
			token: "file://",
			setupFile: func(t *testing.T) string {
				tmpFile, err := os.CreateTemp(t.TempDir(), "token-*.txt")
				require.NoError(t, err)
				t.Cleanup(func() { os.Remove(tmpFile.Name()) })
				_, err = tmpFile.WriteString("unexpected-token-format\n")
				require.NoError(t, err)
				tmpFile.Close()
				return tmpFile.Name()
			},
			wantErr:     true,
			errContains: "token format in file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Cannot use t.Parallel() here because some tests use t.Setenv or create temp files

			token := tt.token
			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}
			if tt.setupFile != nil {
				filePath := tt.setupFile(t)
				token = "file://" + filePath
			}

			value, err := getTokenValue(token)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValue, value)
			}
		})
	}
}

func TestExchangeOidcToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		oidcToken    string
		organization string
		scope        string
		expiration   time.Duration
		setupServer  func() *httptest.Server
		wantErr      bool
		errContains  string
		checkResult  func(*testing.T, string, time.Time)
	}{
		{
			name:         "empty oidc token",
			oidcToken:    "",
			organization: "test-org",
			scope:        "org:test-org",
			expiration:   1 * time.Hour,
			wantErr:      true,
			errContains:  "Unauthorized: No credentials provided or are invalid",
		},
		{
			name:         "invalid oidc token format",
			oidcToken:    "invalid-token-format",
			organization: "test-org",
			scope:        "org:test-org",
			expiration:   1 * time.Hour,
			wantErr:      true,
			errContains:  "Failed to read OIDC token",
		},
		{
			name:         "successful token exchange",
			oidcToken:    testJWT,
			organization: "test-org",
			scope:        "org:test-org",
			expiration:   1 * time.Hour,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/oauth/token" {
						resp := apitype.TokenExchangeGrantResponse{
							AccessToken: "pul-jwt-access-token",
							ExpiresIn:   3600,
							TokenType:   "Bearer",
							Scope:       "org:test-org",
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(resp)
					}
				}))
			},
			wantErr: false,
			checkResult: func(t *testing.T, accessToken string, expiresAt time.Time) {
				assert.Equal(t, "pul-jwt-access-token", accessToken)
				assert.False(t, expiresAt.IsZero())
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

			accessToken, expiresAt, err := exchangeOidcToken(
				t.Context(), diagtest.LogSink(t), cloudURL, false, tt.oidcToken, tt.organization, tt.scope, tt.expiration,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, accessToken, expiresAt)
				}
			}
		})
	}
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
				t.Context(), cloudURL, false, tt.accessToken, "", nil,
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

func TestCreateNeoTaskOnError(t *testing.T) {
	t.Parallel()

	t.Run("empty output returns nil", func(t *testing.T) {
		t.Parallel()

		b := &cloudBackend{
			client: &client.Client{},
			d:      diagtest.LogSink(t),
		}
		stackRef := cloudBackendReference{
			name:    tokens.MustParseStackName("my-stack"),
			owner:   "my-org",
			project: "my-project",
		}

		resp, err := b.createNeoTaskOnError(t.Context(), "", stackRef, display.Options{})
		require.NoError(t, err)
		assert.Nil(t, resp)
	})

	t.Run("bad stack reference returns error", func(t *testing.T) {
		t.Parallel()

		b := &cloudBackend{
			client: &client.Client{},
			d:      diagtest.LogSink(t),
		}
		resp, err := b.createNeoTaskOnError(t.Context(), "some error", nil, display.Options{})
		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		neoResponse, err := json.Marshal(client.NeoTaskResponse{TaskID: "task_abc123"})
		require.NoError(t, err)

		var capturedBody []byte
		mockTransport := &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				capturedBody, err = io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(neoResponse)),
					Header:     make(http.Header),
				}, nil
			},
		}

		apiClient := client.NewClient(client.PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
		apiClient.WithHTTPClient(&http.Client{Transport: mockTransport})
		b := &cloudBackend{
			client: apiClient,
			d:      diagtest.LogSink(t),
		}

		stackRef := cloudBackendReference{
			name:    tokens.MustParseStackName("my-stack"),
			owner:   "my-org",
			project: "my-project",
		}

		resp, err := b.createNeoTaskOnError(t.Context(), "resource failed to create", stackRef, display.Options{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "task_abc123", resp.TaskID)

		assert.Contains(
			t, string(capturedBody),
			"Help me debug the following Pulumi error for project my-project and stack my-stack",
		)
		assert.Contains(t, string(capturedBody), "resource failed to create")
	})

	t.Run("API error is propagated", func(t *testing.T) {
		t.Parallel()

		mockTransport := &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":500,"message":"server error"}`))),
					Header:     make(http.Header),
				}, nil
			},
		}

		apiClient := client.NewClient(client.PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
		apiClient.WithHTTPClient(&http.Client{Transport: mockTransport})
		b := &cloudBackend{
			client: apiClient,
			d:      diagtest.LogSink(t),
		}

		stackRef := cloudBackendReference{
			name:    tokens.MustParseStackName("my-stack"),
			owner:   "my-org",
			project: "my-project",
		}

		resp, err := b.createNeoTaskOnError(t.Context(), "something broke", stackRef, display.Options{})
		require.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestRunEngineActionPropagatesSnapshotJournalerError(t *testing.T) {
	t.Parallel()

	stackName, err := tokens.ParseStackName("stack")
	require.NoError(t, err)
	mgr := failingSecretsManager{err: errors.New("encrypt boom")}
	snap := &deploy.Snapshot{
		Resources: []*pkgresource.State{{
			Type: tokens.Type("test:index:Resource"),
			URN: resource.NewURN(
				stackName.Q(), tokens.PackageName("project"), "",
				tokens.Type("test:index:Resource"), "resource"),
			Outputs: resource.PropertyMap{
				"secret": resource.MakeSecret(resource.NewProperty("value")),
			},
		}},
	}
	fx := newRunEngineActionFixture(t, snap, nil, mgr)

	var runErr error
	require.NotPanics(t, func() {
		_, _, runErr = fx.backend.runEngineAction(
			t.Context(), apitype.UpdateUpdate, fx.stackRef, fx.op, fx.update,
			"lease-token", "", nil, false, 0,
		)
	})
	require.Error(t, runErr)
	require.ErrorContains(t, runErr, "encrypt boom")
}

type failingSecretsManager struct{ err error }

func (m failingSecretsManager) Type() string                { return "failing" }
func (m failingSecretsManager) State() json.RawMessage      { return nil }
func (m failingSecretsManager) Encrypter() config.Encrypter { return m }
func (m failingSecretsManager) Decrypter() config.Decrypter { return config.NopDecrypter }

func (m failingSecretsManager) EncryptValue(context.Context, string) (string, error) {
	return "", m.err
}

func (m failingSecretsManager) BatchEncrypt(context.Context, []string) ([]string, error) {
	return nil, m.err
}

func TestRunEngineActionPropagatesJournalManagerError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		journalVersion int64
		snap           *deploy.Snapshot
		provider       secrets.Provider
		wantWrap       string
	}{
		{
			name:           "journalVersion=1",
			journalVersion: 1,
			snap:           &deploy.Snapshot{},
			provider:       nil,
			// Wrap from cloudJournaler.AddJournalEntry.
			wantWrap: "serializing journal entry",
		},
		{
			name:           "journalVersion=0",
			journalVersion: 0,
			snap:           &deploy.Snapshot{SecretsManager: b64.NewBase64SecretsManager()},
			provider: (&secrets.MockProvider{}).Add(b64.Type, func(json.RawMessage) (secrets.Manager, error) {
				return b64.NewBase64SecretsManager(), nil
			}),
			wantWrap: "failed to serialize journal entry",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mgr := failingBatchingSecretsManager{err: errors.New("batch boom")}
			fx := newRunEngineActionFixture(t, tc.snap, tc.provider, mgr)

			var runErr error
			require.NotPanics(t, func() {
				_, _, runErr = fx.backend.runEngineAction(
					t.Context(), apitype.UpdateUpdate, fx.stackRef, fx.op, fx.update,
					"lease-token", "", nil, false, tc.journalVersion,
				)
			})
			require.Error(t, runErr)
			require.ErrorContains(t, runErr, "batch boom")
			require.ErrorContains(t, runErr, tc.wantWrap)
		})
	}
}

type runEngineActionFixture struct {
	backend  *cloudBackend
	stackRef cloudBackendReference
	op       backend.UpdateOperation
	update   client.UpdateIdentifier
}

func newRunEngineActionFixture(
	t *testing.T,
	snap *deploy.Snapshot,
	secretsProvider secrets.Provider,
	opSecretsManager secrets.Manager,
) runEngineActionFixture {
	t.Helper()

	stackName, err := tokens.ParseStackName("stack")
	require.NoError(t, err)

	deployment, err := stack.SerializeUntypedDeployment(t.Context(), snap, &stack.SerializeOptions{ShowSecrets: true})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/stacks/owner/project/stack/export":
			require.NoError(t, json.NewEncoder(w).Encode(apitype.ExportStackResponse(*deployment)))
		case "/api/stacks/owner/project/stack":
			require.NoError(t, json.NewEncoder(w).Encode(apitype.Stack{
				OrgName: "owner", ProjectName: "project", StackName: stackName.Q(),
			}))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	apiClient := client.NewClient(server.URL, "token", false, sink).WithHTTPClient(server.Client())
	b := &cloudBackend{
		d:      sink,
		url:    server.URL,
		client: apiClient,
		capabilities: promise.Run(func() (apitype.Capabilities, error) {
			return apitype.Capabilities{}, nil
		}),
	}
	stackRef := cloudBackendReference{
		name:    stackName,
		project: tokens.Name("project"),
		owner:   "owner",
		b:       b,
	}

	op := backend.UpdateOperation{
		Proj: &workspace.Project{
			Name:    tokens.PackageName("project"),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		},
		Opts: backend.UpdateOptions{Display: display.Options{
			Color:            colors.Never,
			Stdout:           io.Discard,
			Stderr:           io.Discard,
			SuppressProgress: true,
		}},
		SecretsManager:  opSecretsManager,
		SecretsProvider: secretsProvider,
		StackConfiguration: backend.StackConfiguration{
			Config:    config.Map{},
			Decrypter: opSecretsManager.Decrypter(),
		},
	}
	update := client.UpdateIdentifier{
		StackIdentifier: client.StackIdentifier{Owner: "owner", Project: "project", Stack: stackName},
		UpdateID:        "update-id",
	}

	return runEngineActionFixture{backend: b, stackRef: stackRef, op: op, update: update}
}

type failingBatchingSecretsManager struct{ err error }

func (m failingBatchingSecretsManager) Type() string                { return "failing-batch" }
func (m failingBatchingSecretsManager) State() json.RawMessage      { return nil }
func (m failingBatchingSecretsManager) Encrypter() config.Encrypter { return config.NopEncrypter }
func (m failingBatchingSecretsManager) Decrypter() config.Decrypter { return config.NopDecrypter }

func (m failingBatchingSecretsManager) BeginBatchEncryption() (stack.BatchEncrypter, stack.CompleteCrypterBatch) {
	return nopBatchEncrypter{}, func(context.Context) error { return m.err }
}

func (m failingBatchingSecretsManager) BeginBatchDecryption() (stack.BatchDecrypter, stack.CompleteCrypterBatch) {
	return nopBatchDecrypter{}, func(context.Context) error { return nil }
}

type nopBatchEncrypter struct{}

func (nopBatchEncrypter) EncryptValue(context.Context, string) (string, error) {
	return "", nil
}

func (nopBatchEncrypter) BatchEncrypt(context.Context, []string) ([]string, error) {
	return nil, nil
}

func (nopBatchEncrypter) Enqueue(context.Context, *resource.Secret, string, *apitype.SecretV1) error {
	return nil
}

type nopBatchDecrypter struct{}

func (nopBatchDecrypter) DecryptValue(context.Context, string) (string, error) {
	return "", nil
}

func (nopBatchDecrypter) BatchDecrypt(context.Context, []string) ([]string, error) {
	return nil, nil
}

func (nopBatchDecrypter) Enqueue(context.Context, string, *resource.Secret) error {
	return nil
}
