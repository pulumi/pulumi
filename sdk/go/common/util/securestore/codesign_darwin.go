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

// The native keychain backend is only worth using on a real (Developer ID /
// App Store) signature: that is what the item's per-app ACL binds to, and an
// ad-hoc binary gets a new identity every rebuild, re-prompting each time.
// Uses SecCodeCopySelf, not csops, which misreports on macOS 26/arm64.

// Constants from Security/CSCommon.h and Security/SecCode.h.
const (
	kSecCSDefaultFlags       = 0
	kSecCSSigningInformation = 1 << 1
	kSecCodeSignatureAdhoc   = 0x0002
)

// nativeSelfCheckOverride lets tests exercise the native store: test binaries
// are at best ad-hoc signed, so the real check would always reject them.
var nativeSelfCheckOverride func() error

func nativeSelfCheck() error {
	if nativeSelfCheckOverride != nil {
		return nativeSelfCheckOverride()
	}
	if err := loadDarwinAPI(); err != nil {
		return err
	}

	var self uintptr
	if status := sec.codeCopySelf(kSecCSDefaultFlags, &self); status != errSecSuccess {
		return fmt.Errorf("determining own code identity: code signing error %d", status)
	}
	defer cf.release(self)

	var info uintptr
	if status := sec.codeCopySigningInformation(self, kSecCSSigningInformation, &info); status != errSecSuccess {
		return fmt.Errorf("reading own code signature: code signing error %d", status)
	}
	defer cf.release(info)

	// Per SecCode.h, kSecCodeInfoIdentifier is present iff the code is
	// signed. Dictionary values are borrowed (Get rule): do not release.
	if cf.dictionaryGetValue(info, sec.codeInfoIdentifier) == 0 {
		return errors.New("binary is not code-signed")
	}

	// TODO(secure-credentials): pin to Pulumi's Team ID + identifier via
	// SecRequirement once Apple enrollment completes. Until then any real
	// (non-ad-hoc, team-identified) signature is accepted.
	team := cf.dictionaryGetValue(info, sec.codeInfoTeamIdentifier)
	if team == 0 || cf.getTypeID(team) != cf.stringGetTypeID() || cf.goString(team) == "" {
		return errors.New("binary's code signature has no Team ID (ad-hoc or unsigned build)")
	}

	if flagsRef := cf.dictionaryGetValue(info, sec.codeInfoFlags); flagsRef != 0 {
		var flags int64
		if cf.numberGetValue(flagsRef, kCFNumberSInt64Type, &flags) && flags&kSecCodeSignatureAdhoc != 0 {
			return errors.New("binary is ad-hoc signed")
		}
	}
	return nil
}
