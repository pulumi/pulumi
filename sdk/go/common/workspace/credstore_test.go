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

package workspace

import (
	"bytes"
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinSecureCreds points credential paths at a temp dir, installs the
// in-memory secure store, and selects the given PULUMI_CREDENTIAL_STORE mode.
func pinSecureCreds(t *testing.T, mode string) {
	t.Helper()
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_STORE", mode)
	securestore.MockInit(t)
	resetWriteStoreForTesting()
	t.Cleanup(resetWriteStoreForTesting)
}

func testCreds() Credentials {
	return Credentials{
		Current:      "https://api.pulumi.com",
		AccessTokens: map[string]string{"https://api.pulumi.com": "pul-secret-token"},
	}
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestStoreCredentialsEncryptsInAutoMode(t *testing.T) {
	pinSecureCreds(t, "auto")

	require.NoError(t, StoreCredentials(testCreds()))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw))
	assert.False(t, bytes.Contains(raw, []byte("pul-secret-token")), "token must not be on disk in plaintext")

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.AccessTokens["https://api.pulumi.com"])
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestStoreCredentialsPlaintextByDefault(t *testing.T) {
	pinSecureCreds(t, "")

	require.NoError(t, StoreCredentials(testCreds()))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw), "unset mode keeps today's plaintext behavior")
	assert.Contains(t, string(raw), "pul-secret-token")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestStoreCredentialsPlaintextModeExplicit(t *testing.T) {
	pinSecureCreds(t, "plaintext")

	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestGetStoredCredentialsMigratesPlaintextOnDemand(t *testing.T) {
	pinSecureCreds(t, "auto")

	// Simulate a pre-existing plaintext credentials file from an older CLI.
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile,
		[]byte(`{"current":"https://api.pulumi.com","accessTokens":{"https://api.pulumi.com":"pul-legacy"}}`), 0o600))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-legacy", creds.AccessTokens["https://api.pulumi.com"])

	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), "plaintext file must be migrated to an envelope on read")
	assert.False(t, bytes.Contains(raw, []byte("pul-legacy")))

	// And it still reads back after migration.
	creds, err = GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-legacy", creds.AccessTokens["https://api.pulumi.com"])
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestNoMigrationWhenModeUnset(t *testing.T) {
	pinSecureCreds(t, "")

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	plaintext := []byte(`{"current":"x","accessTokens":{"x":"tok"}}`)
	require.NoError(t, os.WriteFile(credsFile, plaintext, 0o600))

	_, err = GetStoredCredentials()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw), "no migration without opt-in")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestEncryptedFileReadableRegardlessOfMode(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	// Even with the mode flipped to plaintext, an existing envelope must
	// still be readable (reads always use the envelope's recorded backend).
	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
	resetWriteStoreForTesting()

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.AccessTokens["https://api.pulumi.com"])
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestLostKeyProducesActionableError(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	st, err := securestore.Resolve(securestore.ModeAuto)
	require.NoError(t, err)
	require.NoError(t, st.DeleteKey())

	_, err = GetStoredCredentials()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pulumi login")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsRemovesKeyAndState(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	st, err := securestore.Resolve(securestore.ModeAuto)
	require.NoError(t, err)
	_, err = st.GetKey()
	require.NoError(t, err, "key exists after storing")

	require.NoError(t, DeleteAllAccounts())

	_, err = st.GetKey()
	assert.ErrorIs(t, err, securestore.ErrKeyNotFound)
	statePath, err := credStoreStatePath()
	require.NoError(t, err)
	_, err = os.Stat(statePath)
	assert.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestAgentCredentialsEncryptedToo(t *testing.T) {
	pinSecureCreds(t, "auto")
	t.Setenv("PULUMI_TEST_AGENT_PULUMI_DIR", t.TempDir())

	require.NoError(t, StoreAgentCredentials(testCreds()))

	credsFile, err := getAgentCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), "agent credentials use the same secure write path")

	account, err := GetAgentAccount("https://api.pulumi.com")
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", account.AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestWarnPlaintextFallbackOnlyOnce(t *testing.T) {
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	// Force non-headless so the warning is not suppressed.
	t.Setenv("CI", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")
	t.Setenv("AI_AGENT", "")
	if headlessEnvironment() {
		t.Skip("cannot force a non-headless environment here (agent env vars present)")
	}

	var buf bytes.Buffer
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = os.Stderr })

	warnPlaintextFallback(securestore.ErrUnavailable)
	first := buf.String()
	assert.Contains(t, first, "plaintext")
	assert.Contains(t, first, "PULUMI_CREDENTIAL_STORE")

	buf.Reset()
	warnPlaintextFallback(securestore.ErrUnavailable)
	assert.Empty(t, buf.String())
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestInvalidModeSurfacesOnWrite(t *testing.T) {
	pinSecureCreds(t, "bogus")
	err := StoreCredentials(testCreds())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PULUMI_CREDENTIAL_STORE")
}
