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
// Operations are prompt-free and time-bounded, so the package is safe in CI.
// The exception is unlocking a locked store when the user opted in and someone
// can answer; that wait has no deadline, matching sudo and gpg.
package securestore

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Backend is recorded in the on-disk envelope so reads use the same mechanism.
type Backend string

const (
	// ACL bound to our code-signing identity; signed builds only.
	BackendMacOSACL      Backend = "macos-acl"
	BackendMacOSSecurity Backend = "macos-security"

	BackendWindowsCredMan        Backend = "windows-credman"
	BackendWindowsCredManTPM     Backend = "windows-credman-tpm"
	BackendLinuxSecretService    Backend = "linux-secretservice"
	BackendLinuxSecretServiceTPM Backend = "linux-secretservice-tpm"
	// TPM present but no usable OS store (headless servers).
	BackendTPMFile Backend = "tpm-file"
	// No protection available; callers keep plaintext behavior.
	BackendPlaintext Backend = "plaintext"
)

// Mode mirrors PULUMI_CREDENTIAL_STORE.
type Mode int

const (
	// No explicit choice: plaintext for writes until encryption is the default.
	ModeDefault Mode = iota
	// Best available backend, else plaintext.
	ModeAuto
	// Require a backend, error when none is usable.
	ModeOS
	ModePlaintext
)

var (
	ErrUnavailable = errors.New("no usable OS credential protection")
	// Nothing the user does locally will read this data.
	ErrBackendUnsupported = fmt.Errorf("%w: no such credential store on this platform", ErrUnavailable)
	// Wraps ErrUnavailable: same effect, named cause.
	ErrLocked = fmt.Errorf("%w: the OS credential store is locked", ErrUnavailable)
	// Never a reason to fall back — that would contradict what the user said.
	ErrDeclined    = errors.New("the OS credential store was not unlocked")
	ErrKeyNotFound = errors.New("no key stored in the OS credential store")
	ErrWrongKey    = errors.New("data cannot be decrypted with the stored key")
)

// itemStore persists one opaque string item. Implementations must be
// prompt-free and time-bounded.
type itemStore interface {
	available() (Outcome, error)
	// get returns ErrKeyNotFound if absent.
	get() (string, error)
	set(value string) error
	// delete of a missing item is not an error.
	delete() error
}

// keyWrapper converts between the raw key and the persisted payload (identity
// for "raw", TPM sealing for "tpm").
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

type Store struct {
	b              backendImpl
	fallbackReason error
}

func (s *Store) Backend() Backend { return s.b.id }

// FallbackReason reports why a ModeAuto resolution fell back to plaintext.
func (s *Store) FallbackReason() error { return s.fallbackReason }

// Both callers have established the user opted in, so prompting is governed
// by attendance alone.
func candidates(pulumiHome string) []backendImpl {
	return platformCandidatesHook(someoneCanAnswerAPasswordDialog(), pulumiHome)
}

var platformCandidatesHook = platformCandidates

// Resolve picks the backend for writing. A plaintext resolution is not an
// error: its key operations fail with ErrUnavailable, which callers take as
// the signal to keep plaintext behavior.
//
// pulumiHome locates file-based key material; empty disables that backend.
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

// ForBackend returns a store for the backend that produced an existing
// envelope, regardless of mode — reading data back must always be attempted.
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

// GetKey never creates a key.
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

// Deleting a missing key is not an error.
func (s *Store) DeleteKey() error {
	if s.b.id == BackendPlaintext {
		return nil
	}
	return s.b.store.delete()
}
