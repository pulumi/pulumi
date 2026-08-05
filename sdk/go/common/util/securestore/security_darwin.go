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

import "fmt"

// Security framework bindings via purego. A plain SecItemAdd already yields a
// per-app ACL bound to the creating app's signing identity, so no SecAccess or
// SecTrustedApplication is needed.

// OSStatus codes from Security/SecBase.h.
const (
	errSecSuccess               = 0
	errSecUserCanceled          = -128 // the user canceled an authorization dialog
	errSecNotAvailable          = -25291
	errSecAuthFailed            = -25293
	errSecDuplicateItem         = -25299
	errSecItemNotFound          = -25300
	errSecInteractionNotAllowed = -25308
	errSecInteractionRequired   = -25315
)

// kSecUnlockStateStatus from Security/SecKeychain.h: set in the
// SecKeychainGetStatus mask when the keychain is unlocked.
const kSecUnlockStateStatus = 1

// osStatusError maps an OSStatus to a package error, keeping the numeric code.
func osStatusError(status int32) error {
	switch status {
	case errSecSuccess:
		return nil
	case errSecItemNotFound:
		return ErrKeyNotFound
	case errSecUserCanceled:
		return fmt.Errorf("%w: keychain error %d: the keychain prompt was canceled", ErrDeclined, status)
	case errSecInteractionNotAllowed, errSecInteractionRequired:
		return fmt.Errorf("%w: keychain error %d: user interaction required; "+
			"the keychain may be locked (e.g. in an SSH session)", ErrLocked, status)
	case errSecNotAvailable:
		return fmt.Errorf("%w: keychain error %d: no keychain is available", ErrUnavailable, status)
	default:
		return fmt.Errorf("keychain error %d", status)
	}
}

type secAPI struct {
	itemAdd          func(attrs uintptr, result *uintptr) int32
	itemCopyMatching func(query uintptr, result *uintptr) int32
	itemUpdate       func(query, attrs uintptr) int32
	itemDelete       func(query uintptr) int32

	codeCopySelf               func(flags uint32, self *uintptr) int32
	codeCopySigningInformation func(code uintptr, flags uint32, info *uintptr) int32

	// Deprecated, with no modern replacement for what we need; see
	// withoutKeychainUI.
	keychainCopyDefault          func(keychain *uintptr) int32
	keychainGetStatus            func(keychain uintptr, status *uint32) int32
	keychainUnlock               func(keychain uintptr, passwordLength uint32, password uintptr, usePassword bool) int32
	keychainSetUserInteractionOK func(allowed bool) int32

	// SecItem attribute/value CFStringRef constants.
	class                uintptr
	classGenericPassword uintptr
	attrService          uintptr
	attrAccount          uintptr
	attrLabel            uintptr
	valueData            uintptr
	returnData           uintptr
	matchLimit           uintptr
	matchLimitOne        uintptr

	// SecCodeCopySigningInformation dictionary keys.
	codeInfoIdentifier     uintptr
	codeInfoTeamIdentifier uintptr
	codeInfoFlags          uintptr
}

func newSecAPI(l *lib) *secAPI {
	s := &secAPI{}
	l.fn(&s.itemAdd, "SecItemAdd")
	l.fn(&s.itemCopyMatching, "SecItemCopyMatching")
	l.fn(&s.itemUpdate, "SecItemUpdate")
	l.fn(&s.itemDelete, "SecItemDelete")
	l.fn(&s.codeCopySelf, "SecCodeCopySelf")
	l.fn(&s.codeCopySigningInformation, "SecCodeCopySigningInformation")
	l.fn(&s.keychainCopyDefault, "SecKeychainCopyDefault")
	l.fn(&s.keychainGetStatus, "SecKeychainGetStatus")
	l.fn(&s.keychainUnlock, "SecKeychainUnlock")
	l.fn(&s.keychainSetUserInteractionOK, "SecKeychainSetUserInteractionAllowed")
	s.class = l.constant("kSecClass")
	s.classGenericPassword = l.constant("kSecClassGenericPassword")
	s.attrService = l.constant("kSecAttrService")
	s.attrAccount = l.constant("kSecAttrAccount")
	s.attrLabel = l.constant("kSecAttrLabel")
	s.valueData = l.constant("kSecValueData")
	s.returnData = l.constant("kSecReturnData")
	s.matchLimit = l.constant("kSecMatchLimit")
	s.matchLimitOne = l.constant("kSecMatchLimitOne")
	s.codeInfoIdentifier = l.constant("kSecCodeInfoIdentifier")
	s.codeInfoTeamIdentifier = l.constant("kSecCodeInfoTeamIdentifier")
	s.codeInfoFlags = l.constant("kSecCodeInfoFlags")
	return s
}
