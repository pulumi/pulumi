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

	"github.com/zalando/go-keyring"
)

const (
	// keyringService/keyringAccount identify the item in the OS store. Never
	// change them: existing users' encrypted files reference this exact item.
	keyringService = "Pulumi CLI"
	keyringAccount = "credentials-key"
)

// keyringStore is an itemStore over zalando/go-keyring, which selects the
// platform mechanism automatically: /usr/bin/security on macOS (prompt-free
// for any binary, incl. across rebuilds), Credential Manager on Windows, and
// Secret Service on Linux. preCheck lets platforms fail fast before touching
// the store (e.g. the Linux D-Bus/locked-collection probes).
type keyringStore struct {
	preCheck func() error
}

func (s keyringStore) available() error {
	if s.preCheck != nil {
		if err := s.preCheck(); err != nil {
			return err
		}
	}
	_, err := s.get()
	if err == nil || errors.Is(err, ErrKeyNotFound) {
		return nil
	}
	if errors.Is(err, ErrUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

func (s keyringStore) get() (string, error) {
	value, err := withTimeout(func() (string, error) {
		return keyring.Get(keyringService, keyringAccount)
	})
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrKeyNotFound
	}
	return value, err
}

func (s keyringStore) set(value string) error {
	_, err := withTimeout(func() (struct{}, error) {
		return struct{}{}, keyring.Set(keyringService, keyringAccount, value)
	})
	return err
}

func (s keyringStore) delete() error {
	_, err := withTimeout(func() (struct{}, error) {
		return struct{}{}, keyring.Delete(keyringService, keyringAccount)
	})
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
