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
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

var stackDeploymentConfigFile string

func loadProjectStackDeployment(stack backend.Stack) (*workspace.ProjectStackDeployment, error) {
	if stackDeploymentConfigFile == "" {
		return workspace.DetectProjectStackDeployment(stack.Ref().Name().Q())
	}
	return workspace.LoadProjectStackDeployment(stackDeploymentConfigFile)
}

func saveProjectStackDeployment(psd *workspace.ProjectStackDeployment, stack backend.Stack) error {
	if stackDeploymentConfigFile == "" {
		return workspace.SaveProjectStackDeployment(stack.Ref().Name().Q(), psd)
	}
	return psd.Save(stackDeploymentConfigFile)
}

func newDeploymentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deployment",
		Short: "Manage stack deployments",
		Long: "Manage stack deployments.\n" +
			"\n" +
			"Use this command to manage stack deployments, " +
			"e.g. configuring deployment settings",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		&stackConfigFile, "config-file", "",
		"Use the configuration values in the specified file rather than detecting the file name")

	cmd.AddCommand(newDeploymentSettingsCmd())

	return cmd
}

func newDeploymentSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Args:  cmdutil.ExactArgs(1),
		Short: "Manages the stack's deployment settings",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}

	cmd.AddCommand(newDeploymentSettingsPullCmd())
	cmd.AddCommand(newDeploymentSettingsUpdateCmd())

	return cmd
}

func newDeploymentSettingsPullCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Use:   "pull",
		Args:  cmdutil.ExactArgs(0),
		Short: "Pulls the stack's deployment settings and updates the deployment yaml file",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := cmdutil.Interactive()

			displayOpts := display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
			}

			project, _, err := readProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBe, err := currentBackend(ctx, project, displayOpts)
			if err != nil {
				return err
			}

			if !currentBe.SupportsDeployments() {
				return fmt.Errorf("backends of this type %q do not support deployments",
					currentBe.Name())
			}

			s, err := requireStack(ctx, stack, stackOfferNew|stackSetCurrent, displayOpts)
			if err != nil {
				return err
			}

			deploymentSettings, err := currentBe.GetStackDeploymentSettings(ctx, s)
			if err != nil {
				return err
			}

			newStackDeployment := &workspace.ProjectStackDeployment{
				DeploymentSettings: *deploymentSettings,
			}

			err = saveProjectStackDeployment(newStackDeployment, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}

func newDeploymentSettingsUpdateCmd() *cobra.Command {
	var stack string
	cmd := &cobra.Command{
		Use:        "up",
		Aliases:    []string{"update"},
		SuggestFor: []string{"apply", "deploy", "push"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Updates the stack's deployment settings with the data in the deployment yaml file",
		Long:       "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := cmdutil.Interactive()

			displayOpts := display.Options{
				Color:         cmdutil.GetGlobalColorization(),
				IsInteractive: interactive,
			}

			project, _, err := readProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			currentBe, err := currentBackend(ctx, project, displayOpts)
			if err != nil {
				return err
			}

			if !currentBe.SupportsDeployments() {
				return fmt.Errorf("backends of this type %q do not support deployments",
					currentBe.Name())
			}

			s, err := requireStack(ctx, stack, stackOfferNew|stackSetCurrent, displayOpts)
			if err != nil {
				return err
			}

			sd, err := loadProjectStackDeployment(s)
			if err != nil {
				return err
			}

			err = currentBe.UpdateStackDeploymentSettings(ctx, s, sd.DeploymentSettings)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}
