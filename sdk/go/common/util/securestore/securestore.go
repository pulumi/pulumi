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
// The Backend constants below name the per-platform mechanisms.
//
// Operations are prompt-free and time-bounded, so the package is safe in CI
// and AI-agent contexts. The one exception is unlocking a locked store when
// the user opted in and someone can answer; that wait has no deadline,
// matching sudo and gpg.
package securestore

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Backend identifies the mechanism protecting the key. It is recorded in the
// on-disk envelope so reads use the same mechanism.
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
	// ModeDefault is no explicit user choice: plaintext for writes until
	// encryption becomes the default.
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
	// ErrBackendUnsupported indicates an envelope recorded under a backend this
	// build cannot implement, so nothing the user does locally will read it.
	ErrBackendUnsupported = fmt.Errorf("%w: no such credential store on this platform", ErrUnavailable)
	// ErrLocked indicates a store that could not be unlocked without a prompt
	// nobody could answer. Wraps ErrUnavailable: same effect, named cause.
	ErrLocked = fmt.Errorf("%w: the OS credential store is locked", ErrUnavailable)
	// ErrDeclined indicates the user dismissed the unlock prompt. Never a
	// reason to fall back — that would contradict what they just said.
	ErrDeclined = errors.New("the OS credential store was not unlocked")
	// ErrKeyNotFound indicates the backend works but holds no key yet.
	ErrKeyNotFound = errors.New("no key stored in the OS credential store")
	// ErrWrongKey indicates decryption failed authentication — almost always a
	// stored key that does not match the file.
	ErrWrongKey = errors.New("data cannot be decrypted with the stored key")
)

// itemStore persists one opaque string item in an OS credential store (or
// file). Implementations must be prompt-free and time-bounded.
type itemStore interface {
	// available must never prompt or block.
	available() (Outcome, error)
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

type backendImpl struct {
	id    Backend
	store itemStore
	wrap  keyWrapper
}

func (b backendImpl) available() (Outcome, error) {
	outcome, err := b.store.available()
	if outcome != Ready {
		return outcome, err
	}
	if err := b.wrap.available(); err != nil {
		return Absent, err
	}
	return Ready, nil
}

// Store is a resolved secure store.
type Store struct {
	b              backendImpl
	fallbackReason error
}

// Backend reports which mechanism this store uses.
func (s *Store) Backend() Backend { return s.b.id }

// FallbackReason reports why a ModeAuto resolution fell back to plaintext,
// or nil for any other resolution.
func (s *Store) FallbackReason() error { return s.fallbackReason }

// candidates returns the platform's backends in preference order. Only
// Resolve and ForBackend call it, and both have already established that the
// caller opted in, so prompting is governed by attendance alone.
func candidates(pulumiHome string) []backendImpl {
	return platformCandidatesHook(someoneCanAnswerAPasswordDialog(), pulumiHome)
}

var platformCandidatesHook = platformCandidates

// Resolve picks the backend for writing under the given mode. A plaintext
// resolution returns a *Store whose Backend() is BackendPlaintext and whose
// key operations fail with ErrUnavailable; callers use it as the signal to
// keep today's plaintext behavior. The returned error is non-nil when the
// user declined an unlock in any mode, when ModeOS finds no usable backend,
// or on an invalid mode.
//
// pulumiHome is the directory for file-based key material (the TPM-sealed
// key file used when no OS credential store exists); an empty string makes
// that backend unavailable.
func Resolve(mode Mode, pulumiHome string) (*Store, error) {
	switch mode {
	case ModePlaintext, ModeDefault:
		return &Store{b: backendImpl{id: BackendPlaintext}}, nil
	case ModeAuto, ModeOS:
	default:
		return nil, fmt.Errorf("invalid secure store mode %d", mode)
	}

	var firstErr error
	for _, cand := range candidates(pulumiHome) {
		outcome, err := cand.available()
		switch outcome {
		case Ready:
		case Declined:
			logging.V(7).Infof("secure store backend %q: %s", cand.id, outcome)
			return nil, err
		case Absent, Locked:
			logging.V(7).Infof("secure store backend %q: %s (%v)", cand.id, outcome, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		logging.V(7).Infof("secure store resolved to backend %q", cand.id)
		return &Store{b: cand}, nil
	}
	logging.V(7).Infof("no secure store backend usable")
	if firstErr == nil {
		firstErr = ErrUnavailable
	}
	if mode == ModeOS {
		return nil, fmt.Errorf("PULUMI_CREDENTIAL_STORE=os but %w "+
			"(set PULUMI_CREDENTIAL_STORE=plaintext to override): %v",
			ErrUnavailable, firstErr)
	}
	return &Store{b: backendImpl{id: BackendPlaintext}, fallbackReason: firstErr}, nil
}

// ForBackend returns a store for the exact backend that produced an existing
// envelope, regardless of mode — reading data back must always be attempted.
// The error explains why the recorded backend is not usable here (moved
// machine, missing TPM, changed binary signature, locked keychain, ...).
// pulumiHome is as for Resolve.
func ForBackend(id Backend, pulumiHome string) (*Store, error) {
	if id == BackendPlaintext {
		return &Store{b: backendImpl{id: BackendPlaintext}}, nil
	}
	for _, cand := range candidates(pulumiHome) {
		if cand.id == id {
			outcome, err := cand.available()
			if outcome == Declined {
				return nil, err
			}
			if err != nil {
				return nil, fmt.Errorf("credential store backend %q is not usable here: %w", id, err)
			}
			return &Store{b: cand}, nil
		}
	}
	return nil, fmt.Errorf("credential store backend %q is not available on this platform: %w",
		id, ErrBackendUnsupported)
}

// GetKey returns the stored key without ever creating one.
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

// createKeyMu serializes first-time key creation: OS store "add" is not
// atomic (`security` returns "duplicate item" to the loser of a race).
var createKeyMu sync.Mutex

// GetOrCreateKey returns the stored key, creating one only when the backend
// cleanly reports none exists. Regenerating on a transient error would
// permanently orphan the user's encrypted data.
func (s *Store) GetOrCreateKey() ([]byte, error) {
	key, err := s.GetKey()
	if err == nil {
		return key, nil
	}
	createKeyMu.Lock()
	defer createKeyMu.Unlock()
	// Re-check under the lock: a concurrent caller may have created the key.
	key, err = s.GetKey()
	switch {
	case err == nil:
		return key, nil
	case errors.Is(err, ErrKeyNotFound):
	default:
		// A raw-wrapped item under a TPM backend means the machine gained a
		// TPM later. Re-wrap in place; failing would downgrade to plaintext.
		if upgraded, upErr := s.upgradeKeyWrap(); upErr == nil && upgraded != nil {
			return upgraded, nil
		}
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
	if setErr := s.b.store.set(formatItem(s.b.wrap.kind(), blob)); setErr != nil {
		// The add can lose a race, or hit store state a preceding lookup did
		// not yet see (securityd settling after a delete). The persisted item
		// wins.
		if stored, getErr := s.GetKey(); getErr == nil {
			return stored, nil
		}
		return nil, fmt.Errorf("storing key: %w", setErr)
	}
	// Writes have been observed to silently fail on some CI images; the
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

// upgradeKeyWrap re-protects a raw-wrapped key with this backend's wrapper,
// returning (nil, nil) when the item is not raw. Only that direction works:
// a TPM-wrapped key cannot be unwrapped without its TPM.
func (s *Store) upgradeKeyWrap() ([]byte, error) {
	if s.b.wrap.kind() == wrapRaw {
		return nil, nil
	}
	value, err := s.b.store.get()
	if err != nil {
		return nil, err
	}
	kind, blob, err := parseItem(value)
	if err != nil || kind != wrapRaw || len(blob) != 32 {
		return nil, nil
	}
	wrapped, err := s.b.wrap.wrap(blob)
	if err != nil {
		return nil, err
	}
	if err := s.b.store.set(formatItem(s.b.wrap.kind(), wrapped)); err != nil {
		return nil, err
	}
	return blob, nil
}

// DeleteKey removes the key and any wrapper material. Deleting a missing key
// is not an error; the plaintext backend is a no-op.
func (s *Store) DeleteKey() error {
	if s.b.id == BackendPlaintext {
		return nil
	}
	return s.b.store.delete()
}
