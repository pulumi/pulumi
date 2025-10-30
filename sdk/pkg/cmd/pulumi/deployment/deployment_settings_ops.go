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

package deployment

import (
	"errors"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func verifyInteractiveMode(yes bool) error {
	interactive := cmdutil.Interactive()

	if !interactive && !yes {
		return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

	return nil
}

func newDeploymentSettingsPullCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Use:   "pull",
		Args:  cmdutil.ExactArgs(0),
		Short: "Pull the stack's deployment settings from Pulumi Cloud into the deployment.yaml file",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := initializeDeploymentSettingsCmd(cmd.Context(), pkgWorkspace.Instance, stack)
			if err != nil {
				return err
			}

			ds, err := d.Backend.GetStackDeploymentSettings(d.Ctx, d.Stack)
			if err != nil {
				return err
			}

			newStackDeployment := &workspace.ProjectStackDeployment{
				DeploymentSettings: *ds,
			}

			err = saveProjectStackDeployment(newStackDeployment, d.Stack)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func newDeploymentSettingsUpdateCmd() *cobra.Command {
	var stack string
	var yes bool

	cmd := &cobra.Command{
		Use:        "push",
		Aliases:    []string{"update", "up"},
		SuggestFor: []string{"apply", "deploy", "push"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Update stack deployment settings from deployment.yaml",
		Long:       "",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := verifyInteractiveMode(yes); err != nil {
				return err
			}

			d, err := initializeDeploymentSettingsCmd(cmd.Context(), pkgWorkspace.Instance, stack)
			if err != nil {
				return err
			}

			if d.Deployment == nil {
				return errors.New("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			confirm := askForConfirmation("This action will override the stack's deployment settings, "+
				"do you want to continue?", d.DisplayOptions.Color, true, yes)

			if !confirm {
				return nil
			}

			err = d.Backend.UpdateStackDeploymentSettings(ctx, d.Stack, d.Deployment.DeploymentSettings)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically confirm every confirmation prompt")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func newDeploymentSettingsDestroyCmd() *cobra.Command {
	var stack string
	var yes bool

	cmd := &cobra.Command{
		Use:        "destroy",
		Aliases:    []string{"down", "dn", "clear"},
		SuggestFor: []string{"delete", "kill", "remove", "rm", "stop"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Delete all the stack's deployment settings",
		Long:       "",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := verifyInteractiveMode(yes); err != nil {
				return err
			}

			d, err := initializeDeploymentSettingsCmd(cmd.Context(), pkgWorkspace.Instance, stack)
			if err != nil {
				return err
			}

			confirm := askForConfirmation("This action will clear the stack's deployment settings, "+
				"do you want to continue?", d.DisplayOptions.Color, true, yes)

			if !confirm {
				return nil
			}

			err = d.Backend.DestroyStackDeploymentSettings(ctx, d.Stack)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically confirm every confirmation prompt")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func newDeploymentSettingsEnvCmd() *cobra.Command {
	var stack string
	var secret bool
	var remove bool

	cmd := &cobra.Command{
		Use:   "env <key> [value]",
		Args:  cmdutil.RangeArgs(1, 2),
		Short: "Update stack's deployment settings secrets",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			d, err := initializeDeploymentSettingsCmd(cmd.Context(), pkgWorkspace.Instance, stack)
			if err != nil {
				return err
			}

			if d.Deployment == nil {
				return errors.New("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			var (
				key   string
				value string
			)

			key = args[0]

			if len(args) == 2 {
				if remove {
					return errors.New("value not supported when removing keys")
				}
				value = args[1]
			} else {
				if !remove {
					return errors.New("value cannot be empty")
				}
			}

			if d.Deployment.DeploymentSettings.Operation == nil {
				d.Deployment.DeploymentSettings.Operation = &apitype.OperationContext{}
			}

			if d.Deployment.DeploymentSettings.Operation.EnvironmentVariables == nil {
				d.Deployment.DeploymentSettings.Operation.EnvironmentVariables = make(map[string]apitype.SecretValue)
			}

			if remove {
				delete(d.Deployment.DeploymentSettings.Operation.EnvironmentVariables, key)
			} else {
				var secretValue *apitype.SecretValue
				if secret {
					secretValue, err = d.Backend.EncryptStackDeploymentSettingsSecret(ctx, d.Stack, value)
					if err != nil {
						return err
					}
				} else {
					secretValue = &apitype.SecretValue{
						Value:  value,
						Secret: false,
					}
				}

				d.Deployment.DeploymentSettings.Operation.EnvironmentVariables[key] = *secretValue
			}

			err = saveProjectStackDeployment(d.Deployment, d.Stack)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"whether the value should be treated as a secret and be encrypted")

	cmd.PersistentFlags().BoolVar(
		&remove, "remove", false,
		"whether the key should be removed")

	cmd.MarkFlagsMutuallyExclusive("secret", "remove")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}
