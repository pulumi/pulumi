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

// Package securestore keeps a per-user 32-byte encryption key in the most
// protective mechanism the platform offers and encrypts local files with it.
//
// The backend is chosen at runtime from platform capability and, on macOS,
// the binary's code-signing identity:
//
//   - macOS, Developer-ID-signed build: a native Security-framework keychain
//     item whose ACL is bound to our code-signing identity — only this binary
//     reads it silently, every other same-user process is denied or prompted.
//   - macOS, unsigned/ad-hoc build: the item is managed via Apple's signed
//     /usr/bin/security tool (prompt-free across rebuilds, same-user readable).
//   - Windows: a Credential Manager item; where a TPM is present the stored
//     value is a TPM-wrapped blob rather than the raw key, so a harvested
//     item or offline store dump is useless off the originating machine.
//   - Linux: a Secret Service item with the same TPM upgrade; with a TPM but
//     no Secret Service (headless servers) the sealed blob lives in a file.
//
// Every operation is prompt-free and time-bounded by construction: probing
// never triggers OS dialogs and never blocks beyond a short timeout, so the
// package is safe to use from CI and AI-agent contexts.
package securestore

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
)

// Backend identifies the mechanism protecting the key. It is recorded in the
// envelope written to disk so that reads can use the same mechanism and
// mismatches (machine moved, binary signature changed) produce clear errors.
type Backend string

const (
	// BackendMacOSNative is a native SecItem keychain item with a
	// code-signing-bound ACL (Developer-ID-signed builds only).
	BackendMacOSNative Backend = "macos-native"
	// BackendMacOSSecurity manages the keychain item via /usr/bin/security
	// (unsigned/ad-hoc builds; same-user readable, prompt-free).
	BackendMacOSSecurity Backend = "macos-security"
	// BackendWindowsCredMan stores the raw key in the Windows Credential
	// Manager (no TPM present).
	BackendWindowsCredMan Backend = "windows-credman"
	// BackendWindowsCredManTPM stores a TPM-wrapped blob in the Windows
	// Credential Manager.
	BackendWindowsCredManTPM Backend = "windows-credman-tpm"
	// BackendLinuxSecretService stores the raw key in the Secret Service
	// (no TPM present).
	BackendLinuxSecretService Backend = "linux-secretservice"
	// BackendLinuxSecretServiceTPM stores a TPM2-sealed blob in the Secret
	// Service.
	BackendLinuxSecretServiceTPM Backend = "linux-secretservice-tpm"
	// BackendTPMFile stores a TPM-sealed blob in a private file — used when a
	// TPM is present but no OS credential store is usable (headless servers).
	BackendTPMFile Backend = "tpm-file"
	// BackendPlaintext means no protection is available; callers keep their
	// existing plaintext behavior.
	BackendPlaintext Backend = "plaintext"
)

// Mode controls backend selection, mirroring PULUMI_CREDENTIAL_STORE.
type Mode int

const (
	// ModeDefault is the behavior with no explicit user choice. Until
	// encryption becomes the default this is the same as ModePlaintext for
	// writes; reads of an existing envelope always use its recorded backend.
	ModeDefault Mode = iota
	// ModeAuto selects the best available backend, falling back to plaintext
	// when none is usable.
	ModeAuto
	// ModeOS requires a protective backend and errors when none is usable.
	ModeOS
	// ModePlaintext disables the secure store for writes.
	ModePlaintext
)

var (
	// ErrUnavailable indicates no usable protective backend in this
	// environment (headless session, missing daemon/TPM, timeout, ...).
	ErrUnavailable = errors.New("no usable OS credential protection")
	// ErrKeyNotFound indicates the backend works but holds no key yet.
	ErrKeyNotFound = errors.New("no key stored in the OS credential store")
	// ErrWrongKey is returned by Open when decryption fails authentication,
	// which almost always means the stored key does not match the file.
	ErrWrongKey = errors.New("data cannot be decrypted with the stored key")
)

// itemStore persists one opaque string item in an OS credential store (or
// file). Implementations must be prompt-free and time-bounded.
type itemStore interface {
	// available reports nil when the store is usable right now, without any
	// risk of prompting or blocking.
	available() error
	// get returns the stored item, or ErrKeyNotFound if absent.
	get() (string, error)
	set(value string) error
	// delete removes the item; deleting a missing item is not an error.
	delete() error
}

// keyWrapper converts between the raw 32-byte key and the payload persisted
// in the item store (identity for "raw", TPM sealing for "tpm").
type keyWrapper interface {
	kind() wrapKind
	available() error
	wrap(key []byte) ([]byte, error)
	unwrap(blob []byte) ([]byte, error)
}

// backendImpl couples a store and a wrapper under a Backend id.
type backendImpl struct {
	id    Backend
	store itemStore
	wrap  keyWrapper
}

func (b backendImpl) available() error {
	if err := b.store.available(); err != nil {
		return err
	}
	return b.wrap.available()
}

// Store is a resolved secure store the caller can read and write the key
// through.
type Store struct {
	b backendImpl
}

// Backend reports which mechanism this store uses.
func (s *Store) Backend() Backend { return s.b.id }

// mockResolver, when non-nil, overrides platform resolution (tests only).
var mockResolver func() []backendImpl

// candidates returns the platform's backends in preference order.
func candidates() []backendImpl {
	if mockResolver != nil {
		return mockResolver()
	}
	return platformCandidates()
}

// Resolve picks the backend for writing under the given mode. A plaintext
// resolution returns a *Store whose Backend() is BackendPlaintext and whose
// key operations fail with ErrUnavailable; callers use it as the signal to
// keep today's plaintext behavior. The returned error is non-nil only in
// ModeOS when no protective backend is usable, or on an invalid mode.
func Resolve(mode Mode) (*Store, error) {
	switch mode {
	case ModePlaintext, ModeDefault:
		return &Store{backendImpl{id: BackendPlaintext}}, nil
	case ModeAuto, ModeOS:
		// fall through to probing
	default:
		return nil, fmt.Errorf("invalid secure store mode %d", mode)
	}

	var firstErr error
	for _, cand := range candidates() {
		if err := cand.available(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return &Store{cand}, nil
	}
	if mode == ModeOS {
		if firstErr == nil {
			firstErr = ErrUnavailable
		}
		return nil, fmt.Errorf("PULUMI_CREDENTIAL_STORE=os but %w "+
			"(set PULUMI_CREDENTIAL_STORE=plaintext to override): %v",
			ErrUnavailable, firstErr)
	}
	return &Store{backendImpl{id: BackendPlaintext}}, nil
}

// ForBackend returns a store for the exact backend that produced an existing
// envelope, regardless of mode — reading data back must always be attempted.
// The error explains why the recorded backend is not usable here (moved
// machine, missing TPM, changed binary signature, locked keychain, ...).
func ForBackend(id Backend) (*Store, error) {
	if id == BackendPlaintext {
		return &Store{backendImpl{id: BackendPlaintext}}, nil
	}
	for _, cand := range candidates() {
		if cand.id == id {
			if err := cand.available(); err != nil {
				return nil, fmt.Errorf("credential store backend %q is not usable here: %w", id, err)
			}
			return &Store{cand}, nil
		}
	}
	return nil, fmt.Errorf("credential store backend %q is not available on this platform: %w", id, ErrUnavailable)
}

// GetKey returns the stored 32-byte key without ever creating one. It returns
// ErrKeyNotFound when the backend holds no key, and ErrUnavailable on the
// plaintext backend.
func (s *Store) GetKey() ([]byte, error) {
	if s.b.id == BackendPlaintext {
		return nil, ErrUnavailable
	}
	value, err := s.b.store.get()
	if err != nil {
		return nil, err
	}
	kind, blob, err := parseItem(value)
	if err != nil {
		return nil, err
	}
	if kind != s.b.wrap.kind() {
		return nil, fmt.Errorf("stored key was protected with %q but this backend uses %q: %w",
			kind, s.b.wrap.kind(), ErrUnavailable)
	}
	key, err := s.b.wrap.unwrap(blob)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("stored key is corrupt (%d bytes, want 32)", len(key))
	}
	return key, nil
}

// GetOrCreateKey returns the stored key, generating and persisting a new
// random 32-byte key if — and only if — the backend cleanly reports that no
// key exists. Any other failure is returned as-is: regenerating a key on a
// transient error would permanently orphan the user's encrypted data.
func (s *Store) GetOrCreateKey() ([]byte, error) {
	key, err := s.GetKey()
	switch {
	case err == nil:
		return key, nil
	case errors.Is(err, ErrKeyNotFound):
		// fall through to creation
	default:
		return nil, err
	}

	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	blob, err := s.b.wrap.wrap(key)
	if err != nil {
		return nil, fmt.Errorf("protecting new key: %w", err)
	}
	if err := s.b.store.set(formatItem(s.b.wrap.kind(), blob)); err != nil {
		return nil, fmt.Errorf("storing key: %w", err)
	}
	// Read back and reconcile: store writes have been observed to silently
	// fail on some CI images, and concurrent first runs may race — the
	// persisted item wins.
	stored, err := s.GetKey()
	if err != nil {
		return nil, fmt.Errorf("verifying stored key: %w", err)
	}
	if subtle.ConstantTimeCompare(key, stored) != 1 {
		key = stored
	}
	return key, nil
}

// DeleteKey removes the key and any wrapper material. Deleting a missing key
// is not an error; the plaintext backend is a no-op.
func (s *Store) DeleteKey() error {
	if s.b.id == BackendPlaintext {
		return nil
	}
	return s.b.store.delete()
}
