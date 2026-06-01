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

package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsPrintsAgentAuthRequiredInstruction(t *testing.T) {
	oldAgentCreds, err := workspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, workspace.DeleteAgentCredentials())
		require.NoError(t, workspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, workspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	t.Setenv("CODEX_SANDBOX", "1")
	cloudURL := "https://api.example.com"
	expiresAt := time.Now().Add(2*24*time.Hour + 3*time.Hour + 9*time.Minute)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "locally-unexpired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(24 * time.Hour),
		CloudURL:   cloudURL,
	})
	require.NoError(t, err)

	var output bytes.Buffer
	err = processCmdErrors(t.Context(), backenderr.LoginRequiredError{}, &output)

	assert.True(t, result.IsBail(err))
	assert.Contains(t, output.String(), "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, output.String(), "ACTION_REQUIRED=Tell the user to run pulumi login.")
	expectedInstruction := "INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate"
	assert.Contains(t, output.String(), expectedInstruction)
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsKeepsGenericLoginRequiredOutsideAgentMode(t *testing.T) {
	t.Setenv("CODEX_SANDBOX", "")
	t.Setenv("AI_AGENT", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CURSOR_TRACE_ID", "")
	t.Setenv("CURSOR_AGENT", "")
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CLAUDE_CODE", "")

	err := backenderr.LoginRequiredError{}
	assert.Equal(t, err, processCmdErrors(t.Context(), err, io.Discard))
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsPrintsAgentClaimWarningForNonLoginError(t *testing.T) {
	oldAgentCreds, err := workspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, workspace.DeleteAgentCredentials())
		require.NoError(t, workspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, workspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	t.Setenv("CODEX_SANDBOX", "1")
	cloudURL := "https://api.non-login-error.example.com"
	expiresAt := time.Now().Add(time.Hour)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/non-login-error",
		CloudURL:   cloudURL,
		ValidUntil: time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	ctx := httpstate.ContextWithAgentCredentialUse(t.Context())
	httpstate.MarkAgentCredentialsUsed(ctx, cloudURL)

	inputErr := errors.New("something failed")
	var output bytes.Buffer
	err = processCmdErrors(ctx, inputErr, &output)

	assert.Same(t, inputErr, err)
	assert.Contains(t, output.String(), "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, output.String(), "CLAIM_URL=https://app.pulumi.com/claim/non-login-error")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsDoesNotPrintClaimURLForUnauthorizedClaimedAccount(t *testing.T) {
	oldAgentCreds, err := workspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, workspace.DeleteAgentCredentials())
		require.NoError(t, workspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, workspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/api/agents/signup/validate/claimed-token", req.URL.Path)
		rw.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	t.Setenv("CODEX_SANDBOX", "1")
	expiresAt := time.Now().Add(-time.Minute)
	err = workspace.StoreAgentAccount(server.URL, workspace.Account{
		AccessToken: "agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/claimed-token",
		ClaimToken: "claimed-token",
		CloudURL:   server.URL,
		ValidUntil: time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	ctx := httpstate.ContextWithAgentCredentialUse(t.Context())
	httpstate.MarkAgentCredentialsUsed(ctx, server.URL)

	var output bytes.Buffer
	err = processCmdErrors(ctx, httpstate.ErrUnauthorized, &output)

	assert.True(t, result.IsBail(err))
	assert.Contains(t, output.String(), "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.NotContains(t, output.String(), "CLAIM_URL=")
	assert.Contains(t, output.String(), "claim URL is no longer claimable")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsPrintsAgentAuthRequiredInstructionForESCAPIUnauthorized(t *testing.T) {
	oldAgentCreds, err := workspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, workspace.DeleteAgentCredentials())
		require.NoError(t, workspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, workspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	t.Setenv("CODEX_SANDBOX", "1")
	cloudURL := "https://api.example.com"
	expiresAt := time.Now().Add(2 * 24 * time.Hour)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "locally-unexpired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/api-401",
		ValidUntil: time.Now().Add(24 * time.Hour),
		CloudURL:   cloudURL,
	})
	require.NoError(t, err)

	var output bytes.Buffer
	err = processCmdErrors(t.Context(), MarkESCError(&apitype.ErrorResponse{
		Code:    http.StatusUnauthorized,
		Message: "No credentials provided or are invalid.",
	}), &output)

	assert.True(t, result.IsBail(err))
	assert.Contains(t, output.String(), "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, output.String(), "ACTION_REQUIRED=Tell the user to run pulumi login.")
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestProcessCmdErrorsDoesNotPrintAgentAuthRequiredInstructionForNonESCAPIUnauthorized(t *testing.T) {
	oldAgentCreds, err := workspace.GetAgentStoredCredentials()
	require.NoError(t, err)
	oldAgentClaim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, workspace.DeleteAgentCredentials())
		require.NoError(t, workspace.StoreAgentCredentials(oldAgentCreds))
		if oldAgentClaim.ClaimURL != "" {
			require.NoError(t, workspace.StoreAgentClaim(oldAgentClaim))
		}
	})

	t.Setenv("CODEX_SANDBOX", "1")
	cloudURL := "https://api.example.com"
	expiresAt := time.Now().Add(2 * 24 * time.Hour)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "locally-unexpired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/api-401",
		ValidUntil: time.Now().Add(24 * time.Hour),
		CloudURL:   cloudURL,
	})
	require.NoError(t, err)

	inputErr := &apitype.ErrorResponse{
		Code:    http.StatusUnauthorized,
		Message: "No credentials provided or are invalid.",
	}
	var output bytes.Buffer
	err = processCmdErrors(t.Context(), inputErr, &output)

	assert.Same(t, inputErr, err)
	assert.NotContains(t, output.String(), "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
}
