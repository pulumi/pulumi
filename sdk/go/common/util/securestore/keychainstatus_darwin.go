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
	"fmt"
	"os"
	"sync"
)

// The login keychain can genuinely be locked: lock-keychain, the
// lock-on-sleep/inactivity settings, key-authenticated SSH (the PAM unlock
// never ran), or a keychain password diverged from the login password.

// The caller releases the returned ref.
func copyDefaultKeychain() (kc uintptr, ok bool) {
	if loadDarwinAPI() != nil {
		return 0, false
	}
	if status := sec.keychainCopyDefault(&kc); status != errSecSuccess || kc == 0 {
		return 0, false
	}
	return kc, true
}

// Hooked so tests can exercise the locked branches without locking the
// developer's own keychain.
var defaultKeychainLockedHook = realDefaultKeychainLocked

func defaultKeychainLocked() (locked, ok bool) { return defaultKeychainLockedHook() }

func realDefaultKeychainLocked() (locked, ok bool) {
	kc, ok := copyDefaultKeychain()
	if !ok {
		return false, false
	}
	defer cf.release(kc)
	var status uint32
	if s := sec.keychainGetStatus(kc, &status); s != errSecSuccess {
		return false, false
	}
	return status&kSecUnlockStateStatus == 0, true
}

// notifyWaitingForKeychainUnlock is printed before an operation that is about
// to wait on securityd's password dialog, so a dialog on another space does
// not look like a hang. Swapped in tests.
var notifyWaitingForKeychainUnlock = func() {
	fmt.Fprintln(os.Stderr,
		"Pulumi needs the key protecting your credentials: answer the keychain password prompt to continue.")
}

// interactionMu serializes the process-wide user-interaction setting below.
var interactionMu sync.Mutex

// withoutKeychainUI suppresses securityd's dialogs process-wide, so a locked
// keychain or an ACL mismatch errors instead of drawing UI.
//
// SecKeychainSetUserInteractionAllowed is deprecated, but the modern
// replacement (kSecUseAuthenticationContext + LAContext) was measured to be
// inert for legacy keychain items and drew the dialog it should suppress.
func withoutKeychainUI[T any](fn func() (T, error)) (T, error) {
	if err := loadDarwinAPI(); err != nil {
		var zero T
		return zero, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	interactionMu.Lock()
	defer interactionMu.Unlock()
	if status := sec.keychainSetUserInteractionOK(false); status != errSecSuccess {
		var zero T
		return zero, fmt.Errorf("%w: could not disable keychain dialogs: %v",
			ErrUnavailable, osStatusError(status))
	}
	// Leaving interaction disabled would break every later keychain user.
	defer sec.keychainSetUserInteractionOK(true) //nolint:errcheck // best effort restore
	return fn()
}

// keychainPrecheck classifies the default keychain before anything that could
// draw a dialog; both macOS tiers go through it. Locked plus silent reports
// Locked untouched, locked plus permitted gets one unlock attempt with no
// deadline, and an unknown state is left to the operation.
var keychainPrecheck = memoizePrecheck(probeKeychain)

func probeKeychain(allowPrompt bool) (Outcome, error) {
	locked, ok := defaultKeychainLocked()
	if !ok || !locked {
		return Ready, nil
	}
	if !allowPrompt {
		return Locked, fmt.Errorf("%w: unlock it or set PULUMI_CREDENTIAL_STORE=plaintext", ErrLocked)
	}
	kc, ok := copyDefaultKeychain()
	if !ok {
		return Ready, nil
	}
	defer cf.release(kc)
	notifyWaitingForKeychainUnlock()
	err := osStatusError(sec.keychainUnlock(kc, 0, 0, false))
	if err != nil {
		return outcomeOf(err), err
	}
	return Ready, nil
}
