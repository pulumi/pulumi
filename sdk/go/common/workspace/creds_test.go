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

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	ptesting.IsolateCredentials(t)

	// use test creds that have at least 1 account to force a
	// disk write and contention
	testCreds := Credentials{
		Accounts: map[string]Account{
			"token-name": {AccessToken: "token-value"},
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

	for range n {
		go func() {
			defer wg.Done()
			err := StoreCredentials(testCreds)
			require.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			creds, err := GetStoredCredentials()
			require.NoError(t, err)
			assert.Equal(t, "token-value", creds.Accounts["token-name"].AccessToken)
		}()
	}
	wg.Wait()
}

func TestCredentialsDoNotFallbackToTemp(t *testing.T) {
	ptesting.IsolateCredentials(t)

	homeParent := t.TempDir()
	homePath := filepath.Join(homeParent, "not-a-directory")
	require.NoError(t, os.WriteFile(homePath, []byte("not a directory"), 0o600))
	t.Setenv("PULUMI_HOME", homePath)

	err := StoreAccount("https://api.example.com", Account{AccessToken: "token-value"}, true)
	require.Error(t, err)
}

func TestExplicitCredentialsPathDoesNotFallbackToTemp(t *testing.T) {
	ptesting.IsolateCredentials(t)
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

func TestAccountSaveErrorsOnEmptySource(t *testing.T) {
	t.Parallel()
	// A literal Account has no source (it wasn't loaded from a file). Save must refuse rather
	// than silently writing somewhere a caller didn't ask for.
	err := Account{AccessToken: "x"}.Save("https://api.example.com", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not loaded")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestAccountSaveWritesToSourceFile(t *testing.T) {
	// File-as-a-unit invariant: an account loaded from a file must Save back to that same file.
	// Loaded-from-default must not bleed into agent, and loaded-from-agent must not bleed into
	// default (the bug the source field exists to prevent).

	const cloudURL = "https://api.example.com"

	t.Run("loaded from default writes back to default", func(t *testing.T) {
		ptesting.IsolateCredentials(t)
		require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "orig-default"}, false))
		require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "orig-agent"}, false))

		loaded, err := GetAccount(cloudURL)
		require.NoError(t, err)
		loaded.AccessToken = "updated-default"
		require.NoError(t, loaded.Save(cloudURL, false))

		fromDefault, err := GetAccount(cloudURL)
		require.NoError(t, err)
		assert.Equal(t, "updated-default", fromDefault.AccessToken)

		fromAgent, err := GetAgentAccount(cloudURL)
		require.NoError(t, err)
		assert.Equal(t, "orig-agent", fromAgent.AccessToken,
			"saving a default-sourced account must not touch the agent file")
	})

	t.Run("loaded from agent writes back to agent", func(t *testing.T) {
		ptesting.IsolateCredentials(t)
		require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "orig-default"}, false))
		require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "orig-agent"}, false))

		loaded, err := GetAgentAccount(cloudURL)
		require.NoError(t, err)
		loaded.AccessToken = "updated-agent"
		require.NoError(t, loaded.Save(cloudURL, false))

		fromAgent, err := GetAgentAccount(cloudURL)
		require.NoError(t, err)
		assert.Equal(t, "updated-agent", fromAgent.AccessToken)

		fromDefault, err := GetAccount(cloudURL)
		require.NoError(t, err)
		assert.Equal(t, "orig-default", fromDefault.AccessToken,
			"saving an agent-sourced account must not touch the default file")
	})
}

func TestGetAccountWithAgentFallbackUsesRefreshOnlyDefaultAccount(t *testing.T) {
	// An account with only a refresh token (no access token) must be treated as usable rather
	// than skipped in favour of the agent fallback — the wrapper will mint the first access
	// token on the initial 401.
	ptesting.IsolateCredentials(t)
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.refresh-only.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{RefreshToken: "refresh-only"}, true))
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent, "refresh-only default account must not fall through to agent")
	assert.Equal(t, "refresh-only", account.RefreshToken)
	assert.Empty(t, account.AccessToken)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestAccountRefreshTokenRoundTrip(t *testing.T) {
	ptesting.IsolateCredentials(t)
	// The refresh token is held off-the-wire and exchanged at /api/oauth/token for short-lived
	// access tokens. It needs to survive credentials.json read/write so the CLI can use it across
	// process invocations.

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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestAgentCredentialsAndClaim(t *testing.T) {
	ptesting.IsolateCredentials(t)

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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestMarkAgentClaimUnavailable(t *testing.T) {
	ptesting.IsolateCredentials(t)

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

	require.NoError(t, ClearAgentClaimUnavailable())
	claim, err = GetAgentClaim()
	require.NoError(t, err)
	assert.Nil(t, claim.ClaimUnavailableAt)
	assert.Equal(t, "abc123", claim.ClaimToken, "clearing the marker keeps the rest of the claim")

	require.NoError(t, ClearAgentClaimUnavailable(), "clearing an unset marker is a no-op")
}

func TestCredentialsMarshalJSON(t *testing.T) {
	t.Parallel()

	raw, err := json.MarshalIndent(Credentials{
		Current: "https://api.example.com",
		Accounts: map[string]Account{
			"https://api.example.com": {AccessToken: "token-value", Username: "user"},
			"file://~":                {},
		},
	}, "", "    ")
	require.NoError(t, err)

	assert.Equal(t, `{
    "version": 1,
    "current": "https://api.example.com",
    "accessTokens": {
        "file://~": "",
        "https://api.example.com": "token-value"
    },
    "accounts": {
        "file://~": {
            "lastValidatedAt": "0001-01-01T00:00:00Z"
        },
        "https://api.example.com": {
            "accessToken": "token-value",
            "username": "user",
            "lastValidatedAt": "0001-01-01T00:00:00Z"
        }
    }
}`, string(raw))
}

func TestCredentialsMarshalJSONWithoutAccounts(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(Credentials{Current: "https://api.example.com"})
	require.NoError(t, err)

	assert.Equal(t, `{"version":1,"current":"https://api.example.com"}`, string(raw))
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestGetStoredCredentialsIgnoresAccessTokensObject(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	require.NoError(t, os.WriteFile(filepath.Join(credsDir, "credentials.json"),
		[]byte(`{"current":"https://api.example.com","accessTokens":{"https://api.example.com":"token-value"}}`),
		0o600))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, Credentials{Current: "https://api.example.com"}, creds)
}

func TestAgentPulumiDirTestOverride(t *testing.T) {
	ptesting.IsolateCredentials(t)

	override := filepath.Join(t.TempDir(), "agent")
	t.Setenv(pulumiTestAgentPulumiDirEnvVar, override)

	dir, err := getAgentPulumiDir()
	require.NoError(t, err)
	assert.Equal(t, override, dir)
	assert.Equal(t, filepath.Join(override, "credentials.json"), getAgentCredsFilePathNoEnsure())
	assert.Equal(t, filepath.Join(override, "agent-claim.json"), getAgentClaimFilePathNoEnsure())
	assert.Equal(t, filepath.Join(override, "config.json"), getAgentConfigFilePathNoEnsure())
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestGetAgentAccessTokenExpiresAt(t *testing.T) {
	ptesting.IsolateCredentials(t)

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

func TestGetAccountWithAgentFallbackPrefersDefaultCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.default-wins.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "default-token"}, true))
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Equal(t, "default-token", account.AccessToken)
}

func TestGetAccountWithAgentFallbackUsesAgentCredentials(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.agent-fallback.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.True(t, fromAgent)
	assert.Equal(t, "agent-token", account.AccessToken)
}

func TestGetAccountWithAgentFallbackDoesNotMergeFieldsAcrossFiles(t *testing.T) {
	// File-as-a-unit invariant on the read side: a default account with an access token but no
	// refresh token must not silently acquire a refresh token from the agent file. The loaded
	// account is wholly from the source file, never a merge across the two.
	ptesting.IsolateCredentials(t)
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
	t.Setenv("CODEX_SANDBOX", "1")

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

func TestGetAccountWithAgentFallbackDisabledOutsideAgentMode(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")

	cloudURL := "https://api.no-agent-fallback.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Empty(t, account.AccessToken)
}

func TestGetAccountWithAgentFallbackDisabledWithExplicitHome(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.explicit-home.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Empty(t, account.AccessToken)
}

func TestGetAccountWithAgentFallbackDisabledWithExplicitCredentialsPath(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")

	t.Setenv("CODEX_SANDBOX", "1")
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())

	cloudURL := "https://api.explicit-credentials-path.example.com"
	require.NoError(t, StoreAgentAccount(cloudURL, Account{AccessToken: "agent-token"}, true))

	account, fromAgent, err := GetAccountWithAgentFallback(cloudURL)
	require.NoError(t, err)
	assert.False(t, fromAgent)
	assert.Empty(t, account.AccessToken)
}

func TestAgentClaimActive(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	assert.True(t, AgentClaim{ClaimURL: "https://app.example.com/claim/t", ValidUntil: future}.Active(now))
	assert.True(t, AgentClaim{ClaimURL: "https://app.example.com/claim/t"}.Active(now),
		"a claim with no recorded expiry is active")
	assert.False(t, AgentClaim{ValidUntil: future}.Active(now), "a claim without a URL is not active")
	assert.False(t, AgentClaim{ClaimURL: "https://app.example.com/claim/t", ValidUntil: past}.Active(now))
	assert.False(t, AgentClaim{
		ClaimURL:           "https://app.example.com/claim/t",
		ValidUntil:         future,
		ClaimUnavailableAt: &past,
	}.Active(now))
}

//nolint:paralleltest // The Windows case changes TMP and TEMP with t.Setenv.
func TestDefaultAgentPulumiDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		tempDir := t.TempDir()
		t.Setenv("TMP", tempDir)
		t.Setenv("TEMP", tempDir)
		assert.Equal(t, filepath.Join(tempDir, BookkeepingDir), defaultAgentPulumiDir())
		return
	}
	assert.Equal(t, filepath.Join("/tmp", BookkeepingDir), defaultAgentPulumiDir())
}

func TestAgentPulumiConfigUsesDefaultPathWhenWritable(t *testing.T) {
	dirs := ptesting.IsolateCredentials(t)
	pulumiHome, agentPulumiDir := dirs.Home, dirs.AgentDir
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAccountDeletesBackendConfig(t *testing.T) {
	ptesting.IsolateCredentials(t)

	err := StoreCredentials(Credentials{
		Accounts: map[string]Account{
			"https://api.example.com":       {AccessToken: "token-value"},
			"https://api.other.example.com": {AccessToken: "other-token"},
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
	assert.NotContains(t, creds.Accounts, "https://api.example.com")
	assert.Equal(t, "other-token", creds.Accounts["https://api.other.example.com"].AccessToken)
	config, err := GetPulumiConfig()
	require.NoError(t, err)
	assert.NotContains(t, config.BackendConfig, "https://api.example.com")
	assert.Equal(t, "other-org", config.BackendConfig["https://api.other.example.com"].DefaultOrg)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAccountDeletesBackendConfigFileWhenEmpty(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	err := StoreCredentials(Credentials{
		Accounts: map[string]Account{
			"https://api.example.com": {AccessToken: "token-value"},
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAllAccountsDeletesBackendConfig(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	err := StoreCredentials(Credentials{
		Accounts: map[string]Account{
			"https://api.example.com": {AccessToken: "token-value"},
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAllAccountsReturnsCredentialsDeleteError(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	credentialsPath := filepath.Join(credsDir, "credentials.json")
	require.NoError(t, os.Mkdir(credentialsPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(credentialsPath, "file"), []byte("token-value"), 0o600))

	err := DeleteAllAccounts()
	require.Error(t, err)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAllAccountsReturnsBackendConfigDeleteError(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	require.NoError(t, StoreCredentials(Credentials{
		Accounts: map[string]Account{
			"https://api.example.com": {AccessToken: "token-value"},
		},
	}))
	require.NoError(t, os.Mkdir(filepath.Join(credsDir, "config.json"), 0o700))

	err := DeleteAllAccounts()
	require.ErrorContains(t, err, "reading")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteBackendConfigMissingFile(t *testing.T) {
	ptesting.IsolateCredentials(t)

	err := deleteBackendConfig("https://api.example.com")
	require.NoError(t, err)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteBackendConfigInvalidJSON(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	require.NoError(t, os.WriteFile(filepath.Join(credsDir, "config.json"), []byte("{"), 0o600))

	err := deleteBackendConfig("https://api.example.com")
	require.ErrorContains(t, err, "failed to read Pulumi agent config file")
}

func TestDeleteBackendConfigPathError(t *testing.T) {
	ptesting.IsolateCredentials(t)
	credsPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(credsPath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, credsPath)

	err := deleteBackendConfig("https://api.example.com")
	require.ErrorContains(t, err, "failed to create")
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAllBackendConfigMissingFile(t *testing.T) {
	ptesting.IsolateCredentials(t)

	err := deleteAllBackendConfig()
	require.NoError(t, err)
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAllBackendConfigInvalidJSON(t *testing.T) {
	credsDir := ptesting.IsolateCredentials(t).Home

	require.NoError(t, os.WriteFile(filepath.Join(credsDir, "config.json"), []byte("{"), 0o600))

	err := deleteAllBackendConfig()
	require.ErrorContains(t, err, "failed to read Pulumi config file")
}

func TestDeleteAllBackendConfigPathError(t *testing.T) {
	ptesting.IsolateCredentials(t)
	credsPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(credsPath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, credsPath)

	err := deleteAllBackendConfig()
	require.ErrorContains(t, err, "failed to create")
}

func TestAgentPulumiConfigExplicitPathDoesNotFallbackToAgentPath(t *testing.T) {
	agentPulumiDir := ptesting.IsolateCredentials(t).AgentDir
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
	badCredentialsPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badCredentialsPath, []byte("not a directory"), 0o600))
	t.Setenv(PulumiCredentialsPathEnvVar, badCredentialsPath)
	t.Setenv("CODEX_SANDBOX", "1")

	err := SetBackendConfigDefaultOrg("https://api.example.com", "agent-org")
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(agentPulumiDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

func TestAgentPulumiConfigExplicitHomeDoesNotFallbackToAgentPath(t *testing.T) {
	agentPulumiDir := ptesting.IsolateCredentials(t).AgentDir
	badHomePath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(badHomePath, []byte("not a directory"), 0o600))
	t.Setenv("PULUMI_HOME", badHomePath)
	t.Setenv("CODEX_SANDBOX", "1")

	err := SetBackendConfigDefaultOrg("https://api.example.com", "agent-org")
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(agentPulumiDir, "config.json"))
	require.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteExpiredAgentCredentials(t *testing.T) {
	agentPulumiDir := ptesting.IsolateCredentials(t).AgentDir
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteExpiredAgentCredentialsDoesNotReuseUnrelatedValidAccount(t *testing.T) {
	ptesting.IsolateCredentials(t)
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

//nolint:paralleltest // IsolateCredentials changes process-wide environment variables.
func TestDeleteAgentAccount(t *testing.T) {
	ptesting.IsolateCredentials(t)

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

func TestAgentCredentialsRequireAccessibleTempDir(t *testing.T) {
	ptesting.IsolateCredentials(t)
	parent := t.TempDir()
	agentPulumiDir := filepath.Join(parent, "not-a-directory")
	t.Setenv(pulumiTestAgentPulumiDirEnvVar, agentPulumiDir)
	require.NoError(t, os.WriteFile(agentPulumiDir, []byte("not a directory"), 0o600))

	_, err := GetAgentStoredCredentials()
	require.ErrorContains(t, err, "agent mode requires read/write access to "+agentPulumiDir)
}

func TestAgentCredentialsRequireNonSymlinkDir(t *testing.T) {
	ptesting.IsolateCredentials(t)
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	agentPulumiDir := filepath.Join(parent, "link")
	t.Setenv(pulumiTestAgentPulumiDirEnvVar, agentPulumiDir)
	require.NoError(t, os.Symlink(target, agentPulumiDir))

	_, err := GetAgentStoredCredentials()
	require.ErrorContains(t, err, "must not be a symlink")
}

func TestAgentCredentialsRepairInsecurePermissions(t *testing.T) {
	ptesting.IsolateCredentials(t)
	if runtime.GOOS == "windows" {
		t.Skip("chmod permission bits vary on Windows")
	}
	agentPulumiDir := filepath.Join(t.TempDir(), ".pulumi")
	t.Setenv(pulumiTestAgentPulumiDirEnvVar, agentPulumiDir)
	require.NoError(t, os.Mkdir(agentPulumiDir, 0o777))

	dir, err := getAgentPulumiDir()
	require.NoError(t, err)
	assert.Equal(t, agentPulumiDir, dir)
	info, err := os.Stat(agentPulumiDir)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o700), info.Mode().Perm())
}
