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

package cloud

import (
	"context"
	"crypto/rand"
	"encoding/json"

	"github.com/pkg/errors"
	gosecrets "gocloud.dev/secrets"
	_ "gocloud.dev/secrets/awskms"        // support for awskms://
	_ "gocloud.dev/secrets/azurekeyvault" // support for azurekeyvault://
	_ "gocloud.dev/secrets/gcpkms"        // support for gcpkms://
	_ "gocloud.dev/secrets/hashivault"    // support for hashivault://

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
)

// Type is the type of secrets managed by this secrets provider
const Type = "cloud"

type cloudState struct {
	URL     string `json:"url"`
	DataKey []byte `json:"dataKey"`
}

type provider struct{}

func (p *provider) FromState(state json.RawMessage) (secrets.Manager, error) {
	var s cloudState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, errors.Wrap(err, "unmarshalling state")
	}

	return NewSecretsManager(s.URL, s.DataKey)
}

// NewProvider returns a new manager provider which hands back Base64SecretsManagers
func NewProvider() secrets.ManagerProvider {
	return &provider{}
}

// GenerateNewDataKey generates a new DataKey seeded by a fresh random 32-byte key and encrypted
// using the target coud key management service.
func GenerateNewDataKey(url string) ([]byte, error) {
	plaintextDataKey := make([]byte, 32)
	_, err := rand.Read(plaintextDataKey)
	if err != nil {
		return nil, err
	}
	keeper, err := gosecrets.OpenKeeper(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return keeper.Encrypt(context.Background(), plaintextDataKey)
}

// NewSecretsManager returns a secrets manager that uses the target cloud key management service to
// encrypt/decrypt a data key used for envelope encyrtion of secrets values.
func NewSecretsManager(url string, encryptedDataKey []byte) (*Manager, error) {
	keeper, err := gosecrets.OpenKeeper(context.Background(), url)
	if err != nil {
		return nil, err
	}
	plaintextDataKey, err := keeper.Decrypt(context.Background(), encryptedDataKey)
	if err != nil {
		return nil, err
	}
	crypter := config.NewSymmetricCrypter(plaintextDataKey)
	return &Manager{
		crypter: crypter,
		state: cloudState{
			URL:     url,
			DataKey: encryptedDataKey,
		},
	}, nil
}

// Manager is the secrets.Manager implementation for cloud key management services
type Manager struct {
	crypter config.Crypter
	state   cloudState
}

func (m *Manager) Type() string                         { return Type }
func (m *Manager) State() interface{}                   { return m.state }
func (m *Manager) Encrypter() (config.Encrypter, error) { return m.crypter, nil }
func (m *Manager) Decrypter() (config.Decrypter, error) { return m.crypter, nil }
func (m *Manager) DataKey() []byte                      { return m.state.DataKey }
