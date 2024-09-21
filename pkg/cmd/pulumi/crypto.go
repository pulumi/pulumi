// Copyright 2016-2024, Pulumi Corporation.
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
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A stackSecretsManagerLoader provides methods for loading secrets managers and
// their encrypters and decrypters for a given stack and project stack. A loader
// encapsulates the logic for determining which secrets manager to use based on
// a given configuration, such as whether or not to fallback to the stack state
// if there is no secrets manager configured in the project stack.
type stackSecretsManagerLoader struct {
	// True if the loader should fallback to the stack state if there is no
	// secrets manager configured in the project stack.
	FallbackToState bool
}

// The state of a stack's secret manager configuration following an operation.
type stackSecretsManagerState string

const (
	// The state of the stack's secret manager configuration is unchanged.
	stackSecretsManagerUnchanged stackSecretsManagerState = "unchanged"

	// The stack's secret manager configuration has changed and should be saved to
	// the stack configuration file if possible. If saving is not possible, the
	// configuration can be restored by falling back to the state file.
	stackSecretsManagerShouldSave stackSecretsManagerState = "should-save"

	// The stack's secret manager configuration has changed and must be saved to the
	// stack configuration file. Changes have been made that do not align with the
	// state and so the state file cannot be used to restore the configuration.
	stackSecretsManagerMustSave stackSecretsManagerState = "must-save"
)

// Creates a new stack secrets manager loader from the environment.
func newStackSecretsManagerLoaderFromEnv() stackSecretsManagerLoader {
	return stackSecretsManagerLoader{
		FallbackToState: env.FallbackToStateSecretsManager.Value(),
	}
}

// Returns a decrypter for the given stack and project stack.
func (l *stackSecretsManagerLoader) getDecrypter(
	ctx context.Context,
	s backend.Stack,
	ps *workspace.ProjectStack,
) (config.Decrypter, stackSecretsManagerState, error) {
	sm, state, err := l.getSecretsManager(ctx, s, ps)
	if err != nil {
		return nil, stackSecretsManagerUnchanged, err
	}

	dec, err := sm.Decrypter()
	if err != nil {
		return nil, state, err
	}
	return dec, state, nil
}

// Returns an encrypter for the given stack and project stack.
func (l *stackSecretsManagerLoader) getEncrypter(
	ctx context.Context,
	s backend.Stack,
	ps *workspace.ProjectStack,
) (config.Encrypter, stackSecretsManagerState, error) {
	sm, state, err := l.getSecretsManager(ctx, s, ps)
	if err != nil {
		return nil, stackSecretsManagerUnchanged, err
	}

	enc, err := sm.Encrypter()
	if err != nil {
		return nil, state, err
	}
	return enc, state, nil
}

// Returns a secrets manager for the given stack and project stack.
func (l *stackSecretsManagerLoader) getSecretsManager(
	ctx context.Context,
	s backend.Stack,
	ps *workspace.ProjectStack,
) (secrets.Manager, stackSecretsManagerState, error) {
	oldConfig := deepcopy.Copy(ps).(*workspace.ProjectStack)

	var sm secrets.Manager
	var err error

	fellBack := false
	if ps.SecretsProvider != passphrase.Type && ps.SecretsProvider != "default" && ps.SecretsProvider != "" {
		sm, err = cloud.NewCloudSecretsManager(
			ps,
			ps.SecretsProvider,
			false, /* rotateSecretsProvider */
		)
	} else if ps.EncryptionSalt != "" {
		sm, err = passphrase.NewPromptingPassphraseSecretsManager(
			ps,
			false, /* rotateSecretsProvider */
		)
	} else {
		var fallbackManager secrets.Manager

		// If the loader has been configured to fallback to the stack state, we will
		// attempt to use the current snapshot secrets manager, if there is one.
		// This ensures that in cases where stack configuration is missing (or is
		// present but missing secrets provider configuration), we will keep using
		// what is already specified in the snapshot, rather than creating a new
		// default secrets manager which differs from what the user has previously
		// specified.
		if l.FallbackToState {
			snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
			if err != nil {
				return nil, stackSecretsManagerUnchanged, err
			}

			if snap != nil {
				fallbackManager = snap.SecretsManager
			}
		}

		if fallbackManager != nil {
			sm, fellBack = fallbackManager, true

			// We need to ensure the fallback manager picked saves to the stack state. TODO: It would be
			// really nice if the format of secrets state in the config file matched what managers reported
			// for state. That would go well with the pluginification of secret providers as well, but for now
			// just switch on the secret provider type and ask it to fill in the config file for us.
			if sm.Type() == passphrase.Type {
				err = passphrase.EditProjectStack(ps, sm.State())
			} else if sm.Type() == cloud.Type {
				err = cloud.EditProjectStack(ps, sm.State())
			} else {
				// Anything else assume we can just clear all the secret bits
				ps.EncryptionSalt = ""
				ps.SecretsProvider = ""
				ps.EncryptedKey = ""
			}
		} else {
			sm, err = s.DefaultSecretManager(ps)
		}
	}
	if err != nil {
		return nil, stackSecretsManagerUnchanged, err
	}

	// First, work out if the new configuration is different to the old one. If it is, and we fell back
	// to the state, we will return that the configuration *should* be saved. If the configurations
	// differ and we didn't fall back to the state (i.e. some brand new configuration was supplied to
	// us from an unknown source) then we will return that the configuration *must* be saved, lest it
	// be lost.
	needsSave := needsSaveProjectStackAfterSecretManger(oldConfig, ps)
	var state stackSecretsManagerState
	if needsSave {
		if fellBack {
			state = stackSecretsManagerShouldSave
		} else {
			state = stackSecretsManagerMustSave
		}
	} else {
		state = stackSecretsManagerUnchanged
	}

	return stack.NewCachingSecretsManager(sm), state, nil
}

func needsSaveProjectStackAfterSecretManger(
	old *workspace.ProjectStack, new *workspace.ProjectStack,
) bool {
	// We should only save the ProjectStack at this point IF we have changed the
	// secrets provider.
	// If we do not check to see if the secrets provider has changed, then we will actually
	// reload the configuration file to be sorted or an empty {} when creating a stack
	// this is not the desired behaviour.
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
