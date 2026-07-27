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
	"fmt"
)

// envelopeVersion is bumped when the on-disk format changes.
const envelopeVersion = 1

// envelope is the on-disk JSON shape of an encrypted file. The marker field
// distinguishes it from any legacy plaintext credentials file, whose schema
// has no key starting with "$".
type envelope struct {
	Marker  int    `json:"$pulumiSecureStore"`
	Backend string `json:"backend"`
	Algo    string `json:"algo"`
	Nonce   string `json:"nonce"`
	Data    string `json:"data"`
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
	sealed := aead.Seal(nil, nonce, plaintext, nil)
	return json.MarshalIndent(envelope{
		Marker:  envelopeVersion,
		Backend: string(backend),
		Algo:    "aes-256-gcm",
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
	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrWrongKey
	}
	return plaintext, nil
}

// IsEnvelope reports whether data looks like a Seal-produced envelope rather
// than a legacy plaintext file.
func IsEnvelope(data []byte) bool {
	_, err := parseEnvelope(data)
	return err == nil
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
	if env.Marker != envelopeVersion {
		return envelope{}, fmt.Errorf("unsupported secure-store envelope version %d", env.Marker)
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
