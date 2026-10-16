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

// The login keychain is genuinely lockable: lock-keychain, lock-on-sleep,
// key-authenticated SSH (no PAM unlock), or a diverged keychain password.

// Caller releases the ref. Not ok also means the deprecated lookup is gone,
// which reads as "lock state unknown" rather than failing the tier.
func copyDefaultKeychain() (kc uintptr, ok bool) {
	if loadDarwinAPI() != nil || sec.keychainCopyDefault == nil || sec.keychainGetStatus == nil {
		return 0, false
	}
	if status := sec.keychainCopyDefault(&kc); status != errSecSuccess || kc == 0 {
		return 0, false
	}
	return kc, true
}

// Hooked so tests need not lock the developer's own keychain.
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

// Printed before blocking, so a dialog on another space is not seen as a hang.
var notifyWaitingForKeychainUnlock = func() {
	fmt.Fprintln(os.Stderr,
		"Pulumi needs the key protecting your credentials: answer the keychain password prompt to continue.")
}

var interactionMu sync.Mutex

// Deprecated, but the modern replacement (kSecUseAuthenticationContext +
// LAContext) was measured inert for legacy keychain items — it drew the very
// dialog it should suppress.
func withoutKeychainUI[T any](fn func() (T, error)) (T, error) {
	if err := loadDarwinAPI(); err != nil {
		var zero T
		return zero, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if sec.keychainSetUserInteractionOK == nil {
		// Silence cannot be promised, so decline rather than risk a dialog.
		var zero T
		return zero, fmt.Errorf("%w: cannot suppress keychain dialogs", ErrUnavailable)
	}
	interactionMu.Lock()
	defer interactionMu.Unlock()
	if status := sec.keychainSetUserInteractionOK(false); status != errSecSuccess {
		var zero T
		return zero, fmt.Errorf("%w: could not disable keychain dialogs: %v",
			ErrUnavailable, osStatusError(status))
	}
	// Leaving interaction disabled would break every later keychain user.
	defer sec.keychainSetUserInteractionOK(true)
	return fn()
}

// Both macOS tiers go through this before anything that could draw a dialog.
// Locked gets one unlock attempt when permitted, and is reported untouched
// when not; unknown state is left to the operation.
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
	if !ok || sec.keychainUnlock == nil {
		return Locked, fmt.Errorf("%w: it cannot be unlocked from here", ErrLocked)
	}
	defer cf.release(kc)
	notifyWaitingForKeychainUnlock()
	err := osStatusError(sec.keychainUnlock(kc, 0, 0, false))
	if err != nil {
		return outcomeOf(err), err
	}
	return Ready, nil
}
