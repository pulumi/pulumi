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
	"encoding/base64"
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

const Type = "cloud"

type cloudSecretsManagerState struct {
	URL string `json:"url"`
}

type provider struct{}

func (p *provider) FromState(state json.RawMessage) (secrets.Manager, error) {
	var s cloudSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, errors.Wrap(err, "unmarshalling state")
	}
	return &manager{
		url: s.URL,
	}, nil
}

// NewProvider returns a new manager provider which hands back Base64SecretsManagers
func NewProvider() secrets.ManagerProvider {
	return &provider{}
}

// NewSecretsManager returns a secrets manager that just base64 encodes instead of encrypting. Useful for testing.
func NewSecretsManager(url string) secrets.Manager {
	return &manager{
		url: url,
	}
}

type manager struct {
	url string
}

func (m *manager) Type() string                         { return Type }
func (m *manager) State() interface{}                   { return map[string]string{"url": m.url} }
func (m *manager) Encrypter() (config.Encrypter, error) { return &awsKmsCrypter{url: m.url}, nil }
func (m *manager) Decrypter() (config.Decrypter, error) { return &awsKmsCrypter{url: m.url}, nil }

type awsKmsCrypter struct {
	url string
}

func (c *awsKmsCrypter) EncryptValue(s string) (string, error) {
	keeper, err := gosecrets.OpenKeeper(context.Background(), c.url)
	if err != nil {
		return "", err
	}
	ciphertext, err := keeper.Encrypt(context.Background(), []byte(s))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *awsKmsCrypter) DecryptValue(s string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	keeper, err := gosecrets.OpenKeeper(context.Background(), c.url)
	if err != nil {
		return "", err
	}

	plaintext, err := keeper.Decrypt(context.Background(), ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
