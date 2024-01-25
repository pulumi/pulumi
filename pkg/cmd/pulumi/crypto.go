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
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func getStackEncrypter(s backend.Stack, ps *workspace.ProjectStack) (config.Encrypter, bool, error) {
	sm, needsSave, err := getStackSecretsManager(s, ps, nil)
	if err != nil {
		return nil, false, err
	}

	enc, err := sm.Encrypter()
	if err != nil {
		return nil, needsSave, err
	}
	return enc, needsSave, nil
}

func getStackDecrypter(s backend.Stack, ps *workspace.ProjectStack) (config.Decrypter, bool, error) {
	sm, needsSave, err := getStackSecretsManager(s, ps, nil)
	if err != nil {
		return nil, false, err
	}

	dec, err := sm.Decrypter()
	if err != nil {
		return nil, needsSave, err
	}
	return dec, needsSave, nil
}

func getStackSecretsManager(
	s backend.Stack, ps *workspace.ProjectStack, fallbackManager secrets.Manager,
) (secrets.Manager, bool, error) {
	smTy, smState, err := secrets.GetConfig(ps)
	if err != nil {
		return nil, false, err
	}

	var sm secrets.Manager
	if smTy == "" {
		if fallbackManager != nil {
			sm = fallbackManager
		} else {
			sm, err = s.DefaultSecretManager()
		}
	} else {
		sm, err = stack.DefaultSecretsProvider.OfType(smTy, smState)
	}
	if err != nil {
		return nil, false, err
	}

	oldConfig := deepcopy.Copy(ps).(*workspace.ProjectStack)
	err = secrets.SetConfig(sm.Type(), sm.State(), ps)
	if err != nil {
		return nil, false, err
	}

	// Handle if the configuration changed any of EncryptedKey, etc
	needsSave := needsSaveProjectStackAfterSecretManger(s, oldConfig, ps)
	return stack.NewCachingSecretsManager(sm), needsSave, nil
}

func needsSaveProjectStackAfterSecretManger(stack backend.Stack,
	old *workspace.ProjectStack, new *workspace.ProjectStack,
) bool {
	// We should only save the ProjectStack at this point IF we have changed the
	// secrets provider.
	// If we do not check to see if the secrets provider has changed, then we will actually
	// reload the configuration file to be sorted or an empty {} when creating a stack
	// this is not the desired behavior.
	if old.EncryptedKey != new.EncryptedKey ||
		old.EncryptionSalt != new.EncryptionSalt ||
		old.SecretsProvider != new.SecretsProvider {
		return true
	}
	return false
}

func validateSecretsProvider(typ string) error {
	kind := strings.SplitN(typ, ":", 2)[0]
	supportedKinds := []string{"default", "passphrase", "awskms", "azurekeyvault", "gcpkms", "hashivault"}
	for _, supportedKind := range supportedKinds {
		if kind == supportedKind {
			return nil
		}
	}
	return fmt.Errorf("unknown secrets provider type '%s' (supported values: %s)",
		kind,
		strings.Join(supportedKinds, ","))
}
