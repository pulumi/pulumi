// Copyright 2016-2018, Pulumi Corporation.
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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

const (
	possibleSecretsProviderChoices = "The type of the provider that should be used to encrypt and decrypt secrets\n" +
		"(possible choices: default, passphrase, awskms, azurekeyvault, gcpkms, hashivault)"
)

func newStackInitCmd() *cobra.Command {
	var secretsProvider string
	var stackName string
	var stackToCopy string

	cmd := &cobra.Command{
		Use:   placeholder.Use,
		Args:  cmdutil.MaximumNArgs(1),
		Short: placeholder.Short,
		Long:  placeholder.Long,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			b, err := currentBackend(opts)
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

			var createOpts interface{} // Backend-specific config options, none currently.
			newStack, err := createStack(b, stackRef, createOpts, true /*setCurrent*/, secretsProvider)
			if err != nil {
				return err
			}

			if stackToCopy != "" {
				// load the old stack and its project
				copyStack, err := requireStack(stackToCopy, false, opts, false /*setCurrent*/)
				if err != nil {
					return err
				}
				copyProjectStack, err := loadProjectStack(copyStack)
				if err != nil {
					return err
				}

				// get the project for the newly created stack
				newProjectStack, err := loadProjectStack(newStack)
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
	return cmd
}
