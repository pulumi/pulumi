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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolateSecureCredentials(t *testing.T, mode string) *fakeKeyStore {
	t.Helper()
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_STORE", mode)
	return useFakeStores(t)
}

func testCreds() Credentials {
	return Credentials{
		Current:  "https://api.pulumi.com",
		Accounts: map[string]Account{"https://api.pulumi.com": {AccessToken: "pul-secret-token"}},
	}
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestStoreCredentialsEncryptsInAutoMode(t *testing.T) {
	isolateSecureCredentials(t, "auto")

	require.NoError(t, StoreCredentials(testCreds()))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw))
	assert.False(t, bytes.Contains(raw, []byte("pul-secret-token")), "token must not be on disk in plaintext")

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestStoreCredentialsPlaintextByDefault(t *testing.T) {
	isolateSecureCredentials(t, "")

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
	isolateSecureCredentials(t, "plaintext")

	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestPlaintextFileMigratesOnWriteNotRead(t *testing.T) {
	isolateSecureCredentials(t, "auto")

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile, []byte(
		`{"current":"https://api.pulumi.com","accounts":{"https://api.pulumi.com":{"accessToken":"pul-legacy"}}}`,
	), 0o600))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-legacy", creds.Accounts["https://api.pulumi.com"].AccessToken)
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
	assert.Equal(t, "pul-legacy", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestNoMigrationWhenModeUnset(t *testing.T) {
	isolateSecureCredentials(t, "")

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	plaintext := []byte(`{"current":"x","accounts":{"x":{"accessToken":"tok"}}}`)
	require.NoError(t, os.WriteFile(credsFile, plaintext, 0o600))

	_, err = GetStoredCredentials()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.False(t, securestore.IsEnvelope(raw), "no migration without opt-in")
}

func TestEncryptedFileReadableRegardlessOfMode(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	// Reads always use the envelope's recorded backend.
	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestLostKeyProducesActionableError(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, st.DeleteKey())

	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pulumi login")
}

func TestDeleteAllAccountsKeepsKeySharedWithOtherHomes(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	homeA := os.Getenv("PULUMI_HOME")
	require.NoError(t, StoreCredentials(testCreds()))

	t.Setenv("PULUMI_HOME", t.TempDir())
	require.NoError(t, StoreCredentials(testCreds()))
	require.NoError(t, DeleteAllAccounts())

	t.Setenv("PULUMI_HOME", homeA)
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsDropsCorruptKey(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	st.getErr = fmt.Errorf("%w: unrecognized format", securestore.ErrKeyCorrupt)

	require.NoError(t, DeleteAllAccounts())

	_, err := st.GetKey()
	assert.ErrorIs(t, err, securestore.ErrKeyNotFound)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsKeepsKeyOnTransientError(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	st.getErr = errors.New("dbus timeout")

	require.NoError(t, DeleteAllAccounts())

	st.getErr = nil
	_, err := st.GetKey()
	require.NoError(t, err)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteCredentialsKeyRemovesKey(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, DeleteAllAccountsAndCredentialsKey())

	_, err := st.GetKey()
	assert.ErrorIs(t, err, securestore.ErrKeyNotFound)
}

func TestDeleteCredentialsKeyWithoutEnvelopeOrMode(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_CREDENTIAL_STORE", "")

	require.NoError(t, DeleteAllAccountsAndCredentialsKey())
	_, err := st.GetKey()
	assert.ErrorIs(t, err, securestore.ErrKeyNotFound)
}

//nolint:paralleltest // isolateSecureCredentials changes environment variables and installs a fake store.
func TestAgentCredentialsEncryptedToo(t *testing.T) {
	isolateSecureCredentials(t, "auto")

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

// Makes the attended check pass on CI hosts and headless machines.
func forceAttended(t *testing.T) {
	t.Helper()
	prev := cmdutil.DisableInteractive
	cmdutil.DisableInteractive = false
	t.Cleanup(func() { cmdutil.DisableInteractive = prev })
	t.Setenv("CI", "")
	t.Setenv("PULUMI_DISABLE_CI_DETECTION", "1")
	if runtime.GOOS == "linux" {
		t.Setenv("DISPLAY", ":0")
	}
}

// withStderrCapture runs fn with os.Stderr redirected and returns what it
// wrote. The warnings under test are far smaller than the pipe buffer, so fn
// cannot block on the unread pipe.
func withStderrCapture(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	prev := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = prev }()
	fn()
	os.Stderr = prev
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(data)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestAutoFallbackToPlaintextIsQuiet(t *testing.T) {
	// Auto without a usable store resolves to plaintext by design: that is
	// the promised best effort, not worth a warning.
	st := isolateSecureCredentials(t, "auto")
	forceAttended(t)
	st.absent = true

	out := withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Empty(t, out)

	// Rewrites of the resulting plaintext file are just as quiet.
	out = withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Empty(t, out)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestKeyFailureFallbackWarnsWithReason(t *testing.T) {
	// A store that resolved as available but failed its key operation lands
	// the write on plaintext unexpectedly — that does deserve a warning.
	st := isolateSecureCredentials(t, "auto")
	forceAttended(t)
	st.createErr = errors.New("keychain hiccup")

	out := withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Contains(t, out, "plaintext")
	assert.Contains(t, out, "keychain hiccup", "the warning must carry the reason")
}

func TestRecoveryDowngradeToPlaintextWarns(t *testing.T) {
	// A previously encrypted user whose store disappeared between losing the
	// key and logging back in is being downgraded — warn about it.
	st := isolateSecureCredentials(t, "auto")
	forceAttended(t)
	require.NoError(t, StoreCredentials(testCreds()))
	require.NoError(t, st.DeleteKey())

	t.Setenv("PULUMI_CREDENTIAL_STORE", "")
	require.NoError(t, ResetStoredCredentials())
	st.absent = true

	out := withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Contains(t, out, "plaintext")
}

func TestUnsetModePreservesExistingEncryption(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	t.Setenv("PULUMI_CREDENTIAL_STORE", "")

	updated := testCreds()
	updated.Accounts["https://api.other.com"] = Account{AccessToken: "pul-second-token"}
	require.NoError(t, StoreCredentials(updated))

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), "encryption must be sticky when no mode is configured")
	assert.False(t, bytes.Contains(raw, []byte("pul-second-token")))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-second-token", creds.Accounts["https://api.other.com"].AccessToken)
}

func TestExplicitPlaintextModeDowngrades(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")

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
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, st.DeleteKey())
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.True(t, IsUndecryptableCredentials(err))

	// Storing a fresh account is what login does after authenticating.
	require.NoError(t, StoreAccount("https://api.pulumi.com",
		Account{AccessToken: "pul-fresh-token"}, true))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-fresh-token", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestResetStoredCredentialsClearsUndecryptableState(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, st.DeleteKey())

	require.NoError(t, ResetStoredCredentials())

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	_, err = os.Stat(credsFile)
	assert.True(t, os.IsNotExist(err))

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Empty(t, creds.Accounts)
}

func TestDeleteAllAccountsAndCredentialsKeyUsesEnvelopeBackend(t *testing.T) {
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_STORE", "auto")
	promote := useUpgradableStores(t)
	require.NoError(t, StoreCredentials(testCreds()))
	promote()

	require.NoError(t, DeleteAllAccountsAndCredentialsKey())

	weak, err := stores.ForBackend(fakeBackend)
	require.NoError(t, err)
	_, err = weak.GetKey()
	assert.ErrorIs(t, err, securestore.ErrKeyNotFound)
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	_, err = os.Stat(credsFile)
	assert.True(t, os.IsNotExist(err))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsAndCredentialsKeyKeepsKeyWhenFileRemains(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.Remove(credsFile))
	require.NoError(t, os.MkdirAll(filepath.Join(credsFile, "undeletable"), 0o700))

	require.Error(t, DeleteAllAccountsAndCredentialsKey())

	_, err = st.GetKey()
	require.NoError(t, err)
}

func TestDeleteCredentialsKeyForUnparseableEnvelopeRegardlessOfMode(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile, futureEnvelope(t), 0o600))

	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")

	require.NoError(t, DeleteAllAccountsAndCredentialsKey())
	_, err = st.GetKey()
	assert.ErrorIs(t, err, securestore.ErrKeyNotFound)
}

func TestResetStoredCredentialsKeepsKeySharedWithOtherHomes(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	homeA := os.Getenv("PULUMI_HOME")
	require.NoError(t, StoreCredentials(testCreds()))

	t.Setenv("PULUMI_HOME", t.TempDir())
	otherKey := make([]byte, 32)
	foreign, err := securestore.Seal(otherKey, fakeBackend, []byte(`{"accounts":{"x":{"accessToken":"tok"}}}`))
	require.NoError(t, err)
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile, foreign, 0o600))
	_, err = GetStoredCredentials()
	require.True(t, IsUndecryptableCredentials(err))

	require.NoError(t, ResetStoredCredentials())

	t.Setenv("PULUMI_HOME", homeA)
	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestDeleteAllAccountsWorksWhenUndecryptable(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, st.DeleteKey())

	require.NoError(t, DeleteAllAccounts(), "logout --all must not require reading the file")

	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	_, err = os.Stat(credsFile)
	assert.True(t, os.IsNotExist(err))
}

func futureEnvelope(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	env, err := securestore.Seal(key, fakeBackend, []byte(`{"accounts":{"x":{"accessToken":"tok"}}}`))
	require.NoError(t, err)
	future := bytes.Replace(env, []byte(`"$pulumiSecureStore": 1`), []byte(`"$pulumiSecureStore": 99`), 1)
	require.NotEqual(t, env, future)
	return future
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestFutureEnvelopeIsNotReadAsLoggedOut(t *testing.T) {
	isolateSecureCredentials(t, "")
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
		isolateSecureCredentials(t, mode)
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

//nolint:paralleltest // isolateSecureCredentials changes environment variables and installs a fake store.
func TestDeclinedUnlockNeverWritesPlaintext(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
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
	// Their credentials are intact and one click away.
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	st.declineErr = securestore.ErrDeclined
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.ErrorIs(t, err, securestore.ErrDeclined)
	assert.NotContains(t, err.Error(), "re-authenticate")
	// Not UndecryptableCredentialsError: login reacts by deleting the file.
	assert.False(t, IsUndecryptableCredentials(err))
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestUnusableStoreIsNotTreatedAsUnreadableCredentials(t *testing.T) {
	// A store unusable right now must not authorise replacing the file.
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	st.absent = true
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.False(t, IsUndecryptableCredentials(err),
		"a store that is unusable right now must not authorise replacing the file")
	assert.Contains(t, err.Error(), "not usable here")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestTransientKeyErrorIsNotTreatedAsUnreadableCredentials(t *testing.T) {
	// An unexpected failure from an available store may be transient, so it
	// must not authorise `pulumi login` to replace the file.
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	st.getErr = errors.New("dbus timeout")
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.False(t, IsUndecryptableCredentials(err))
	assert.Contains(t, err.Error(), "Retry")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestLostKeyStillAllowsRecovery(t *testing.T) {
	// The key is genuinely gone, so login may replace the data.
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, st.DeleteKey())
	_, err := GetStoredCredentials()
	require.Error(t, err)
	assert.True(t, IsUndecryptableCredentials(err))
}

func TestDeclinedUnlockOnStickyWriteKeepsTheEnvelope(t *testing.T) {
	// Must fail rather than fall through to "refusing to overwrite".
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	before, err := os.ReadFile(credsFile)
	require.NoError(t, err)

	t.Setenv("PULUMI_CREDENTIAL_STORE", "")
	st.declineErr = securestore.ErrDeclined

	err = StoreCredentials(testCreds())
	require.Error(t, err)
	assert.ErrorIs(t, err, securestore.ErrDeclined)
	after, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the encrypted file must be left untouched")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestInvalidModeSurfacesOnWrite(t *testing.T) {
	isolateSecureCredentials(t, "bogus")
	err := StoreCredentials(testCreds())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PULUMI_CREDENTIAL_STORE")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestInvalidModeSurfacesOnRead(t *testing.T) {
	isolateSecureCredentials(t, "bogus")
	_, err := GetStoredCredentials()
	require.Error(t, err, "read-only commands must reject an invalid mode too")
	assert.Contains(t, err.Error(), "PULUMI_CREDENTIAL_STORE")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestModeIsCaseInsensitive(t *testing.T) {
	isolateSecureCredentials(t, "OS")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.True(t, securestore.IsEnvelope(raw), `"OS" must mean "os", not silent plaintext`)
}

func TestAgentFallbackSurfacesUndecryptableCredentials(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	t.Setenv(pulumiTestAllowAgentFallbackEnvVar, "true")
	t.Setenv("CODEX_SANDBOX", "1")

	cloudURL := "https://api.undecryptable.example.com"
	require.NoError(t, StoreAccount(cloudURL, Account{AccessToken: "tok"}, true))

	require.NoError(t, st.DeleteKey())

	_, _, err := GetAccountWithAgentFallback(cloudURL)
	require.Error(t, err, "agent fallback must not swallow the undecryptable error")
	assert.True(t, IsUndecryptableCredentials(err))
}

func TestWriteUpgradesToStrongerBackend(t *testing.T) {
	// Data must be re-encrypted under a stronger backend once one appears,
	// staying readable throughout via the envelope's recorded backend.
	ptesting.IsolateCredentials(t)
	t.Setenv("PULUMI_CREDENTIAL_STORE", "auto")
	promote := useUpgradableStores(t)

	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	raw, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	backend, err := securestore.EnvelopeBackend(raw)
	require.NoError(t, err)
	require.Equal(t, fakeBackend, backend)

	promote()

	creds, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", creds.Accounts["https://api.pulumi.com"].AccessToken)

	creds.Accounts["https://api.other.com"] = Account{AccessToken: "pul-second-token"}
	require.NoError(t, StoreCredentials(creds))

	raw, err = os.ReadFile(credsFile)
	require.NoError(t, err)
	backend, err = securestore.EnvelopeBackend(raw)
	require.NoError(t, err)
	assert.Equal(t, fakeStrongBackend, backend, "write must upgrade to the stronger backend")
	assert.False(t, bytes.Contains(raw, []byte("pul-second-token")))

	upgraded, err := GetStoredCredentials()
	require.NoError(t, err)
	assert.Equal(t, "pul-secret-token", upgraded.Accounts["https://api.pulumi.com"].AccessToken, "existing data preserved")
	assert.Equal(t, "pul-second-token", upgraded.Accounts["https://api.other.com"].AccessToken)

	// Left in place: the shared agent file may still be encrypted under it.
	weak, err := stores.ForBackend(fakeBackend)
	require.NoError(t, err)
	_, err = weak.GetKey()
	require.NoError(t, err)
}

func TestRecoveryFromLostKeyStaysEncrypted(t *testing.T) {
	// Recovery is not the explicit "plaintext" opt-out.
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))

	require.NoError(t, st.DeleteKey())
	_, err := GetStoredCredentials()
	require.True(t, IsUndecryptableCredentials(err))

	t.Setenv("PULUMI_CREDENTIAL_STORE", "")
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
	assert.Equal(t, "pul-secret-token", creds.Accounts["https://api.pulumi.com"].AccessToken)
}

func TestRecoveryInExplicitPlaintextModeWritesPlaintext(t *testing.T) {
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	require.NoError(t, st.DeleteKey())

	t.Setenv("PULUMI_CREDENTIAL_STORE", "plaintext")
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
		isolateSecureCredentials(t, mode)
		forceAttended(t)
		credsFile, err := getCredsFilePath()
		require.NoError(t, err)
		plaintext := []byte(`{"current":"x","accounts":{"x":{"accessToken":"tok"}}}`)
		require.NoError(t, os.WriteFile(credsFile, plaintext, 0o600))

		out := withStderrCapture(t, func() {
			_, err = GetStoredCredentials()
			require.NoError(t, err)
		})
		assert.Contains(t, out, "plaintext", "mode %q must warn about a plaintext file", mode)

		raw, err := os.ReadFile(credsFile)
		require.NoError(t, err)
		assert.Equal(t, plaintext, raw, "the warning must not come with a rewrite")

		out = withStderrCapture(t, func() {
			_, err = GetStoredCredentials()
			require.NoError(t, err)
		})
		assert.Empty(t, out, "the warning fires once per process")
	}
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestMigrationConfirmsEncryption(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	forceAttended(t)
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile,
		[]byte(`{"current":"x","accounts":{"x":{"accessToken":"tok"}}}`), 0o600))

	out := withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Contains(t, out, "now encrypted")

	out = withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Empty(t, out, "only the migrating write confirms")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestFreshEncryptedWriteDoesNotClaimMigration(t *testing.T) {
	isolateSecureCredentials(t, "auto")
	forceAttended(t)

	out := withStderrCapture(t, func() {
		require.NoError(t, StoreCredentials(testCreds()))
	})
	assert.Empty(t, out, "nothing was migrated, so there is nothing to confirm")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestSuppressPlaintextPendingWarning(t *testing.T) {
	// `pulumi login` migrates the file itself, so the "next login will
	// encrypt" warning from its own credential reads must stay silent.
	isolateSecureCredentials(t, "os")
	forceAttended(t)
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile,
		[]byte(`{"current":"x","accounts":{"x":{"accessToken":"tok"}}}`), 0o600))

	SuppressPlaintextPendingWarning()
	out := withStderrCapture(t, func() {
		_, err = GetStoredCredentials()
		require.NoError(t, err)
	})
	assert.Empty(t, out)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestPendingWarningRequiresUsableStore(t *testing.T) {
	// With no usable store the next login falls back to plaintext, so
	// promising "will encrypt" would be false — and would nag every command.
	st := isolateSecureCredentials(t, "auto")
	forceAttended(t)
	st.absent = true
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile,
		[]byte(`{"current":"x","accounts":{"x":{"accessToken":"tok"}}}`), 0o600))

	out := withStderrCapture(t, func() {
		_, err = GetStoredCredentials()
		require.NoError(t, err)
	})
	assert.Empty(t, out)
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestKeyFailureOnAvailableStoreKeepsTheEnvelope(t *testing.T) {
	// A store that resolves but then fails its key operation must not
	// downgrade an encrypted file to plaintext, even in auto mode.
	st := isolateSecureCredentials(t, "auto")
	require.NoError(t, StoreCredentials(testCreds()))
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	before, err := os.ReadFile(credsFile)
	require.NoError(t, err)

	st.createErr = errors.New("keychain hiccup")

	err = StoreCredentials(testCreds())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keychain hiccup")
	after, err := os.ReadFile(credsFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the encrypted file must be left untouched")
}

//nolint:paralleltest // t.Setenv and the package-global secure-store mock forbid parallel runs
func TestUnsetModePlaintextReadDoesNotWarn(t *testing.T) {
	isolateSecureCredentials(t, "")
	forceAttended(t)
	credsFile, err := getCredsFilePath()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsFile, []byte(`{"current":"x","accounts":{"x":{"accessToken":"tok"}}}`), 0o600))

	out := withStderrCapture(t, func() {
		_, err = GetStoredCredentials()
		require.NoError(t, err)
	})
	assert.Empty(t, out, "plaintext is the expected default without opt-in")
}
