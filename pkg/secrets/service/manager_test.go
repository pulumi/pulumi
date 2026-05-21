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

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestGetServiceSecretsAccountFallsBackToAgentCredentials(t *testing.T) {
	isolateAgentCredentials(t)
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
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, "")
	t.Setenv(workspace.PulumiHomeEnvVar, "")

	cloudURL := "https://api.service-secrets-agent.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	account, err := getServiceSecretsAccount(cloudURL)
	require.NoError(t, err)
	assert.Equal(t, "agent-token", account.AccessToken)
}

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestGetServiceSecretsAccountDoesNotFallbackWithExplicitPath(t *testing.T) {
	isolateAgentCredentials(t)
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
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv(workspace.PulumiHomeEnvVar, "")

	cloudURL := "https://api.service-secrets-explicit.example.com"
	err = workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	account, err := getServiceSecretsAccount(cloudURL)
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
}

func isolateAgentCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())
}
