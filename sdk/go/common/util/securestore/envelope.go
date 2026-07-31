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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// envelopeVersion is bumped when the on-disk format changes.
const envelopeVersion = 1

const envelopeAlgo = "aes-256-gcm"

// ErrUnsupportedVersion indicates an envelope whose version this build does
// not understand. Detection (IsEnvelope) still succeeds for such files so
// they are never mistaken for plaintext and overwritten.
var ErrUnsupportedVersion = errors.New("was encrypted by a newer version of the Pulumi CLI")

// envelope is the on-disk JSON shape of an encrypted file. The marker field
// distinguishes it from any legacy plaintext credentials file, whose schema
// has no key starting with "$". It is a pointer so that presence (any
// version) and support (this version) are separate decisions.
type envelope struct {
	Marker  *int   `json:"$pulumiSecureStore"`
	Backend string `json:"backend"`
	Algo    string `json:"algo"`
	Nonce   string `json:"nonce"`
	Data    string `json:"data"`
}

func aad(version int, backend Backend, algo string) []byte {
	return fmt.Appendf(nil, "%d|%s|%s", version, backend, algo)
}

// Seal encrypts plaintext with AES-256-GCM under the given 32-byte key and
// returns a versioned JSON envelope recording the backend that protects the
// key, suitable for writing to disk.
func Seal(key []byte, backend Backend, plaintext []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, plaintext, aad(envelopeVersion, backend, envelopeAlgo))
	version := envelopeVersion
	return json.MarshalIndent(envelope{
		Marker:  &version,
		Backend: string(backend),
		Algo:    envelopeAlgo,
		Nonce:   base64.StdEncoding.EncodeToString(nonce),
		Data:    base64.StdEncoding.EncodeToString(sealed),
	}, "", "    ")
}

// Open decrypts an envelope produced by Seal.
func Open(key, data []byte) ([]byte, error) {
	env, err := parseEnvelope(data)
	if err != nil {
		return nil, err
	}
	if env.Algo != envelopeAlgo {
		return nil, fmt.Errorf("unsupported secure-store envelope algorithm %q", env.Algo)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("parsing secure-store envelope nonce: %w", err)
	}
	sealed, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("parsing secure-store envelope data: %w", err)
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("invalid secure-store envelope nonce length %d", len(nonce))
	}
	plaintext, err := aead.Open(nil, nonce, sealed, aad(*env.Marker, Backend(env.Backend), env.Algo))
	if err != nil {
		return nil, ErrWrongKey
	}
	return plaintext, nil
}

// IsEnvelope reports whether data looks like a Seal-produced envelope rather
// than a legacy plaintext file. It is detection only: it accepts any
// envelope version, including ones this build cannot open, so callers never
// treat an unreadable envelope as plaintext.
func IsEnvelope(data []byte) bool {
	var probe struct {
		Marker *int `json:"$pulumiSecureStore"`
	}
	return json.Unmarshal(data, &probe) == nil && probe.Marker != nil
}

// EnvelopeBackend returns the backend recorded in an envelope, so reads can
// be attempted with the same mechanism that protected the key.
func EnvelopeBackend(data []byte) (Backend, error) {
	env, err := parseEnvelope(data)
	if err != nil {
		return "", err
	}
	return Backend(env.Backend), nil
}

func parseEnvelope(data []byte) (envelope, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return envelope{}, fmt.Errorf("parsing secure-store envelope: %w", err)
	}
	if env.Marker == nil {
		return envelope{}, errors.New("not a secure-store envelope")
	}
	if *env.Marker != envelopeVersion {
		return envelope{}, fmt.Errorf("%w (envelope version %d)", ErrUnsupportedVersion, *env.Marker)
	}
	return env, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secure-store key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
