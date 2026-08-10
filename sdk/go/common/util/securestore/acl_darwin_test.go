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

//go:build darwin

package securestore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withSelfCheckOverride(t *testing.T, fn func() error) {
	t.Helper()
	prev := aclSelfCheckOverride
	aclSelfCheckOverride = fn
	t.Cleanup(func() { aclSelfCheckOverride = prev })
}

// A throwaway item, so tests never touch the real one.
func testACLStore(t *testing.T) *aclStore {
	t.Helper()
	suffix := make([]byte, 8)
	_, err := rand.Read(suffix)
	require.NoError(t, err)
	store := &aclStore{
		service: "Pulumi CLI Test " + hex.EncodeToString(suffix),
		account: "credentials-key-acl-test",
		label:   "Pulumi CLI test item (safe to delete)",
	}
	t.Cleanup(func() {
		// The body skips when the keychain is unusable; nothing to clean up.
		if err := store.delete(); err != nil && !errors.Is(err, ErrUnavailable) {
			t.Errorf("cleaning up test keychain item: %v", err)
		}
	})
	return store
}

func TestACLBackendShape(t *testing.T) {
	t.Parallel()
	b := aclKeychainBackend(true)
	assert.Equal(t, BackendMacOSACL, b.id)
	assert.Equal(t, rawWrapper{}, b.wrap)
	store, ok := b.store.(*aclStore)
	require.True(t, ok)
	assert.Equal(t, "Pulumi CLI", store.service)
	assert.Equal(t, "credentials-key-acl", store.account,
		"must differ from the fallback backend's account to avoid item collisions")
	assert.NotEmpty(t, store.label)
	assert.True(t, store.allowPrompt, "the prompt policy must reach the store")
	silent, ok := aclKeychainBackend(false).store.(*aclStore)
	require.True(t, ok)
	assert.False(t, silent.allowPrompt)
}

//nolint:paralleltest // must not overlap tests that set aclSelfCheckOverride
func TestACLSelfCheckRejectsTestBinary(t *testing.T) {
	require.Nil(t, aclSelfCheckOverride)

	err := aclSelfCheck()
	require.Error(t, err, "an ad-hoc-signed test binary must not pass the self-check")

	outcome, availErr := aclKeychainBackend(false).available()
	require.Error(t, availErr)
	assert.Equal(t, Absent, outcome)
	assert.True(t, errors.Is(availErr, ErrUnavailable), "available() = %v, want ErrUnavailable", availErr)
}

//nolint:paralleltest // mutates package-global aclSelfCheckOverride
func TestACLStoreRoundTrip(t *testing.T) {
	withSelfCheckOverride(t, func() error { return nil })
	store := testACLStore(t)

	if _, err := store.available(); err != nil {
		if errors.Is(err, ErrUnavailable) {
			t.Skipf("keychain not usable in this environment: %v", err)
		}
		t.Fatalf("available() = %v", err)
	}

	_, err := store.get()
	assert.True(t, errors.Is(err, ErrKeyNotFound), "get on missing item = %v, want ErrKeyNotFound", err)
	require.NoError(t, store.delete(), "deleting a missing item is not an error")

	first := formatItem(wrapRaw, []byte("first value \x00\xff with bytes ✓"))
	require.NoError(t, store.set(first))
	got, err := store.get()
	require.NoError(t, err)
	assert.Equal(t, first, got)

	second := formatItem(wrapRaw, []byte("second value"))
	require.NoError(t, store.set(second))
	got, err = store.get()
	require.NoError(t, err)
	assert.Equal(t, second, got)

	_, availableErr := store.available()
	require.NoError(t, availableErr)

	require.NoError(t, store.delete())
	_, err = store.get()
	assert.True(t, errors.Is(err, ErrKeyNotFound), "get after delete = %v, want ErrKeyNotFound", err)
	require.NoError(t, store.delete())
}

//nolint:paralleltest // mutates package-global aclSelfCheckOverride
func TestACLAvailableUsesSelfCheckError(t *testing.T) {
	boom := errors.New("not signed enough")
	withSelfCheckOverride(t, func() error { return boom })
	_, err := (&aclStore{service: "unused", account: "unused"}).available()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnavailable))
	assert.Contains(t, err.Error(), "not signed enough")
}

func TestOSStatusErrorMapping(t *testing.T) {
	t.Parallel()
	require.NoError(t, osStatusError(errSecSuccess))

	assert.True(t, errors.Is(osStatusError(errSecItemNotFound), ErrKeyNotFound))

	for _, status := range []int32{errSecInteractionNotAllowed, errSecInteractionRequired} {
		err := osStatusError(status)
		assert.True(t, errors.Is(err, ErrLocked), "status %d must map to ErrLocked", status)
		assert.True(t, errors.Is(err, ErrUnavailable), "ErrLocked wraps ErrUnavailable")
		assert.Equal(t, Locked, outcomeOf(err), "a locked keychain must classify as Locked, not Absent")
		assert.Contains(t, err.Error(), fmt.Sprintf("keychain error %d", status))
		assert.Contains(t, err.Error(), "locked")
	}

	err := osStatusError(errSecUserCanceled)
	assert.True(t, errors.Is(err, ErrDeclined), "a canceled prompt is a refusal, never a fallback")
	assert.Equal(t, Declined, outcomeOf(err))

	err = osStatusError(errSecNotAvailable)
	assert.True(t, errors.Is(err, ErrUnavailable))
	assert.Contains(t, err.Error(), "keychain error -25291")

	err = osStatusError(errSecAuthFailed) // not a special case
	assert.False(t, errors.Is(err, ErrUnavailable))
	assert.False(t, errors.Is(err, ErrKeyNotFound))
	assert.Contains(t, err.Error(), "keychain error -25293")
}

// With UI suppressed a locked keychain reports errSecAuthFailed, which would
// classify as Absent. Verified interactively on macOS 26.5.
//
//nolint:paralleltest // mutates the package-global lock-state hook
func TestSilentOpsOnLockedKeychainReportLocked(t *testing.T) {
	fakeKeychainLock(t, true)

	store := &aclStore{service: "unused", account: "unused", allowPrompt: false}

	_, err := store.get()
	assert.True(t, errors.Is(err, ErrLocked), "get on a locked keychain = %v, want ErrLocked", err)
	assert.Equal(t, Locked, outcomeOf(err))

	assert.True(t, errors.Is(store.set("value"), ErrLocked))
	assert.True(t, errors.Is(store.delete(), ErrLocked))
}

// Re-arms the memoized precheck, which would otherwise carry a cached answer
// between tests.
func fakeKeychainLock(t *testing.T, locked bool) {
	t.Helper()
	prev := defaultKeychainLockedHook
	defaultKeychainLockedHook = func() (bool, bool) { return locked, true }
	keychainPrecheck = memoizePrecheck(probeKeychain)
	t.Cleanup(func() {
		defaultKeychainLockedHook = prev
		keychainPrecheck = memoizePrecheck(probeKeychain)
	})
}

//nolint:paralleltest // mutates package-global hooks
func TestSilentPathNeverAnnouncesAWait(t *testing.T) {
	fakeKeychainLock(t, true)

	notified := 0
	prevNotify := notifyWaitingForKeychainUnlock
	notifyWaitingForKeychainUnlock = func() { notified++ }
	t.Cleanup(func() { notifyWaitingForKeychainUnlock = prevNotify })

	store := &aclStore{service: "unused", account: "unused", allowPrompt: false}
	_, _ = store.get()
	_ = store.set("value")
	assert.Zero(t, notified, "a silent operation announces nothing because it never waits")
}

// Losing UI suppression means silence cannot be promised, so the tier must
// step aside rather than risk a dialog.
//
//nolint:paralleltest // mutates the package-global bindings
func TestMissingUISuppressionDisablesSilentOps(t *testing.T) {
	require.NoError(t, loadDarwinAPI())
	prev := sec.keychainSetUserInteractionOK
	sec.keychainSetUserInteractionOK = nil
	t.Cleanup(func() { sec.keychainSetUserInteractionOK = prev })

	store := &aclStore{service: "unused", account: "unused", allowPrompt: false}
	_, err := store.get()
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Contains(t, err.Error(), "suppress")

	outcome, probeErr := store.probe()
	assert.Equal(t, Absent, outcome, "the tier must step aside, not claim to be usable")
	assert.ErrorIs(t, probeErr, ErrUnavailable)
}

//nolint:paralleltest // mutates the package-global bindings
func TestMissingLockLookupReadsAsUnknown(t *testing.T) {
	require.NoError(t, loadDarwinAPI())
	prev := sec.keychainGetStatus
	sec.keychainGetStatus = nil
	t.Cleanup(func() { sec.keychainGetStatus = prev })

	locked, ok := realDefaultKeychainLocked()
	assert.False(t, ok, "an absent lookup is unknown state, not a false answer")
	assert.False(t, locked)
}
