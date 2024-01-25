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

// Package secrets defines the interface common to all secret managers.
package secrets

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Manager provides the interface for providing stack encryption.
type Manager interface {
	// Type returns a string that reflects the type of this provider. This is serialized along with the state of
	// the manager into the deployment such that we can re-construct the correct manager when deserializing a
	// deployment into a snapshot.
	Type() string
	// An opaque JSON blob, which can be used later to reconstruct the provider when deserializing the
	// deployment into a snapshot.
	State() json.RawMessage
	// Encrypter returns a `config.Encrypter` that can be used to encrypt values when serializing a snapshot into a
	// deployment, or an error if one can not be constructed.
	Encrypter() (config.Encrypter, error)
	// Decrypter returns a `config.Decrypter` that can be used to decrypt values when deserializing a snapshot from a
	// deployment, or an error if one can not be constructed.
	Decrypter() (config.Decrypter, error)
}

// AreCompatible returns true if the two Managers are of the same type and have the same state.
func AreCompatible(a, b Manager) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if a.Type() != b.Type() {
		return false
	}

	as := a.State()
	bs := b.State()
	return bytes.Equal(as, bs)
}

// SetConfig sets the manager for the given type and state on the project. This holds all the compatibility code for
// setting the legacy EncryptionSalt, Key, etc fields.
func SetConfig(ty string, state json.RawMessage, project *workspace.ProjectStack) error {
	if ty == "service" {
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
		project.EncryptionSalt = ""
		project.SecretsProvider = ""
		project.EncryptedKey = ""
		return nil
	} else if ty == "passphrase" {
		// If there are any other secrets providers set in the config, remove them, as the passphrase
		// provider deals only with EncryptionSalt, not EncryptedKey or SecretsProvider.
		project.EncryptedKey = ""

		// If the secrets provider is explicitly set to "passphrase" we should maintain that when setting
		// passphrase config. But for all other cases we should clear it out and rely on just having
		// EncryptionSalt set.
		if project.SecretsProvider != "passphrase" {
			project.SecretsProvider = ""
		}

		var s struct {
			Salt string `json:"salt"`
		}
		err := json.Unmarshal(state, &s)
		if err != nil {
			return fmt.Errorf("unmarshalling passphrase state: %w", err)
		}
		project.EncryptionSalt = s.Salt
		/*
			// If our encryption salt is empty we need to set SecretsProvider to "passphrase" so we roundtrip back
			// to the passphrase type.
			if project.EncryptionSalt == "" {
				project.SecretsProvider = "passphrase"
			}
		*/
		return nil
	} else if ty == "cloud" {
		// Only a passphrase provider has an encryption salt. So changing a secrets provider from passphrase to a cloud
		// secrets provider should ensure that we remove the encryptionsalt as it's a legacy artifact and needs to be
		// removed
		project.EncryptionSalt = ""

		var s struct {
			URL          string `json:"url"`
			EncryptedKey []byte `json:"encryptedkey"`
		}
		err := json.Unmarshal(state, &s)
		if err != nil {
			return fmt.Errorf("unmarshalling cloud state: %w", err)
		}
		project.SecretsProvider = s.URL
		project.EncryptedKey = b64.StdEncoding.EncodeToString(s.EncryptedKey)
	} else {
		// For now assume anything else doesn't have config. Longer term (i.e. when we get secret plugins) we'll need a
		// place in the config file to store their state.
		project.EncryptionSalt = ""
		project.SecretsProvider = ""
		project.EncryptedKey = ""
	}

	return nil
}

// GetConfig
func GetConfig(project *workspace.ProjectStack) (string, json.RawMessage, error) {
	isCloud := project.SecretsProvider != "" &&
		project.SecretsProvider != "passphrase" &&
		project.SecretsProvider != "default"

	isPassphrase := project.EncryptionSalt != ""

	if isCloud {
		var s struct {
			URL          string `json:"url,omitempty"`
			EncryptedKey []byte `json:"encryptedkey,omitempty"`
		}
		ek, err := b64.StdEncoding.DecodeString(project.EncryptedKey)
		if err != nil {
			return "", nil, fmt.Errorf("decoding encrypted key: %w", err)
		}
		s.EncryptedKey = ek
		s.URL = project.SecretsProvider

		bytes, err := json.Marshal(s)
		contract.Requiref(err == nil, "err", "marshalling cloud state: %v", err)
		return "cloud", bytes, nil
	} else if isPassphrase {
		var s struct {
			Salt string `json:"salt,omitempty"`
		}
		s.Salt = project.EncryptionSalt

		bytes, err := json.Marshal(s)
		contract.Requiref(err == nil, "err", "marshalling passphrase state: %v", err)
		return "passphrase", bytes, nil
	}

	return "", nil, nil
}
