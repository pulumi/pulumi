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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// wrapKind names how the persisted item protects the key material.
type wrapKind string

const (
	wrapRaw wrapKind = "raw"
	wrapTPM wrapKind = "tpm"
)

// itemPrefix versions the stored item so future format changes are
// detectable. Items are stored as strings (base64 payload) because that is
// the lowest common denominator across OS stores: gnome-keyring drops the
// content type, KWallet rejects non-UTF-8, and some Secret Service providers
// hard-fail on raw binary.
const itemPrefix = "pulumi-securestore-v1"

// opTimeout bounds every store operation. Linux Secret Service calls can
// hang on D-Bus activation or invisible unlock prompts; the gh CLI wraps its
// keyring calls in a timeout for the same reason.
const opTimeout = 3 * time.Second

// formatItem encodes a wrapped key payload for storage.
func formatItem(kind wrapKind, blob []byte) string {
	return itemPrefix + ":" + string(kind) + ":" + base64.StdEncoding.EncodeToString(blob)
}

// parseItem decodes a stored item into its wrap kind and payload.
func parseItem(value string) (wrapKind, []byte, error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[0] != itemPrefix {
		return "", nil, errors.New("stored key has an unrecognized format")
	}
	kind := wrapKind(parts[1])
	if kind != wrapRaw && kind != wrapTPM {
		return "", nil, fmt.Errorf("stored key has unknown protection kind %q", parts[1])
	}
	blob, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", nil, fmt.Errorf("stored key is corrupt (not base64): %w", err)
	}
	return kind, blob, nil
}

// withTimeout runs fn on a goroutine and abandons it after opTimeout. An
// abandoned goroutine may leak for the process lifetime; that is acceptable
// for a CLI and matches the gh CLI's approach.
func withTimeout[T any](fn func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(opTimeout):
		var zero T
		return zero, fmt.Errorf("%w: operation timed out after %s", ErrUnavailable, opTimeout)
	}
}

// rawWrapper is the identity keyWrapper for backends where the OS store item
// holds the key itself.
type rawWrapper struct{}

func (rawWrapper) kind() wrapKind                     { return wrapRaw }
func (rawWrapper) available() error                   { return nil }
func (rawWrapper) wrap(key []byte) ([]byte, error)    { return key, nil }
func (rawWrapper) unwrap(blob []byte) ([]byte, error) { return blob, nil }
