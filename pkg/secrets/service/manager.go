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

// Package service implements support for the Pulumi Service secret manager.
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const Type = "service"

// serviceCrypter is an encrypter/decrypter that uses the Pulumi servce to encrypt/decrypt a stack's secrets.
type serviceCrypter struct {
	client *client.Client
	stack  client.StackIdentifier
}

func newServiceCrypter(client *client.Client, stack client.StackIdentifier) config.Crypter {
	return &serviceCrypter{client: client, stack: stack}
}

func (c *serviceCrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	ciphertext, err := c.client.EncryptValue(ctx, c.stack, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *serviceCrypter) DecryptValue(ctx context.Context, cipherstring string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherstring)
	if err != nil {
		return "", err
	}

	plaintext, err := c.client.DecryptValue(ctx, c.stack, ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (c *serviceCrypter) BulkDecrypt(ctx context.Context, secrets []string) ([]string, error) {
	secretsToDecrypt := slice.Prealloc[[]byte](len(secrets))
	for _, val := range secrets {
		ciphertext, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return nil, err
		}
		secretsToDecrypt = append(secretsToDecrypt, ciphertext)
	}

	decryptedList, err := c.client.BulkDecryptValue(ctx, c.stack, secretsToDecrypt)
	if err != nil {
		return nil, err
	}

	decryptedSecrets := make([]string, len(secrets))
	for i, ciphertext := range secrets {
		decrypted, ok := decryptedList[ciphertext]
		contract.Assertf(ok, "decrypted value not found in bulk response")
		decryptedSecrets[i] = string(decrypted)
	}

	return decryptedSecrets, nil
}

type serviceSecretsManagerState struct {
	URL      string `json:"url,omitempty"`
	Owner    string `json:"owner"`
	Project  string `json:"project"`
	Stack    string `json:"stack"`
	Insecure bool   `json:"insecure,omitempty"`
}

var _ secrets.Manager = &serviceSecretsManager{}

type serviceSecretsManager struct {
	state   json.RawMessage
	crypter config.Crypter
}

func (sm *serviceSecretsManager) Type() string {
	return Type
}

func (sm *serviceSecretsManager) State() json.RawMessage {
	return sm.state
}

func (sm *serviceSecretsManager) Decrypter() (config.Decrypter, error) {
	contract.Assertf(sm.crypter != nil, "decrypter not initialized")
	return sm.crypter, nil
}

func (sm *serviceSecretsManager) Encrypter() (config.Encrypter, error) {
	contract.Assertf(sm.crypter != nil, "encrypter not initialized")
	return sm.crypter, nil
}

func NewServiceSecretsManager(
	client *client.Client, id client.StackIdentifier, info *workspace.ProjectStack,
) (secrets.Manager, error) {
	// To change the secrets provider to a serviceSecretsManager we would need to ensure that there are no
	// remnants of the old secret manager To remove those remnants, we would set those values to be empty in
	// the project stack.
	// A passphrase secrets provider has an encryption salt, therefore, changing
	// from passphrase to serviceSecretsManager requires the encryption salt
	// to be removed.
	// A cloud secrets manager has an encryption key and a secrets provider,
	// therefore, changing from cloud to serviceSecretsManager requires the
	// encryption key and secrets provider to be removed.
	// Regardless of what the current secrets provider is, all of these values
	// need to be empty otherwise `getStackSecretsManager` in crypto.go can
	// potentially return the incorrect secret type for the stack.
	info.EncryptionSalt = ""
	info.SecretsProvider = ""
	info.EncryptedKey = ""

	state, err := json.Marshal(serviceSecretsManagerState{
		URL:      client.URL(),
		Owner:    id.Owner,
		Project:  id.Project,
		Stack:    id.Stack.String(),
		Insecure: client.Insecure(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling state: %w", err)
	}

	return &serviceSecretsManager{
		state:   state,
		crypter: newServiceCrypter(client, id),
	}, nil
}

// NewServiceSecretsManagerFromState returns a Pulumi service-based secrets manager based on the
// existing state.
func NewServiceSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	var s serviceSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, fmt.Errorf("unmarshalling state: %w", err)
	}

	account, err := workspace.GetAccount(s.URL)
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}
	token := account.AccessToken

	if token == "" {
		return nil, fmt.Errorf("could not find access token for %s, have you logged in?", s.URL)
	}

	stack, err := tokens.ParseStackName(s.Stack)
	if err != nil {
		return nil, fmt.Errorf("parsing stack name: %w", err)
	}

	id := client.StackIdentifier{
		Owner:   s.Owner,
		Project: s.Project,
		Stack:   stack,
	}
	c := client.NewClient(s.URL, token, s.Insecure, diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	}))

	return &serviceSecretsManager{
		state:   state,
		crypter: newServiceCrypter(c, id),
	}, nil
}
