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

package securestore

import (
	"fmt"
	"sync"
	"testing"
)

// memStore is an in-memory itemStore for tests.
type memStore struct {
	mu    sync.Mutex
	value string
	set_  bool
}

func (m *memStore) available() error { return nil }

func (m *memStore) get() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.set_ {
		return "", ErrKeyNotFound
	}
	return m.value, nil
}

func (m *memStore) set(value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value, m.set_ = value, true
	return nil
}

func (m *memStore) delete() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.value, m.set_ = "", false
	return nil
}

// BackendMock is the backend id used by MockInit; it never resolves outside
// tests.
const BackendMock Backend = "mock"

// BackendMockStrong is the preferred backend installed by MockInitDual; it
// never resolves outside tests.
const BackendMockStrong Backend = "mock-strong"

// gatedStore wraps an itemStore whose availability can be toggled, letting
// tests simulate a platform gaining a stronger protection tier over time
// (e.g. a binary becoming signed, or a TPM becoming usable).
type gatedStore struct {
	inner   itemStore
	enabled *bool
}

func (g *gatedStore) available() error {
	if !*g.enabled {
		return fmt.Errorf("%w: mock backend not yet enabled", ErrUnavailable)
	}
	return g.inner.available()
}

func (g *gatedStore) get() (string, error) { return g.inner.get() }
func (g *gatedStore) set(value string) error { return g.inner.set(value) }
func (g *gatedStore) delete() error        { return g.inner.delete() }

// MockInit replaces platform backend resolution with a single in-memory
// backend for the duration of a test. Tests that touch the secure store MUST
// call this to avoid writing to the developer's real keychain/TPM.
func MockInit(t *testing.T) {
	t.Helper()
	mem := &memStore{}
	mockResolver = func() []backendImpl {
		return []backendImpl{{id: BackendMock, store: mem, wrap: rawWrapper{}}}
	}
	t.Cleanup(func() { mockResolver = nil })
}

// MockInitDual installs two in-memory backends: BackendMockStrong (preferred
// by Resolve but initially unavailable) and BackendMock (always available).
// The returned promote function makes the strong backend available,
// simulating a platform gaining a better protection tier — such as a signed
// binary unlocking the native macOS keychain, or a TPM becoming usable — so
// tests can exercise the cross-backend upgrade of encrypted data.
func MockInitDual(t *testing.T) (promote func()) {
	t.Helper()
	enabled := false
	weak, strong := &memStore{}, &memStore{}
	mockResolver = func() []backendImpl {
		return []backendImpl{
			{id: BackendMockStrong, store: &gatedStore{inner: strong, enabled: &enabled}, wrap: rawWrapper{}},
			{id: BackendMock, store: weak, wrap: rawWrapper{}},
		}
	}
	t.Cleanup(func() { mockResolver = nil })
	return func() { enabled = true }
}
