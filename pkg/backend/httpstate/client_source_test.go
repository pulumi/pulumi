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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientSourceKeepsRefreshedAccount(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/oauth/token":
			_ = json.NewEncoder(w).Encode(apitype.TokenExchangeGrantResponse{
				AccessToken: "fresh", RefreshToken: "rotated", TokenType: "Bearer", ExpiresIn: 3600,
			})
		case "/api/user":
			if r.Header.Get("Authorization") != "token fresh" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"githubLogin":"alice"}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	require.NoError(t, workspace.StoreAccount(server.URL, workspace.Account{
		AccessToken: "stale", RefreshToken: "refresh",
	}, false))
	source := NewClientSource(credentialhelper.NewSession(), nil)
	c, err := source.ClientForURL(t.Context(), server.URL, false)
	require.NoError(t, err)
	_, _, _, err = c.GetPulumiAccountDetails(t.Context())
	require.NoError(t, err)
	resolved, err := source.resolve(t.Context(), server.URL, false)
	require.NoError(t, err)
	assert.Same(t, c, resolved.client)
	assert.Equal(t, "fresh", resolved.account.AccessToken)
	assert.Equal(t, "rotated", resolved.account.RefreshToken)
}

func TestClientSourceCachesAuthenticatedClients(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_HELPER", "none")
	const backendURL = "https://api.example.com"
	source := NewClientSource(credentialhelper.NewSession(), nil)
	_, err := source.ClientForURL(t.Context(), "", false)
	require.ErrorContains(t, err, "requires a backend URL")
	_, err = source.ClientForURL(t.Context(), backendURL, false)
	require.ErrorContains(t, err, "could not find credentials")
	require.NoError(t, workspace.StoreAccount(backendURL, workspace.Account{AccessToken: "token"}, false))
	first, err := source.ClientForURL(t.Context(), backendURL, false)
	require.NoError(t, err)
	again, err := source.ClientForURL(t.Context(), backendURL+"/", false)
	require.NoError(t, err)
	assert.Same(t, first, again)
	insecure, err := source.ClientForURL(t.Context(), backendURL, true)
	require.NoError(t, err)
	assert.NotSame(t, first, insecure)
	assert.True(t, insecure.Insecure())
}

func TestClientSourceFallsBackToAgentCredentials(t *testing.T) {
	isolateAgentCredentials(t)

	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, "")
	t.Setenv(env.Home.Var().Name(), "")

	cloudURL := "https://api.service-secrets-agent.example.com"
	err := workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	source := NewClientSource(credentialhelper.NewSession(), nil)
	resolved, err := source.resolve(t.Context(), cloudURL, false)
	account := resolved.account
	require.NoError(t, err)
	assert.Equal(t, "agent-token", account.AccessToken)
	ctx := ContextWithAgentCredentialUse(t.Context())
	again, err := source.ClientForURL(ctx, cloudURL, false)
	require.NoError(t, err)
	assert.Same(t, resolved.client, again)
	assert.True(t, AgentCredentialsUsed(ctx, cloudURL))
}

func TestClientSourceDoesNotFallbackWithExplicitPath(t *testing.T) {
	isolateAgentCredentials(t)

	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv(env.Home.Var().Name(), "")

	cloudURL := "https://api.service-secrets-explicit.example.com"
	err := workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	resolved, err := NewClientSource(credentialhelper.NewSession(), nil).resolve(t.Context(), cloudURL, false)
	account := resolved.account
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
}

func isolateAgentCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
}
