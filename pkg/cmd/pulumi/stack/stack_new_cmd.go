// Copyright 2016, Pulumi Corporation.
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
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	possibleSecretsProviderChoices = "The type of the provider that should be used to encrypt and decrypt secrets\n" +
		"(possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)"
)

func newStackNewCmd() *cobra.Command {
	var sicmd stackNewCmd
	cmd := &cobra.Command{
		Use:     "new",
		Aliases: []string{"init"},
		Short:   "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.\n" +
			"\n" +
			"To create a stack in an organization when logged in to the Pulumi Cloud,\n" +
			"prefix the stack name with the organization name and a slash (e.g. 'acmecorp/dev')\n" +
			"\n" +
			"By default, a stack created using the pulumi.com backend will use the pulumi.com secrets\n" +
			"provider and a stack created using the local or cloud object storage backend will use the\n" +
			"`passphrase` secrets provider.  A different secrets provider can be selected by passing the\n" +
			"`--secrets-provider` flag.\n" +
			"\n" +
			"To use the `passphrase` secrets provider with the pulumi.com backend, use:\n" +
			"\n" +
			"* `pulumi stack new --secrets-provider=passphrase`\n" +
			"\n" +
			"To use a cloud secrets provider with any backend, use one of the following:\n" +
			"\n" +
			"* `pulumi stack new --secrets-provider=\"awskms://alias/ExampleAlias?region=us-east-1\"`\n" +
			"* `pulumi stack new --secrets-provider=\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
			"* `pulumi stack new --secrets-provider=\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
			"* `pulumi stack new --secrets-provider=\"gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>\"`\n" +
			"* `pulumi stack new --secrets-provider=\"hashivault://mykey\"`\n" +
			"\n" +
			"To re-use existing encryption material (e.g. when restoring from backup or sharing across\n" +
			"environments), pass `--encrypted-key` (cloud-based providers) or `--encryption-salt`\n" +
			"(passphrase provider). Passphrase-protected stacks still require `PULUMI_CONFIG_PASSPHRASE`\n" +
			"or `PULUMI_CONFIG_PASSPHRASE_FILE` to be set; supplying the salt alone is not sufficient.\n" +
			"\n" +
			"A stack can be created based on the configuration of an existing stack by passing the\n" +
			"`--copy-config-from` flag:\n" +
			"\n" +
			"* `pulumi stack new --copy-config-from dev`",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return sicmd.Run(ctx, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "stack-name", Usage: "[org-name/]<stack-name>"},
		},
		Required: 0,
	})

	cmd.PersistentFlags().StringVarP(
		&sicmd.stackName, "stack", "s", "", "The name of the stack to create")
	cmd.PersistentFlags().StringVar(
		&sicmd.secretsProvider, "secrets-provider", "", possibleSecretsProviderChoices)
	cmd.PersistentFlags().StringVar(
		&sicmd.encryptedKey, "encrypted-key", "",
		"Pre-existing KMS-encrypted ciphertext for the data key (cloud-based secrets providers only)")
	cmd.PersistentFlags().StringVar(
		&sicmd.encryptionSalt, "encryption-salt", "",
		"Pre-existing encryption salt in `v1:<base64-salt>:<encrypted-verifier>` form "+
			"(passphrase-based secrets providers only)")
	cmd.PersistentFlags().StringVar(
		&sicmd.environment, "environment", "",
		"Reference to an ESC environment that will store this stack's configuration remotely. "+
			"Implies remote configuration storage.")
	cmd.PersistentFlags().StringVar(
		&sicmd.stackToCopy, "copy-config-from", "", "The name of the stack to copy existing config from")
	cmd.PersistentFlags().BoolVar(
		&sicmd.noSelect, "no-select", false, "Do not select the stack")
	cmd.PersistentFlags().StringArrayVar(&sicmd.teams, "teams", nil, "A list of team "+
		"names that should have permission to read and update this stack,"+
		" once created")
	cmd.PersistentFlags().BoolVar(
		&sicmd.remoteConfig, "remote-config", false, "Store stack configuration remotely",
	)
	_ = cmd.PersistentFlags().MarkHidden("remote-config")
	return cmd
}

// stackNewCmd implements the `pulumi stack new` command (with `pulumi stack init` retained as a back-alias).
type stackNewCmd struct {
	secretsProvider string
	stackName       string
	stackToCopy     string
	noSelect        bool
	teams           []string
	remoteConfig    bool
	encryptedKey    string
	encryptionSalt  string
	environment     string

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

func (cmd *stackNewCmd) Run(ctx context.Context, args []string) error {
	if cmd.secretsProvider == "" {
		cmd.secretsProvider = "default"
	}
	if cmd.currentBackend == nil {
		cmd.currentBackend = cmdBackend.CurrentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	ssml := NewStackSecretsManagerLoaderFromEnv()
	ws := pkgWorkspace.Instance

	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	b, err := currentBackend(ctx, ws, cmdBackend.DefaultLoginManager, project, opts)
	if err != nil {
		return err
	}

	if len(args) > 0 {
		if cmd.stackName != "" {
			return errors.New("only one of --stack or argument stack name may be specified, not both")
		}

		cmd.stackName = args[0]
	}

	// Validate secrets provider type
	if err := ValidateSecretsProvider(cmd.secretsProvider); err != nil {
		return err
	}

	// Validate the new encryption-material flags. These short-circuit secrets-manager construction so they're only
	// meaningful when paired with the matching provider; mixing them is a user error.
	if err := validateEncryptionFlags(cmd.secretsProvider, cmd.encryptedKey, cmd.encryptionSalt); err != nil {
		return err
	}

	if cmd.stackName == "" && cmdutil.Interactive() {
		if b.SupportsOrganizations() {
			fmt.Print("Please enter your desired stack name.\n" +
				"To create a stack in an organization, " +
				"use the format <org-name>/<stack-name> (e.g. `acmecorp/dev`).\n")
		}

		name, nameErr := ui.PromptForValue(false, "stack name", "dev", false, b.ValidateStackName, opts)
		if nameErr != nil {
			return nameErr
		}
		cmd.stackName = name
	}

	if cmd.stackName == "" {
		return errors.New("missing stack name")
	}

	if err := b.ValidateStackName(cmd.stackName); err != nil {
		return err
	}

	stackRef, err := b.ParseStackReference(cmd.stackName)
	if err != nil {
		return err
	}

	proj, root, projectErr := ws.ReadProject()
	if projectErr != nil && !errors.Is(projectErr, workspace.ErrProjectNotFound) {
		return projectErr
	}

	teams := sanitizeTeams(cmd.teams)
	enc := CreateStackEncryption{
		EncryptedKey:   cmd.encryptedKey,
		EncryptionSalt: cmd.encryptionSalt,
		Environment:    cmd.environment,
	}
	// --environment implies storing config remotely; --remote-config remains supported for the legacy default name.
	useRemoteConfig := cmd.remoteConfig || cmd.environment != ""
	newStack, err := CreateStack(ctx, cmdutil.Diag(), ws, b, stackRef, root, teams,
		!cmd.noSelect, cmd.secretsProvider, useRemoteConfig, enc)
	if err != nil {
		if errors.Is(err, backend.ErrTeamsNotSupported) {
			return fmt.Errorf("stack %s uses the %s backend: "+
				"%s does not support --teams", cmd.stackName, b.Name(), b.Name())
		}
		return err
	}

	if cmd.stackToCopy != "" {
		if projectErr != nil {
			return projectErr
		}

		// load the old stack and its project
		copyStack, err := RequireStack(
			ctx,
			cmdutil.Diag(),
			ws,
			cmdBackend.DefaultLoginManager,
			cmd.stackToCopy,
			LoadOnly,
			opts,
		)
		if err != nil {
			return err
		}
		copyProjectStack, err := LoadProjectStack(ctx, cmdutil.Diag(), proj, copyStack)
		if err != nil {
			return err
		}

		// get the project for the newly created stack
		newProjectStack, err := LoadProjectStack(ctx, cmdutil.Diag(), proj, newStack)
		if err != nil {
			return err
		}

		// copy the config from the old to the new
		requiresSaving, err := CopyEntireConfigMap(
			ctx,
			ssml,
			copyStack,
			copyProjectStack,
			newStack,
			newProjectStack,
		)
		if err != nil {
			return err
		}

		// The use of `requiresSaving` here ensures that there was actually some config
		// that needed saved, otherwise it's an unnecessary save call
		if requiresSaving {
			err := SaveProjectStack(ctx, newStack, newProjectStack)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// validateEncryptionFlags rejects nonsensical combinations of --secrets-provider / --encrypted-key / --encryption-salt.
//
// Note on the salt format: the issue and apitype.StackConfig.EncryptionSalt docstring describe this value as a
// "base64-encoded encryption salt", but the value stored locally in Pulumi.<stack>.yaml (and required by the passphrase
// secrets manager to verify a passphrase) is the full `v1:<base64-salt>:<encrypted-verifier>` state produced by
// passphrase.NewPassphraseSecretsManager. We accept and store that full state form here for symmetry with the local
// workspace format; reconciling the API docs is tracked separately.
func validateEncryptionFlags(secretsProvider, encryptedKey, encryptionSalt string) error {
	if encryptedKey == "" && encryptionSalt == "" {
		return nil
	}
	if encryptedKey != "" && encryptionSalt != "" {
		return errors.New("--encrypted-key and --encryption-salt are mutually exclusive")
	}
	isDefault := secretsProvider == "" || secretsProvider == "default"
	isPassphrase := secretsProvider == passphrase.Type
	if encryptionSalt != "" && !isPassphrase {
		return errors.New("--encryption-salt requires --secrets-provider=passphrase")
	}
	if encryptedKey != "" && (isPassphrase || isDefault) {
		return errors.New(
			"--encrypted-key requires a cloud secrets provider (awskms://, azurekeyvault://, gcpkms://, hashivault://)")
	}
	return nil
}

// sanitizeTeams strips empty / whitespace-only entries from the --teams list so the backend never sees them.
func sanitizeTeams(teams []string) []string {
	validTeams := teams[:0] // reuse storage.
	for _, team := range teams {
		team = strings.TrimSpace(team)
		if len(team) > 0 {
			validTeams = append(validTeams, team)
		}
	}

	return validTeams
}
