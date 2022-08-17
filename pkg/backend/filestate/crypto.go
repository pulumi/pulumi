// Copyright 2016-2022, Pulumi Corporation.
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

package filestate

import (
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewPassphraseSecretsManager(stackName tokens.Name, configFile string,
	rotatePassphraseSecretsProvider bool) (secrets.Manager, error) {
	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")

	if configFile == "" {
		f, err := workspace.DetectProjectStackPath(stackName.Q())
		if err != nil {
			return nil, err
		}
		configFile = f
	}

	info, err := workspace.LoadProjectStack(configFile)
	if err != nil {
		return nil, err
	}

	if rotatePassphraseSecretsProvider {
		info.EncryptionSalt = ""
	}

	// If there are any other secrets providers set in the config, remove them, as the passphrase
	// provider deals only with EncryptionSalt, not EncryptedKey or SecretsProvider.
	if info.EncryptedKey != "" || info.SecretsProvider != "" {
		info.EncryptedKey = ""
		info.SecretsProvider = ""
	}

	// If we have a salt, we can just use it.
	if info.EncryptionSalt != "" {
		return passphrase.NewPromptingPassphraseSecretsManager(info.EncryptionSalt)
	}

	// Otherwise, prompt the user for a new passphrase.
	salt, sm, err := passphrase.PromptForNewPassphrase(rotatePassphraseSecretsProvider)
	if err != nil {
		return nil, err
	}

	// Store the salt and save it.
	info.EncryptionSalt = salt
	if err = info.Save(configFile); err != nil {
		return nil, err
	}

	// Return the passphrase secrets manager.
	return sm, nil
}
