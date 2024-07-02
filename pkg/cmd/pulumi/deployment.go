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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	survey "github.com/AlecAivazis/survey/v2"
	git "github.com/go-git/go-git/v5"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
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
	cmd.AddCommand(newDeploymentRunCmd())

	return cmd
}

func newDeploymentRunCmd() *cobra.Command {
	// Flags for remote operations.
	remoteArgs := RemoteArgs{}

	var stack string
	var suppressPermalink bool

	cmd := &cobra.Command{
		Use:   "run <operation> [url]",
		Short: "Launch a deployment job on Pulumi Cloud",
		Long:  "",
		Args:  cmdutil.RangeArgs(1, 2),
		Run: cmdutil.RunResultFunc(func(cmd *cobra.Command, args []string) result.Result {
			ctx := cmd.Context()

			operation, err := apitype.ParsePulumiOperation(args[0])
			if err != nil {
				return result.FromError(err)
			}

			var url string
			if len(args) > 1 {
				url = args[1]
			}

			display := display.Options{
				Color: cmdutil.GetGlobalColorization(),
				// we only suppress permalinks if the user passes true. the default is an empty string
				// which we pass as 'false'
				SuppressPermalink: suppressPermalink,
			}

			project, _, err := readProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return result.FromError(err)
			}

			currentBe, err := currentBackend(ctx, project, display)
			if err != nil {
				return result.FromError(err)
			}

			if !currentBe.SupportsDeployments() {
				return result.FromError(fmt.Errorf("backends of this type %q do not support deployments",
					currentBe.Name()))
			}

			s, err := requireStack(ctx, stack, stackOfferNew|stackSetCurrent, display)
			if err != nil {
				return result.FromError(err)
			}

			if errResult := validateDeploymentFlags(url, remoteArgs); errResult != nil {
				return errResult
			}

			return runDeployment(ctx, cmd, display, operation, s.Ref().FullyQualifiedName().String(), url, remoteArgs)
		}),
	}

	// Remote flags
	remoteArgs.applyFlagsForDeploymentCommand(cmd)

	cmd.PersistentFlags().BoolVar(
		&suppressPermalink, "suppress-permalink", false,
		"Suppress display of the state permalink")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

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
	cmd.AddCommand(newDeploymentSettingsEnvCmd())
	cmd.AddCommand(newDeploymentSettingsSetCmd())

	return cmd
}

func newDeploymentSettingsInitCmd() *cobra.Command {
	var stack string
	var gitSSHPrivateKeyPath string

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

			newStackDeployment := &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{},
			}

			err = configureGit(ctx, displayOpts, currentBe, s, newStackDeployment, gitSSHPrivateKeyPath)
			if err != nil {
				return err
			}

			err = configureImageRepository(ctx, displayOpts, currentBe, s, newStackDeployment)
			if err != nil {
				return err
			}

			err = configureAdvancedSettings(displayOpts, newStackDeployment)
			if err != nil {
				return err
			}

			var option string
			if err := survey.AskOne(&survey.Select{
				Message: "Do you want to configure an OpenID Connect integration?",
				Options: []string{
					"No",
					"Enable AWS integration",
					"Enable Azure integration",
					"Enable Google Cloud integration",
				},
				Default: "No",
			}, &option, surveyIcons(displayOpts.Color)); err != nil {
				return fmt.Errorf("Failed to select oidc options")
			}

			switch option {
			case "Enable AWS integration":
				err = configureOidcAws(newStackDeployment)
			case "Enable Azure integration":
				err = configureOidcAzure(newStackDeployment)
			case "Enable Google Cloud integration":
				err = configureOidcGCP(newStackDeployment)
			}

			if err != nil {
				return err
			}

			err = saveProjectStackDeployment(newStackDeployment, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&gitSSHPrivateKeyPath, "git-ssh-private-key", "k", "",
		"Private key path")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

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

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

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

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

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

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func newDeploymentSettingsSetCmd() *cobra.Command {
	var stack string
	var gitSSHPrivateKeyPath string

	cmd := &cobra.Command{
		Use:   "set [configuration]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Updates stack's deployment settings secrets",
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

			sd, err := loadProjectStackDeployment(s)
			if err != nil {
				return err
			}

			if sd == nil {
				return fmt.Errorf("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			var configuration string
			if len(args) == 1 {
				configuration = args[0]
			} else {
				var option string
				if err := survey.AskOne(&survey.Select{
					Message: "Configure:",
					Options: []string{
						"Git",
						"Executor image",
						"Advanced settings",
						"AWS OpenID Connect integration",
						"Azure OpenID Connect integration",
						"GCP OpenID Connect integration",
					},
				}, &option, surveyIcons(displayOpts.Color)); err != nil {
					return fmt.Errorf("confirmation cancelled")
				}

				switch option {
				case "Git":
					configuration = "git"
				case "Executor image":
					configuration = "executor-image"
				case "Advanced settings":
					configuration = "advanced-settings"
				case "AWS OpenID Connect integration":
					configuration = "oidc-aws"
				case "Azure OpenID Connect integration":
					configuration = "oidc-azure"
				case "GCP OpenID Connect integration":
					configuration = "oidc-gcp"
				}
			}

			switch configuration {
			case "git":
				err = configureGit(ctx, displayOpts, currentBe, s, sd, gitSSHPrivateKeyPath)
			case "executor-image":
				err = configureImageRepository(ctx, displayOpts, currentBe, s, sd)
			case "advanced-settings":
				err = configureAdvancedSettings(displayOpts, sd)
			case "oidc-aws":
				err = configureOidcAws(sd)
			case "oidc-azure":
				err = configureOidcAzure(sd)
			case "oidc-gcp":
				err = configureOidcGCP(sd)
			default:
				err = fmt.Errorf("Invalid option %q", configuration)
			}
			if err != nil {
				return err
			}

			err = saveProjectStackDeployment(sd, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&gitSSHPrivateKeyPath, "git-ssh-private-key", "k", "",
		"Private key path")

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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			interactive := cmdutil.Interactive()

			var (
				key   string
				value string
			)

			if len(args) > 1 {
				key = args[0]
			}

			if len(args) == 2 {
				if remove {
					return errors.New("value not supported when removing keys")
				}
				value = args[1]
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

			var secretValue *apitype.SecretValue
			if secret {
				secretValue, err = currentBe.EncryptStackDeploymentSettingsSecret(ctx, s, value)
				if err != nil {
					return err
				}
			} else {
				secretValue = &apitype.SecretValue{
					Value:  value,
					Secret: false,
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
				sd.DeploymentSettings.Operation.EnvironmentVariables[key] = *secretValue
			}

			err = saveProjectStackDeployment(sd, s)
			if err != nil {
				return err
			}

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVar(
		&secret, "secret", false,
		"Key to update")

	cmd.PersistentFlags().BoolVar(
		&remove, "remove", false,
		"Key to update")

	cmd.MarkFlagsMutuallyExclusive("secret", "remove")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func configureGit(ctx context.Context, displayOpts display.Options, be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment, gitSSHPrivateKeyPath string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoRoot, err := detectGitPath(wd)
	if err != nil {
		return err
	}

	defaultRepoDir, err := filepath.Rel(repoRoot, wd)
	if err != nil {
		return err
	}

	if sd.DeploymentSettings.SourceContext == nil {
		sd.DeploymentSettings.SourceContext = &apitype.SourceContext{}
	}
	if sd.DeploymentSettings.SourceContext.Git == nil {
		sd.DeploymentSettings.SourceContext.Git = &apitype.SourceContextGit{}
	}
	if sd.DeploymentSettings.SourceContext.Git.RepoDir != "" {
		defaultRepoDir = sd.DeploymentSettings.SourceContext.Git.RepoDir
	}

	repoDir, err := ReadConsoleWithDefault("Enter the repo directory", defaultRepoDir)
	if err != nil {
		return err
	}
	sd.DeploymentSettings.SourceContext.Git.RepoDir = repoDir

	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return err
	}

	h, err := repo.Head()
	if err != nil {
		return err
	}

	var branch string
	if sd.DeploymentSettings.SourceContext.Git.Branch != "" {
		branch, err = ReadConsoleWithDefault("Enter the branch name", sd.DeploymentSettings.SourceContext.Git.Branch)
	} else if h.Name().IsBranch() {
		branch, err = ReadConsoleWithDefault("Enter the branch name", h.Name().String())
	} else {
		branch, err = cmdutil.ReadConsole("Enter the branch name")
	}
	if err != nil {
		return err
	}
	sd.DeploymentSettings.SourceContext.Git.Branch = branch

	remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")
	if err != nil {
		return err
	}

	if vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL); err == nil {
		useGiHub := vcsInfo.Kind == gitutil.GitHubHostName

		if useGiHub {
			var option string
			if err := survey.AskOne(&survey.Select{
				Message: "A GitHub repository was detected, do you want to use the Pulumi GitHub App?",
				Options: []string{"yes", "no"},
				Default: string("yes"),
			}, &option, surveyIcons(displayOpts.Color)); err != nil {
				return fmt.Errorf("confirmation cancelled")
			}

			useGiHub = option == "yes"
		}

		if useGiHub {
			err := configureGitHubRepo(displayOpts, sd, vcsInfo)
			if err != nil {
				return err
			}
		} else {
			err := configureBareGitRepo(ctx, displayOpts, be, s, sd, remoteURL, gitSSHPrivateKeyPath)
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("detecting VCS info from stack tags for remote %v: %w", remoteURL, err)
	}

	return nil
}

func configureGitHubRepo(displayOpts display.Options, sd *workspace.ProjectStackDeployment, vcsInfo *gitutil.VCSInfo) error {
	sd.DeploymentSettings.GitHub = &apitype.DeploymentSettingsGitHub{
		Repository: fmt.Sprintf("%s/%s", vcsInfo.Owner, vcsInfo.Repo),
	}

	var options []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "What kind of authentication it should use?",
		Options: []string{
			"Run previews for pull requests",
			"Run updates for pushed commits",
			"Use this stack as a template for pull request stacks",
		},
	}, &options, surveyIcons(displayOpts.Color)); err != nil {
		return fmt.Errorf("Failed to select git options")
	}

	if slices.Contains(options, "Run previews for pull requests") {
		sd.DeploymentSettings.GitHub.PreviewPullRequests = true
	}

	if slices.Contains(options, "Run updates for pushed commits") {
		sd.DeploymentSettings.GitHub.DeployCommits = true
	}

	if slices.Contains(options, "Use this stack as a template for pull request stacks") {
		sd.DeploymentSettings.GitHub.PullRequestTemplate = true
	}

	return nil
}

func configureBareGitRepo(ctx context.Context, displayOpts display.Options, be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment, remoteURL string, gitSSHPrivateKeyPath string) error {
	sd.DeploymentSettings.SourceContext.Git.RepoURL = remoteURL

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: "What kind of authentication it should use?",
		Options: []string{
			"Username/Password",
			"SSH key",
		},
	}, &option, surveyIcons(displayOpts.Color)); err != nil {
		return fmt.Errorf("Failed to select git authentication")
	}
	switch option {
	case "Username/Password":
		return configureGitPassword(ctx, be, s, sd)
	case "SSH key":
		return configureGitSSH(ctx, be, s, sd, gitSSHPrivateKeyPath)
	}

	return nil
}

func configureGitPassword(ctx context.Context, be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment) error {
	var username string
	var password string
	var err error

	if sd.DeploymentSettings.SourceContext == nil {
		sd.DeploymentSettings.SourceContext = &apitype.SourceContext{}
	}

	if sd.DeploymentSettings.SourceContext.Git == nil {
		sd.DeploymentSettings.SourceContext.Git = &apitype.SourceContextGit{}
	}

	if sd.DeploymentSettings.SourceContext.Git.GitAuth == nil {
		sd.DeploymentSettings.SourceContext.Git.GitAuth = &apitype.GitAuthConfig{}
	}

	if sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth == nil {
		sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth = &apitype.BasicAuth{}
	}

	if sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value != "" && !sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Secret {
		username, err = ReadConsoleWithDefault("Enter the git username", sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
	} else {
		username, err = cmdutil.ReadConsole("Enter the git username")
	}
	if err != nil {
		return err
	}

	if password, err = cmdutil.ReadConsoleNoEcho("Enter the git password"); err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("Invalid empty password")
	}

	secret, err := be.EncryptStackDeploymentSettingsSecret(ctx, s, password)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.SourceContext.Git.GitAuth = &apitype.GitAuthConfig{
		BasicAuth: &apitype.BasicAuth{
			UserName: apitype.SecretValue{Value: username},
			Password: *secret,
		},
	}

	return nil
}

func configureGitSSH(ctx context.Context, be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment, gitSSHPrivateKeyPath string) error {
	if gitSSHPrivateKeyPath == "" {
		return fmt.Errorf("No SSH private key was provided")
	}

	privateKey, err := os.ReadFile(gitSSHPrivateKeyPath)
	if err != nil {
		return err
	}

	secret, err := be.EncryptStackDeploymentSettingsSecret(ctx, s, string(privateKey))
	if err != nil {
		return err
	}

	if sd.DeploymentSettings.SourceContext == nil {
		sd.DeploymentSettings.SourceContext = &apitype.SourceContext{}
	}

	if sd.DeploymentSettings.SourceContext.Git == nil {
		sd.DeploymentSettings.SourceContext.Git = &apitype.SourceContextGit{}
	}

	sd.DeploymentSettings.SourceContext.Git.GitAuth = &apitype.GitAuthConfig{
		SSHAuth: &apitype.SSHAuth{
			SSHPrivateKey: *secret,
		},
	}

	var password string

	if password, err = cmdutil.ReadConsoleNoEcho("Enter the private key password []"); err != nil {
		return err
	}

	if password != "" {
		secret, err := be.EncryptStackDeploymentSettingsSecret(ctx, s, password)
		if err != nil {
			return err
		}

		sd.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password = secret
	}

	return nil
}

func configureOidcAws(sd *workspace.ProjectStackDeployment) error {
	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.OIDC == nil {
		sd.DeploymentSettings.Operation.OIDC = &apitype.OperationContextOIDCConfiguration{}
	}

	if sd.DeploymentSettings.Operation.OIDC.AWS == nil {
		sd.DeploymentSettings.Operation.OIDC.AWS = &apitype.OperationContextAWSOIDCConfiguration{}
	}

	var roleARN string
	var sessionName string
	var err error

	if sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN != "" {
		roleARN, err = ReadConsoleWithDefault("AWS role ARN", sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN)
	} else {
		roleARN, err = cmdutil.ReadConsole("AWS role ARN")
	}
	if err != nil {
		return err
	}
	if roleARN == "" {
		return fmt.Errorf("Role ARN is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.AWS.SessionName != "" {
		sessionName, err = ReadConsoleWithDefault("AWS session name", sd.DeploymentSettings.Operation.OIDC.AWS.SessionName)
	} else {
		sessionName, err = cmdutil.ReadConsole("AWS session name")
	}
	if err != nil {
		return err
	}
	if sessionName == "" {
		return fmt.Errorf("Session name is required")
	}

	sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN = roleARN
	sd.DeploymentSettings.Operation.OIDC.AWS.SessionName = sessionName

	return nil
}

func configureOidcGCP(sd *workspace.ProjectStackDeployment) error {
	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.OIDC == nil {
		sd.DeploymentSettings.Operation.OIDC = &apitype.OperationContextOIDCConfiguration{}
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP == nil {
		sd.DeploymentSettings.Operation.OIDC.GCP = &apitype.OperationContextGCPOIDCConfiguration{}
	}

	var projectID string
	var providerID string
	var workloadPoolID string
	var serviceAccount string
	var err error

	if sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID != "" {
		projectID, err = ReadConsoleWithDefault("GCP project id", sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID)
	} else {
		projectID, err = cmdutil.ReadConsole("GCP project id")
	}
	if err != nil {
		return err
	}
	if projectID == "" {
		return fmt.Errorf("Project id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID != "" {
		providerID, err = ReadConsoleWithDefault("GCP provider id", sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID)
	} else {
		providerID, err = cmdutil.ReadConsole("GCP provider id")
	}
	if err != nil {
		return err
	}
	if providerID == "" {
		return fmt.Errorf("Provider id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID != "" {
		workloadPoolID, err = ReadConsoleWithDefault("GCP identity provider id", sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID)
	} else {
		workloadPoolID, err = cmdutil.ReadConsole("GCP identity provider id")
	}
	if err != nil {
		return err
	}
	if workloadPoolID == "" {
		return fmt.Errorf("Identity provider id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount != "" {
		serviceAccount, err = ReadConsoleWithDefault("GCP service account email address", sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount)
	} else {
		serviceAccount, err = cmdutil.ReadConsole("GCP service account email address")
	}
	if err != nil {
		return err
	}
	if serviceAccount == "" {
		return fmt.Errorf("service account email address is required")
	}

	sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID = projectID
	sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID = providerID
	sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID = workloadPoolID
	sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount = serviceAccount

	return nil
}

func configureOidcAzure(sd *workspace.ProjectStackDeployment) error {
	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.OIDC == nil {
		sd.DeploymentSettings.Operation.OIDC = &apitype.OperationContextOIDCConfiguration{}
	}

	if sd.DeploymentSettings.Operation.OIDC.Azure == nil {
		sd.DeploymentSettings.Operation.OIDC.Azure = &apitype.OperationContextAzureOIDCConfiguration{}
	}

	var clientID string
	var tenantID string
	var subscriptionID string
	var err error

	if sd.DeploymentSettings.Operation.OIDC.Azure.ClientID != "" {
		clientID, err = ReadConsoleWithDefault("Azure client id", sd.DeploymentSettings.Operation.OIDC.Azure.ClientID)
	} else {
		clientID, err = cmdutil.ReadConsole("Azure client id")
	}
	if err != nil {
		return err
	}
	if clientID == "" {
		return fmt.Errorf("Client id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.Azure.TenantID != "" {
		tenantID, err = ReadConsoleWithDefault("Azure tenant id", sd.DeploymentSettings.Operation.OIDC.Azure.TenantID)
	} else {
		tenantID, err = cmdutil.ReadConsole("Azure tenant id")
	}
	if err != nil {
		return err
	}
	if tenantID == "" {
		return fmt.Errorf("Tenant id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID != "" {
		subscriptionID, err = ReadConsoleWithDefault("Azure subscription id", sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID)
	} else {
		subscriptionID, err = cmdutil.ReadConsole("Azure subscription id")
	}
	if err != nil {
		return err
	}
	if subscriptionID == "" {
		return fmt.Errorf("Subscription id is required")
	}

	sd.DeploymentSettings.Operation.OIDC.Azure.ClientID = clientID
	sd.DeploymentSettings.Operation.OIDC.Azure.TenantID = tenantID
	sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID = subscriptionID

	return nil
}

func configureAdvancedSettings(displayOpts display.Options, sd *workspace.ProjectStackDeployment) error {
	var options []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Advanced settings",
		Options: []string{
			"Skip automatic dependency installation step",
			"Skip intermediate deployments",
		},
	}, &options, surveyIcons(displayOpts.Color)); err != nil {
		return fmt.Errorf("confirmation cancelled")
	}

	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.Options == nil {
		sd.DeploymentSettings.Operation.Options = &apitype.OperationContextOptions{}
	}

	if slices.Contains(options, "Skip automatic dependency installation step") {
		sd.DeploymentSettings.Operation.Options.SkipInstallDependencies = true
	}

	if slices.Contains(options, "Skip intermediate deployments") {
		sd.DeploymentSettings.Operation.Options.SkipIntermediateDeployments = true
	}

	return nil
}

func configureImageRepository(ctx context.Context, displayOpts display.Options, be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment) error {
	var imageReference string
	var username string
	var password string
	var err error

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: "Do you want to use a custom executor image?",
		Options: []string{"yes", "no"},
		Default: string("no"),
	}, &option, surveyIcons(displayOpts.Color)); err != nil {
		return fmt.Errorf("confirmation cancelled")
	}

	if option == "no" {
		sd.DeploymentSettings.Executor = nil
		return nil
	}

	if sd.DeploymentSettings.Executor == nil {
		sd.DeploymentSettings.Executor = &apitype.ExecutorContext{}
	}

	if sd.DeploymentSettings.Executor.ExecutorImage == nil {
		sd.DeploymentSettings.Executor.ExecutorImage = &apitype.DockerImage{}
	}

	if sd.DeploymentSettings.Executor.ExecutorImage.Credentials == nil {
		sd.DeploymentSettings.Executor.ExecutorImage.Credentials = &apitype.DockerImageCredentials{}
	}

	if sd.DeploymentSettings.Executor.ExecutorImage.Reference != "" {
		imageReference, err = ReadConsoleWithDefault("Enter the image reference", sd.DeploymentSettings.Executor.ExecutorImage.Reference)
	} else {
		imageReference, err = cmdutil.ReadConsole("Enter the image reference")
	}

	if err != nil {
		return err
	}
	if imageReference == "" {
		return fmt.Errorf("Invalid empty image reference")
	}

	if sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Username != "" {
		imageReference, err = ReadConsoleWithDefault("Enter the image repository username", sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Username)
	} else {
		username, err = cmdutil.ReadConsole("Enter the image repository username")
	}

	if err != nil {
		return err
	}

	sd.DeploymentSettings.Executor.ExecutorImage = &apitype.DockerImage{
		Reference: imageReference,
	}

	if username == "" {
		return nil
	}

	if password, err = cmdutil.ReadConsoleNoEcho("Enter the image repository password"); err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("Invalid empty password")
	}

	secret, err := be.EncryptStackDeploymentSettingsSecret(ctx, s, password)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Executor.ExecutorImage.Credentials = &apitype.DockerImageCredentials{
		Username: username,
		Password: *secret,
	}

	return nil
}

// As go-git do not support an analogous to `git rev-parse --show-toplevel`,
// I am bringing this from a suggestion in an open issue tracking the requirement
// https://github.com/go-git/go-git/issues/74#issuecomment-647779420
func detectGitPath(path string) (string, error) {
	// normalize the path
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	for {
		fi, err := os.Stat(filepath.Join(path, ".git"))
		if err == nil {
			if !fi.IsDir() {
				return "", fmt.Errorf(".git exist but is not a directory")
			}
			return path, nil
		}
		if !os.IsNotExist(err) {
			// unknown error
			return "", err
		}

		// detect bare repo
		ok, err := isGitDir(path)
		if err != nil {
			return "", err
		}
		if ok {
			return path, nil
		}

		if parent := filepath.Dir(path); parent == path {
			return "", fmt.Errorf(".git not found")
		} else {
			path = parent
		}
	}
}

func isGitDir(path string) (bool, error) {
	markers := []string{"HEAD", "objects", "refs"}

	for _, marker := range markers {
		_, err := os.Stat(filepath.Join(path, marker))
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			// unknown error
			return false, err
		} else {
			return false, nil
		}
	}

	return true, nil
}

func ReadConsoleWithDefault(prompt string, defaultValue string) (string, error) {
	promptMessage := fmt.Sprintf("%s [%s]", prompt, defaultValue)
	value, err := cmdutil.ReadConsole(promptMessage)
	if err != nil {
		return "", err
	}

	if value == "" {
		value = defaultValue
	}

	return value, nil
}
