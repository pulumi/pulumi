// Copyright 2025, Pulumi Corporation.
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

package backend

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// errorCatchingSecretsProvider decorates a secrets.Provider with the ability to catch and control the response to
// certain kinds of errors, such as those encountered when decrypting a value. This can be used to, for example, ignore
// errors caused by secret decryption so that plain/insecure values can still be used.
type errorCatchingSecretsProvider struct {
	// The underlying secrets.Provider that will be used to perform actual operations.
	delegateProvider secrets.Provider

	// A callback that will be invoked when an error is encountered during decryption. The callback can return a new error
	// to be returned, or nil to indicate that the error should be ignored and that one or more empty objects should be
	// returned as plaintext(s) instead.
	onDecryptError func(error) error
}

var _ secrets.Provider = (*errorCatchingSecretsProvider)(nil)

// newErrorCatchingSecretsProvider creates a new errorCatchingSecretsProvider that wraps the given delegate
// secrets.Provider. The onDecryptError callback will be invoked when an error is encountered during decryption. The
// callback can return a new error to be returned, or nil to indicate that the error should be ignored and that one or
// more empty objects should be returned as plaintext(s) instead.
func newErrorCatchingSecretsProvider(
	delegate secrets.Provider,
	onDecryptError func(error) error,
) *errorCatchingSecretsProvider {
	return &errorCatchingSecretsProvider{
		delegateProvider: delegate,
		onDecryptError:   onDecryptError,
	}
}

func (p *errorCatchingSecretsProvider) OfType(ty string, state json.RawMessage) (secrets.Manager, error) {
	delegateManager, err := p.delegateProvider.OfType(ty, state)
	if err != nil {
		return nil, err
	}

	manager := &errorCatchingSecretsManager{
		delegateManager: delegateManager,
		onDecryptError:  p.onDecryptError,
	}

	return manager, nil
}

// errorCatchingSecretsManager decorates a secrets.Manager with the ability to catch and control the response to certain
// kinds of errors, such as those encountered when decrypting a value. errorCatchingSecretsManager is used to wrap
// results returned by errorCatchingSecretsProvider.OfType.
type errorCatchingSecretsManager struct {
	// The underlying secrets.Manager that will be used to perform actual operations.
	delegateManager secrets.Manager
	// A callback that will be invoked when an error is encountered during decryption. The callback can return a new error
	// to be returned, or nil to indicate that the error should be ignored and that one or more empty objects should be
	// returned as plaintext(s) instead.
	onDecryptError func(error) error
}

var _ secrets.Manager = (*errorCatchingSecretsManager)(nil)

func (m *errorCatchingSecretsManager) Type() string {
	return m.delegateManager.Type()
}

func (m *errorCatchingSecretsManager) State() json.RawMessage {
	return m.delegateManager.State()
}

func (m *errorCatchingSecretsManager) Encrypter() config.Encrypter {
	return m
}

func (m *errorCatchingSecretsManager) Decrypter() config.Decrypter {
	return m
}

func (m *errorCatchingSecretsManager) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return "", errors.New("error catching secrets manager does not support encryption")
}

func (m *errorCatchingSecretsManager) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	return nil, errors.New("error catching secrets manager does not support batch encryption")
}

func (m *errorCatchingSecretsManager) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	dec := m.delegateManager.Decrypter()
	if dec == nil {
		return "", errors.New("error catching secrets manager delegate returned a nil decrypter")
	}

	plaintext, err := dec.DecryptValue(ctx, ciphertext)
	if err == nil {
		return plaintext, nil
	}

	// We encountered an error decrypting the value. Pass it to our onDecryptError and return any error that the callback
	// returns.
	rethrownErr := m.onDecryptError(err)
	if rethrownErr != nil {
		return "", rethrownErr
	}

	// onDecryptError returned nil, so we will smother the error and return an empty object.
	return "{}", nil
}

func (m *errorCatchingSecretsManager) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	dec := m.delegateManager.Decrypter()
	if dec == nil {
		return nil, errors.New("error catching secrets manager delegate returned a nil decrypter")
	}

	plaintexts, err := dec.BatchDecrypt(ctx, ciphertexts)
	if err == nil {
		return plaintexts, nil
	}

	// We encountered an error decrypting the values. Pass it to our onDecryptError and return any error that the callback
	// returns.
	rethrownErr := m.onDecryptError(err)
	if rethrownErr != nil {
		return nil, rethrownErr
	}

	// onDecryptError returned nil, so we will smother the error and return a slice of empty objects whose length matches
	// that of the slice of ciphertexts we were given.
	results := make([]string, len(ciphertexts))
	for i := range ciphertexts {
		results[i] = "{}"
	}

	return results, nil
}
