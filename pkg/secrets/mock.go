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
	EncryptValueF func() string
	BulkEncryptF  func() []string
}

func (me *MockEncrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	if me.EncryptValueF != nil {
		return me.EncryptValueF(), nil
	}

	return "", errors.New("mock value not provided")
}

func (me *MockEncrypter) SupportsBulkEncryption(ctx context.Context) bool {
	return me.BulkEncryptF != nil
}

func (me *MockEncrypter) BulkEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	if me.BulkEncryptF == nil {
		return nil, errors.New("mock value not provided")
	}
	return me.BulkEncryptF(), nil
}

type MockDecrypter struct {
	DecryptValueF func() string
	BulkDecryptF  func() []string
}

func (md *MockDecrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	if md.DecryptValueF != nil {
		return md.DecryptValueF(), nil
	}

	return "", errors.New("mock value not provided")
}

func (md *MockDecrypter) BulkDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	if md.BulkDecryptF != nil {
		return md.BulkDecryptF(), nil
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
