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

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	netUrl "net/url"
	"os"

	gosecrets "gocloud.dev/secrets"
	_ "gocloud.dev/secrets/awskms"        // support for awskms://
	_ "gocloud.dev/secrets/azurekeyvault" // support for azurekeyvault://
	"gocloud.dev/secrets/gcpkms"          // support for gcpkms://
	_ "gocloud.dev/secrets/hashivault"    // support for hashivault://
	"google.golang.org/api/cloudkms/v1"

	"github.com/pulumi/pulumi/pkg/v3/authhelpers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	secretsrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/secrets"
)

type cloudSecretsState struct {
	URL          string `json:"url"`
	EncryptedKey []byte `json:"encryptedkey"`
}

// openKeeper opens the keeper, handling pulumi-specifc cases in the URL.
func openKeeper(ctx context.Context, url string) (*gosecrets.Keeper, error) {
	u, err := netUrl.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the secrets provider URL: %w", err)
	}

	switch u.Scheme {
	case gcpkms.Scheme:
		credentials, err := authhelpers.ResolveGoogleCredentials(ctx, cloudkms.CloudkmsScope)
		if err != nil {
			return nil, fmt.Errorf("missing google credentials: %w", err)
		}

		kmsClient, _, err := gcpkms.Dial(ctx, credentials.TokenSource)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to gcpkms: %w", err)
		}
		opener := gcpkms.URLOpener{
			Client: kmsClient,
		}

		return opener.OpenKeeperURL(ctx, u)
	default:
		return gosecrets.OpenKeeper(ctx, url)
	}
}

func NewCloudSecretPlugin() secretsrpc.SecretsProviderServer {
	return &cloudSecretPlugin{}
}

type cloudSecretPlugin struct {
	crypter config.Crypter
}

func (p *cloudSecretPlugin) Encrypt(ctx context.Context, req *secretsrpc.EncryptRequest) (*secretsrpc.EncryptResponse, error) {
	if p.crypter == nil {
		return nil, fmt.Errorf("plugin not setup")
	}

	ciphertexts := make([]string, len(req.Plaintexts))
	for i, plaintext := range req.Plaintexts {
		ciphertext, err := p.crypter.EncryptValue(ctx, plaintext)
		if err != nil {
			return nil, err
		}
		ciphertexts[i] = ciphertext
	}

	return &secretsrpc.EncryptResponse{
		Ciphertexts: ciphertexts,
	}, nil
}

func (p *cloudSecretPlugin) Decrypt(ctx context.Context, req *secretsrpc.DecryptRequest) (*secretsrpc.DecryptResponse, error) {
	if p.crypter == nil {
		return nil, fmt.Errorf("plugin not setup")
	}

	plaintexts := make([]string, len(req.Ciphertexts))
	for i, ciphertext := range req.Ciphertexts {
		plaintext, err := p.crypter.DecryptValue(ctx, ciphertext)
		if err != nil {
			return nil, err
		}
		plaintexts[i] = plaintext
	}

	return &secretsrpc.DecryptResponse{
		Plaintexts: plaintexts,
	}, nil
}

func (p *cloudSecretPlugin) Configure(ctx context.Context, req *secretsrpc.ConfigureRequest) (*secretsrpc.ConfigureResponse, error) {
	if p.crypter != nil {
		return nil, fmt.Errorf("already setup")
	}

	var s cloudSecretsState
	if err := json.Unmarshal([]byte(req.State), &s); err != nil {
		return nil, fmt.Errorf("unmarshalling state: %w", err)
	}
	keeper, err := openKeeper(ctx, s.URL)
	if err != nil {
		return nil, err
	}
	defer keeper.Close()

	plaintextDataKey, err := keeper.Decrypt(ctx, s.EncryptedKey)
	if err != nil {
		return nil, err
	}
	p.crypter = config.NewSymmetricCrypter(plaintextDataKey)
	return &secretsrpc.ConfigureResponse{}, nil
}

func (p *cloudSecretPlugin) Initialize(ctx context.Context, req *secretsrpc.InitializeRequest) (*secretsrpc.InitializeResponse, error) {
	if p.crypter != nil {
		return nil, fmt.Errorf("already setup")
	}

	// We expect one argument here the url
	if len(req.Args) != 1 {
		return nil, fmt.Errorf("expected one argument got %d", len(req.Args))
	}

	url := req.Args[0]

	// Allow per-execution override of the secrets provider via an environment
	// variable. This allows a temporary replacement without updating the stack
	// config, such a during CI.
	if override := os.Getenv("PULUMI_CLOUD_SECRET_OVERRIDE"); override != "" {
		url = override
	}

	keeper, err := openKeeper(ctx, url)
	if err != nil {
		return nil, err
	}
	defer keeper.Close()

	plaintextDataKey := make([]byte, 32)
	_, err = rand.Read(plaintextDataKey)
	if err != nil {
		return nil, err
	}
	encryptedKey, err := keeper.Encrypt(ctx, plaintextDataKey)
	if err != nil {
		return nil, err
	}

	state := cloudSecretsState{
		URL:          url,
		EncryptedKey: encryptedKey,
	}
	jsonState, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	p.crypter = config.NewSymmetricCrypter(plaintextDataKey)
	return &secretsrpc.InitializeResponse{State: string(jsonState)}, nil
}
