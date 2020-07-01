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

package main

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/backend/cli"
	"github.com/pulumi/pulumi/pkg/v2/backend/pulumi"
	"github.com/pulumi/pulumi/pkg/v2/resource/stack"
	"github.com/pulumi/pulumi/pkg/v2/secrets"
	"github.com/pulumi/pulumi/pkg/v2/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v2/secrets/service"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
)

func getStackEncrypter(s *cli.Stack) (config.Encrypter, error) {
	sm, err := getStackSecretsManager(s)
	if err != nil {
		return nil, err
	}

	return sm.Encrypter()
}

func getStackDecrypter(s *cli.Stack) (config.Decrypter, error) {
	sm, err := getStackSecretsManager(s)
	if err != nil {
		return nil, err
	}

	return sm.Decrypter()
}

func getStackSecretsManager(s *cli.Stack) (secrets.Manager, error) {
	ps, err := loadProjectStack(s)
	if err != nil {
		return nil, err
	}

	sm, err := func() (secrets.Manager, error) {
		secretsProvider := ps.SecretsProvider
		if ps.SecretsProvider == "default" || ps.SecretsProvider == "" {
			secretsProvider = s.Backend().Client().DefaultSecretsManager()
		}

		switch secretsProvider {
		case passphrase.Type:
			return newPassphraseSecretsManager(s.ID(), stackConfigFile,
				false /* rotatePassphraseSecretsProvider */)
		case service.Type:
			// If there's already an encryption salt, use the passphrase secrets manager.
			if ps.EncryptionSalt != "" {
				return newPassphraseSecretsManager(s.ID(), stackConfigFile,
					false /* rotatePassphraseSecretsProvider */)
			}

			// Ensure that this stack is bound to an API server.
			client, ok := s.Backend().Client().(*pulumi.Client)
			if !ok {
				return nil, errors.Errorf("only the service backend supports service-managed secrets")
			}
			return newServiceSecretsManager(s.ID(), stackConfigFile, client.APIClient())
		default:
			return newCloudSecretsManager(s.ID(), stackConfigFile, ps.SecretsProvider)
		}
	}()
	if err != nil {
		return nil, err
	}
	return stack.NewCachingSecretsManager(sm), nil
}

func validateSecretsProvider(typ string) error {
	kind := strings.SplitN(typ, ":", 2)[0]
	supportedKinds := []string{"default", "passphrase", "awskms", "azurekeyvault", "gcpkms", "hashivault"}
	for _, supportedKind := range supportedKinds {
		if kind == supportedKind {
			return nil
		}
	}
	return errors.Errorf(
		"unknown secrets provider type '%s' (supported values: %s)",
		kind,
		strings.Join(supportedKinds, ","),
	)
}
