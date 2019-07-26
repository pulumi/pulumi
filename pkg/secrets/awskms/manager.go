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

package awskms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	gosecrets "gocloud.dev/secrets"
	_ "gocloud.dev/secrets/awskms" // driver for awskms://

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
)

const Type = "awskms"

type provider struct{}

func (p *provider) FromState(state json.RawMessage) (secrets.Manager, error) {
	return &manager{}, nil
}

// NewProvider returns a new manager provider which hands back Base64SecretsManagers
func NewProvider() secrets.ManagerProvider {
	return &provider{}
}

// NewAwsKmsSecretsManager returns a secrets manager that just base64 encodes instead of encrypting. Useful for testing.
func NewAwsKmsSecretsManager() secrets.Manager {
	return &manager{}
}

type manager struct{}

func (m *manager) Type() string                         { return Type }
func (m *manager) State() interface{}                   { return map[string]string{} }
func (m *manager) Encrypter() (config.Encrypter, error) { return &awsKmsCrypter{}, nil }
func (m *manager) Decrypter() (config.Decrypter, error) { return &awsKmsCrypter{}, nil }

type awsKmsCrypter struct{}

func (c *awsKmsCrypter) EncryptValue(s string) (string, error) {
	fmt.Printf("encyrpting with awskms://alias/LukeTesting?region=us-west-2\n")
	keeper, err := gosecrets.OpenKeeper(context.Background(), "awskms://alias/LukeTesting?region=us-west-2")
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
	fmt.Printf("decrypting with awskms://alias/LukeTesting?region=us-west-2\n")
	ciphertext, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	keeper, err := gosecrets.OpenKeeper(context.Background(), "awskms://alias/LukeTesting?region=us-west-2")
	if err != nil {
		return "", err
	}

	plaintext, err := keeper.Decrypt(context.Background(), ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
