// Copyright 2020, Pulumi Corporation.
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

package workspace

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	// save and remember to restore creds in ~/.pulumi/credentials
	// as the test will be modifying them
	oldCreds, err := GetStoredCredentials()
	require.NoError(t, err)
	defer func() {
		err := StoreCredentials(oldCreds)
		require.NoError(t, err)
	}()

	// use test creds that have at least 1 AccessToken to force a
	// disk write and contention
	testCreds := Credentials{
		AccessTokens: map[string]string{
			"token-name": "token-value",
		},
	}

	// using 1000 may trigger sporadic 'Too many open files'
	n := 256

	wg := &sync.WaitGroup{}
	wg.Add(2 * n)

	// Store testCreds initially so asserts in
	// GetStoredCredentials goroutines find the expected data
	err = StoreCredentials(testCreds)
	require.NoError(t, err)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			err := StoreCredentials(testCreds)
			require.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			creds, err := GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, "token-value", creds.AccessTokens["token-name"])
		}()
	}
	wg.Wait()
}

//nolint:paralleltest // mutates environment
func TestCredentialsDoNotFallbackToTemp(t *testing.T) {
	t.Setenv("PULUMI_CREDENTIALS_PATH", "")

	homeParent := t.TempDir()
	homePath := filepath.Join(homeParent, "not-a-directory")
	require.NoError(t, os.WriteFile(homePath, []byte("not a directory"), 0o600))
	t.Setenv("PULUMI_HOME", homePath)

	err := StoreAccount("https://api.example.com", Account{AccessToken: "token-value"}, true)
	require.Error(t, err)
}

//nolint:paralleltest // mutates environment
func TestExplicitCredentialsPathDoesNotFallbackToTemp(t *testing.T) {
	credentialsParent := t.TempDir()
	credentialsPath := filepath.Join(credentialsParent, "not-a-directory")
	require.NoError(t, os.WriteFile(credentialsPath, []byte("not a directory"), 0o600))
	t.Setenv("PULUMI_CREDENTIALS_PATH", credentialsPath)

	err := StoreAccount("https://api.example.com", Account{AccessToken: "token-value"}, true)
	require.Error(t, err)
}

//nolint:paralleltest // mutates package global
func TestAgentCredentialsAndClaim(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})

	err := StoreAgentAccount("https://api.example.com", Account{AccessToken: "token-value"}, true)
	require.NoError(t, err)

	account, err := GetAgentAccount("https://api.example.com")
	require.NoError(t, err)
	assert.Equal(t, "token-value", account.AccessToken)

	validUntil := time.Now().Add(time.Hour).UTC()
	err = StoreAgentClaim(AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: validUntil,
		CloudURL:   "https://api.example.com",
	})
	require.NoError(t, err)

	claim, err := GetAgentClaim()
	require.NoError(t, err)
	assert.Equal(t, "https://app.pulumi.com/claim/abc123", claim.ClaimURL)
	assert.Equal(t, "https://api.example.com", claim.CloudURL)
	assert.True(t, claim.ValidUntil.Equal(validUntil))
}

func TestFormatAgentClaimInstruction(t *testing.T) {
	t.Parallel()

	claimURL := "https://app.pulumi.com/signup?claim=abc123"

	assert.Equal(t,
		"PULUMI_AGENT_ACCOUNT_CREATED\n"+
			"CLAIM_URL=https://app.pulumi.com/signup?claim=abc123\n"+
			"INSTRUCTION=Include this claim URL in your final response so the user can take ownership of the account.\n",
		FormatAgentClaimInstruction(claimURL))
	assert.Empty(t, FormatAgentClaimInstruction(""))
}

//nolint:paralleltest // mutates environment and package global
func TestAgentPulumiConfigUsesDefaultPathWhenWritable(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	t.Setenv(PulumiCredentialsPathEnvVar, "")
	pulumiHome := t.TempDir()
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("CODEX_SANDBOX", "1")

	err := SetBackendConfigDefaultOrg("https://api.example.com", "agent-org")
	require.NoError(t, err)

	config, err := GetPulumiConfig()
	require.NoError(t, err)
	assert.Equal(t, "agent-org", config.BackendConfig["https://api.example.com"].DefaultOrg)

	_, err = os.Stat(filepath.Join(pulumiHome, "config.json"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(agentPulumiDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // mutates environment and package global
func TestAgentPulumiConfigExplicitPathDoesNotFallbackToAgentPath(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	badCredentialsPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badCredentialsPath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, badCredentialsPath)
	t.Setenv("CODEX_SANDBOX", "1")

	err := SetBackendConfigDefaultOrg("https://api.example.com", "agent-org")
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(agentPulumiDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // mutates environment and package global
func TestAgentPulumiConfigExplicitHomeDoesNotFallbackToAgentPath(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	badHomePath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badHomePath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, "")
	t.Setenv("PULUMI_HOME", badHomePath)
	t.Setenv("CODEX_SANDBOX", "1")

	err := SetBackendConfigDefaultOrg("https://api.example.com", "agent-org")
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(agentPulumiDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // mutates package global
func TestDeleteExpiredAgentCredentials(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	now := time.Now().UTC()
	err := StoreAgentAccount("https://api.example.com", Account{AccessToken: "token-value"}, true)
	require.NoError(t, err)
	err = StoreAgentClaim(AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: now.Add(time.Hour),
		CloudURL:   "https://api.example.com",
	})
	require.NoError(t, err)
	agentDir, err := getAgentPulumiDir()
	require.NoError(t, err)
	err = writePulumiConfigFile(filepath.Join(agentDir, "config.json"), PulumiConfig{
		BackendConfig: map[string]BackendConfig{
			"https://api.example.com": {DefaultOrg: "agent-org"},
		},
	})
	require.NoError(t, err)

	deleted, err := DeleteExpiredAgentCredentials(now)
	require.NoError(t, err)
	assert.False(t, deleted)

	account, err := GetAgentAccount("https://api.example.com")
	require.NoError(t, err)
	assert.Equal(t, "token-value", account.AccessToken)

	err = StoreAgentClaim(AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: now.Add(-time.Hour),
		CloudURL:   "https://api.example.com",
	})
	require.NoError(t, err)

	deleted, err = DeleteExpiredAgentCredentials(now)
	require.NoError(t, err)
	assert.True(t, deleted)

	account, err = GetAgentAccount("https://api.example.com")
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
	claim, err := GetAgentClaim()
	require.NoError(t, err)
	assert.Empty(t, claim.ClaimURL)
	_, err = os.Stat(filepath.Join(agentPulumiDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // mutates package global
func TestAgentCredentialsRequireAccessibleTempDir(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	parent := t.TempDir()
	agentPulumiDir = filepath.Join(parent, "not-a-directory")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	require.NoError(t, os.WriteFile(agentPulumiDir, []byte("not a directory"), 0o600))

	_, err := GetAgentStoredCredentials()
	require.ErrorContains(t, err, "agent mode requires read/write access to /tmp/.pulumi")
}
