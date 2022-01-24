// Copyright 2016-2018, Pulumi Corporation.
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

package config

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"golang.org/x/crypto/pbkdf2"
)

// Encrypter encrypts plaintext into its encrypted ciphertext.
type Encrypter interface {
	EncryptValue(plaintext string) (string, error)
}

// Decrypter decrypts encrypted ciphertext to its plaintext representation.
type Decrypter interface {
	DecryptValue(ciphertext string) (string, error)
	BulkDecrypt(ciphertexts []string) (map[string]string, error)
}

// Crypter can both encrypt and decrypt values.
type Crypter interface {
	Encrypter
	Decrypter
}

// A nopCrypter simply returns the ciphertext as-is.
type nopCrypter struct{}

var NopDecrypter Decrypter = nopCrypter{}
var NopEncrypter Encrypter = nopCrypter{}

func (nopCrypter) DecryptValue(ciphertext string) (string, error) {
	return ciphertext, nil
}

func (nopCrypter) BulkDecrypt(ciphertexts []string) (map[string]string, error) {
	secretMap := map[string]string{}
	for _, c := range ciphertexts {
		secretMap[c] = c
	}
	return secretMap, nil
}

func (nopCrypter) EncryptValue(plaintext string) (string, error) {
	return plaintext, nil
}

// TrackingDecrypter is a Decrypter that keeps track if decrypted values, which
// can be retrieved via SecureValues().
type TrackingDecrypter interface {
	Decrypter
	SecureValues() []string
}

// NewTrackingDecrypter returns a Decrypter that keeps track of decrypted values.
func NewTrackingDecrypter(decrypter Decrypter) TrackingDecrypter {
	return &trackingDecrypter{decrypter: decrypter}
}

type trackingDecrypter struct {
	decrypter    Decrypter
	secureValues []string
}

func (t *trackingDecrypter) DecryptValue(ciphertext string) (string, error) {
	v, err := t.decrypter.DecryptValue(ciphertext)
	if err != nil {
		return "", err
	}
	t.secureValues = append(t.secureValues, v)
	return v, nil
}

func (t *trackingDecrypter) BulkDecrypt(ciphertexts []string) (map[string]string, error) {
	secretMap := map[string]string{}
	for _, c := range ciphertexts {
		if _, ok := secretMap[c]; ok {
			continue
		}
		v, err := t.decrypter.DecryptValue(c)
		if err != nil {
			return secretMap, err
		}
		secretMap[c] = v
		t.secureValues = append(t.secureValues, v)
	}
	return secretMap, nil
}

func (t *trackingDecrypter) SecureValues() []string {
	return t.secureValues
}

// BlindingCrypter returns a Crypter that instead of decrypting or encrypting data, just returns "[secret]", it can
// be used when you want to display configuration information to a user but don't want to prompt for a password
// so secrets will not be decrypted or encrypted.
var BlindingCrypter Crypter = blindingCrypter{}

// NewBlindingDecrypter returns a blinding decrypter.
func NewBlindingDecrypter() Decrypter {
	return blindingCrypter{}
}

type blindingCrypter struct{}

func (b blindingCrypter) DecryptValue(_ string) (string, error) {
	return "[secret]", nil //nolint:goconst
}

func (b blindingCrypter) BulkDecrypt(ciphertexts []string) (map[string]string, error) {
	secretMap := map[string]string{}
	for _, c := range ciphertexts {
		if _, ok := secretMap[c]; ok {
			continue
		}
		secretMap[c] = "[secret]"
	}
	return secretMap, nil
}

func (b blindingCrypter) EncryptValue(plaintext string) (string, error) {
	return "[secret]", nil
}

type CachedDecrypter interface {
	Decrypter
}

// cachedDecrypter is a Decrypter that keeps track if decrypted values, which
// can be retrieved via SecureValues().
type cachedDecrypter struct {
	decrypter Decrypter
	cache     map[string]string
}

func NewCachedDecrypter(decrypter Decrypter) CachedDecrypter {
	return &cachedDecrypter{decrypter: decrypter}
}

func (c *cachedDecrypter) BulkDecrypt(ciperTexts []string) (map[string]string, error) {
	secretMap, err := c.decrypter.BulkDecrypt(ciperTexts)
	if err != nil {
		return nil, err
	}

	if c.cache == nil {
		c.cache = make(map[string]string, len(secretMap))
	}

	// lets ensure that we loop over the cipertexts to ensure that when we write to the cache
	// an existing entry in the cache doesn't get updated with a new version of the decrypted value
	// if this happens, we error as this may be a bug
	for k, v := range secretMap {
		if plaintext, ok := c.cache[k]; ok && plaintext != v {
			return nil, fmt.Errorf("inconsistent decryption value found for cipertext: %q", k)
		}

		c.cache[k] = v
	}

	return secretMap, nil
}

func (c *cachedDecrypter) DecryptValue(ciperText string) (string, error) {
	if plainText, ok := c.cache[ciperText]; ok {
		return plainText, nil
	}

	// The value is not currently in the cache so we need to decrypt it
	// and add it to the cache
	plainText, err := c.decrypter.DecryptValue(ciperText)
	if err != nil {
		return "", err
	}

	if c.cache == nil {
		c.cache = make(map[string]string)
	}
	c.cache[ciperText] = plainText

	return plainText, nil
}

// NewPanicCrypter returns a new config crypter that will panic if used.
func NewPanicCrypter() Crypter {
	return &panicCrypter{}
}

type panicCrypter struct{}

func (p panicCrypter) EncryptValue(_ string) (string, error) {
	panic("attempt to encrypt value")
}

func (p panicCrypter) BulkDecrypt(_ []string) (map[string]string, error) {
	return nil, nil
}

func (p panicCrypter) DecryptValue(_ string) (string, error) {
	panic("attempt to decrypt value")
}

// NewSymmetricCrypter creates a crypter that encrypts and decrypts values using AES-256-GCM.  The nonce is stored with
// the value itself as a pair of base64 values separated by a colon and a version tag `v1` is prepended.
func NewSymmetricCrypter(key []byte) Crypter {
	contract.Requiref(len(key) == SymmetricCrypterKeyBytes, "key", "AES-256-GCM needs a 32 byte key")
	return &symmetricCrypter{key}
}

// NewSymmetricCrypterFromPassphrase uses a passphrase and salt to generate a key, and then returns a crypter using it.
func NewSymmetricCrypterFromPassphrase(phrase string, salt []byte) Crypter {
	// Generate a key using PBKDF2 to slow down attempts to crack it.  1,000,000 iterations was chosen because it
	// took a little over a second on an i7-7700HQ Quad Core processor
	key := pbkdf2.Key([]byte(phrase), salt, 1000000, SymmetricCrypterKeyBytes, sha256.New)
	return NewSymmetricCrypter(key)
}

// SymmetricCrypterKeyBytes is the required key size in bytes.
const SymmetricCrypterKeyBytes = 32

type symmetricCrypter struct {
	key []byte
}

func (s symmetricCrypter) EncryptValue(value string) (string, error) {
	secret, nonce := encryptAES256GCGM(value, s.key)
	return fmt.Sprintf("v1:%s:%s",
		base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(secret)), nil
}

func (s symmetricCrypter) DecryptValue(value string) (string, error) {
	vals := strings.Split(value, ":")

	if len(vals) != 3 {
		return "", errors.New("bad value")
	}

	if vals[0] != "v1" {
		return "", errors.New("unknown value version")
	}

	nonce, err := base64.StdEncoding.DecodeString(vals[1])
	if err != nil {
		return "", errors.Wrap(err, "bad value")
	}

	enc, err := base64.StdEncoding.DecodeString(vals[2])
	if err != nil {
		return "", errors.Wrap(err, "bad value")
	}

	return decryptAES256GCM(enc, s.key, nonce)
}

func (s symmetricCrypter) BulkDecrypt(ciphertexts []string) (map[string]string, error) {
	secretMap := map[string]string{}
	for _, c := range ciphertexts {
		if _, ok := secretMap[c]; ok {
			continue
		}
		v, err := s.DecryptValue(c)
		if err != nil {
			return nil, err
		}
		secretMap[c] = v
	}
	return secretMap, nil
}

// encryptAES256GCGM returns the ciphertext and the generated nonce
func encryptAES256GCGM(plaintext string, key []byte) ([]byte, []byte) {
	contract.Requiref(len(key) == SymmetricCrypterKeyBytes, "key", "AES-256-GCM needs a 32 byte key")

	nonce := make([]byte, 12)

	_, err := cryptorand.Read(nonce)
	contract.Assertf(err == nil, "could not read from system random source")

	block, err := aes.NewCipher(key)
	contract.AssertNoError(err)

	aesgcm, err := cipher.NewGCM(block)
	contract.AssertNoError(err)

	msg := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	return msg, nonce
}

func decryptAES256GCM(ciphertext []byte, key []byte, nonce []byte) (string, error) {
	contract.Requiref(len(key) == SymmetricCrypterKeyBytes, "key", "AES-256-GCM needs a 32 byte key")

	block, err := aes.NewCipher(key)
	contract.AssertNoError(err)

	aesgcm, err := cipher.NewGCM(block)
	contract.AssertNoError(err)

	msg, err := aesgcm.Open(nil, nonce, ciphertext, nil)

	return string(msg), err
}

// Crypter that just adds a prefix to the plaintext string when encrypting,
// and removes the prefix from the ciphertext when decrypting, for use in tests.
type prefixCrypter struct {
	prefix string
}

func newPrefixCrypter(prefix string) Crypter {
	return prefixCrypter{prefix: prefix}
}

func (c prefixCrypter) DecryptValue(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, c.prefix), nil
}

func (c prefixCrypter) EncryptValue(plaintext string) (string, error) {
	return c.prefix + plaintext, nil
}

func (c prefixCrypter) BulkDecrypt(ciphertexts []string) (map[string]string, error) {
	secretMap := map[string]string{}
	for _, cip := range ciphertexts {
		if _, ok := secretMap[cip]; ok {
			continue
		}
		v, err := c.DecryptValue(cip)
		if err != nil {
			return nil, err
		}
		secretMap[cip] = v
	}
	return secretMap, nil
}
