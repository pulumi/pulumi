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
	// Never change: encrypted files reference this exact item. The account
	// differs from keyringAccount so the two macOS backends never collide —
	// this item's ACL is ours alone, the fallback must stay readable by
	// /usr/bin/security.
	nativeService = "Pulumi CLI"
	nativeAccount = "credentials-key-native"
	nativeLabel   = "Pulumi CLI credentials key"
)

// The per-app ACL comes from SecItemAdd itself, which binds access to the
// creating app's code-signing identity.
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

// Deliberately minimal: no synchronizable attribute (never iCloud-synced) and
// no explicit access attributes — SecItemAdd's default ACL is what we want,
// and setting access attributes prompts.
type nativeStore struct {
	service, account, label string
	allowPrompt             bool
}

// Silent ops suppress dialogs even though the precheck just read the lock
// state: the keychain can lock in between (sleep, inactivity, screen lock).
// Prompt-permitted ops run untimed, since the user may be typing a password.
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

// Suppresses UI whatever the prompt policy: deciding usability must never
// itself draw a dialog.
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

// Requests the data, not just attributes: attributes stay readable on a
// locked keychain, so only a data read exposes the lock.
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
			// Data only — re-setting access attributes on an existing item prompts.
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
