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
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

//nolint:paralleltest // mutates env vars and shared temporary agent credentials
func TestDeleteAccountFallsBackToAgentCredentials(t *testing.T) {
	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, "")
	t.Setenv(env.Home.Var().Name(), "")
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	cloudURL := "https://api.logout-agent.example.com"
	err := pkgWorkspace.StoreAgentAccount(cloudURL, pkgWorkspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)
	err = pkgWorkspace.StoreAgentClaim(pkgWorkspace.AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/logout-agent",
		ClaimToken: "logout-agent",
		CloudURL:   cloudURL,
		ValidUntil: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	err = deleteAccount(cloudURL)
	require.NoError(t, err)

	account, err := pkgWorkspace.GetAgentAccount(cloudURL)
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
	claim, err := pkgWorkspace.GetAgentClaim()
	require.NoError(t, err)
	assert.Empty(t, claim.ClaimURL)
}

func TestCredentialsContainAccountIncludesTokenlessCurrentBackend(t *testing.T) {
	t.Parallel()

	cloudURL := "file://~"
	creds := pkgWorkspace.Credentials{
		Current: cloudURL,
		AccessTokens: map[string]string{
			cloudURL: "",
		},
		Accounts: map[string]pkgWorkspace.Account{
			cloudURL: {},
		},
	}

	assert.True(t, credentialsContainAccount(creds, cloudURL))
}

//nolint:paralleltest // mutates env vars
func TestDeleteAccountSkipsAgentFallbackWhenExplicitPathSet(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv(env.Home.Var().Name(), "")
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	err := pkgWorkspace.StoreCredentials(pkgWorkspace.Credentials{
		AccessTokens: map[string]string{
			"https://api.logout-explicit.example.com": "default-token",
		},
	})
	require.NoError(t, err)

	err = deleteAccount("https://api.logout-explicit.example.com")
	require.NoError(t, err)

	creds, err := pkgWorkspace.GetStoredCredentials()
	require.NoError(t, err)
	assert.NotContains(t, creds.AccessTokens, "https://api.logout-explicit.example.com")
}

//nolint:paralleltest // mutates env vars
func TestDeleteAllAccountsSkipsAgentFallbackOutsideAgentMode(t *testing.T) {
	credsDir := t.TempDir()
	credsPath := filepath.Join(credsDir, "credentials.json")
	t.Setenv("CODEX_SANDBOX", "")
	t.Setenv("AI_AGENT", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv(env.Home.Var().Name(), "")

	err := pkgWorkspace.StoreCredentials(pkgWorkspace.Credentials{
		AccessTokens: map[string]string{
			"https://api.logout-all.example.com": "default-token",
		},
	})
	require.NoError(t, err)

	err = deleteAllAccounts()
	require.NoError(t, err)

	_, err = os.Stat(credsPath)
	assert.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // mutates env vars
func TestLogoutCommandAll(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv(env.Home.Var().Name(), "")
	require.NoError(t, pkgWorkspace.StoreCredentials(pkgWorkspace.Credentials{
		AccessTokens: map[string]string{
			"https://api.logout-command-all.example.com": "default-token",
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

//nolint:paralleltest // mutates env vars
func TestLogoutCommandCloudURL(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv(env.Home.Var().Name(), "")
	cloudURL := "https://api.logout-command.example.com"
	require.NoError(t, pkgWorkspace.StoreCredentials(pkgWorkspace.Credentials{
		AccessTokens: map[string]string{
			cloudURL: "default-token",
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
	agentDir := t.TempDir()
	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(pkgWorkspace.PulumiCredentialsPathEnvVar, "")
	t.Setenv(env.Home.Var().Name(), "")
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", agentDir)

	cloudURL := "https://api.logout-agent-current.example.com"
	err := pkgWorkspace.StoreAgentAccount(cloudURL, pkgWorkspace.Account{AccessToken: "agent-token"}, true)
	require.NoError(t, err)

	cmd := NewLogoutCmd(&pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (pkgWorkspace.Credentials, error) {
			return pkgWorkspace.Credentials{}, assert.AnError
		},
	})
	var output bytes.Buffer
	cmd.SetOut(&output)

	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, output.String(), "Logged out of "+cloudURL)

	account, err := pkgWorkspace.GetAgentAccount(cloudURL)
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
