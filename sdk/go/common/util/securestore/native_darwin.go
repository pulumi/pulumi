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
// which binds access to the creating app's code-signing identity.
func nativeKeychainBackend() backendImpl {
	return backendImpl{
		id: BackendMacOSNative,
		store: &nativeStore{
			service: nativeService,
			account: nativeAccount,
			label:   nativeLabel,
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
}

// available reports whether the native backend is usable: the binary must
// pass the code-signing self-check and the keychain must answer our item
// query without requiring interaction. Both checks are prompt-free and the
// whole probe is time-bounded.
func (s *nativeStore) available() error {
	_, err := withTimeout(func() (struct{}, error) {
		if err := nativeSelfCheck(); err != nil {
			if errors.Is(err, ErrUnavailable) {
				return struct{}{}, err
			}
			return struct{}{}, fmt.Errorf("%w: native keychain requires a signed binary: %v",
				ErrUnavailable, err)
		}
		return struct{}{}, s.probe()
	})
	return err
}

// probe asks the keychain about our item without returning data. A missing
// item still proves the keychain is reachable and unlocked.
func (s *nativeStore) probe() error {
	if err := loadDarwinAPI(); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	service := cf.newString(s.service)
	defer cf.release(service)
	account := cf.newString(s.account)
	defer cf.release(account)
	query := cf.newDict(
		[]uintptr{sec.class, sec.attrService, sec.attrAccount, sec.returnData, sec.matchLimit},
		[]uintptr{sec.classGenericPassword, service, account, cf.booleanFalse, sec.matchLimitOne},
	)
	defer cf.release(query)

	status := sec.itemCopyMatching(query, nil)
	if status == errSecSuccess || status == errSecItemNotFound {
		return nil
	}
	err := osStatusError(status)
	if errors.Is(err, ErrUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

func (s *nativeStore) get() (string, error) {
	return withTimeout(func() (string, error) {
		if err := loadDarwinAPI(); err != nil {
			return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		service := cf.newString(s.service)
		defer cf.release(service)
		account := cf.newString(s.account)
		defer cf.release(account)
		query := cf.newDict(
			[]uintptr{sec.class, sec.attrService, sec.attrAccount, sec.returnData, sec.matchLimit},
			[]uintptr{sec.classGenericPassword, service, account, cf.booleanTrue, sec.matchLimitOne},
		)
		defer cf.release(query)

		var result uintptr
		if status := sec.itemCopyMatching(query, &result); status != errSecSuccess {
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
	_, err := withTimeout(func() (struct{}, error) {
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
	_, err := withTimeout(func() (struct{}, error) {
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
