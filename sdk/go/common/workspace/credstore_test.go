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
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
	useFakeStores(t)
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
func TestPlaintextFileMigratesOnWriteNotRead(t *testing.T) {
	pinSecureCreds(t, "auto")

	// Simulate a pre-existing plaintext credentials file from an older CLI.
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile,
		[]byte(`{"current":"https://api.pulumi.com","accessTokens":{"https://api.pulumi.com":"pul-legacy"}}`), 0o600))

	// Reads must not rewrite the file: read-only commands cannot opt the
	// user into encryption as a side effect.
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-legacy", creds.AccessTokens["https://api.pulumi.com"])
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw), "a read must leave the file untouched")

	require.NoError(t, StoreCredentials(creds))
	raw, err = os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), "the next write must encrypt the file")
	assert.False(t, bytes.Contains(raw, []byte("pul-legacy")))

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

	require.NoError(t, fakeStore(t).DeleteKey())

	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pulumi login")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsRemovesKeyAndState(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	st := fakeStore(t)
	_, err := st.GetKey()
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

// forceAttended pins the tri-state interactivity to "attended" so warning
// tests are deterministic on CI hosts, mirroring --non-interactive=false.
func forceAttended(t *testing.T) {
	t.Helper()
	prevDisable, prevStated := cmdutil.DisableInteractive, cmdutil.InteractivityStated
	cmdutil.DisableInteractive, cmdutil.InteractivityStated = false, true
	t.Cleanup(func() {
		cmdutil.DisableInteractive, cmdutil.InteractivityStated = prevDisable, prevStated
	})
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestWarnPlaintextFallbackOnlyOnce(t *testing.T) {
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	forceAttended(t)

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
func TestUnsetModePreservesExistingEncryption(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	// Re-store with the env var unset: the file must stay encrypted —
	// otherwise every ordinary command would silently downgrade migrated
	// credentials back to plaintext.
	t.Setenv("PULUMI_CREDENTIAL_STORE", "")
	resetWriteStoreForTesting()

	updated := testCreds()
	updated.AccessTokens["https://api.other.com"] = "pul-second-token"
	require.NoError(t, StoreCredentials(updated))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), "encryption must be sticky when no mode is configured")
	assert.False(t, bytes.Contains(raw, []byte("pul-second-token")))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-second-token", creds.AccessTokens["https://api.other.com"])
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestExplicitPlaintextModeDowngrades(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	// The explicit plaintext mode is the deliberate escape hatch: writes
	// decrypt the file back to plaintext.
	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
	resetWriteStoreForTesting()

	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw))
	assert.Contains(t, string(raw), "pul-secret-token")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestStoreAccountRecoversFromUndecryptableFile(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	// Lose the key: reads must fail with the typed error...
	require.NoError(t, fakeStore(t).DeleteKey())
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.True(t, IsUndecryptableCredentials(err))

	// ...but storing a fresh account (what login does after authenticating)
	// replaces the unreadable file instead of failing.
	require.NoError(t, StoreAccount("https://api.pulumi.com",
		Account{AccessToken: "pul-fresh-token"}, true))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-fresh-token", creds.AccessTokens["https://api.pulumi.com"])
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestResetStoredCredentialsClearsUndecryptableState(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, fakeStore(t).DeleteKey())

	require.NoError(t, ResetStoredCredentials())

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	_, err = os.Stat(credsFile)
	assert.True(t, os.IsNotExist(err))

	// A fresh read now behaves like a logged-out machine.
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Empty(t, creds.AccessTokens)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsWorksWhenUndecryptable(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, fakeStore(t).DeleteKey())

	require.NoError(t, DeleteAllAccounts(), "logout --all must not require reading the file")

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	_, err = os.Stat(credsFile)
	assert.True(t, os.IsNotExist(err))
}

func futureEnvelope(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	env, err := securestore.Seal(key, fakeBackend, []byte(`{"accessTokens":{"x":"tok"}}`))
	require.NoError(t, err)
	future := bytes.Replace(env, []byte(`"$pulumiSecureStore": 1`), []byte(`"$pulumiSecureStore": 99`), 1)
	require.NotEqual(t, env, future)
	return future
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestFutureEnvelopeIsNotReadAsLoggedOut(t *testing.T) {
	pinSecureCreds(t, "")
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile, futureEnvelope(t), 0o600))

	_, err = GetStoredCredentials()
	require.Error(t, err, "a future-version envelope must error, not read as empty credentials")
	assert.Contains(t, err.Error(), "newer version")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestWriteRefusesToClobberFutureEnvelope(t *testing.T) {
	for _, mode := range []string{"", "auto"} {
		pinSecureCreds(t, mode)
		credsFile, err := getCredsFilePath()
		require.NoError(t, err)
		future := futureEnvelope(t)
		require.NoError(t, os.WriteFile(credsFile, future, 0o600))

		err = StoreCredentials(testCreds())
		require.Error(t, err, "mode %q must refuse to overwrite a future-version envelope", mode)

		raw, err := os.ReadFile(credsFile)
		require.NoError(t, err)
		assert.Equal(t, future, raw, "the file must be left untouched")
	}
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeclinedUnlockNeverWritesPlaintext(t *testing.T) {
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_STORE", "auto")
	st := useFakeStores(t)
	st.declineErr = securestore.ErrDeclined

	err := StoreCredentials(testCreds())
	require.Error(t, err)
	assert.ErrorIs(t, err, securestore.ErrDeclined)

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	_, statErr := os.Stat(credsFile)
	assert.True(t, os.IsNotExist(statErr), "nothing may be written when the store was refused")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeclinedUnlockOnReadIsNotAdviceToReAuthenticate(t *testing.T) {
	// Dismissing the dialog while reading must say the store was not
	// unlocked, not send the user off to `pulumi login`: their credentials
	// are intact and one click away.
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	fakeStore(t).declineErr = securestore.ErrDeclined
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.ErrorIs(t, err, securestore.ErrDeclined)
	assert.NotContains(t, err.Error(), "re-authenticate")
	// Crucially not an UndecryptableCredentialsError: `pulumi login` reacts to
	// that by deleting the file, so a dismissed dialog would destroy readable
	// credentials and write plaintext in their place.
	assert.False(t, IsUndecryptableCredentials(err))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestUnusableStoreIsNotTreatedAsUnreadableCredentials(t *testing.T) {
	// A locked or absent store says nothing about the credentials: they stay
	// readable once it works again. Reporting this as undecryptable would let
	// `pulumi login` delete the file and write plaintext in its place.
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	fakeStore(t).absent = true
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.False(t, IsUndecryptableCredentials(err),
		"a store that is unusable right now must not authorise replacing the file")
	assert.Contains(t, err.Error(), "not usable here")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestLostKeyStillAllowsRecovery(t *testing.T) {
	// The opposite case: the key is genuinely gone, so the data cannot be
	// recovered and `pulumi login` may replace it.
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, fakeStore(t).DeleteKey())
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.True(t, IsUndecryptableCredentials(err))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeclinedUnlockOnStickyWriteKeepsTheEnvelope(t *testing.T) {
	// An existing envelope plus a refused unlock must fail, not fall through
	// to the "refusing to overwrite" path, and must leave the file alone.
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	before, err := os.ReadFile(credsFile)
	require.NoError(t, err)

	t.Setenv("PULUMI_CREDENTIAL_STORE", "")
	resetWriteStoreForTesting()
	fakeStore(t).declineErr = securestore.ErrDeclined

	err = StoreCredentials(testCreds())
	require.Error(t, err)
	assert.ErrorIs(t, err, securestore.ErrDeclined)
	after, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the encrypted file must be left untouched")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestPlaintextFallbackWarningNamesTheReason(t *testing.T) {
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_STORE", "auto")
	forceAttended(t)
	absent := &fakeKeyStore{backend: fakeBackend, absent: true}
	installStores(t, &fakeStores{
		byBackend: map[securestore.Backend]*fakeKeyStore{fakeBackend: absent},
		preferred: []securestore.Backend{fakeBackend},
	})

	var buf bytes.Buffer
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = os.Stderr })

	require.NoError(t, StoreCredentials(testCreds()))
	assert.Contains(t, buf.String(), "plaintext")
	assert.Contains(t, buf.String(), securestore.ErrUnavailable.Error(),
		"the warning must carry the reason the store was unusable")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestInvalidModeSurfacesOnWrite(t *testing.T) {
	pinSecureCreds(t, "bogus")
	err := StoreCredentials(testCreds())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PULUMI_CREDENTIAL_STORE")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestInvalidModeSurfacesOnRead(t *testing.T) {
	pinSecureCreds(t, "bogus")
	_, err := GetStoredCredentials()
	require.Error(t, err, "read-only commands must reject an invalid mode too")
	assert.Contains(t, err.Error(), "PULUMI_CREDENTIAL_STORE")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestModeIsCaseInsensitive(t *testing.T) {
	pinSecureCreds(t, "OS")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), `"OS" must mean "os", not silent plaintext`)
}

//nolint:paralleltest // t.Setenv, the package-global secure-store mock, and the real credentials file
func TestAgentFallbackSurfacesUndecryptableCredentials(t *testing.T) {
	// The agent fallback must not mask an undecryptable default credentials
	// file as "not logged in" when no usable agent credentials stand in.
	// Follows the established agent-test pattern: temporarily operate on the
	// real credentials path (saved and restored), agent dir redirected.
	useFakeStores(t)

	oldCreds, err := GetStoredCredentials()
	require.NoError(t, err)
	oldAgentPulumiDir := agentPulumiDir
	agentPulumiDir = filepath.Join(t.TempDir(), ".pulumi")
	t.Cleanup(func() {
		t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
		resetWriteStoreForTesting()
		require.NoError(t, StoreCredentials(oldCreds))
		require.NoError(t, DeleteAgentCredentials())
		agentPulumiDir = oldAgentPulumiDir
	})

	setAgentEnv(t)
	t.Setenv(PulumiCredentialsPathEnvVar, "")
	t.Setenv("PULUMI_HOME", "")
	t.Setenv("PULUMI_CREDENTIAL_STORE", "auto")
	resetWriteStoreForTesting()

	cloudURL := "https://api.undecryptable.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "tok"}, true))

	// Lose the key: the file is now an undecryptable envelope.
	require.NoError(t, fakeStore(t).DeleteKey())

	_, _, err = GetAccountWithAgentFallback(cloudURL)
	require.Error(t, err, "agent fallback must not swallow the undecryptable error")
	assert.True(t, IsUndecryptableCredentials(err))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestWriteUpgradesToStrongerBackend(t *testing.T) {
	// The upgrade story behind shipping now and strengthening later: data
	// encrypted under one backend must be silently re-encrypted under a
	// stronger backend once it becomes available (a signed binary unlocking
	// the native macOS keychain, a TPM appearing) on the next write, while
	// staying readable throughout via the envelope's recorded backend.
	t.Setenv(PulumiCredentialsPathEnvVar, t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_STORE", "auto")
	promote := useUpgradableStores(t)

	// Phase 1: only the weaker backend is available.
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	backend, err := securestore.EnvelopeBackend(raw)
	require.NoError(t, err)
	require.Equal(t, fakeBackend, backend)

	// Phase 2: the stronger backend becomes available (new release).
	promote()
	resetWriteStoreForTesting()

	// Reads still work through the recorded (weaker) backend.
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.AccessTokens["https://api.pulumi.com"])

	// The next write upgrades: re-encrypted under the stronger backend with
	// all data preserved, no user action required.
	creds.AccessTokens["https://api.other.com"] = "pul-second-token"
	require.NoError(t, StoreCredentials(creds))

	raw, err = os.ReadFile(credsFile)
	require.NoError(t, err)
	backend, err = securestore.EnvelopeBackend(raw)
	require.NoError(t, err)
	assert.Equal(t, fakeStrongBackend, backend, "write must upgrade to the stronger backend")
	assert.False(t, bytes.Contains(raw, []byte("pul-second-token")))

	upgraded, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", upgraded.AccessTokens["https://api.pulumi.com"], "existing data preserved")
	assert.Equal(t, "pul-second-token", upgraded.AccessTokens["https://api.other.com"])

	// The weaker backend's key is deliberately left in place: the shared
	// agent credentials file may still be encrypted under it.
	weak, err := stores.ForBackend(fakeBackend)
	require.NoError(t, err)
	_, err = weak.GetKey()
	require.NoError(t, err)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestRecoveryFromLostKeyStaysEncrypted(t *testing.T) {
	// Replacing an undecryptable envelope is recovery, not the explicit
	// "plaintext" opt-out: the file `pulumi login` writes back must be an
	// envelope again even when no mode is configured at recovery time.
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, fakeStore(t).DeleteKey())
	_, err := GetStoredCredentials()
	require.True(t, IsUndecryptableCredentials(err))

	t.Setenv("PULUMI_CREDENTIAL_STORE", "")
	resetWriteStoreForTesting()
	require.NoError(t, ResetStoredCredentials())
	require.NoError(t, StoreCredentials(testCreds()))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw),
		"recovery must not downgrade a previously encrypted user to plaintext")

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.AccessTokens["https://api.pulumi.com"])
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestRecoveryInExplicitPlaintextModeWritesPlaintext(t *testing.T) {
	pinSecureCreds(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	require.NoError(t, fakeStore(t).DeleteKey())

	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
	resetWriteStoreForTesting()
	require.NoError(t, ResetStoredCredentials())
	require.NoError(t, StoreCredentials(testCreds()))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw), "explicit plaintext mode governs recovery too")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestOptedInPlaintextReadWarnsOnce(t *testing.T) {
	for _, mode := range []string{"auto", "os"} {
		pinSecureCreds(t, mode)
		forceAttended(t)
		credsFile, err := getCredsFilePath()
		require.NoError(t, err)
		plaintext := []byte(`{"current":"x","accessTokens":{"x":"tok"}}`)
		require.NoError(t, os.WriteFile(credsFile, plaintext, 0o600))

		var buf bytes.Buffer
		warnWriter = &buf
		t.Cleanup(func() { warnWriter = os.Stderr })

		_, err = GetStoredCredentials()
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "plaintext", "mode %q must warn about a plaintext file", mode)

		raw, err := os.ReadFile(credsFile)
		require.NoError(t, err)
		assert.Equal(t, plaintext, raw, "the warning must not come with a rewrite")

		buf.Reset()
		_, err = GetStoredCredentials()
		require.NoError(t, err)
		assert.Empty(t, buf.String(), "the warning fires once per process")
	}
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestUnsetModePlaintextReadDoesNotWarn(t *testing.T) {
	pinSecureCreds(t, "")
	forceAttended(t)
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile, []byte(`{"current":"x","accessTokens":{"x":"tok"}}`), 0o600))

	var buf bytes.Buffer
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = os.Stderr })

	_, err = GetStoredCredentials()
	require.NoError(t, err)
	assert.Empty(t, buf.String(), "plaintext is the expected default without opt-in")
}
