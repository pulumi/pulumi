// Copyright 2016-2022, Pulumi Corporation.
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

import "context"

// A CiphertextSecret is a secret config value represented as ciphertext.
type CiphertextSecret struct {
	value string
}

// Decrypt decrypts a ciphertext secret into its plaintext.
func (c CiphertextSecret) Decrypt(ctx context.Context, dec Decrypter) (PlaintextSecret, error) {
	plaintext, err := dec.DecryptValue(ctx, c.value)
	if err != nil {
		return "", err
	}
	return PlaintextSecret(plaintext), nil
}

// A PlaintextSecret is a secret configuration value represented as plaintext.
type PlaintextSecret string

// Encrypt encrypts a plaintext value into its ciphertext.
func (p PlaintextSecret) Encrypt(ctx context.Context, enc Encrypter) (CiphertextSecret, error) {
	ciphertext, err := enc.EncryptValue(ctx, string(p))
	if err != nil {
		return CiphertextSecret{}, err
	}
	return CiphertextSecret{ciphertext}, nil
}
