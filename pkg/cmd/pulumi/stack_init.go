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

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

//nolint:lll
type StackInitArgs struct {
	SecretsProvider          string   `args:"secrets-provider" argsUsage:"The type of the provider that should be used to encrypt and decrypt secrets\n(possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)"`
	Stack                    string   `argsShort:"s" argsUsage:"The name of the stack to create"`
	ConfigurationSourceStack string   `args:"copy-config-from" argsUsage:"The name of the stack to copy existing config from"`
	NoSelect                 bool     `args:"no-select" argsUsage:"Do not select the stack"`
	Teams                    []string `argsCommaSplit:"false" argsUsage:"A list of team names that should have permission to read and update this stack, once created"`
}

// stackInitCmd implements the `pulumi stack init` command.
type stackInitCmd struct {
	Args StackInitArgs

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(context.Context, *workspace.Project, display.Options) (backend.Backend, error)
}

func newStackInitCmd(
	v *viper.Viper,
	parentStackCmd *cobra.Command,
) *cobra.Command {
	var sicmd stackInitCmd
	cmd := &cobra.Command{
		Use:   "init [<org-name>/]<stack-name>",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Create an empty stack with the given name, ready for updates",
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
			"* `pulumi stack init --secrets-provider=passphrase`\n" +
			"\n" +
			"To use a cloud secrets provider with any backend, use one of the following:\n" +
			"\n" +
			"* `pulumi stack init --secrets-provider=\"awskms://alias/ExampleAlias?region=us-east-1\"`\n" +
			"* `pulumi stack init --secrets-provider=\"awskms://1234abcd-12ab-34cd-56ef-1234567890ab?region=us-east-1\"`\n" +
			"* `pulumi stack init --secrets-provider=\"azurekeyvault://mykeyvaultname.vault.azure.net/keys/mykeyname\"`\n" +
			"* `pulumi stack init --secrets-provider=\"gcpkms://projects/<p>/locations/<l>/keyRings/<r>/cryptoKeys/<k>\"`\n" +
			"* `pulumi stack init --secrets-provider=\"hashivault://mykey\"\n`" +
			"\n" +
			"A stack can be created based on the configuration of an existing stack by passing the\n" +
			"`--copy-config-from` flag.\n" +
			"* `pulumi stack init --copy-config-from dev`",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			sicmd.Args = UnmarshalArgs[StackInitArgs](v, cmd)

			ctx := cmd.Context()
			return sicmd.Run(ctx, cmdArgs)
		}),
	}

	parentStackCmd.AddCommand(cmd)
	BindFlags[StackInitArgs](v, cmd)

	return cmd
}

func (cmd *stackInitCmd) Run(ctx context.Context, args []string) error {
	if cmd.Args.SecretsProvider == "" {
		cmd.Args.SecretsProvider = "default"
	}
	if cmd.currentBackend == nil {
		cmd.currentBackend = currentBackend
	}
	currentBackend := cmd.currentBackend // shadow the top-level function

	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	// Try to read the current project
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return err
	}

	b, err := currentBackend(ctx, project, opts)
	if err != nil {
		return err
	}

	if len(args) > 0 {
		if cmd.Args.Stack != "" {
			return errors.New("only one of --stack or argument stack name may be specified, not both")
		}

		cmd.Args.Stack = args[0]
	}

	// Validate secrets provider type
	if err := validateSecretsProvider(cmd.Args.SecretsProvider); err != nil {
		return err
	}

	if cmd.Args.Stack == "" && cmdutil.Interactive() {
		if b.SupportsOrganizations() {
			fmt.Print("Please enter your desired stack name.\n" +
				"To create a stack in an organization, " +
				"use the format <org-name>/<stack-name> (e.g. `acmecorp/dev`).\n")
		}

		name, nameErr := promptForValue(false, "stack name", "dev", false, b.ValidateStackName, opts)
		if nameErr != nil {
			return nameErr
		}
		cmd.Args.Stack = name
	}

	if cmd.Args.Stack == "" {
		return errors.New("missing stack name")
	}

	if err := b.ValidateStackName(cmd.Args.Stack); err != nil {
		return err
	}

	stackRef, err := b.ParseStackReference(cmd.Args.Stack)
	if err != nil {
		return err
	}

	proj, root, projectErr := readProject()
	if projectErr != nil && !errors.Is(projectErr, workspace.ErrProjectNotFound) {
		return projectErr
	}

	createOpts := newCreateStackOptions(cmd.Args.Teams)
	newStack, err := createStack(
		ctx,
		b,
		stackRef,
		root,
		createOpts,
		!cmd.Args.NoSelect,
		cmd.Args.SecretsProvider,
	)
	if err != nil {
		if errors.Is(err, backend.ErrTeamsNotSupported) {
			return fmt.Errorf("stack %s uses the %s backend: "+
				"%s does not support --teams", cmd.Args.Stack, b.Name(), b.Name())
		}
		return err
	}

	if cmd.Args.ConfigurationSourceStack != "" {
		if projectErr != nil {
			return projectErr
		}

		// load the old stack and its project
		copyStack, err := requireStack(ctx, cmd.Args.ConfigurationSourceStack, stackLoadOnly, opts)
		if err != nil {
			return err
		}
		copyProjectStack, err := loadProjectStack(proj, copyStack)
		if err != nil {
			return err
		}

		// get the project for the newly created stack
		newProjectStack, err := loadProjectStack(proj, newStack)
		if err != nil {
			return err
		}

		// copy the config from the old to the new
		requiresSaving, err := copyEntireConfigMap(copyStack, copyProjectStack, newStack, newProjectStack)
		if err != nil {
			return err
		}

		// The use of `requiresSaving` here ensures that there was actually some config
		// that needed saved, otherwise it's an unnecessary save call
		if requiresSaving {
			err := saveProjectStack(newStack, newProjectStack)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// newCreateStackOptions constructs a backend.CreateStackOptions object
// from the provided options.
func newCreateStackOptions(teams []string) *backend.CreateStackOptions {
	// Remove any strings from the list that are empty or just whitespace.
	validTeams := teams[:0] // reuse storage.
	for _, team := range teams {
		team = strings.TrimSpace(team)
		if len(team) > 0 {
			validTeams = append(validTeams, team)
		}
	}

	return &backend.CreateStackOptions{
		Teams: validTeams,
	}
}
