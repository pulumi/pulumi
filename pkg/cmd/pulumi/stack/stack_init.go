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
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	possibleSecretsProviderChoices = "The type of the provider that should be used to encrypt and decrypt secrets\n" +
		"(possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)"
)

func newStackInitCmd() *cobra.Command {
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
			"* `pulumi stack init --secrets-provider=\"hashivault://mykey\"`\n" +
			"\n" +
			"A stack can be created based on the configuration of an existing stack by passing the\n" +
			"`--copy-config-from` flag:\n" +
			"\n" +
			"* `pulumi stack init --copy-config-from dev`",
		RunE: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return sicmd.Run(ctx, args)
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&sicmd.stackName, "stack", "s", "", "The name of the stack to create")
	cmd.PersistentFlags().StringVar(
		&sicmd.secretsProvider, "secrets-provider", "", possibleSecretsProviderChoices)
	cmd.PersistentFlags().StringVar(
		&sicmd.stackToCopy, "copy-config-from", "", "The name of the stack to copy existing config from")
	cmd.PersistentFlags().BoolVar(
		&sicmd.noSelect, "no-select", false, "Do not select the stack")
	cmd.PersistentFlags().StringArrayVar(&sicmd.teams, "teams", nil, "A list of team "+
		"names that should have permission to read and update this stack,"+
		" once created")
	return cmd
}

// stackInitCmd implements the `pulumi stack init` command.
type stackInitCmd struct {
	secretsProvider string
	stackName       string
	stackToCopy     string
	noSelect        bool
	teams           []string

	// currentBackend is a reference to the top-level currentBackend function.
	// This is used to override the default implementation for testing purposes.
	currentBackend func(
		context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, *workspace.Project, display.Options,
	) (backend.Backend, error)
}

func (cmd *stackInitCmd) Run(ctx context.Context, args []string) error {
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

	createOpts := newCreateStackOptions(cmd.teams)
	newStack, err := CreateStack(ctx, ws, b, stackRef, root, createOpts, !cmd.noSelect, cmd.secretsProvider)
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
			ws,
			cmdBackend.DefaultLoginManager,
			cmd.stackToCopy,
			LoadOnly,
			opts,
		)
		if err != nil {
			return err
		}
		copyProjectStack, err := LoadProjectStack(proj, copyStack)
		if err != nil {
			return err
		}

		// get the project for the newly created stack
		newProjectStack, err := LoadProjectStack(proj, newStack)
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
			err := SaveProjectStack(newStack, newProjectStack)
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
