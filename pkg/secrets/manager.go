// Copyright 2016, Pulumi Corporation.
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

// Package secrets defines the interface common to all secret managers.
package secrets

import (
	"bytes"
	"encoding/json"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// Manager provides the interface for providing stack encryption.
type Manager interface {
	// Type returns a string that reflects the type of this provider. This is serialized along with the state of
	// the manager into the deployment such that we can re-construct the correct manager when deserializing a
	// deployment into a snapshot.
	Type() string
	// An opaque JSON blob, which can be used later to reconstruct the provider when deserializing the
	// deployment into a snapshot.
	State() json.RawMessage
	// Encrypter returns a `config.Encrypter` that can be used to encrypt values when serializing a snapshot into a
	// deployment.
	Encrypter() config.Encrypter
	// Decrypter returns a `config.Decrypter` that can be used to decrypt values when deserializing a snapshot from a
	// deployment.
	Decrypter() config.Decrypter
	// Equivalent reports whether this manager would produce and consume the same ciphertexts as a
	// manager constructed from `otherState` (of the same Type). Providers decide which fields of
	// their State are semantically load-bearing, so that inconsequential differences (formatting,
	// cached derived fields) do not cause spurious mismatches. The caller is responsible for
	// checking that Type matches before calling.
	Equivalent(otherState json.RawMessage) bool
}

// AreCompatible returns true if the two Managers are of the same type and would produce and
// consume identical ciphertexts.
func AreCompatible(a, b Manager) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Type() != b.Type() {
		return false
	}
	return a.Equivalent(b.State())
}

// StateJSONEqual is a helper for providers whose Equivalent is defined by byte-wise equality of canonicalized JSON
// (i.e., every field in State is semantically meaningful). Those providers can implement Equivalent as `return
// secrets.StateJSONEqual(m.state, otherState)`.
func StateJSONEqual(a, b json.RawMessage) bool {
	canon := func(raw json.RawMessage) ([]byte, bool) {
		if len(raw) == 0 {
			return nil, true
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, false
		}
		out, err := json.Marshal(v)
		if err != nil {
			return nil, false
		}
		return out, true
	}
	ca, ok := canon(a)
	if !ok {
		return false
	}
	cb, ok := canon(b)
	if !ok {
		return false
	}
	return bytes.Equal(ca, cb)
}
