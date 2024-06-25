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
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
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
		// This is temporarily hidden while we iterate over the new set of commands,
		// we will remove before releasing these new set of features.
		Hidden: true,
		Use:    "deployment",
		Short:  "Manage stack deployments on Pulumi Cloud",
		Long: "Manage stack deployments on Pulumi Cloud.\n" +
			"\n" +
			"Use this command to trigger deployment jobs and manage deployment settings.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		&stackDeploymentConfigFile, "config-file", "",
		"Override the file name where the deployment settings are specified. Default is Pulumi.[stack].deploy.yaml")

	cmd.AddCommand(newDeploymentSettingsCmd())

	return cmd
}

func newDeploymentSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Args:  cmdutil.ExactArgs(1),
		Short: "Manage stack deployment settings",
		Long: "Manage stack deployment settings\n" +
			"\n" +
			"Use this command to manage a stack's deployment settings like\n" +
			"generating the deployment file, updating secrets or pushing the\n" +
			"updated settings to Pulumi Cloud.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}

	cmd.AddCommand(newDeploymentSettingsInitCmd())
	cmd.AddCommand(newDeploymentSettingsPullCmd())
	cmd.AddCommand(newDeploymentSettingsUpdateCmd())
	cmd.AddCommand(newDeploymentSettingsDestroyCmd())

	return cmd
}

func newDeploymentSettingsInitCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Use:        "init",
		SuggestFor: []string{"new", "create"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Initialize the stack's deployment.yaml file",
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

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			repo, err := git.PlainOpen(wd)
			if err != nil {
				return err
			}

			newStackDeployment := &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{
					SourceContext: &apitype.SourceContext{
						Git: &apitype.SourceContextGit{
							// Setting this by default to be included in the deployment file
							// so it is easier to manually change by users. This will be completed
							// by the user when this gets converted to a wizard.
							RepoDir: ".",
						},
					},
					Operation: &apitype.OperationContext{
						Options: &apitype.OperationContextOptions{},
					},
				},
			}

			remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")
			if err != nil {
				return err
			}

			h, err := repo.Head()
			if err != nil {
				return err
			}

			if vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL); err == nil {
				// If it is a GitHub repo, we will configure it to be used with the App. Otherwise, we will
				// configure it as a barebone git repository (we wont be configuring credentials at this point
				// users can use the `set` command to configure those afterwards).
				if vcsInfo.Kind == gitutil.GitHubHostName {
					newStackDeployment.DeploymentSettings.GitHub = &apitype.DeploymentSettingsGitHub{
						Repository:          fmt.Sprintf("%s/%s", vcsInfo.Owner, vcsInfo.Repo),
						PreviewPullRequests: true,
						DeployCommits:       true,
					}
				} else {
					newStackDeployment.DeploymentSettings.SourceContext.Git.RepoURL = remoteURL
				}
			} else {
				return fmt.Errorf("detecting VCS info from stack tags for remote %v: %w", remoteURL, err)
			}

			if h.Name().IsBranch() {
				newStackDeployment.DeploymentSettings.SourceContext.Git.Branch = h.Name().String()
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

func newDeploymentSettingsPullCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Use:   "pull",
		Args:  cmdutil.ExactArgs(0),
		Short: "Pull the stack's deployment settings from Pulumi Cloud into the deployment.yaml file",
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
		Short:      "Update stack deployment settings from deployment.yaml",
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

func newDeploymentSettingsDestroyCmd() *cobra.Command {
	var stack string
	cmd := &cobra.Command{
		Use:        "destroy",
		Aliases:    []string{"down", "dn"},
		SuggestFor: []string{"delete", "kill", "remove", "rm", "stop"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Delete all the stack's deployment settings",
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

			err = currentBe.DestroyStackDeploymentSettings(ctx, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}
