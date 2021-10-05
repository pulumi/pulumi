// Copyright 2016-2021, Pulumi Corporation.
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

// Package service implements support for the Pulumi Service secret manager.
package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io/ioutil"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const Type = "service"

type serviceSecretsManagerState struct {
	URL          string `json:"url,omitempty"`
	Owner        string `json:"owner"`
	Project      string `json:"project"`
	Stack        string `json:"stack"`
	EncryptedKey []byte `json:"encryptedkey"`
}

var _ secrets.Manager = &serviceSecretsManager{}

type serviceSecretsManager struct {
	state   serviceSecretsManagerState
	crypter config.Crypter
}

func (sm *serviceSecretsManager) Type() string {
	return Type
}

func (sm *serviceSecretsManager) State() interface{} {
	return sm.state
}

func (sm *serviceSecretsManager) Decrypter() (config.Decrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func (sm *serviceSecretsManager) Encrypter() (config.Encrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func NewServiceSecretsManager(c *client.Client, id client.StackIdentifier,
	encryptedDataKey []byte) (secrets.Manager, error) {
	plaintextDataKey, err := c.DecryptValue(context.Background(), id, encryptedDataKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt data key")
	}
	crypter := config.NewSymmetricCrypter(plaintextDataKey)
	return &serviceSecretsManager{
		state: serviceSecretsManagerState{
			URL:          c.URL(),
			Owner:        id.Owner,
			Project:      id.Project,
			Stack:        id.Stack,
			EncryptedKey: encryptedDataKey,
		},
		crypter: crypter,
	}, nil
}

// NewServiceSecretsManagerFromState returns a Pulumi service-based secrets manager based on the
// existing state.
func NewServiceSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	var s serviceSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, errors.Wrap(err, "unmarshalling state")
	}

	account, err := workspace.GetAccount(s.URL)
	if err != nil {
		return nil, errors.Wrap(err, "getting access token")
	}
	token := account.AccessToken

	if token == "" {
		return nil, errors.Errorf("could not find access token for %s, have you logged in?", s.URL)
	}

	id := client.StackIdentifier{
		Owner:   s.Owner,
		Project: s.Project,
		Stack:   s.Stack,
	}
	c := client.NewClient(s.URL, token, diag.DefaultSink(ioutil.Discard, ioutil.Discard, diag.FormatOptions{}))
	return NewServiceSecretsManager(c, id, s.EncryptedKey)
}

// GenerateNewDataKey generates a new DataKey seeded by a fresh random 32-byte key and encrypted
// using the target cloud key management service.
func GenerateNewDataKey(stack client.StackIdentifier, c *client.Client) ([]byte, error) {
	plaintextDataKey := make([]byte, 32)
	_, err := rand.Read(plaintextDataKey)
	if err != nil {
		return nil, err
	}
	return c.EncryptValue(context.Background(), stack, plaintextDataKey)
}
