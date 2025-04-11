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

package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

type stackChangeSecretsProviderCmd struct {
	stdout io.Writer

	stack string

	secretsProvider secrets.Provider
}

func newStackChangeSecretsProviderCmd() *cobra.Command {
	var scspcmd stackChangeSecretsProviderCmd
	cmd := &cobra.Command{
		Use:   "change-secrets-provider <new-secrets-provider>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Change the secrets provider for a stack",
		Long: "Change the secrets provider for a stack. " +
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
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return scspcmd.Run(ctx, args)
		},
	}

	cmd.PersistentFlags().StringVarP(
		&scspcmd.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func (cmd *stackChangeSecretsProviderCmd) Run(ctx context.Context, args []string) error {
	stdout := cmd.stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if cmd.secretsProvider == nil {
		cmd.secretsProvider = stack.DefaultSecretsProvider
	}

	// For change-secrets-provider, we explicitly don't want any fallback behaviour when loading secrets providers.
	ssml := SecretsManagerLoader{}

	ws := pkgWorkspace.Instance

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	if err := ValidateSecretsProvider(args[0]); err != nil {
		return err
	}

	project, _, err := ws.ReadProject()
	if err != nil {
		return err
	}

	// Get the current stack and its project
	currentStack, err := RequireStack(
		ctx,
		ws,
		cmdBackend.DefaultLoginManager,
		cmd.stack,
		LoadOnly,
		opts,
	)
	if err != nil {
		return err
	}

	currentProjectStack, err := currentStack.Load(ctx, project)
	if err != nil {
		return err
	}

	// Build decrypter based on the existing secrets provider
	var decrypter config.Decrypter
	if currentProjectStack.Config.HasSecureValue() {
		dec, state, decerr := ssml.GetDecrypter(ctx, currentStack, currentProjectStack)
		if decerr != nil {
			return decerr
		}
		contract.Assertf(
			state == SecretsManagerUnchanged,
			"We're reading a secure value so the encryption information must be present already",
		)
		decrypter = dec
	} else {
		decrypter = config.NewPanicCrypter()
	}

	secretsProvider := args[0]
	// If we're setting the secrets provider to the same provider then do a rotation.
	rotateProvider := secretsProvider == currentProjectStack.SecretsProvider ||
		// passphrase doesn't get saved to stack state, so if we're changing to passphrase see if
		// the current secrets provider is empty
		((secretsProvider == "passphrase") && (currentProjectStack.SecretsProvider == ""))
	// Create the new secrets provider and set to the currentStack
	if err := CreateSecretsManagerForExistingStack(ctx, ws, currentStack, secretsProvider, rotateProvider,
		false /*creatingStack*/); err != nil {
		return err
	}

	// Fixup the checkpoint
	fmt.Fprintf(stdout, "Migrating old configuration and state to new secrets provider\n")
	return migrateOldConfigAndCheckpointToNewSecretsProvider(
		ctx,
		ssml,
		cmd.secretsProvider,
		project,
		currentStack,
		currentProjectStack,
		decrypter,
	)
}

func migrateOldConfigAndCheckpointToNewSecretsProvider(
	ctx context.Context,
	ssml SecretsManagerLoader,
	secretsProvider secrets.Provider,
	project *workspace.Project,
	currentStack backend.Stack,
	currentConfig *workspace.ProjectStack, decrypter config.Decrypter,
) error {
	// Reload the project stack after the new secrets provider is in place
	reloadedProjectStack, err := currentStack.Load(ctx, project)
	if err != nil {
		return err
	}

	// Get the newly created secrets manager for the stack
	newSecretsManager, state, err := ssml.GetSecretsManager(ctx, currentStack, reloadedProjectStack)
	if err != nil {
		return err
	}
	contract.Assertf(
		state == SecretsManagerUnchanged,
		"We've just saved and reloaded the stack, so the encryption information must be present already",
	)

	// get the encrypter for the new secrets manager
	newEncrypter := newSecretsManager.Encrypter()

	// Create a copy of the current config map and re-encrypt using the new secrets provider
	newProjectConfig, err := currentConfig.Config.Copy(decrypter, newEncrypter)
	if err != nil {
		return err
	}

	for key, val := range newProjectConfig {
		if err := reloadedProjectStack.Config.Set(key, val, false); err != nil {
			return err
		}
	}

	if err := currentStack.Save(ctx, reloadedProjectStack); err != nil {
		return err
	}

	// Load the current checkpoint so those secrets can also be decrypted
	checkpoint, err := currentStack.ExportDeployment(ctx)
	if err != nil {
		return err
	}
	snap, err := stack.DeserializeUntypedDeployment(ctx, checkpoint, secretsProvider)
	if err != nil {
		return checkDeploymentVersionError(err, currentStack.Ref().Name().String())
	}

	// Reserialize the Snapshopshot with the NewSecrets Manager
	snap.SecretsManager = newSecretsManager
	reserializedDeployment, err := stack.SerializeDeployment(ctx, snap, false /*showSecrets*/)
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
	return currentStack.ImportDeployment(ctx, &dep)
}
