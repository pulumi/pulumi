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
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	possibleSecretsProviderChoices = "The type of the provider that should be used to encrypt and decrypt secrets\n" +
		"(possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)"
)

func newStackInitCmd() *cobra.Command {
	var secretsProvider string
	var stackName string
	var stackToCopy string
	var noSelect bool
	// teams is the list of teams who should have access to this stack, once created.
	var teams []string

	cmd := &cobra.Command{
		Use:   "init [<org-name>/]<stack-name>",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Create an empty stack with the given name, ready for updates",
		Long: "Create an empty stack with the given name, ready for updates\n" +
			"\n" +
			"This command creates an empty stack with the given name.  It has no resources,\n" +
			"but afterwards it can become the target of a deployment using the `update` command.\n" +
			"\n" +
			"To create a stack in an organization when logged in to the Pulumi service,\n" +
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := commandContext()
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
				if stackName != "" {
					return errors.New("only one of --stack or argument stack name may be specified, not both")
				}

				stackName = args[0]
			}

			// Validate secrets provider type
			if err := validateSecretsProvider(secretsProvider); err != nil {
				return err
			}

			if stackName == "" && cmdutil.Interactive() {
				if b.SupportsOrganizations() {
					fmt.Print("Please enter your desired stack name.\n" +
						"To create a stack in an organization, " +
						"use the format <org-name>/<stack-name> (e.g. `acmecorp/dev`).\n")
				}

				name, nameErr := promptForValue(false, "stack name", "dev", false, b.ValidateStackName, opts)
				if nameErr != nil {
					return nameErr
				}
				stackName = name
			}

			if stackName == "" {
				return errors.New("missing stack name")
			}

			if err := b.ValidateStackName(stackName); err != nil {
				return err
			}

			stackRef, err := b.ParseStackReference(stackName)
			if err != nil {
				return err
			}

			proj, root, projectErr := readProject()
			if projectErr != nil && !errors.Is(projectErr, workspace.ErrProjectNotFound) {
				return projectErr
			}

			// Backend-specific config options. Currently only applicable to the HTTP backend.
			createOpts, err := validateCreateStackOpts(stackName, b, teams)
			if err != nil {
				return err
			}

			newStack, err := createStack(ctx, b, stackRef, root, createOpts, !noSelect, secretsProvider)
			if err != nil {
				return err
			}

			if stackToCopy != "" {
				if projectErr != nil {
					return projectErr
				}

				// load the old stack and its project
				copyStack, err := requireStack(ctx, stackToCopy, stackLoadOnly, opts)
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
				return copyEntireConfigMap(copyStack, copyProjectStack, newStack, newProjectStack)
			}

			return nil
		}),
	}
	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "", "The name of the stack to create")
	cmd.PersistentFlags().StringVar(
		&secretsProvider, "secrets-provider", "default", possibleSecretsProviderChoices)
	cmd.PersistentFlags().StringVar(
		&stackToCopy, "copy-config-from", "", "The name of the stack to copy existing config from")
	cmd.PersistentFlags().BoolVar(
		&noSelect, "no-select", false, "Do not select the stack")
	cmd.PersistentFlags().StringArrayVar(&teams, "teams", nil, "A list of team "+
		"names that should have permission to read and update this stack,"+
		" once created")
	return cmd
}

// This function constructs a createStackOptions object if
// valid, otherwise returning nil. Most backends expect nil
// options, and error if options are non-nil.
func validateCreateStackOpts(stackName string, b backend.Backend, teams []string) (backend.CreateStackOptions, error) {
	// • If the user provided teams but the backend doesn't support them,
	//   return an error.
	if len(teams) > 0 && !b.SupportsTeams() {
		return nil, newTeamsUnsupportedError(stackName, b.Name())
	}
	// • Otherwise, validate the teams and pass them along.
	//   Remove any strings from the list that are empty or just whitespace.
	validatedTeams := teams[:0] // reuse storage.
	for _, team := range teams {
		teamStr := strings.TrimSpace(team)
		if len(teamStr) > 0 {
			validatedTeams = append(validatedTeams, teamStr)
		}
	}

	// • We can return stack options that contain the provided teams.
	//   since this will be zerod for non-Service backends.
	return backend.NewStandardCreateStackOpts(validatedTeams), nil
}

// TeamsUnsupportedError is the error returned when the --teams
// flag is provided on a backend that doesn't support teams.
type teamsUnsupportedError struct {
	stackName   string
	backendType string
}

// NewTeamsUnsupportedError constructs an error for when users provide the --teams flag
// for non-Service backends, or when options with teams are incorrectly provided to
// a backend during stack creation.
func newTeamsUnsupportedError(stackName, backendType string) *teamsUnsupportedError {
	return &teamsUnsupportedError{
		stackName:   stackName,
		backendType: backendType,
	}
}

func (err teamsUnsupportedError) Error() string {
	return fmt.Sprintf("The stack %s uses the %s backend, which does not supports teams", err.stackName, err.backendType)
}
