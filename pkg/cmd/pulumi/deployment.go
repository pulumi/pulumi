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
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

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

	cmd.AddCommand(newDeploymentSettingsCmd())
	cmd.AddCommand(newDeploymentRunCmd())

	return cmd
}

func newDeploymentRunCmd() *cobra.Command {
	// Flags for remote operations.
	remoteArgs := RemoteArgs{}

	var stackName string
	var jsonDisplay bool
	var suppressPermalink string

	cmd := &cobra.Command{
		Use:   "run <operation> [url]",
		Short: "Launches deployment jobs on Pulumi Cloud",
		Long:  "",
		Args:  cmdutil.RangeArgs(1, 2),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := cmd.Context()

			operation, err := apitype.StrToPulumiOperation(args[0])
			if err != nil {
				return result.FromError(err)
			}

			var url string
			if len(args) > 1 {
				url = args[1]
			}

			display := display.Options{
				Color:       cmdutil.GetGlobalColorization(),
				JSONDisplay: jsonDisplay,
				// we only suppress permalinks if the user passes true. the default is an empty string
				// which we pass as 'false'
				SuppressPermalink: suppressPermalink == "true",
			}

			return runDeployment(ctx, display, operation, stackName, url, remoteArgs)
		}),
	}

	// Remote flags
	remoteArgs.applyFlagsForDeploymentCommand(cmd)

	cmd.PersistentFlags().StringVar(
		&suppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")

	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the update diffs, operations, and overall output as JSON")

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

	cmd.AddCommand(newDeploymentSettingsInitCmd())
	cmd.AddCommand(newDeploymentSettingsPullCmd())
	cmd.AddCommand(newDeploymentSettingsUpdateCmd())
	cmd.AddCommand(newDeploymentSettingsSetCmd())
	cmd.AddCommand(newDeploymentSettingsEnvCmd())
	cmd.AddCommand(newDeploymentSettingsDestroyCmd())

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

			deploymentSettings, err := currentBe.GetStackDeployment(ctx, s)
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

func newDeploymentSettingsSetCmd() *cobra.Command {
	var stack string

	var executorRepositoryPassword string
	var gitPassword string
	var gitSSHPrivateKey string
	var gitSSHPrivateKeyPassword string

	cmd := &cobra.Command{
		Use:   "set-secret",
		Args:  cmdutil.ExactArgs(0),
		Short: "Updates stack's deployment settings secrets",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := cmdutil.Interactive()

			if executorRepositoryPassword == "" &&
				gitPassword == "" &&
				gitSSHPrivateKey == "" &&
				gitSSHPrivateKeyPassword == "" {
				return errors.New("No scecrets provided")
			}

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

			if gitPassword != "" {
				if sd.DeploymentSettings.SourceContext.Git.GitAuth != nil &&
					sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth != nil {

					encrypted, err := currentBe.EncryptStackDeploymentSecret(ctx, s, gitPassword)
					if err != nil {
						return err
					}

					sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.Password = apitype.SecretValue{
						Secret: true,
						Value:  encrypted,
					}
				} else {
					return errors.New("git basic auth is not configured")
				}
			}

			if gitSSHPrivateKey != "" || gitSSHPrivateKeyPassword != "" {
				if sd.DeploymentSettings.SourceContext.Git.GitAuth != nil &&
					sd.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth != nil {

					if gitSSHPrivateKey != "" {
						encrypted, err := currentBe.EncryptStackDeploymentSecret(ctx, s, gitSSHPrivateKey)
						if err != nil {
							return err
						}

						sd.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.SSHPrivateKey = apitype.SecretValue{
							Secret: true,
							Value:  encrypted,
						}
					}

					if gitSSHPrivateKeyPassword != "" {
						encrypted, err := currentBe.EncryptStackDeploymentSecret(ctx, s, gitSSHPrivateKeyPassword)
						if err != nil {
							return err
						}

						sd.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password = &apitype.SecretValue{
							Secret: true,
							Value:  encrypted,
						}
					}
				} else {
					return errors.New("git private key auth is not configured")
				}
			}

			if executorRepositoryPassword != "" {
				if sd.DeploymentSettings.Executor != nil && sd.DeploymentSettings.Executor.ExecutorImage != nil {
					encrypted, err := currentBe.EncryptStackDeploymentSecret(ctx, s, executorRepositoryPassword)
					if err != nil {
						return err
					}

					sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Password = apitype.SecretValue{
						Secret: true,
						Value:  encrypted,
					}
				} else {
					return errors.New("custom executor is not configured")
				}
			}

			err = saveProjectStackDeployment(sd, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		&executorRepositoryPassword, "executor-repository-password", "",
		"Key to update")

	cmd.PersistentFlags().StringVar(
		&gitPassword, "git-password", "",
		"Value for the updated key")

	cmd.PersistentFlags().StringVar(
		&gitSSHPrivateKey, "git-ssh-private-key", "",
		"Value for the updated key")

	cmd.PersistentFlags().StringVar(
		&gitSSHPrivateKeyPassword, "git-ssh-private-key-password", "",
		"Value for the updated key")

	return cmd
}

func newDeploymentSettingsEnvCmd() *cobra.Command {
	var stack string

	var secret bool
	var remove bool
	var key string
	var value string

	cmd := &cobra.Command{
		Use:   "env",
		Args:  cmdutil.ExactArgs(0),
		Short: "Updates stack's deployment settings secrets",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := cmdutil.Interactive()

			if remove {
				if key == "" {
					return errors.New("key is required")
				}

				if value != "" {
					return errors.New("value not supported when removing keys")
				}

				if secret {
					return errors.New("secret not supported when removing keys")
				}
			} else {
				if key == "" || value == "" {
					return errors.New("key and value are required")
				}
			}

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

			if secret {
				value, err = currentBe.EncryptStackDeploymentSecret(ctx, s, value)
				if err != nil {
					return err
				}
			}

			if sd.DeploymentSettings.Operation == nil {
				sd.DeploymentSettings.Operation = &apitype.OperationContext{}
			}

			if sd.DeploymentSettings.Operation.EnvironmentVariables == nil {
				sd.DeploymentSettings.Operation.EnvironmentVariables = make(map[string]apitype.SecretValue)
			}

			if remove {
				delete(sd.DeploymentSettings.Operation.EnvironmentVariables, key)
			} else {
				sd.DeploymentSettings.Operation.EnvironmentVariables[key] = apitype.SecretValue{
					Value:  value,
					Secret: secret,
				}
			}

			err = saveProjectStackDeployment(sd, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVar(
		&key, "key", "",
		"Key to update")

	cmd.PersistentFlags().StringVar(
		&value, "value", "",
		"Key to update")

	cmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"Key to update")

	cmd.PersistentFlags().BoolVar(
		&remove, "remove", false,
		"Key to update")

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

			err = currentBe.UpdateStackDeployment(ctx, s, sd.DeploymentSettings)
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
		Short:      "Deletes all the stack deployment settings",
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

			err = currentBe.DestroyStackDeployment(ctx, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	return cmd
}

func newDeploymentSettingsInitCmd() *cobra.Command {
	var stack string

	cmd := &cobra.Command{
		Use:        "init",
		SuggestFor: []string{"new", "create"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Initializes the stack deployment settings file",
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
				return fmt.Errorf("detecting VCS info for stack tags for remote %v: %w", remoteURL, err)
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
