// Copyright 2024, Pulumi Corporation.
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

package stack

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Creates a secrets manager for an existing stack, using the stack to pick defaults if necessary and writing any
// changes back to the stack's configuration where applicable.
func CreateSecretsManagerForExistingStack(
	_ context.Context, ws pkgWorkspace.Context, stack backend.Stack, secretsProvider string,
	rotateSecretsProvider, creatingStack bool,
) error {
	// As part of creating the stack, we also need to configure the secrets provider for the stack.
	// We need to do this configuration step for cases where we will be using with the passphrase
	// secrets provider or one of the cloud-backed secrets providers.  We do not need to do this
	// for the Pulumi Cloud backend secrets provider.
	// we have an explicit flag to rotate the secrets manager ONLY when it's a passphrase!
	isDefaultSecretsProvider := secretsProvider == "" || secretsProvider == "default"

	// If we're creating the stack, it's the default secrets provider, and it's the cloud backend
	// return early to avoid probing for the project and stack config files, which otherwise
	// would fail when creating a stack from a directory that does not have a project file.
	if isDefaultSecretsProvider && creatingStack {
		if _, isCloud := stack.Backend().(httpstate.Backend); isCloud {
			return nil
		}
	}

	project, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	ps, err := LoadProjectStack(project, stack)
	if err != nil {
		return err
	}

	oldConfig := deepcopy.Copy(ps).(*workspace.ProjectStack)
	if isDefaultSecretsProvider {
		_, err = stack.DefaultSecretManager(ps)
	} else if secretsProvider == passphrase.Type {
		_, err = passphrase.NewPromptingPassphraseSecretsManager(ps, rotateSecretsProvider)
	} else {
		// All other non-default secrets providers are handled by the cloud secrets provider which
		// uses a URL schema to identify the provider
		_, err = cloud.NewCloudSecretsManager(ps, secretsProvider, rotateSecretsProvider)
	}
	if err != nil {
		return err
	}

	// Handle if the configuration changed any of EncryptedKey, etc
	if needsSaveProjectStackAfterSecretManger(oldConfig, ps) {
		if err = workspace.SaveProjectStack(stack.Ref().Name().Q(), ps); err != nil {
			return fmt.Errorf("saving stack config: %w", err)
		}
	}

	return nil
}

// Creates a secrets manager for a new stack. If a stack configuration already exists (e.g. the user has created a
// Pulumi.<stack>.yaml file themselves, prior to stack initialisation), try to respect the settings therein. Otherwise,
// fall back to a default defined by the backend that will manage the stack.
func createSecretsManagerForNewStack(
	ws pkgWorkspace.Context,
	b backend.Backend,
	stackRef backend.StackReference,
	secretsProvider string,
) (*workspace.ProjectStack, bool, secrets.Manager, error) {
	var sm secrets.Manager

	// Attempt to read a stack configuration, since it's possible that the user may have supplied one even though the
	// stack has not actually been created yet. If we fail to read one, that's OK -- we'll just create a new one and
	// populate it as we go.
	var ps *workspace.ProjectStack
	project, _, err := ws.ReadProject()
	if err != nil {
		ps = &workspace.ProjectStack{}
	} else {
		ps, err = loadProjectStackByReference(project, stackRef)
		if err != nil {
			ps = &workspace.ProjectStack{}
		}
	}

	oldConfig := deepcopy.Copy(ps).(*workspace.ProjectStack)

	isDefaultSecretsProvider := secretsProvider == "" || secretsProvider == "default"
	if isDefaultSecretsProvider {
		sm, err = b.DefaultSecretManager(ps)
	} else if secretsProvider == passphrase.Type {
		sm, err = passphrase.NewPromptingPassphraseSecretsManager(ps, false /*rotateSecretsProvider*/)
	} else {
		sm, err = cloud.NewCloudSecretsManager(ps, secretsProvider, false /*rotateSecretsProvider*/)
	}
	if err != nil {
		return nil, false, nil, err
	}

	needsSave := needsSaveProjectStackAfterSecretManger(oldConfig, ps)
	return ps, needsSave, sm, err
}

// A SecretsManagerLoader provides methods for loading secrets managers and
// their encrypters and decrypters for a given stack and project stack. A loader
// encapsulates the logic for determining which secrets manager to use based on
// a given configuration, such as whether or not to fallback to the stack state
// if there is no secrets manager configured in the project stack.
type SecretsManagerLoader struct {
	// True if the loader should fallback to the stack state if there is no
	// secrets manager configured in the project stack.
	FallbackToState bool
}

// The state of a stack's secret manager configuration following an operation.
type SecretsManagerState string

const (
	// The state of the stack's secret manager configuration is unchanged.
	SecretsManagerUnchanged SecretsManagerState = "unchanged"

	// The stack's secret manager configuration has changed and should be saved to
	// the stack configuration file if possible. If saving is not possible, the
	// configuration can be restored by falling back to the state file.
	SecretsManagerShouldSave SecretsManagerState = "should-save"

	// The stack's secret manager configuration has changed and must be saved to the
	// stack configuration file. Changes have been made that do not align with the
	// state and so the state file cannot be used to restore the configuration.
	SecretsManagerMustSave SecretsManagerState = "must-save"
)

// Creates a new stack secrets manager loader from the environment.
func NewStackSecretsManagerLoaderFromEnv() SecretsManagerLoader {
	return SecretsManagerLoader{
		FallbackToState: env.FallbackToStateSecretsManager.Value(),
	}
}

// Returns a decrypter for the given stack and project stack.
func (l *SecretsManagerLoader) GetDecrypter(
	ctx context.Context,
	s backend.Stack,
	ps *workspace.ProjectStack,
) (config.Decrypter, SecretsManagerState, error) {
	sm, state, err := l.GetSecretsManager(ctx, s, ps)
	if err != nil {
		return nil, SecretsManagerUnchanged, err
	}

	dec := sm.Decrypter()
	return dec, state, nil
}

// Returns an encrypter for the given stack and project stack.
func (l *SecretsManagerLoader) GetEncrypter(
	ctx context.Context,
	s backend.Stack,
	ps *workspace.ProjectStack,
) (config.Encrypter, SecretsManagerState, error) {
	sm, state, err := l.GetSecretsManager(ctx, s, ps)
	if err != nil {
		return nil, SecretsManagerUnchanged, err
	}

	enc := sm.Encrypter()
	return enc, state, nil
}

// Returns a secrets manager for the given stack and project stack.
func (l *SecretsManagerLoader) GetSecretsManager(
	ctx context.Context,
	s backend.Stack,
	ps *workspace.ProjectStack,
) (secrets.Manager, SecretsManagerState, error) {
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
				return nil, SecretsManagerUnchanged, err
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
		return nil, SecretsManagerUnchanged, err
	}

	// First, work out if the new configuration is different to the old one. If it is, and we fell back
	// to the state, we will return that the configuration *should* be saved. If the configurations
	// differ and we didn't fall back to the state (i.e. some brand new configuration was supplied to
	// us from an unknown source) then we will return that the configuration *must* be saved, lest it
	// be lost.
	needsSave := needsSaveProjectStackAfterSecretManger(oldConfig, ps)
	var state SecretsManagerState
	if needsSave {
		if fellBack {
			state = SecretsManagerShouldSave
		} else {
			state = SecretsManagerMustSave
		}
	} else {
		state = SecretsManagerUnchanged
	}

	return stack.NewBatchingSecretsManager(sm), state, nil
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

func ValidateSecretsProvider(typ string) error {
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

// we only want to log a secrets decryption for a Pulumi Cloud backend project
// we will allow any secrets provider to be used (Pulumi Cloud or passphrase/cloud/etc)
// we will log the message and not worry about the response. The types
// of messages we will log here will range from single secret decryption events
// to requesting a list of secrets in an individual event e.g. stack export
// the logging event will only happen during the `--show-secrets` path within the cli
func Log3rdPartySecretsProviderDecryptionEvent(ctx context.Context, backend backend.Stack,
	secretName, commandName string,
) {
	if stack, ok := backend.(httpstate.Stack); ok {
		// we only want to do something if this is a Pulumi Cloud backend
		if be, ok := stack.Backend().(httpstate.Backend); ok {
			client := be.Client()
			if client != nil {
				id := backend.(httpstate.Stack).StackIdentifier()
				// we don't really care if these logging calls fail as they should not stop the execution
				if secretName != "" {
					contract.Assertf(commandName == "", "Command name must be empty if secret name is set")
					err := client.Log3rdPartySecretsProviderDecryptionEvent(ctx, id, secretName)
					contract.IgnoreError(err)
				}

				if commandName != "" {
					contract.Assertf(secretName == "", "Secret name must be empty if command name is set")
					err := client.LogBulk3rdPartySecretsProviderDecryptionEvent(ctx, id, commandName)
					contract.IgnoreError(err)
				}
			}
		}
	}
}
