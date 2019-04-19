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

	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/resource/config"
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
