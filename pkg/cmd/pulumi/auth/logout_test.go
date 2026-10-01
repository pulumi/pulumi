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

package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDeleteAccountFallsBackToAgentCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.logout-agent.example.com"
	err := workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)
	err = workspace.StoreAgentClaim(workspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/logout-agent",
		ClaimToken: "logout-agent",
		CloudURL:   cloudURL,
		ValidUntil: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	err = deleteAccount(cloudURL)
	require.NoError(t, err)

	account, err := workspace.GetAgentAccount(cloudURL)
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
	claim, err := workspace.GetAgentClaim()
	require.NoError(t, err)
	assert.Empty(t, claim.ClaimURL)
}

func TestCredentialsContainAccountIncludesTokenlessCurrentBackend(t *testing.T) {
	t.Parallel()

	cloudURL := "file://~"
	creds := workspace.Credentials{
		Current: cloudURL,
		Accounts: map[string]workspace.Account{
			cloudURL: {},
		},
	}

	assert.True(t, credentialsContainAccount(creds, cloudURL))
}

func TestDeleteAccountSkipsAgentFallbackWhenExplicitPathSet(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	credsDir := t.TempDir()
	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(workspace.PulumiCredentialsPathEnvVar, credsDir)

	err := workspace.StoreCredentials(workspace.Credentials{
		Accounts: map[string]workspace.Account{
			"https://api.logout-explicit.example.com": {AccessToken: "default-token"},
		},
	})
	require.NoError(t, err)

	err = deleteAccount("https://api.logout-explicit.example.com")
	require.NoError(t, err)

	creds, err := workspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.NotContains(t, creds.Accounts, "https://api.logout-explicit.example.com")
}

func TestDeleteAllAccountsSkipsAgentFallbackOutsideAgentMode(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	credsPath := filepath.Join(credsDir, "credentials.json")

	err := workspace.StoreCredentials(workspace.Credentials{
		Accounts: map[string]workspace.Account{
			"https://api.logout-all.example.com": {AccessToken: "default-token"},
		},
	})
	require.NoError(t, err)

	err = deleteAllAccounts(false)
	require.NoError(t, err)

	_, err = os.Stat(credsPath)
	assert.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestLogoutCommandAll(t *testing.T) {
	ptesting.IsolateCredentials(t)
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
		Accounts: map[string]workspace.Account{
			"https://api.logout-command-all.example.com": {AccessToken: "default-token"},
		},
	}))

	cmd := NewLogoutCmd(&pkgWorkspace.MockContext{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--all"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Logged out of everything")
}

func TestLogoutCommandDeleteCredentialsKeyRequiresAll(t *testing.T) {
	t.Parallel()

	cmd := NewLogoutCmd(&pkgWorkspace.MockContext{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--delete-credentials-key"})

	err := cmd.Execute()
	assert.ErrorContains(t, err, "--delete-credentials-key requires --all")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestLogoutCommandCloudURL(t *testing.T) {
	ptesting.IsolateCredentials(t)
	cloudURL := "https://api.logout-command.example.com"
	require.NoError(t, workspace.StoreCredentials(workspace.Credentials{
		Accounts: map[string]workspace.Account{
			cloudURL: {AccessToken: "default-token"},
		},
	}))

	cmd := NewLogoutCmd(&pkgWorkspace.MockContext{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--cloud-url", cloudURL})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Logged out of "+cloudURL)
}

func TestLogoutCommandFallsBackToAgentCurrentCloud(t *testing.T) {
	agentDir := ptesting.IsolateCredentials(t).AgentDir
	t.Setenv("PULUMI_TEST_ALLOW_AGENT_FALLBACK", "true")
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.logout-agent-current.example.com"
	err := workspace.StoreAgentAccount(cloudURL, workspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	cmd := NewLogoutCmd(&pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return workspace.Credentials{}, assert.AnError
		},
	})
	var output bytes.Buffer
	cmd.SetOut(&output)

	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Logged out of "+cloudURL)

	account, err := workspace.GetAgentAccount(cloudURL)
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)

	agentCredsFile := filepath.Join(agentDir, "credentials.json")
	contents, err := os.ReadFile(agentCredsFile)
	if !os.IsNotExist(err) {
		require.NoError(t, err)
		assert.NotContains(t, string(contents), cloudURL)
		assert.NotContains(t, string(contents), "agent-token")
	}
}
