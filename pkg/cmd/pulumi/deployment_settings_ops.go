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
	"errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func verifyInteractiveMode(yes bool) error {
	interactive := cmdutil.Interactive()

	if !interactive && !yes {
		return errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

	return nil
}

type DeploymentSettingsPullArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
}

func newDeploymentSettingsPullCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Args:  cmdutil.ExactArgs(0),
		Short: "Pull the stack's deployment settings from Pulumi Cloud into the deployment.yaml file",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			config := UnmarshalArgs[DeploymentSettingsPullArgs](v, cmd)
			d, err := initializeDeploymentSettingsCmd(cmd.Context(), config.Stack)
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
		}),
	}

	parentCmd.AddCommand(cmd)
	BindFlags[DeploymentSettingsPullArgs](v, cmd)

	return cmd
}

type DeploymentSettingsUpdateArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	Yes   bool   `argsShort:"y" argsUsage:"Automatically confirm every confirmation prompt"`
}

func newDeploymentSettingsUpdateCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "up",
		Aliases:    []string{"update"},
		SuggestFor: []string{"apply", "deploy", "push"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Update stack deployment settings from deployment.yaml",
		Long:       "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			config := UnmarshalArgs[DeploymentSettingsUpdateArgs](v, cmd)

			if err := verifyInteractiveMode(config.Yes); err != nil {
				return err
			}

			d, err := initializeDeploymentSettingsCmd(cmd.Context(), config.Stack)
			if err != nil {
				return err
			}

			if d.Deployment == nil {
				return errors.New("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			confirm := askForConfirmation("This action will override the stack's deployment settings, "+
				"do you want to continue?", d.DisplayOptions.Color, false, config.Yes)

			if !confirm {
				return nil
			}

			err = d.Backend.UpdateStackDeploymentSettings(ctx, d.Stack, d.Deployment.DeploymentSettings)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	parentCmd.AddCommand(cmd)
	BindFlags[DeploymentSettingsUpdateArgs](v, cmd)

	return cmd
}

type DeploymentSettingsDestroyArgs struct {
	Stack string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	Yes   bool   `argsShort:"y" argsUsage:"Automatically confirm every confirmation prompt"`
}

func newDeploymentSettingsDestroyCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "destroy",
		Aliases:    []string{"down", "dn", "clear"},
		SuggestFor: []string{"delete", "kill", "remove", "rm", "stop"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Delete all the stack's deployment settings",
		Long:       "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			config := UnmarshalArgs[DeploymentSettingsDestroyArgs](v, cmd)

			if err := verifyInteractiveMode(config.Yes); err != nil {
				return err
			}

			d, err := initializeDeploymentSettingsCmd(cmd.Context(), config.Stack)
			if err != nil {
				return err
			}

			confirm := askForConfirmation("This action will clear the stack's deployment settings, "+
				"do you want to continue?", d.DisplayOptions.Color, false, config.Yes)
			if !confirm {
				return nil
			}

			err = d.Backend.DestroyStackDeploymentSettings(ctx, d.Stack)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	parentCmd.AddCommand(cmd)
	BindFlags[DeploymentSettingsDestroyArgs](v, cmd)

	return cmd
}

type DeploymentSettingsEnvArgs struct {
	Stack  string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	Secret bool   `argsUsage:"whether the value should be treated as a secret and be encrypted"`
	Remove bool   `argsUsage:"whether the key should be removed"`
}

func newDeploymentSettingsEnvCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env <key> [value]",
		Args:  cmdutil.RangeArgs(1, 2),
		Short: "Update stack's deployment settings secrets",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			config := UnmarshalArgs[DeploymentSettingsEnvArgs](v, cmd)

			d, err := initializeDeploymentSettingsCmd(cmd.Context(), config.Stack)
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
				if config.Remove {
					return errors.New("value not supported when removing keys")
				}
				value = args[1]
			} else {
				if !config.Remove {
					return errors.New("value cannot be empty")
				}
			}

			if d.Deployment.DeploymentSettings.Operation == nil {
				d.Deployment.DeploymentSettings.Operation = &apitype.OperationContext{}
			}

			if d.Deployment.DeploymentSettings.Operation.EnvironmentVariables == nil {
				d.Deployment.DeploymentSettings.Operation.EnvironmentVariables = make(map[string]apitype.SecretValue)
			}

			if config.Remove {
				delete(d.Deployment.DeploymentSettings.Operation.EnvironmentVariables, key)
			} else {
				var secretValue *apitype.SecretValue
				if config.Secret {
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
		}),
	}

	parentCmd.AddCommand(cmd)
	BindFlags[DeploymentSettingsEnvArgs](v, cmd)
	cmd.MarkFlagsMutuallyExclusive("secret", "remove")

	return cmd
}
