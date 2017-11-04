// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
)

const aes256GCMKeyBytes = 32

// blindingDecrypter is a config.ValueDecrypter that instead of decrypting data, just returns "********", it can
// be used when you want to display configuration information to a user but don't want to prompt for a password
// so secrets will not be decrypted.
type blindingDecrypter struct{}

func (b blindingDecrypter) DecryptValue(ciphertext string) (string, error) {
	return "********", nil
}

// This implements to config.EncrypterDecrypter interface but panics if any methods are called.
type panicCrypter struct{}

func (p panicCrypter) EncryptValue(plaintext string) (string, error) {
	panic("attempt to encrypt value")
}

func (p panicCrypter) DecryptValue(ciphertext string) (string, error) {
	panic("attempt to decrypt value")
}

// symmetricCrypter encrypts and decrypts values using AES-256-GCM. The nonce is stored with the value itself as a pair of base64 values
// separated by a colon and a version tag `v1` is prepended.
type symmetricCrypter struct {
	key []byte
}

func (s symmetricCrypter) EncryptValue(value string) (string, error) {
	secret, nonce := encryptAES256GCGM(value, s.key)

	return fmt.Sprintf("v1:%s:%s", base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(secret)), nil
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

// given a passphrase and an encryption state, construct a config.ValueEncrypterDecrypter from it. Our encryption
// state value is a version tag followed by version specific state information. Presently, we only have one version
// we support (`v1`) which is AES-256-GCM using a key derived from a passphrase using 1,000,000 iterations of PDKDF2
// using SHA256.
func symmetricCrypterFromPhraseAndState(phrase string, state string) (config.ValueEncrypterDecrypter, error) {
	splits := strings.SplitN(state, ":", 3)
	if len(splits) != 3 {
		return nil, errors.New("malformed state value")
	}

	if splits[0] != "v1" {
		return nil, errors.New("unknown state version")
	}

	salt, err := base64.StdEncoding.DecodeString(splits[1])
	if err != nil {
		return nil, err
	}

	key := keyFromPassphrase(phrase, salt, aes256GCMKeyBytes)
	decrypter := symmetricCrypter{key: key}

	decrypted, err := decrypter.DecryptValue(state[indexN(state, ":", 2)+1:])
	if err != nil || decrypted != "pulumi" {
		return nil, errors.New("incorrect passphrase")
	}

	return symmetricCrypter{key: key}, nil
}

func indexN(s string, substr string, n int) int {
	contract.Require(n > 0, "n")
	scratch := s

	for i := n; i > 0; i-- {
		idx := strings.Index(scratch, substr)
		if i == -1 {
			return -1
		}

		scratch = scratch[idx+1:]
	}

	return len(s) - (len(scratch) + len(substr))
}

// encryptAES256GCGM returns the ciphertext and the generated nonce
func encryptAES256GCGM(plaintext string, key []byte) ([]byte, []byte) {
	contract.Requiref(len(key) == aes256GCMKeyBytes, "key", "AES-256-GCM needs a 32 byte key")

	nonce := make([]byte, 12)

	_, err := cryptorand.Read(nonce)
	contract.Assertf(err == nil, "could not read from system random source")

	block, err := aes.NewCipher(key)
	contract.Assert(err == nil)

	aesgcm, err := cipher.NewGCM(block)
	contract.Assert(err == nil)

	msg := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	return msg, nonce
}

func decryptAES256GCM(ciphertext []byte, key []byte, nonce []byte) (string, error) {
	contract.Requiref(len(key) == aes256GCMKeyBytes, "key", "AES-256-GCM needs a 32 byte key")

	block, err := aes.NewCipher(key)
	contract.Assert(err == nil)

	aesgcm, err := cipher.NewGCM(block)
	contract.Assert(err == nil)

	msg, err := aesgcm.Open(nil, nonce, ciphertext, nil)

	return string(msg), err
}
