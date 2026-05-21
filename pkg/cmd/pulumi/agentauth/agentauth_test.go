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

package agentauth

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates shared temporary agent credentials
func TestMaybePrintClaimWarningRequiresAgentCredentialsUsed(t *testing.T) {
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
	cloudURL := "https://api.unused-agent-creds.example.com"
	expiresAt := time.Now().Add(time.Hour)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/unused",
		CloudURL:   cloudURL,
		ValidUntil: time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)

	var output bytes.Buffer
	MaybePrintClaimWarning(&output)
	assert.Empty(t, output.String())
}

//nolint:paralleltest // mutates shared temporary agent credentials
func TestMaybePrintClaimWarningPrintsForUsedAgentCredentials(t *testing.T) {
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
	cloudURL := "https://api.used-agent-creds.example.com"
	expiresAt := time.Now().Add(time.Hour)
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/used",
		CloudURL:   cloudURL,
		ValidUntil: time.Now().Add(24 * time.Hour),
	})
	require.NoError(t, err)
	httpstate.MarkAgentCredentialsUsed(cloudURL)

	var output bytes.Buffer
	MaybePrintClaimWarning(&output)
	assert.Contains(t, output.String(), "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, output.String(), "CLAIM_URL=https://app.pulumi.com/claim/used")
}

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestAuthRequiredMessagePrintsClaimInstructionWhenTokenExpiredButClaimValid(t *testing.T) {
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
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Minute)
	cloudURL := "https://api.expired-token-valid-claim.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "expired-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/expired-token-valid-claim",
		ClaimToken: "expired-token-valid-claim",
		CloudURL:   cloudURL,
		ValidUntil: now.Add(time.Hour),
	})
	require.NoError(t, err)

	setValidateAgentClaim(t, func(ctx context.Context, cloudURL, claimToken string) (bool, error) {
		assert.Equal(t, "https://api.expired-token-valid-claim.example.com", cloudURL)
		assert.Equal(t, "expired-token-valid-claim", claimToken)
		return true, nil
	})

	message := AuthRequiredMessage(now)
	assert.Contains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, message, "CLAIM_URL=https://app.pulumi.com/claim/expired-token-valid-claim")
	assert.Contains(t, message, "CLAIM_URL_VALID_FOR=1h")
	assert.NotContains(t, message, "ACTION_REQUIRED=Tell the user to run pulumi login.")
}

//nolint:paralleltest // mutates env vars, shared temporary agent credentials, and package global
func TestAuthRequiredMessageChecksClaimWhenTokenLocallyValidButRejected(t *testing.T) {
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
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	cloudURL := "https://api.locally-valid-token.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "locally-valid-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/locally-valid-token",
		ClaimToken: "locally-valid-token",
		CloudURL:   cloudURL,
		ValidUntil: now.Add(time.Hour),
	})
	require.NoError(t, err)
	setValidateAgentClaim(t, func(ctx context.Context, gotCloudURL, claimToken string) (bool, error) {
		assert.Equal(t, cloudURL, gotCloudURL)
		assert.Equal(t, "locally-valid-token", claimToken)
		return true, nil
	})

	message := AuthRequiredMessage(now)
	assert.Contains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, message, "CLAIM_URL=https://app.pulumi.com/claim/locally-valid-token")
	assert.Contains(t, message, "EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=1h")
	assert.NotContains(t, message, "ACTION_REQUIRED=Tell the user to run pulumi login.")
}

//nolint:paralleltest // mutates env vars, shared temporary agent credentials, and package global
func TestAuthRequiredMessageOmitsClaimURLWhenClaimIsNotClaimable(t *testing.T) {
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
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Minute)
	cloudURL := "https://api.claimed-token.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "claimed-agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/claimed-token",
		ClaimToken: "claimed-token",
		CloudURL:   cloudURL,
		ValidUntil: now.Add(time.Hour),
	})
	require.NoError(t, err)
	setValidateAgentClaim(t, func(ctx context.Context, gotCloudURL, claimToken string) (bool, error) {
		assert.Equal(t, cloudURL, gotCloudURL)
		assert.Equal(t, "claimed-token", claimToken)
		return false, nil
	})

	message := AuthRequiredMessage(now)
	assert.Contains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, message, "ACTION_REQUIRED=Tell the user to run pulumi login.")
	assert.NotContains(t, message, "CLAIM_URL=")
	assert.Contains(t, message, "claim URL is no longer claimable")
	claim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	require.NotNil(t, claim.ClaimUnavailableAt)
	assert.True(t, claim.ClaimUnavailableAt.Equal(now))
}

//nolint:paralleltest // mutates env vars, shared temporary agent credentials, and package global
func TestAuthRequiredMessageSkipsValidationWhenClaimMarkedUnavailable(t *testing.T) {
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
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	unavailableAt := now.Add(-time.Minute)
	expiresAt := now.Add(time.Hour)
	cloudURL := "https://api.cached-unavailable-claim.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:           "https://app.pulumi.com/claim/cached-unavailable-claim",
		ClaimToken:         "cached-unavailable-claim",
		CloudURL:           cloudURL,
		ValidUntil:         now.Add(time.Hour),
		ClaimUnavailableAt: &unavailableAt,
	})
	require.NoError(t, err)
	setValidateAgentClaim(t, func(ctx context.Context, gotCloudURL, claimToken string) (bool, error) {
		t.Fatal("validateAgentClaim should not be called for a cached unavailable claim")
		return false, nil
	})

	message := AuthRequiredMessage(now)
	assert.Contains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.NotContains(t, message, "CLAIM_URL=")
	assert.Contains(t, message, "claim URL is no longer claimable")
}

//nolint:paralleltest // mutates env vars, shared temporary agent credentials, and package global
func TestAuthRequiredMessageFallsBackToLocalClaimWhenValidationFails(t *testing.T) {
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
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Minute)
	cloudURL := "https://api.validation-error.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{
		AccessToken: "agent-token",
		TokenInformation: &workspace.TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/validation-error",
		ClaimToken: "validation-error",
		CloudURL:   cloudURL,
		ValidUntil: now.Add(time.Hour),
	})
	require.NoError(t, err)
	setValidateAgentClaim(t, func(ctx context.Context, gotCloudURL, claimToken string) (bool, error) {
		assert.Equal(t, cloudURL, gotCloudURL)
		assert.Equal(t, "validation-error", claimToken)
		return false, errors.New("temporary validation failure")
	})

	message := AuthRequiredMessage(now)
	assert.Contains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, message, "CLAIM_URL=https://app.pulumi.com/claim/validation-error")
	assert.Contains(t, message, "CLAIM_URL_VALID_FOR=1h")
}

func setValidateAgentClaim(
	t *testing.T,
	f func(context.Context, string, string) (bool, error),
) {
	t.Helper()
	old := validateAgentClaim
	validateAgentClaim = f
	t.Cleanup(func() {
		validateAgentClaim = old
	})
}
