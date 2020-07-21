// Copyright 2016-2020, Pulumi Corporation.
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
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/resource/stack"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
)

func newStackChangeSecretsProviderCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "change-secrets-provider <new-secrets-provider>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Change the secrets provider for the current stack",
		Long: "Change the secrets provider for the current stack. " +
			"Valid secret providers types are `default`, `passphrase`, `awskms`, `azurekeyvault`, `gcpkms`, `hashivault`.\n\n" +
			"To change to using the Pulumi Default Secrets Provider, use the following:\n" +
			"\n" +
			"pulumi stack change-secrets-provider default" +
			"\n" +
			"\n" +
			"To change the stack to use a cloud secrets backend, use one of the following:\n" +
			"\n" +
			"* `pulumi stack change-secrets-provider \"awskms://alias/ExampleAlias?region=us-east-1\"" +
			"`\n" +
			"* `pulumi stack change-secrets-provider " +
			"\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
			"* `pulumi stack change-secrets-provider " +
			"\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
			"* `pulumi stack change-secrets-provider " +
			"\"gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>\"`\n" +
			"* `pulumi stack change-secrets-provider \"hashivault://mykey\"`",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// Validate secrets provider type
			if err := validateSecretsProvider(args[0]); err != nil {
				return err
			}

			// Get the current backend
			b, err := currentBackend(opts)
			if err != nil {
				return err
			}

			// Get the current stack and it's project
			currentStack, err := requireStack("", false, opts, true /*setCurrent*/)
			if err != nil {
				return err
			}
			currentProjectStack, err := loadProjectStack(currentStack)
			if err != nil {
				return err
			}

			// Build encrypter and decrypter based on the existing secrets provider
			var decrypter config.Decrypter
			currentConfig := currentProjectStack.Config

			if currentConfig.HasSecureValue() {
				dec, decerr := getStackDecrypter(currentStack)
				if decerr != nil {
					return decerr
				}
				decrypter = dec
			} else {
				decrypter = config.NewPanicCrypter()
			}

			// Create the new secrets provider and set to the currentStack
			if err := createSecretsManager(b, currentStack.Ref(), args[0]); err != nil {
				return err
			}

			// Change the config to use the new secrets provider
			err = migrateConfigToNewSecretsProvider(currentStack, currentConfig, decrypter)
			if err != nil {
				return err
			}

			// Fixup the checkpoint
			err = migrateCheckpointToNewSecretsProvider(commandContext(), currentStack)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}

func migrateCheckpointToNewSecretsProvider(ctx context.Context, currentStack backend.Stack) error {
	// Load the current checkpoint so those secrets can also be decrypted
	checkpoint, err := currentStack.ExportDeployment(ctx)
	if err != nil {
		return err
	}
	snap, err := stack.DeserializeUntypedDeployment(checkpoint, stack.DefaultSecretsProvider)
	if err != nil {
		return checkDeploymentVersionError(err, currentStack.Ref().Name().String())
	}

	// Get the newly created secrets manager for the stack
	newSecretsManager, err := getStackSecretsManager(currentStack)
	if err != nil {
		return err
	}

	// Reserialize the Snapshopshot with the NewSecrets Manager
	reserializedDeployment, err := stack.SerializeDeployment(snap, newSecretsManager, false)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(reserializedDeployment)
	if err != nil {
		return err
	}

	dep := apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	}

	// Import the newly changes Deployment
	if err = currentStack.ImportDeployment(ctx, &dep); err != nil {
		return err
	}

	return err
}

func migrateConfigToNewSecretsProvider(currentStack backend.Stack, currentConfig config.Map,
	decrypter config.Decrypter) error {
	// Get the new encrypter for the current stack
	newEncrypter, err := getStackEncrypter(currentStack)
	if err != nil {
		return err
	}

	// Create a copy of the current config map and re-encrypt using the new secrets provider
	newProjectConfig, err := currentConfig.Copy(decrypter, newEncrypter)
	if err != nil {
		return err
	}

	// Reload the project stack after the new secretsProvider is in place
	reloadedProjectStack, err := loadProjectStack(currentStack)
	if err != nil {
		return err
	}

	for key, val := range newProjectConfig {
		err = reloadedProjectStack.Config.Set(key, val, false)
		if err != nil {
			return err
		}
	}

	err = saveProjectStack(currentStack, reloadedProjectStack)
	if err != nil {
		return err
	}

	return nil
}
