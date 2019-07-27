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
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
	"github.com/pulumi/pulumi/pkg/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func getStackEncrypter(s backend.Stack) (config.Encrypter, error) {
	sm, err := getStackSecretsManager(s)
	if err != nil {
		return nil, err
	}

	return sm.Encrypter()
}

func getStackDencrypter(s backend.Stack) (config.Decrypter, error) {
	sm, err := getStackSecretsManager(s)
	if err != nil {
		return nil, err
	}

	return sm.Decrypter()
}

func getStackSecretsManager(s backend.Stack) (secrets.Manager, error) {
	ps, err := loadProjectStack(s)
	if err != nil {
		return nil, err
	}

	if ps.SecretsProvider != "" {
		return newCloudSecretsManager(s.Ref().Name(), stackConfigFile, ps.SecretsProvider)
	}

	if ps.EncryptionSalt != "" {
		return newPassphraseSecretsManager(s.Ref().Name(), stackConfigFile)
	}

	switch stack := s.(type) {
	case httpstate.Stack:
		return newServiceSecretsManager(stack)
	case filestate.Stack:
		return newPassphraseSecretsManager(s.Ref().Name(), stackConfigFile)
	}

	return nil, errors.Errorf("unknown stack type %s", reflect.TypeOf(s))
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

func newCloudSecretsManager(stackName tokens.QName, configFile string, secretsProvider string) (secrets.Manager, error) {
	contract.Assertf(stackName != "", "stackName %s", "!= \"\"")

	if configFile == "" {
		f, err := workspace.DetectProjectStackPath(stackName)
		if err != nil {
			return nil, err
		}
		configFile = f
	}

	info, err := workspace.LoadProjectStack(configFile)
	if err != nil {
		return nil, err
	}
	info.SecretsProvider = secretsProvider
	if err = info.Save(configFile); err != nil {
		return nil, err
	}
	return cloud.NewSecretsManager(secretsProvider), nil
}
