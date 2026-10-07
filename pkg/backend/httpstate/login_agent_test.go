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

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // isolates credentials with t.Setenv
func TestCurrentInvalidAgentCredentialsWithActiveClaimDoesNotSignup(t *testing.T) {
	ptesting.IsolateCredentials(t)

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/agents/signup" {
			signupCalls++
		}
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiredAt := time.Now().Add(-time.Hour)
	err := workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken: "expired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(time.Hour),
		CloudURL:   server.URL,
	})
	require.NoError(t, err)

	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(t.Context(), server.URL, false, true, "codex", nil)
	require.ErrorIs(t, err, ErrUnauthorized)
	assert.Nil(t, account)
	assert.Equal(t, 0, signupCalls)
}

//nolint:paralleltest // isolates credentials with t.Setenv
func TestCurrentRejectedAgentCredentialsWithUnexpiredTokenDoesNotSignup(t *testing.T) {
	ptesting.IsolateCredentials(t)

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
	err := workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken: "locally-unexpired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(-time.Hour),
		CloudURL:   server.URL,
	})
	require.NoError(t, err)

	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(t.Context(), server.URL, false, true, "codex", nil)
	require.ErrorIs(t, err, ErrUnauthorized)
	require.ErrorIs(t, err, backenderr.LoginRequiredError{})
	assert.ErrorContains(t, err, "ask the user to run `pulumi login`")
	assert.Nil(t, account)
	assert.Equal(t, 0, signupCalls)
}

//nolint:paralleltest // isolates credentials with t.Setenv
func TestCurrentValidAgentCredentialsWithExpiredClaimDoesNotSignup(t *testing.T) {
	ptesting.IsolateCredentials(t)

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/agents/signup" {
			signupCalls++
		}
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiresAt := time.Now().Add(time.Hour)
	err := workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken:     "valid-agent-token",
		Username:        "agent-user",
		Organizations:   []string{"agent-org"},
		LastValidatedAt: time.Now(),
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(-time.Hour),
		CloudURL:   server.URL,
	})
	require.NoError(t, err)

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "valid-agent-token", account.AccessToken)
	assert.True(t, AgentCredentialsUsed(ctx, server.URL))
	assert.Equal(t, 0, signupCalls)

	// Post-validate persistence goes through agentAccount.Save, which writes to the agent file
	// (the account's source) rather than leaking into default credentials.
	fromAgent, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "valid-agent-token", fromAgent.AccessToken)
	fromDefault, err := workspace.GetAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, fromDefault.AccessToken, "agent-sourced account must not be copied into default credentials")
}

func TestCurrentSignupAgentAccountStoresClaimTokenURL(t *testing.T) {
	ptesting.IsolateCredentials(t)
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
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-token", account.AccessToken)
	assert.True(t, AgentCredentialsUsed(ctx, server.URL))
	require.NotNil(t, account.TokenInformation)
	require.NotNil(t, account.TokenInformation.ExpiresAt)
	assert.True(t, account.TokenInformation.ExpiresAt.Equal(accessTokenValidUntil))
	assert.Equal(t, []string{http.MethodGet, http.MethodPost}, signupMethods)

	claim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	assert.Equal(t, "http://app.example.com/claim/claim-token", claim.ClaimURL)
	assert.Equal(t, "claim-token", claim.ClaimToken)
	assert.True(t, claim.ValidUntil.Equal(claimTokenValidUntil))
	assert.Equal(t, server.URL, claim.CloudURL)
}

func TestCurrentSignupAgentAccountStoresRefreshToken(t *testing.T) {
	// The refresh token returned by agent signup must land in the stored Account so the
	// auto-refresh wrapper can use it once the access token expires.
	ptesting.IsolateCredentials(t)
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
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-access-token", account.AccessToken)
	assert.Equal(t, "agent-refresh-token", account.RefreshToken,
		"signup-returned refresh token must be plumbed into the returned Account")

	stored, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "agent-refresh-token", stored.RefreshToken,
		"signup-returned refresh token must be persisted to the agent credentials file")
}

func TestCurrentSignupAgentAccountWithoutRefreshTokenLeavesAccountEmpty(t *testing.T) {
	// Back-compat with a server that doesn't (yet) issue refresh tokens at signup: the response
	// omits refreshToken and the CLI must not error or invent a value.
	ptesting.IsolateCredentials(t)
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
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "agent-access-token", account.AccessToken)
	assert.Empty(t, account.RefreshToken, "no refreshToken in response → none on the Account")

	stored, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Empty(t, stored.RefreshToken, "no refreshToken in response → none persisted")
}

func TestCurrentSignupAgentAccountReplacesExistingRefreshTokenOnResignup(t *testing.T) {
	// When existing agent creds are no longer valid AND the stored refresh token is rejected by
	// the server, the CLI falls through to re-signup. The refresh token returned by the new
	// signup replaces the stale one — the prior value must not survive into the rebuilt Account.
	ptesting.IsolateCredentials(t)
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
	require.NoError(t, workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "new-access-token", account.AccessToken)
	assert.Equal(t, "new-refresh-token", account.RefreshToken)

	stored, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "new-refresh-token", stored.RefreshToken,
		"re-signup must replace the stale refresh token, not preserve it")
}

func TestCurrentAgentAccountRefreshesLocallyExpiredAccessTokenInsteadOfResigning(t *testing.T) {
	// Cold-start in agent mode with a locally-expired access token but a valid refresh token:
	// validateStoredAccount must refresh through /api/oauth/token instead of falling through to
	// re-signup. Re-signup would burn a fresh agent identity and lose the claim association.
	ptesting.IsolateCredentials(t)
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
	require.NoError(t, workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken:  "old-access-token",
		RefreshToken: "stored-refresh-token",
		Username:     "agent-user",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiredAt,
		},
	}, true))

	ctx := ContextWithAgentCredentialUse(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "fresh-access-token", account.AccessToken,
		"locally-expired agent access token must be refreshed in place, not resigned")
	assert.Equal(t, "stored-refresh-token", account.RefreshToken)
	assert.Equal(t, "agent-user", account.Username, "username should survive the refresh path")
	assert.Equal(t, 0, signupCalls, "signup must not be called when refresh succeeds")

	stored, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", stored.AccessToken,
		"agent credentials file should reflect the refreshed access token")
	assert.Equal(t, "stored-refresh-token", stored.RefreshToken)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
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
			ptesting.IsolateCredentials(t)
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

			account, err := defaultLoginManager{}.currentOrSignupAgentAccount(t.Context(), server.URL, false, true, "codex", nil)
			require.ErrorContains(t, err, tt.wantErr)
			assert.Nil(t, account)
		})
	}
}

func TestLoginUsesAgentSignupInNonInteractiveAgentMode(t *testing.T) {
	ptesting.IsolateCredentials(t)

	disableInteractive := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = true
	t.Cleanup(func() {
		cmdutil.DisableInteractive = disableInteractive
	})

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

func TestLoginWithoutAgentSignupDoesNotCreateAgentAccount(t *testing.T) {
	ptesting.IsolateCredentials(t)

	disableInteractive := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = true
	t.Cleanup(func() {
		cmdutil.DisableInteractive = disableInteractive
	})

	t.Setenv("CODEX_SANDBOX", "1")

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/agents/signup" {
			signupCalls++
		}
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	ctx := ContextWithoutAgentSignup(t.Context())
	account, err := NewLoginManager().Login(ctx, server.URL, false, "pulumi", "Pulumi Cloud", nil, true,
		display.Options{})
	require.ErrorAs(t, err, &backenderr.MissingEnvVarForNonInteractiveError{})
	assert.Nil(t, account)
	assert.Equal(t, 0, signupCalls)

	fromAgent, err := workspace.GetAgentAccount(server.URL)
	require.NoError(t, err)
	assert.False(t, fromAgent.HasCredential())
}

//nolint:paralleltest // isolates credentials with t.Setenv
func TestCurrentWithoutAgentSignupReusesExistingAgentCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)

	signupCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/agents/signup" {
			signupCalls++
		}
		rw.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	expiresAt := time.Now().Add(time.Hour)
	err := workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken:     "valid-agent-token",
		Username:        "agent-user",
		LastValidatedAt: time.Now(),
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)

	ctx := ContextWithoutAgentSignup(t.Context())
	account, err := defaultLoginManager{}.currentOrSignupAgentAccount(ctx, server.URL, false, true, "codex", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, "valid-agent-token", account.AccessToken)
	assert.Equal(t, 0, signupCalls)
}
