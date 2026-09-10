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

package workspace

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"
	"github.com/stretchr/testify/require"
)

const (
	fakeBackend       = securestore.Backend("fake")
	fakeStrongBackend = securestore.Backend("fake-strong")
)

// Stands in for a store that is present and unlocked.
type fakeKeyStore struct {
	backend    securestore.Backend
	key        []byte
	absent     bool
	declineErr error
	createErr  error
	getErr     error
}

func (f *fakeKeyStore) Backend() securestore.Backend { return f.backend }

func (f *fakeKeyStore) GetKey() ([]byte, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.key == nil {
		return nil, securestore.ErrKeyNotFound
	}
	return f.key, nil
}

func (f *fakeKeyStore) GetOrCreateKey() ([]byte, error) {
	if f.declineErr != nil {
		return nil, f.declineErr
	}
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.key == nil {
		f.key = make([]byte, 32)
		if _, err := rand.Read(f.key); err != nil {
			return nil, err
		}
	}
	return f.key, nil
}

func (f *fakeKeyStore) DeleteKey() error {
	f.key = nil
	return nil
}

func (f *fakeKeyStore) FallbackReason() error { return nil }

// Mirrors the real resolver: first usable backend wins, envelopes honour
// their recorded one.
type fakeStores struct {
	byBackend map[securestore.Backend]*fakeKeyStore
	preferred []securestore.Backend
}

func (f *fakeStores) Resolve(mode securestore.Mode) (keyStore, error) {
	switch mode {
	case securestore.ModePlaintext, securestore.ModeDefault:
		return plaintextKeyStore{}, nil
	case securestore.ModeAuto, securestore.ModeOS:
	default:
		return nil, fmt.Errorf("invalid secure store mode %d", mode)
	}
	var firstAbsent error
	for _, id := range f.preferred {
		st := f.byBackend[id]
		switch {
		case st == nil:
		case st.declineErr != nil:
			// A refusal stops the search, as it does in production.
			return nil, st.declineErr
		case st.absent:
			if firstAbsent == nil {
				firstAbsent = securestore.ErrUnavailable
			}
		default:
			return st, nil
		}
	}
	if mode == securestore.ModeOS {
		return nil, fmt.Errorf("PULUMI_CREDENTIAL_STORE=os but %w", securestore.ErrUnavailable)
	}
	if firstAbsent == nil {
		firstAbsent = securestore.ErrUnavailable
	}
	return plaintextKeyStore{reason: firstAbsent}, nil
}

func (f *fakeStores) ForBackend(id securestore.Backend) (keyStore, error) {
	if id == securestore.BackendPlaintext {
		return plaintextKeyStore{}, nil
	}
	st := f.byBackend[id]
	if st == nil {
		return nil, fmt.Errorf("credential store backend %q is not available on this platform: %w",
			id, securestore.ErrBackendUnsupported)
	}
	if st.declineErr != nil {
		return nil, st.declineErr
	}
	if st.absent {
		return nil, fmt.Errorf("credential store backend %q is not usable here: %w",
			id, securestore.ErrUnavailable)
	}
	return st, nil
}

// Fails the way the real plaintext backend does.
type plaintextKeyStore struct{ reason error }

func (plaintextKeyStore) Backend() securestore.Backend { return securestore.BackendPlaintext }
func (plaintextKeyStore) GetKey() ([]byte, error)      { return nil, securestore.ErrUnavailable }
func (plaintextKeyStore) GetOrCreateKey() ([]byte, error) {
	return nil, securestore.ErrUnavailable
}
func (plaintextKeyStore) DeleteKey() error        { return nil }
func (p plaintextKeyStore) FallbackReason() error { return p.reason }

// Returned so the test can drop its key or make it refuse.
func useFakeStores(t *testing.T) *fakeKeyStore {
	t.Helper()
	st := &fakeKeyStore{backend: fakeBackend}
	installStores(t, &fakeStores{
		byBackend: map[securestore.Backend]*fakeKeyStore{fakeBackend: st},
		preferred: []securestore.Backend{fakeBackend},
	})
	return st
}

func installStores(t *testing.T, s keyStores) {
	t.Helper()
	previous := stores
	stores = s
	t.Cleanup(func() { stores = previous })
	resetCredStoreForTesting()
	t.Cleanup(resetCredStoreForTesting)
}

func fakeStore(t *testing.T) *fakeKeyStore {
	t.Helper()
	fakes, ok := stores.(*fakeStores)
	require.True(t, ok, "no fake stores installed")
	return fakes.byBackend[fakeBackend]
}

// The stronger backend becomes available only once promote is called.
func useUpgradableStores(t *testing.T) (promote func()) {
	t.Helper()
	weak := &fakeKeyStore{backend: fakeBackend}
	strong := &fakeKeyStore{backend: fakeStrongBackend, absent: true}
	installStores(t, &fakeStores{
		byBackend: map[securestore.Backend]*fakeKeyStore{fakeBackend: weak, fakeStrongBackend: strong},
		preferred: []securestore.Backend{fakeStrongBackend, fakeBackend},
	})
	return func() { strong.absent = false }
}
