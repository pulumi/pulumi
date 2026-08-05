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
	"errors"
	"fmt"
)

const (
	// nativeService/nativeAccount identify the native backend's keychain
	// item. Never change them: existing users' encrypted files reference
	// this exact item. The account deliberately differs from keyringAccount
	// ("credentials-key") so the native and /usr/bin/security backends never
	// collide on one keychain item: this item's ACL is bound to our signing
	// identity, while the fallback item must stay readable by
	// /usr/bin/security.
	nativeService = "Pulumi CLI"
	nativeAccount = "credentials-key-native"
	nativeLabel   = "Pulumi CLI credentials key"
)

// nativeKeychainBackend returns the native SecItem keychain backend. Its
// availability is gated on the running binary carrying a real code signature
// (see nativeSelfCheck); the item's per-app ACL comes from SecItemAdd itself,
// which binds access to the creating app's code-signing identity. allowPrompt
// carries the unlock prompt policy: whether operations may let securityd draw
// its unlock or ACL-confirmation dialogs.
func nativeKeychainBackend(allowPrompt bool) backendImpl {
	return backendImpl{
		id: BackendMacOSACL,
		store: &nativeStore{
			service:     nativeService,
			account:     nativeAccount,
			label:       nativeLabel,
			allowPrompt: allowPrompt,
		},
		wrap: rawWrapper{},
	}
}

// nativeStore is an itemStore over the SecItem API for one generic-password
// item. The item is intentionally minimal: no kSecAttrSynchronizable (so it
// is never iCloud-synced) and no explicit kSecAttrAccess/kSecAttrAccessControl
// (the default ACL from SecItemAdd is exactly the per-app protection wanted,
// and setting access attributes on existing items triggers prompts).
type nativeStore struct {
	service, account, label string
	allowPrompt             bool
}

// runNativeOp applies the unlock prompt policy to one keychain operation.
//
// Silent operations run with securityd's dialogs suppressed process-wide and
// under the usual deadline, so neither a locked keychain nor an ACL mismatch
// can draw UI: both come back as errSecInteractionNotAllowed, classified as
// Locked. Prompt-permitted operations run with dialogs enabled and no
// deadline — the user may take as long as they need to type a password —
// after announcing the wait when the keychain is known to be locked.
func runNativeOp[T any](s *nativeStore, op func() (T, error)) (T, error) {
	if _, err := keychainPrecheck(s.allowPrompt); err != nil {
		var zero T
		return zero, err
	}
	if s.allowPrompt {
		return op()
	}
	return withTimeout(func() (T, error) {
		return withoutKeychainUI(op)
	})
}

// available reports whether the native backend is usable: the binary must
// pass the code-signing self-check and the keychain must answer our item
// query without requiring interaction. Both checks are prompt-free and the
// whole probe is time-bounded regardless of the prompt policy.
func (s *nativeStore) available() (Outcome, error) {
	if _, err := withTimeout(func() (struct{}, error) {
		return struct{}{}, nativeSelfCheck()
	}); err != nil {
		if errors.Is(err, ErrUnavailable) {
			return Absent, err
		}
		return Absent, fmt.Errorf("%w: native keychain requires a signed binary: %v",
			ErrUnavailable, err)
	}
	// Deliberately outside the deadline: a permitted unlock waits on the user.
	if outcome, err := keychainPrecheck(s.allowPrompt); outcome != Ready {
		return outcome, err
	}
	outcome, err := withTimeout(func() (Outcome, error) {
		return s.probe()
	})
	if err != nil && outcome == Ready {
		// withTimeout's zero value for Outcome is Ready; a timeout is Absent.
		outcome = Absent
	}
	return outcome, err
}

// probe reads our item with UI suppressed, whatever the prompt policy: a
// probe decides whether a backend is usable and must never itself draw a
// dialog. A missing item proves the keychain is reachable and unlocked; an
// interaction-required error classifies it as Locked rather than Absent, so
// resolution treats it like a locked keyring instead of falling through.
func (s *nativeStore) probe() (Outcome, error) {
	if err := loadDarwinAPI(); err != nil {
		return Absent, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	status, err := withoutKeychainUI(func() (int32, error) {
		var result uintptr
		status := s.copyMatching(&result)
		if result != 0 {
			cf.release(result)
		}
		return status, nil
	})
	if err != nil {
		return Absent, err
	}
	if status == errSecSuccess || status == errSecItemNotFound {
		return Ready, nil
	}
	statusErr := osStatusError(status)
	if outcome := outcomeOf(statusErr); outcome != Absent {
		return outcome, statusErr
	}
	if errors.Is(statusErr, ErrUnavailable) {
		return Absent, statusErr
	}
	return Absent, fmt.Errorf("%w: %v", ErrUnavailable, statusErr)
}

// copyMatching runs the data-returning item query. Requesting the data (not
// just attributes) is deliberate: item attributes stay readable while the
// keychain is locked, so only a data read makes a locked keychain visible.
func (s *nativeStore) copyMatching(result *uintptr) int32 {
	service := cf.newString(s.service)
	defer cf.release(service)
	account := cf.newString(s.account)
	defer cf.release(account)
	query := cf.newDict(
		[]uintptr{sec.class, sec.attrService, sec.attrAccount, sec.returnData, sec.matchLimit},
		[]uintptr{sec.classGenericPassword, service, account, cf.booleanTrue, sec.matchLimitOne},
	)
	defer cf.release(query)
	return sec.itemCopyMatching(query, result)
}

func (s *nativeStore) get() (string, error) {
	return runNativeOp(s, func() (string, error) {
		if err := loadDarwinAPI(); err != nil {
			return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		var result uintptr
		if status := s.copyMatching(&result); status != errSecSuccess {
			return "", osStatusError(status)
		}
		if result == 0 {
			return "", errors.New("keychain returned no data for the stored item")
		}
		defer cf.release(result)
		return string(cf.dataBytes(result)), nil
	})
}

func (s *nativeStore) set(value string) error {
	_, err := runNativeOp(s, func() (struct{}, error) {
		if err := loadDarwinAPI(); err != nil {
			return struct{}{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		service := cf.newString(s.service)
		defer cf.release(service)
		account := cf.newString(s.account)
		defer cf.release(account)
		label := cf.newString(s.label)
		defer cf.release(label)
		data := cf.newData([]byte(value))
		defer cf.release(data)

		attrs := cf.newDict(
			[]uintptr{sec.class, sec.attrService, sec.attrAccount, sec.attrLabel, sec.valueData},
			[]uintptr{sec.classGenericPassword, service, account, label, data},
		)
		defer cf.release(attrs)

		status := sec.itemAdd(attrs, nil)
		if status == errSecDuplicateItem {
			// Update kSecValueData only. Never touch ACL/access attributes on
			// an existing item: re-setting access triggers authorization
			// prompts, while a pure data update by the item's creator is
			// silent.
			query := cf.newDict(
				[]uintptr{sec.class, sec.attrService, sec.attrAccount},
				[]uintptr{sec.classGenericPassword, service, account},
			)
			defer cf.release(query)
			update := cf.newDict([]uintptr{sec.valueData}, []uintptr{data})
			defer cf.release(update)
			status = sec.itemUpdate(query, update)
		}
		if status != errSecSuccess {
			return struct{}{}, osStatusError(status)
		}
		return struct{}{}, nil
	})
	return err
}

func (s *nativeStore) delete() error {
	_, err := runNativeOp(s, func() (struct{}, error) {
		if err := loadDarwinAPI(); err != nil {
			return struct{}{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		service := cf.newString(s.service)
		defer cf.release(service)
		account := cf.newString(s.account)
		defer cf.release(account)
		query := cf.newDict(
			[]uintptr{sec.class, sec.attrService, sec.attrAccount},
			[]uintptr{sec.classGenericPassword, service, account},
		)
		defer cf.release(query)

		status := sec.itemDelete(query)
		if status == errSecSuccess || status == errSecItemNotFound {
			return struct{}{}, nil
		}
		return struct{}{}, osStatusError(status)
	})
	return err
}
