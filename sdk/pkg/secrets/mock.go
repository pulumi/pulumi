// Copyright 2016-2023, Pulumi Corporation.
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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

type MockSecretsManager struct {
	TypeF      func() string
	StateF     func() json.RawMessage
	EncrypterF func() config.Encrypter
	DecrypterF func() config.Decrypter
}

var _ Manager = &MockSecretsManager{}

func (msm *MockSecretsManager) Type() string {
	if msm.TypeF != nil {
		return msm.TypeF()
	}

	panic("not implemented")
}

func (msm *MockSecretsManager) State() json.RawMessage {
	if msm.StateF != nil {
		return msm.StateF()
	}

	panic("not implemented")
}

func (msm *MockSecretsManager) Encrypter() config.Encrypter {
	if msm.EncrypterF != nil {
		return msm.EncrypterF()
	}

	panic("not implemented")
}

func (msm *MockSecretsManager) Decrypter() config.Decrypter {
	if msm.DecrypterF != nil {
		return msm.DecrypterF()
	}

	panic("not implemented")
}

type MockEncrypter struct {
	EncryptValueF func(plaintext string) string
	BatchEncryptF func(secrets []string) []string
}

func (me *MockEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	if me.EncryptValueF != nil {
		return me.EncryptValueF(plaintext), nil
	}

	return "", errors.New("mock value not provided")
}

func (me *MockEncrypter) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	if me.BatchEncryptF != nil {
		return me.BatchEncryptF(secrets), nil
	}
	return nil, errors.New("batch encrypt mock not provided")
}

type MockDecrypter struct {
	DecryptValueF func(ciphertext string) string
	BatchDecryptF func(ciphertexts []string) []string
}

func (md *MockDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if md.DecryptValueF != nil {
		return md.DecryptValueF(ciphertext), nil
	}

	return "", errors.New("mock value not provided")
}

func (md *MockDecrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	if md.BatchDecryptF != nil {
		return md.BatchDecryptF(ciphertexts), nil
	}

	return nil, errors.New("mock value not provided")
}

type MockProvider struct {
	managers map[string]func(json.RawMessage) (Manager, error)
}

func (mp *MockProvider) OfType(ty string, state json.RawMessage) (Manager, error) {
	if f, ok := mp.managers[ty]; ok {
		return f(state)
	}

	return nil, fmt.Errorf("no known secrets provider for type %q", ty)
}

// Return a new MockProvider with the given type and manager function registered.
func (mp *MockProvider) Add(ty string, f func(json.RawMessage) (Manager, error)) *MockProvider {
	new := &MockProvider{
		managers: make(map[string]func(json.RawMessage) (Manager, error)),
	}
	for k, v := range mp.managers {
		new.managers[k] = v
	}
	new.managers[ty] = f
	return new
}
