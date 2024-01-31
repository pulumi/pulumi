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

// Package cloud implements support for a generic cloud secret manager.
package cloud

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Type is the type of secrets managed by this secrets provider
const Type = "cloud"

type cloudSecretsManagerState struct {
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

// generateNewDataKey generates a new DataKey seeded by a fresh random 32-byte key and encrypted
// using the target cloud key management service.
func generateNewDataKey(url string) ([]byte, error) {
	plaintextDataKey := make([]byte, 32)
	_, err := rand.Read(plaintextDataKey)
	if err != nil {
		return nil, err
	}
	keeper, err := openKeeper(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return keeper.Encrypt(context.Background(), plaintextDataKey)
}

// newCloudSecretsManager returns a secrets manager that uses the target cloud key management
// service to encrypt/decrypt a data key used for envelope encryption of secrets values.
func newCloudSecretsManager(url string, encryptedDataKey []byte) (*Manager, error) {
	keeper, err := openKeeper(context.Background(), url)
	if err != nil {
		return nil, err
	}
	plaintextDataKey, err := keeper.Decrypt(context.Background(), encryptedDataKey)
	if err != nil {
		return nil, err
	}
	state, err := json.Marshal(cloudSecretsManagerState{
		URL:          url,
		EncryptedKey: encryptedDataKey,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling state: %w", err)
	}
	crypter := config.NewSymmetricCrypter(plaintextDataKey)
	return &Manager{
		crypter: crypter,
		state:   state,
	}, nil
}

// Manager is the secrets.Manager implementation for cloud key management services
type Manager struct {
	state   json.RawMessage
	crypter config.Crypter
}

func (m *Manager) Type() string                         { return Type }
func (m *Manager) State() json.RawMessage               { return m.state }
func (m *Manager) Encrypter() (config.Encrypter, error) { return m.crypter, nil }
func (m *Manager) Decrypter() (config.Decrypter, error) { return m.crypter, nil }

func EditProjectStack(info *workspace.ProjectStack, state json.RawMessage) error {
	info.EncryptionSalt = ""

	var s cloudSecretsManagerState
	err := json.Unmarshal(state, &s)
	if err != nil {
		return fmt.Errorf("unmarshalling cloud state: %w", err)
	}

	info.SecretsProvider = s.URL
	info.EncryptedKey = base64.StdEncoding.EncodeToString(s.EncryptedKey)
	return nil
}

// NewCloudSecretsManagerFromState deserialize configuration from state and returns a secrets
// manager that uses the target cloud key management service to encrypt/decrypt a data key used for
// envelope encryption of secrets values.
func NewCloudSecretsManagerFromState(state json.RawMessage) (secrets.Manager, error) {
	var s cloudSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, fmt.Errorf("unmarshalling state: %w", err)
	}

	return newCloudSecretsManager(s.URL, s.EncryptedKey)
}

func NewCloudSecretsManager(info *workspace.ProjectStack,
	secretsProvider string, rotateSecretsProvider bool,
) (secrets.Manager, error) {
	// Only a passphrase provider has an encryption salt. So changing a secrets provider
	// from passphrase to a cloud secrets provider should ensure that we remove the enryptionsalt
	// as it's a legacy artifact and needs to be removed
	info.EncryptionSalt = ""

	var secretsManager *Manager

	// Allow per-execution override of the secrets provider via an environment
	// variable. This allows a temporary replacement without updating the stack
	// config, such a during CI.
	if override := os.Getenv("PULUMI_CLOUD_SECRET_OVERRIDE"); override != "" {
		secretsProvider = override
	}

	// If we're rotating then just clear the key so we create a fresh one below
	if rotateSecretsProvider {
		info.EncryptedKey = ""
	}

	// if there is no key OR the secrets provider is changing
	// then we need to generate the new key based on the new secrets provider
	if info.EncryptedKey == "" || info.SecretsProvider != secretsProvider {
		dataKey, err := generateNewDataKey(secretsProvider)
		if err != nil {
			return nil, err
		}
		info.EncryptedKey = base64.StdEncoding.EncodeToString(dataKey)
	}
	info.SecretsProvider = secretsProvider

	dataKey, err := base64.StdEncoding.DecodeString(info.EncryptedKey)
	if err != nil {
		return nil, err
	}
	secretsManager, err = newCloudSecretsManager(secretsProvider, dataKey)
	if err != nil {
		return nil, err
	}

	return secretsManager, nil
}
