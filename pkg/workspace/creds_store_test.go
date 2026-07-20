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
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/agentdetect"
)

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())

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
	err := StoreCredentials(testCreds)
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

func TestAccountHasCredential(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		acct Account
		want bool
	}{
		{"empty", Account{}, false},
		{"access token only", Account{AccessToken: "a"}, true},
		{"refresh token only", Account{RefreshToken: "r"}, true},
		{"both tokens", Account{AccessToken: "a", RefreshToken: "r"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.acct.HasCredential())
		})
	}
}

//nolint:paralleltest // mutates environment and package global
func TestGetAccountWithAgentFallbackUsesRefreshOnlyDefaultAccount(t *testing.T) {
	// An account with only a refresh token (no access token) must be treated as usable rather
	// than skipped in favour of the agent fallback — the wrapper will mint the first access
	// token on the initial 401.
	t.Setenv("PULUMI_HOME", t.TempDir())
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")

	cloudURL := "https://api.refresh-only.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{RefreshToken: "refresh-only"}, true))
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent, "refresh-only default account must not fall through to agent")
	assert.Equal(t, "refresh-only", account.RefreshToken)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates environment
func TestAccountRefreshTokenRoundTrip(t *testing.T) {
	// The refresh token is held off-the-wire and exchanged at /api/oauth/token for short-lived
	// access tokens. It needs to survive credentials.json read/write so the CLI can use it across
	// process invocations.
	t.Setenv("PULUMI_CREDENTIALS_PATH", filepath.Join(t.TempDir(), "credentials.json"))

	const cloudURL = "https://api.example.com"
	original := Account{
		AccessToken:  "current-access-token",
		RefreshToken: "long-lived-refresh-token",
		Username:     "jane",
	}
	require.NoError(t, StoreAccount(cloudURL, original, true))

	loaded, err := GetAccount(cloudURL)
	require.NoError(t, err)
	assert.Equal(t, original.AccessToken, loaded.AccessToken)
	assert.Equal(t, original.RefreshToken, loaded.RefreshToken)
	assert.Equal(t, original.Username, loaded.Username)

	// An Account with no refresh token must still round-trip cleanly — the field is optional and
	// must serialize as omitted, not as an empty string anyone could mistake for "no refresh".
	plain := Account{AccessToken: "another-token"}
	require.NoError(t, StoreAccount(cloudURL, plain, true))
	loadedPlain, err := GetAccount(cloudURL)
	require.NoError(t, err)
	assert.Equal(t, "another-token", loadedPlain.AccessToken)
	assert.Empty(t, loadedPlain.RefreshToken)
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

//nolint:paralleltest // mutates package global
func TestMarkAgentClaimUnavailable(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})

	validUntil := time.Now().Add(time.Hour).UTC()
	require.NoError(t, StoreAgentClaim(AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ClaimToken: "abc123",
		ValidUntil: validUntil,
		CloudURL:   "https://api.example.com",
	}))
	unavailableAt := time.Now().UTC()
	require.NoError(t, MarkAgentClaimUnavailable(unavailableAt))

	claim, err := GetAgentClaim()
	require.NoError(t, err)
	assert.Equal(t, "https://app.pulumi.com/claim/abc123", claim.ClaimURL)
	assert.Equal(t, "abc123", claim.ClaimToken)
	assert.True(t, claim.ValidUntil.Equal(validUntil))
	require.NotNil(t, claim.ClaimUnavailableAt)
	assert.True(t, claim.ClaimUnavailableAt.Equal(unavailableAt))
}

//nolint:paralleltest // mutates package global
func TestGetAgentAccountUsesLegacyAccessTokenMap(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})

	require.NoError(t, StoreAgentCredentials(Credentials{
		AccessTokens: map[string]string{
			"https://api.legacy-agent-token.example.com": "legacy-token",
		},
	}))

	account, err := GetAgentAccount("https://api.legacy-agent-token.example.com")
	require.NoError(t, err)
	assert.Equal(t, "legacy-token", account.AccessToken)
	account, err = GetAgentAccount("https://api.missing-agent-token.example.com")
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates env vars and package global
func TestAgentPulumiDirTestOverride(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})

	override := filepath.Join(t.TempDir(), "agent")
	t.Setenv(pulumiTestAgentPulumiDirEnvVar, override)

	dir, err := getAgentPulumiDir()
	require.NoError(t, err)
	assert.Equal(t, override, dir)
	assert.Equal(t, filepath.Join(override, "credentials.json"), getAgentCredsFilePathNoEnsure())
	assert.Equal(t, filepath.Join(override, "agent-claim.json"), getAgentClaimFilePathNoEnsure())
	assert.Equal(t, filepath.Join(override, "config.json"), getAgentConfigFilePathNoEnsure())
}

//nolint:paralleltest // mutates package global
func TestGetAgentAccessTokenExpiresAt(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})

	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour)
	require.NoError(t, StoreAgentAccount("https://api.agent-token-expiry.example.com", Account{
		AccessToken: "agent-token",
		TokenInformation: &TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true))

	gotExpiresAt, valid, err := GetAgentAccessTokenExpiresAt("https://api.agent-token-expiry.example.com", now)
	require.NoError(t, err)
	require.NotNil(t, gotExpiresAt)
	assert.True(t, gotExpiresAt.Equal(expiresAt))
	assert.True(t, valid)
}

//nolint:paralleltest // mutates environment, default credentials, and package global
func TestGetAccountWithAgentFallbackPrefersDefaultCredentials(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")

	cloudURL := "https://api.default-wins.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "default-token"}, true))
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Equal(t, "default-token", account.AccessToken)
}

//nolint:paralleltest // mutates environment and package global
func TestGetAccountWithAgentFallbackUsesAgentCredentials(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")
	t.Setenv("PULUMI_HOME", "")

	cloudURL := "https://api.agent-fallback.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.True(t, fromAgent)
	assert.Equal(t, "agent-token", account.AccessToken)
}

//nolint:paralleltest // mutates environment and package global
func TestGetAccountWithAgentFallbackDoesNotMergeFieldsAcrossFiles(t *testing.T) {
	// File-as-a-unit invariant on the read side: a default account with an access token but no
	// refresh token must not silently acquire a refresh token from the agent file. The loaded
	// account is wholly from the source file, never a merge across the two.
	t.Setenv("PULUMI_HOME", t.TempDir())
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")

	cloudURL := "https://api.no-cross-file-merge.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "default-access"}, true))
	require.NoError(t, StoreAgentAccount(cloudURL, Account{
		AccessToken:  "agent-access",
		RefreshToken: "agent-refresh",
	}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent, "default has a credential — must not fall through to agent")
	assert.Equal(t, "default-access", account.AccessToken)
	assert.Empty(t, account.RefreshToken,
		"fields from the agent file must not leak into a default-sourced account")
}

//nolint:paralleltest // mutates environment and package global
func TestGetAccountWithAgentFallbackDisabledOutsideAgentMode(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	clearAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")
	t.Setenv("PULUMI_HOME", "")

	cloudURL := "https://api.no-agent-fallback.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates environment and package global
func TestGetAccountWithAgentFallbackDisabledWithExplicitHome(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")
	t.Setenv("PULUMI_HOME", t.TempDir())

	cloudURL := "https://api.explicit-home.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates environment and package global
func TestGetAccountWithAgentFallbackDisabledWithExplicitCredentialsPath(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv("PULUMI_HOME", "")

	cloudURL := "https://api.explicit-credentials-path.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Empty(t, account.AccessToken)
}

func TestFormatAgentClaimInstruction(t *testing.T) {
	t.Parallel()

	claimURL := "https://app.pulumi.com/claim/abc123"
	now := time.Date(2026, time.May, 17, 8, 24, 56, 0, time.UTC)
	validUntil := now.Add(3*24*time.Hour + 4*time.Hour + 10*time.Minute + 30*time.Second)
	expiresAt := now.Add(2*24*time.Hour + 3*time.Hour + 9*time.Minute)

	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"CLAIM_URL=https://app.pulumi.com/claim/abc123\n"+
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=2d3h9m\n"+
			"ACTION_REQUIRED=Tell the user to claim this Pulumi agent account.\n"+
			"INSTRUCTION=Tell the user Pulumi created an ephemeral agent account. "+
			"It is important to surface the claim URL to the user so they can take ownership of the account. "+
			"Include the claim URL and say they must claim the account within 2d3h9m before agent access expires. "+
			"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
			"so they should let the agent finish its work before claiming it.\n",
		FormatAgentClaimInstruction(claimURL, &expiresAt, validUntil, now))
	expiredAt := now.Add(-time.Minute)
	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"CLAIM_URL=https://app.pulumi.com/claim/abc123\n"+
			"CLAIM_URL_VALID_FOR=3d4h10m\n"+
			"ACTION_REQUIRED=Tell the user to claim this Pulumi agent account.\n"+
			"INSTRUCTION=Tell the user this ephemeral agent account can no longer authenticate, "+
			"but the claim URL is still valid for 3d4h10m. Include the claim URL and the remaining time. "+
			"Tell them claiming the account will lock the Pulumi organization during the claim process, "+
			"so they should let the agent finish its work before claiming it.\n",
		FormatAgentClaimInstruction(claimURL, &expiredAt, validUntil, now))
	assert.Empty(t, FormatAgentClaimInstruction(claimURL, nil, time.Time{}, now))
	assert.Empty(t, FormatAgentClaimInstruction("", &expiresAt, validUntil, now))
}

func TestFormatAgentLoginRequiredInstruction(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 17, 8, 24, 56, 0, time.UTC)
	expiresAt := now.Add(2*24*time.Hour + 3*time.Hour + 9*time.Minute)

	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=2d3h9m\n"+
			"ACTION_REQUIRED=Tell the user to run pulumi login.\n"+
			"INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate "+
			"even though local access had not expired. The account was likely claimed or revoked. "+
			"The stacks the agent was working with may have moved to the user's Pulumi account, so the agent's "+
			"existing access to those stacks may have changed. Ask the user to run pulumi login before retrying.\n",
		FormatAgentLoginRequiredInstruction(AgentLoginTokenRejected, &expiresAt, now))
	assert.Equal(t,
		"PULUMI_EPHEMERAL_AGENT_ACCOUNT\n"+
			"EPHEMERAL_ACCOUNT_ACCESS_EXPIRES_IN=2d3h9m\n"+
			"ACTION_REQUIRED=Tell the user to run pulumi login.\n"+
			"INSTRUCTION=Tell the user this Pulumi ephemeral agent account can no longer authenticate, "+
			"and its claim URL is no longer claimable. The account was likely already claimed, expired, "+
			"or revoked. If it was claimed, the stacks the agent was working with moved to the user's Pulumi account, "+
			"so the agent's existing access to those stacks changed. Ask the user to run pulumi login before retrying.\n",
		FormatAgentLoginRequiredInstruction(AgentLoginClaimUnavailable, &expiresAt, now))
}

func TestFormatAgentClaimValidFor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.May, 17, 8, 24, 56, 0, time.UTC)
	tests := []struct {
		name       string
		validUntil time.Time
		want       string
	}{
		{
			name:       "days hours minutes",
			validUntil: now.Add(3*24*time.Hour + 4*time.Hour + 10*time.Minute + 30*time.Second),
			want:       "3d4h10m",
		},
		{
			name:       "hours only",
			validUntil: now.Add(2 * time.Hour),
			want:       "2h",
		},
		{
			name:       "less than minute",
			validUntil: now.Add(30 * time.Second),
			want:       "<1m",
		},
		{
			name:       "expired",
			validUntil: now.Add(-time.Minute),
			want:       "expired",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, FormatAgentClaimValidFor(tt.validUntil, now))
		})
	}
}

func TestDefaultAgentPulumiDir(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		tempDir := t.TempDir()
		t.Setenv("TMP", tempDir)
		t.Setenv("TEMP", tempDir)
		assert.Equal(t, filepath.Join(tempDir, BookkeepingDir), defaultAgentPulumiDir())
		return
	}
	assert.Equal(t, filepath.Join("/tmp", BookkeepingDir), defaultAgentPulumiDir())
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

func TestDeleteAccountDeletesBackendConfig(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	err := StoreCredentials(Credentials{
		AccessTokens: map[string]string{
			"https://api.example.com":       "token-value",
			"https://api.other.example.com": "other-token",
		},
	})
	require.NoError(t, err)
	err = StorePulumiConfig(PulumiConfig{
		BackendConfig: map[string]BackendConfig{
			"https://api.example.com":       {DefaultOrg: "agent-org"},
			"https://api.other.example.com": {DefaultOrg: "other-org"},
		},
	})
	require.NoError(t, err)

	err = DeleteAccount("https://api.example.com")
	require.NoError(t, err)

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.NotContains(t, creds.AccessTokens, "https://api.example.com")
	assert.Equal(t, "other-token", creds.AccessTokens["https://api.other.example.com"])
	config, err := GetPulumiConfig()
	require.NoError(t, err)
	assert.NotContains(t, config.BackendConfig, "https://api.example.com")
	assert.Equal(t, "other-org", config.BackendConfig["https://api.other.example.com"].DefaultOrg)
}

func TestDeleteAccountDeletesBackendConfigFileWhenEmpty(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	err := StoreCredentials(Credentials{
		AccessTokens: map[string]string{
			"https://api.example.com": "token-value",
		},
	})
	require.NoError(t, err)
	err = StorePulumiConfig(PulumiConfig{
		BackendConfig: map[string]BackendConfig{
			"https://api.example.com": {DefaultOrg: "agent-org"},
		},
	})
	require.NoError(t, err)

	err = DeleteAccount("https://api.example.com")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(credsDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

func TestDeleteAllAccountsDeletesBackendConfig(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	err := StoreCredentials(Credentials{
		AccessTokens: map[string]string{
			"https://api.example.com": "token-value",
		},
	})
	require.NoError(t, err)
	err = StorePulumiConfig(PulumiConfig{
		BackendConfig: map[string]BackendConfig{
			"https://api.example.com": {DefaultOrg: "agent-org"},
		},
	})
	require.NoError(t, err)

	err = DeleteAllAccounts()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(credsDir, "credentials.json"))
	require.True(t, os.IsNotExist(err))
	config, err := GetPulumiConfig()
	require.NoError(t, err)
	assert.Empty(t, config.BackendConfig)
}

func TestDeleteAllAccountsReturnsCredentialsDeleteError(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	credentialsPath := filepath.Join(credsDir, "credentials.json")
	require.NoError(t, os.Mkdir(credentialsPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(credentialsPath, "file"), []byte("token-value"), 0o600))

	err := DeleteAllAccounts()
	require.Error(t, err)
}

func TestDeleteAllAccountsReturnsBackendConfigDeleteError(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	require.NoError(t, StoreCredentials(Credentials{
		AccessTokens: map[string]string{
			"https://api.example.com": "token-value",
		},
	}))
	require.NoError(t, os.Mkdir(filepath.Join(credsDir, "config.json"), 0o700))

	err := DeleteAllAccounts()
	require.ErrorContains(t, err, "reading")
}

func TestDeleteBackendConfigMissingFile(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	err := deleteBackendConfig("https://api.example.com")
	require.NoError(t, err)
}

func TestDeleteBackendConfigInvalidJSON(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	require.NoError(t, os.WriteFile(filepath.Join(credsDir, "config.json"), []byte("{"), 0o600))

	err := deleteBackendConfig("https://api.example.com")
	require.ErrorContains(t, err, "failed to read Pulumi agent config file")
}

func TestDeleteBackendConfigPathError(t *testing.T) {
	credsPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(credsPath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, credsPath)
	t.Setenv("PULUMI_HOME", "")

	err := deleteBackendConfig("https://api.example.com")
	require.ErrorContains(t, err, "failed to create")
}

func TestDeleteAllBackendConfigMissingFile(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	err := deleteAllBackendConfig()
	require.NoError(t, err)
}

func TestDeleteAllBackendConfigInvalidJSON(t *testing.T) {
	credsDir := t.TempDir()
	t.Setenv(PulumiCredentialsPathEnvVar, credsDir)
	t.Setenv("PULUMI_HOME", "")

	require.NoError(t, os.WriteFile(filepath.Join(credsDir, "config.json"), []byte("{"), 0o600))

	err := deleteAllBackendConfig()
	require.ErrorContains(t, err, "failed to read Pulumi config file")
}

func TestDeleteAllBackendConfigPathError(t *testing.T) {
	credsPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(credsPath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, credsPath)
	t.Setenv("PULUMI_HOME", "")

	err := deleteAllBackendConfig()
	require.ErrorContains(t, err, "failed to create")
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
	expiresAt := now.Add(2 * time.Hour)
	err := StoreAgentAccount("https://api.example.com", Account{
		AccessToken: "token-value",
		TokenInformation: &TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
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
	assert.False(t, deleted)

	account, err = GetAgentAccount("https://api.example.com")
	require.NoError(t, err)
	assert.Equal(t, "token-value", account.AccessToken)

	expiredAt := now.Add(-time.Minute)
	account.TokenInformation.ExpiresAt = &expiredAt
	err = StoreAgentAccount("https://api.example.com", account, true)
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
func TestDeleteExpiredAgentCredentialsDoesNotReuseUnrelatedValidAccount(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	now := time.Now().UTC()
	expiresAt := now.Add(2 * time.Hour)
	err := StoreAgentAccount("https://api.example.com", Account{
		AccessToken: "token-value",
		TokenInformation: &TokenInformation{
			ExpiresAt: &expiresAt,
		},
	}, true)
	require.NoError(t, err)
	err = StoreAgentClaim(AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: now.Add(-time.Hour),
		CloudURL:   "https://api.other.example.com",
	})
	require.NoError(t, err)

	deleted, err := DeleteExpiredAgentCredentials(now)
	require.NoError(t, err)
	assert.True(t, deleted)

	account, err := GetAgentAccount("https://api.example.com")
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // mutates package global
func TestDeleteAgentAccount(t *testing.T) {
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})

	err := StoreAgentAccount("https://api.example.com", Account{AccessToken: "token-value"}, true)
	require.NoError(t, err)
	err = StoreAgentAccount("https://api.other.example.com", Account{AccessToken: "other-token"}, false)
	require.NoError(t, err)
	err = StoreAgentClaim(AgentClaim{
		ClaimURL:   "https://app.pulumi.com/claim/abc123",
		ValidUntil: time.Now().Add(time.Hour),
		CloudURL:   "https://api.example.com",
	})
	require.NoError(t, err)
	agentDir, err := getAgentPulumiDir()
	require.NoError(t, err)
	err = writePulumiConfigFile(filepath.Join(agentDir, "config.json"), PulumiConfig{
		BackendConfig: map[string]BackendConfig{
			"https://api.example.com":       {DefaultOrg: "agent-org"},
			"https://api.other.example.com": {DefaultOrg: "other-org"},
		},
	})
	require.NoError(t, err)

	err = DeleteAgentAccount("https://api.example.com")
	require.NoError(t, err)

	account, err := GetAgentAccount("https://api.example.com")
	require.NoError(t, err)
	assert.Empty(t, account.AccessToken)
	account, err = GetAgentAccount("https://api.other.example.com")
	require.NoError(t, err)
	assert.Equal(t, "other-token", account.AccessToken)
	claim, err := GetAgentClaim()
	require.NoError(t, err)
	assert.Empty(t, claim.ClaimURL)
	data, err := os.ReadFile(getAgentConfigFilePathNoEnsure())
	require.NoError(t, err)
	var config PulumiConfig
	require.NoError(t, json.Unmarshal(data, &config))
	assert.NotContains(t, config.BackendConfig, "https://api.example.com")
	assert.Equal(t, "other-org", config.BackendConfig["https://api.other.example.com"].DefaultOrg)
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
	require.ErrorContains(t, err, "agent mode requires read/write access to "+agentPulumiDir)
}

//nolint:paralleltest // mutates package global
func TestAgentCredentialsRequireNonSymlinkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	oldAgentPulumiDir := agentPulumiDir
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	agentPulumiDir = filepath.Join(parent, "link")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	require.NoError(t, os.Symlink(target, agentPulumiDir))

	_, err := GetAgentStoredCredentials()
	require.ErrorContains(t, err, "must not be a symlink")
}

//nolint:paralleltest // mutates package global
func TestAgentCredentialsRepairInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permission bits vary on Windows")
	}
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		agentPulumiDir = oldAgentPulumiDir
	})
	require.NoError(t, os.Mkdir(agentPulumiDir, 0o777))

	dir, err := getAgentPulumiDir()
	require.NoError(t, err)
	assert.Equal(t, agentPulumiDir, dir)
	info, err := os.Stat(agentPulumiDir)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
}

func setAgentEnv(t *testing.T) {
	t.Helper()
	clearAgentEnv(t)
	t.Setenv("CODEX_SANDBOX", "1")
}

func clearAgentEnv(t *testing.T) {
	t.Helper()
	for _, name := range agentdetect.DetectionEnvVars() {
		t.Setenv(name, "")
	}
}
