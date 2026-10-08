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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentRefreshesAccessTokenOn401WhenRefreshTokenStored(t *testing.T) {
	ptesting.IsolateCredentials(t)

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

	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.Account.AccessToken,
		"the stored access token should be refreshed before reporting the account as valid")
	assert.Equal(t, "stored-refresh-token", account.Account.RefreshToken,
		"the refresh token is preserved (Phase 1: server doesn't rotate)")
	assert.Equal(t, "alice", account.Account.Username)

	saved, err := workspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken,
		"credentials.json should reflect the refreshed access token")
	assert.Equal(t, "stored-refresh-token", saved.RefreshToken)
	assert.WithinDuration(t, time.Now(), saved.LastValidatedAt, time.Minute,
		"validateStoredAccount must stamp LastValidatedAt when it actually validates")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentRefreshesFromRefreshOnlyStoredAccount(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// HasCredential opens the gate for accounts with a refresh token but no access token. The
	// wrapper mints the first access token on the initial 401 from an empty bearer.

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

	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		RefreshToken: "only-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "minted-access-token", account.Account.AccessToken)
	assert.Equal(t, "bob", account.Account.Username)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentPersistsRotatedRefreshToken(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// True rotation: the refresh-token grant returns a refresh token DIFFERENT from the one we
	// sent. The wrapper updates the in-memory account and the writeback persists the rotated
	// value to credentials.json. Server-side rotation is a Phase 2 behavior — this test pins the
	// CLI side so we don't need a change when it lands.

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

	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.Account.AccessToken)
	assert.Equal(t, "rotated-refresh-token", account.Account.RefreshToken)

	saved, err := workspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken)
	assert.Equal(t, "rotated-refresh-token", saved.RefreshToken,
		"credentials.json must reflect the rotated refresh token")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentPreservesRefreshTokenWhenGrantResponseOmitsIt(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// RFC 6749 §6: omitted (or empty) refresh_token in the grant response means "keep using
	// yours" — the server is not signalling termination. credentials.json must hold onto the
	// existing refresh token so the next 401 can refresh again.

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

	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.Account.AccessToken)
	assert.Equal(t, "stored-refresh-token", account.Account.RefreshToken,
		"omitted refresh_token in the response must not destroy the existing one")

	saved, err := workspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", saved.AccessToken)
	assert.Equal(t, "stored-refresh-token", saved.RefreshToken,
		"credentials.json must hold onto the existing refresh token")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentRefreshesLocallyExpiredAccessTokenWhenRefreshTokenStored(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// Cold-start with a locally-expired access token: validateStoredAccount must take the refresh
	// path instead of hard-failing, so the next call silently mints a fresh access token and
	// credentials.json is updated in place.

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
	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken:  "stale-access-token",
		RefreshToken: "stored-refresh-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.Account.AccessToken,
		"a locally-expired access token must trigger a refresh instead of failing the validate step")
	assert.Equal(t, "stored-refresh-token", account.Account.RefreshToken)
	assert.Equal(t, "alice", account.Account.Username)

	saved, err := workspace.GetAccount(server.URL)
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentPreservesExpiresAtWhenServerAcceptsLocallyExpiredAccessToken(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// Cold-start with a locally-expired access token whose server-side TTL is actually still
	// valid: validateStoredAccount enters the refresh-or-fetch branch and /api/user succeeds
	// without firing a refresh. /api/user never returns ExpiresAt, so the merge must keep the
	// existing (now-past) ExpiresAt instead of nullifying TokenInformation entirely — otherwise
	// every subsequent run forfeits the cold-start refresh path and the agent-auth banner
	// mis-reports the account as unable to authenticate.

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
	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken:  "live-access-token",
		RefreshToken: "stored-refresh-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	account, err := NewLoginManager().Current(t.Context(), server.URL, false, true)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "live-access-token", account.Account.AccessToken, "no refresh, no rotation")
	assert.Equal(t, "alice", account.Account.Username)
	require.NotNil(t, account.Account.TokenInformation,
		"TokenInformation must survive a fetch that returns no token info of its own")
	require.NotNil(t, account.Account.TokenInformation.ExpiresAt,
		"ExpiresAt must survive the merge so the banner and cold-start path keep working")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestCurrentReturnsNoAccountWhenAccessTokenLocallyExpiredAndNoRefreshToken(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// Cold-start with a locally-expired access token but no refresh token must short-circuit
	// before hitting the network — preserves the pre-refresh-token behavior for accounts that
	// were stored without one.

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		hits++
		t.Errorf("no network call should be made when the access token is locally expired "+
			"and no refresh token is stored: %s", req.URL.Path)
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
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

func TestGetAccountDetailsInstallsRefreshWrapperWhenRefreshTokenSupplied(t *testing.T) {
	t.Parallel()

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
	username, _, _, err := getAccountDetails(t.Context(), nil, server.URL, false,
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
