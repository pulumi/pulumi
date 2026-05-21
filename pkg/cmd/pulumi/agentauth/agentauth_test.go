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
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates shared temporary agent credentials and os.Stderr
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

	output := captureStderr(t, MaybePrintClaimWarning)
	assert.Empty(t, output)
}

//nolint:paralleltest // mutates shared temporary agent credentials and os.Stderr
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

	output := captureStderr(t, MaybePrintClaimWarning)
	assert.Contains(t, output, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, output, "CLAIM_URL=https://app.pulumi.com/claim/used")
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
		CloudURL:   cloudURL,
		ValidUntil: now.Add(time.Hour),
	})
	require.NoError(t, err)

	message := AuthRequiredMessage(now)
	assert.Contains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT")
	assert.Contains(t, message, "CLAIM_URL=https://app.pulumi.com/claim/expired-token-valid-claim")
	assert.Contains(t, message, "CLAIM_URL_VALID_FOR=1h")
	assert.NotContains(t, message, "PULUMI_EPHEMERAL_AGENT_ACCOUNT_AUTH_REQUIRED")
}

func captureStderr(t *testing.T, f func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	f()

	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	os.Stderr = oldStderr
	return string(output)
}
