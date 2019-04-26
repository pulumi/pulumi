// Copyright 2016-2019, Pulumi Corporation.
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
package cmd

import (
	"context"
	"encoding/base64"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
)

// cloudCrypter is an encrypter/decrypter that uses the Pulumi cloud to encrypt/decrypt a stack's secrets.
type cloudCrypter struct {
	client *client.Client
	stack  client.StackIdentifier
}

func newCloudCrypter(client *client.Client, stack client.StackIdentifier) config.Crypter {
	return &cloudCrypter{client: client, stack: stack}
}

func (c *cloudCrypter) EncryptValue(plaintext string) (string, error) {
	ciphertext, err := c.client.EncryptValue(context.Background(), c.stack, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *cloudCrypter) DecryptValue(cipherstring string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherstring)
	if err != nil {
		return "", err
	}
	plaintext, err := c.client.DecryptValue(context.Background(), c.stack, ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

type cloudSecretsManagerState struct {
	URL     string `json:"url,omitempty"`
	Owner   string `json:"owner"`
	Project string `json:"project"`
	Stack   string `json:"stack"`
}

var _ secrets.Manager = &cloudSecretsManager{}

type cloudSecretsManager struct {
	state   cloudSecretsManagerState
	crypter config.Crypter
}

func (sm *cloudSecretsManager) Type() string {
	return "pulumi"
}

func (sm *cloudSecretsManager) State() interface{} {
	return sm.state
}

func (sm *cloudSecretsManager) Decrypter() (config.Decrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func (sm *cloudSecretsManager) Encrypter() (config.Encrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func newCloudSecretsManager(s httpstate.Stack) secrets.Manager {
	b := s.Backend().(httpstate.Backend)
	id := s.StackIdentifier()

	return &cloudSecretsManager{
		state: cloudSecretsManagerState{
			URL:     b.CloudURL(),
			Owner:   id.Owner,
			Project: id.Project,
			Stack:   id.Stack,
		},
		crypter: newCloudCrypter(b.Client(), id),
	}
}
