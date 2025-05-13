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

type stackOutputsProvider struct {
	delegate secrets.Provider

	lastDecryptError error
}

var _ secrets.Provider = (*stackOutputsProvider)(nil)

func newStackOutputsProvider(delegate secrets.Provider) *stackOutputsProvider {
	return &stackOutputsProvider{
		delegate: delegate,
	}
}

func (p *stackOutputsProvider) Error() error {
	return p.lastDecryptError
}

func (p *stackOutputsProvider) OfType(ty string, state json.RawMessage) (secrets.Manager, error) {
	delegateManager, err := p.delegate.OfType(ty, state)
	if err != nil {
		return nil, err
	}

	delegateDecrypter := delegateManager.Decrypter()
	if delegateDecrypter == nil {
		return nil, errors.New("stack outputs manager requires a decrypter")
	}

	manager := &stackOutputsManager{
		parentProvider:    p,
		delegateManager:   delegateManager,
		delegateDecrypter: delegateDecrypter,
	}

	return manager, nil
}

type stackOutputsManager struct {
	parentProvider *stackOutputsProvider

	delegateManager   secrets.Manager
	delegateDecrypter config.Decrypter
}

var _ secrets.Manager = (*stackOutputsManager)(nil)

func (m *stackOutputsManager) Type() string {
	return m.delegateManager.Type()
}

func (m *stackOutputsManager) State() json.RawMessage {
	return m.delegateManager.State()
}

func (m *stackOutputsManager) Encrypter() config.Encrypter {
	return m
}

func (m *stackOutputsManager) Decrypter() config.Decrypter {
	return m
}

func (m *stackOutputsManager) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	return "", errors.New("stack outputs manager does not support encryption")
}

func (m *stackOutputsManager) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	return nil, errors.New("stack outputs manager does not support batch encryption")
}

func (m *stackOutputsManager) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	plaintext, err := m.delegateDecrypter.DecryptValue(ctx, ciphertext)
	if err == nil {
		return plaintext, nil
	}

	m.parentProvider.lastDecryptError = err
	return "{}", nil
}

func (m *stackOutputsManager) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	plaintexts, err := m.delegateDecrypter.BatchDecrypt(ctx, ciphertexts)
	if err == nil {
		return plaintexts, nil
	}

	m.parentProvider.lastDecryptError = err

	results := make([]string, len(ciphertexts))
	for i := range ciphertexts {
		results[i] = "{}"
	}

	return results, nil
}
