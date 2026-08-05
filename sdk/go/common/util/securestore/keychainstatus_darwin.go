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

// The default (login) keychain can genuinely be locked: `security
// lock-keychain`, the lock-after-inactivity/lock-on-sleep keychain settings,
// SSH sessions authenticated by key (the PAM unlock never ran), or a keychain
// password that diverged from the login password. Both macOS tiers consult
// the lock state before an operation that could make securityd draw a dialog.

// copyDefaultKeychain returns the default keychain ref (caller releases) or
// ok=false when it cannot be determined.
func copyDefaultKeychain() (kc uintptr, ok bool) {
	if loadDarwinAPI() != nil {
		return 0, false
	}
	if status := sec.keychainCopyDefault(&kc); status != errSecSuccess || kc == 0 {
		return 0, false
	}
	return kc, true
}

// defaultKeychainLocked reports whether the default keychain is locked,
// without any risk of UI. ok=false when the state cannot be determined.
// Indirected through a hook so tests can exercise the locked branches
// without locking the developer's own keychain.
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
// Keychain operations are rare enough that a global lock costs nothing.
var interactionMu sync.Mutex

// withoutKeychainUI runs fn with securityd's dialogs suppressed for this
// process, so a locked keychain or an ACL mismatch returns
// errSecInteractionNotAllowed instead of drawing UI.
//
// SecKeychainSetUserInteractionAllowed is deprecated, and the modern
// replacement (kSecUseAuthenticationContext holding an LAContext with
// interactionNotAllowed) governs only data-protection keychain items — it was
// measured to be inert for the legacy keychain item this backend must use,
// which drew the dialog it was supposed to suppress. This is the only lever
// that works for legacy items, so it stays until the item can move.
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
	// Restore unconditionally: leaving interaction disabled would silently
	// break every later keychain user in this process.
	defer sec.keychainSetUserInteractionOK(true) //nolint:errcheck // best effort restore
	return fn()
}

// keychainPrecheck classifies the default keychain before an operation that
// could make securityd draw a dialog. Both macOS tiers go through it, so the
// unlock prompt policy is enforced in one place.
//
// A locked keychain in a silent cell is reported as Locked without touching
// it. With prompting permitted it gets exactly one unlock attempt, waited on
// with no deadline because the user may be typing a password — the same rule
// the Linux Secret Service path follows. An unknown lock state is treated as
// usable and left to the operation itself.
func keychainPrecheck(allowPrompt bool) (Outcome, error) {
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
