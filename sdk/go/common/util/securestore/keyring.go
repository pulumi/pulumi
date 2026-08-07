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
	"errors"
	"fmt"
	"sync"

	"github.com/zalando/go-keyring"
)

const (
	// Never change: encrypted files reference this exact item.
	keyringService = "Pulumi CLI"
	keyringAccount = "credentials-key"
)

type getCache struct {
	mu    sync.Mutex
	valid bool
	value string
	err   error
}

// go-keyring picks the platform mechanism itself. preCheck lets platforms fail
// fast before the store is touched.
type keyringStore struct {
	preCheck func() (Outcome, error)
	cache    *getCache
}

func newKeyringStore(preCheck func() (Outcome, error)) keyringStore {
	return keyringStore{preCheck: preCheck, cache: &getCache{}}
}

func (s keyringStore) available() (Outcome, error) {
	if s.preCheck != nil {
		if outcome, err := s.preCheck(); outcome != Ready {
			return outcome, err
		}
	}
	value, err := s.fetch()
	if s.cache != nil {
		s.cache.mu.Lock()
		s.cache.valid, s.cache.value, s.cache.err = true, value, err
		s.cache.mu.Unlock()
	}
	if err == nil || errors.Is(err, ErrKeyNotFound) {
		return Ready, nil
	}
	if errors.Is(err, ErrUnavailable) {
		return Absent, err
	}
	return Absent, fmt.Errorf("%w: %v", ErrUnavailable, err)
}

func (s keyringStore) get() (string, error) {
	if s.cache != nil {
		s.cache.mu.Lock()
		if s.cache.valid {
			value, err := s.cache.value, s.cache.err
			s.cache.valid = false
			s.cache.mu.Unlock()
			return value, err
		}
		s.cache.mu.Unlock()
	}
	return s.fetch()
}

func (s keyringStore) fetch() (string, error) {
	value, err := withTimeout(func() (string, error) {
		return keyring.Get(keyringService, keyringAccount)
	})
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrKeyNotFound
	}
	return value, err
}

func (s keyringStore) invalidate() {
	if s.cache != nil {
		s.cache.mu.Lock()
		s.cache.valid = false
		s.cache.mu.Unlock()
	}
}

func (s keyringStore) set(value string) error {
	s.invalidate()
	_, err := withTimeout(func() (struct{}, error) {
		return struct{}{}, keyring.Set(keyringService, keyringAccount, value)
	})
	return err
}

func (s keyringStore) delete() error {
	s.invalidate()
	_, err := withTimeout(func() (struct{}, error) {
		return struct{}{}, keyring.Delete(keyringService, keyringAccount)
	})
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
