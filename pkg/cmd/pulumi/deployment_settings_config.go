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
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

const (
	optYes                         = "Yes"
	optNo                          = "No"
	optOidcAws                     = "Enable AWS integration"
	optOidcAzure                   = "Enable Azure integration"
	optOidcGcp                     = "Enable Google Cloud integration"
	optGit                         = "Git"
	optExecutorImage               = "Executor image"
	optAdvancedSettings            = "Advanced settings"
	optPreviewPr                   = "Run previews for pull requests"
	optUpdatePushes                = "Run updates for pushed commits"
	optPrTemplate                  = "Use this stack as a template for pull request stacks"
	optUserPass                    = "Username/Password"
	optSSH                         = "SSH key"
	optSkipDepsInstall             = "Skip automatic dependency installation step"
	optSkipIntermediateDeployments = "Skip intermediate deployments"
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
	cmd.AddCommand(newDeploymentSettingsConfigureCmd())

	return cmd
}

type deploymentSettingsCommandDependencies struct {
	DisplayOptions *display.Options
	Stack          backend.Stack
	Deployment     *workspace.ProjectStackDeployment
	Backend        backend.Backend
}

func initializeDeploymentSettingsCmd(cmd *cobra.Command, stack string) (*deploymentSettingsCommandDependencies, error) {
	ctx := cmd.Context()
	interactive := cmdutil.Interactive()

	displayOpts := display.Options{
		Color:         cmdutil.GetGlobalColorization(),
		IsInteractive: interactive,
	}

	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	be, err := currentBackend(ctx, project, displayOpts)
	if err != nil {
		return nil, err
	}

	if !be.SupportsDeployments() {
		return nil, fmt.Errorf("backends of this type %q do not support deployments",
			be.Name())
	}

	s, err := requireStack(ctx, stack, stackOfferNew|stackSetCurrent, displayOpts)
	if err != nil {
		return nil, err
	}

	sd, err := loadProjectStackDeployment(s)
	if err != nil {
		return nil, err
	}

	return &deploymentSettingsCommandDependencies{
		DisplayOptions: &displayOpts,
		Stack:          s,
		Deployment:     sd,
		Backend:        be,
	}, nil
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
			d, err := initializeDeploymentSettingsCmd(cmd, stack)
			if err != nil {
				return err
			}

			newStackDeployment := &workspace.ProjectStackDeployment{
				DeploymentSettings: apitype.DeploymentSettings{},
			}

			err = configureGit(ctx, d.DisplayOptions, d.Backend, d.Stack, newStackDeployment, gitSSHPrivateKeyPath)
			if err != nil {
				return err
			}

			err = configureImageRepository(ctx, d.DisplayOptions, d.Backend, d.Stack, newStackDeployment)
			if err != nil {
				return err
			}

			err = configureAdvancedSettings(d.DisplayOptions, newStackDeployment)
			if err != nil {
				return err
			}

			var option string
			if err := survey.AskOne(&survey.Select{
				Message: "Do you want to configure an OpenID Connect integration?",
				Options: []string{
					optNo,
					optOidcAws,
					optOidcAzure,
					optOidcGcp,
				},
				Default: optNo,
			}, &option, surveyIcons(d.DisplayOptions.Color)); err != nil {
				return errors.New("selection cancelled")
			}

			switch option {
			case optOidcAws:
				err = configureOidcAws(newStackDeployment)
			case optOidcAzure:
				err = configureOidcAzure(newStackDeployment)
			case optOidcGcp:
				err = configureOidcGCP(newStackDeployment)
			}
			if err != nil {
				return err
			}

			err = saveProjectStackDeployment(newStackDeployment, d.Stack)
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

func newDeploymentSettingsConfigureCmd() *cobra.Command {
	var stack string
	var gitSSHPrivateKeyPath string

	cmd := &cobra.Command{
		Use:   "configure",
		Args:  cmdutil.ExactArgs(0),
		Short: "Updates stack's deployment settings secrets",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			d, err := initializeDeploymentSettingsCmd(cmd, stack)
			if err != nil {
				return err
			}

			if d.Deployment == nil {
				return errors.New("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			var option string
			if err := survey.AskOne(&survey.Select{
				Message: "Configure:",
				Options: []string{
					optGit,
					optExecutorImage,
					optAdvancedSettings,
					optOidcAws,
					optOidcAzure,
					optOidcGcp,
				},
			}, &option, surveyIcons(d.DisplayOptions.Color)); err != nil {
				return errors.New("selection cancelled")
			}

			switch option {
			case optGit:
				err = configureGit(ctx, d.DisplayOptions, d.Backend, d.Stack, d.Deployment, gitSSHPrivateKeyPath)
			case optExecutorImage:
				err = configureImageRepository(ctx, d.DisplayOptions, d.Backend, d.Stack, d.Deployment)
			case optAdvancedSettings:
				err = configureAdvancedSettings(d.DisplayOptions, d.Deployment)
			case optOidcAws:
				err = configureOidcAws(d.Deployment)
			case optOidcAzure:
				err = configureOidcAzure(d.Deployment)
			case optOidcGcp:
				err = configureOidcGCP(d.Deployment)
			default:
				return nil
			}
			if err != nil {
				return err
			}

			err = saveProjectStackDeployment(d.Deployment, d.Stack)
			if err != nil {
				return err
			}

			fmt.Println("Done")

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

func newRepoLookup(repoRoot string) (repoLookup, error) {
	if repoRoot != "" {

		repo, err := git.PlainOpen(repoRoot)
		if err != nil {
			return nil, err
		}

		h, err := repo.Head()
		if err != nil {
			return nil, err
		}

		return &repoLookupImpl{
			RepoRoot: repoRoot,
			Repo:     repo,
			Head:     h,
		}, nil
	}
	return &noRepoLookupImpl{}, nil
}

type repoLookup interface {
	GetRootDirectory(wd string) (string, error)
	GetBranchName() string
	RemoteURL() (string, error)
}
type repoLookupImpl struct {
	RepoRoot string
	Repo     *git.Repository
	Head     *plumbing.Reference
}

func (r *repoLookupImpl) GetRootDirectory(wd string) (string, error) {
	return filepath.Rel(r.RepoRoot, wd)
}

func (r *repoLookupImpl) GetBranchName() string {
	return r.Head.Name().String()
}

func (r *repoLookupImpl) RemoteURL() (string, error) {
	return gitutil.GetGitRemoteURL(r.Repo, "origin")
}

type noRepoLookupImpl struct{}

func (r *noRepoLookupImpl) GetRootDirectory(wd string) (string, error) {
	return ".", nil
}

func (r *noRepoLookupImpl) GetBranchName() string {
	return ""
}

func (r *noRepoLookupImpl) RemoteURL() (string, error) {
	return "", nil
}

func configureGit(ctx context.Context, displayOpts *display.Options, be backend.Backend,
	s backend.Stack, sd *workspace.ProjectStackDeployment, gitSSHPrivateKeyPath string,
) error {
	if sd.DeploymentSettings.SourceContext == nil {
		sd.DeploymentSettings.SourceContext = &apitype.SourceContext{}
	}
	if sd.DeploymentSettings.SourceContext.Git == nil {
		sd.DeploymentSettings.SourceContext.Git = &apitype.SourceContextGit{}
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoRoot, err := gitutil.DetectGitRootDirectory(wd)
	if err != nil && !errors.Is(err, gitutil.ErrNoGitRepo) {
		return fmt.Errorf("could not determine the git root directory: %w", err)
	}

	rl, err := newRepoLookup(repoRoot)
	if err != nil {
		return err
	}

	var defaultRepoDir string
	if sd.DeploymentSettings.SourceContext.Git.RepoDir != "" {
		defaultRepoDir = sd.DeploymentSettings.SourceContext.Git.RepoDir
	} else {
		defaultRepoDir, err = rl.GetRootDirectory(wd)
		if err != nil {
			return err
		}
	}

	repoDir, err := cmdutil.ReadConsoleWithDefault("Enter the repo directory", defaultRepoDir)
	if err != nil {
		return err
	}
	sd.DeploymentSettings.SourceContext.Git.RepoDir = repoDir

	var branchName string
	if sd.DeploymentSettings.SourceContext.Git.Branch != "" {
		branchName = sd.DeploymentSettings.SourceContext.Git.Branch
	} else {
		branchName = rl.GetBranchName()
	}

	if branchName != "" {
		branchName, err = cmdutil.ReadConsoleWithDefault("Enter the branch name", branchName)
	} else {
		branchName, err = cmdutil.ReadConsole("Enter the branch name")
	}
	if err != nil {
		return err
	}
	sd.DeploymentSettings.SourceContext.Git.Branch = branchName

	remoteURL, err := rl.RemoteURL()
	if err != nil {
		return err
	}

	if remoteURL != "" {
		remoteURL, err = cmdutil.ReadConsoleWithDefault("Enter the repository URL", remoteURL)
	} else {
		remoteURL, err = cmdutil.ReadConsole("Enter the repository URL")
	}
	if err != nil {
		return err
	}

	vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL)

	var useGiHub bool
	if err == nil {
		useGiHub = vcsInfo.Kind == gitutil.GitHubHostName
	} else {
		// we failed to parse the remote URL, we default to a non-github repo
		useGiHub = false
	}

	if useGiHub {
		useGiHub, err = askForConfirmation("A GitHub repository was detected, do you want to use the Pulumi GitHub App?",
			displayOpts.Color, true)
		if err != nil {
			return err
		}
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

	return nil
}

func configureGitHubRepo(displayOpts *display.Options,
	sd *workspace.ProjectStackDeployment, vcsInfo *gitutil.VCSInfo,
) error {
	sd.DeploymentSettings.GitHub = &apitype.DeploymentSettingsGitHub{
		Repository: fmt.Sprintf("%s/%s", vcsInfo.Owner, vcsInfo.Repo),
	}

	var options []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "What kind of authentication should it use?",
		Options: []string{
			optPreviewPr,
			optUpdatePushes,
			optPrTemplate,
		},
		Default: []string{
			optPreviewPr,
			optUpdatePushes,
		},
	}, &options, surveyIcons(displayOpts.Color)); err != nil {
		return errors.New("selection cancelled")
	}

	if slices.Contains(options, optPreviewPr) {
		sd.DeploymentSettings.GitHub.PreviewPullRequests = true
	}

	if slices.Contains(options, optUpdatePushes) {
		sd.DeploymentSettings.GitHub.DeployCommits = true
	}

	if slices.Contains(options, optPrTemplate) {
		sd.DeploymentSettings.GitHub.PullRequestTemplate = true
	}

	return nil
}

func configureBareGitRepo(ctx context.Context, displayOpts *display.Options,
	be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment,
	remoteURL string, gitSSHPrivateKeyPath string,
) error {
	sd.DeploymentSettings.SourceContext.Git.RepoURL = remoteURL

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: "What kind of authentication should it use?",
		Options: []string{
			optUserPass,
			optSSH,
		},
	}, &option, surveyIcons(displayOpts.Color)); err != nil {
		return errors.New("selection cancelled")
	}
	switch option {
	case optUserPass:
		return configureGitPassword(ctx, be, s, sd)
	case optSSH:
		return configureGitSSH(ctx, be, s, sd, gitSSHPrivateKeyPath)
	}

	return nil
}

func configureGitPassword(ctx context.Context, be backend.Backend,
	s backend.Stack, sd *workspace.ProjectStackDeployment,
) error {
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

	if sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value != "" &&
		!sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Secret {
		username, err = cmdutil.ReadConsoleWithDefault("Enter the git username",
			sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value)
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
		return errors.New("Invalid empty password")
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

func configureGitSSH(ctx context.Context, be backend.Backend, s backend.Stack,
	sd *workspace.ProjectStackDeployment, gitSSHPrivateKeyPath string,
) error {
	if gitSSHPrivateKeyPath == "" {
		return errors.New("No SSH private key was provided")
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
		roleARN, err = cmdutil.ReadConsoleWithDefault("AWS role ARN", sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN)
	} else {
		roleARN, err = cmdutil.ReadConsole("AWS role ARN")
	}
	if err != nil {
		return err
	}
	if roleARN == "" {
		return errors.New("Role ARN is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.AWS.SessionName != "" {
		sessionName, err = cmdutil.ReadConsoleWithDefault("AWS session name",
			sd.DeploymentSettings.Operation.OIDC.AWS.SessionName)
	} else {
		sessionName, err = cmdutil.ReadConsole("AWS session name")
	}
	if err != nil {
		return err
	}
	if sessionName == "" {
		return errors.New("Session name is required")
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
		projectID, err = cmdutil.ReadConsoleWithDefault("GCP project id", sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID)
	} else {
		projectID, err = cmdutil.ReadConsole("GCP project id")
	}
	if err != nil {
		return err
	}

	if projectID == "" {
		return errors.New("Project id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID != "" {
		providerID, err = cmdutil.ReadConsoleWithDefault("GCP provider id",
			sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID)
	} else {
		providerID, err = cmdutil.ReadConsole("GCP provider id")
	}
	if err != nil {
		return err
	}
	if providerID == "" {
		return errors.New("Provider id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID != "" {
		workloadPoolID, err = cmdutil.ReadConsoleWithDefault("GCP identity provider id",
			sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID)
	} else {
		workloadPoolID, err = cmdutil.ReadConsole("GCP identity provider id")
	}
	if err != nil {
		return err
	}
	if workloadPoolID == "" {
		return errors.New("Identity provider id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount != "" {
		serviceAccount, err = cmdutil.ReadConsoleWithDefault("GCP service account email address",
			sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount)
	} else {
		serviceAccount, err = cmdutil.ReadConsole("GCP service account email address")
	}
	if err != nil {
		return err
	}
	if serviceAccount == "" {
		return errors.New("service account email address is required")
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
		clientID, err = cmdutil.ReadConsoleWithDefault("Azure client id", sd.DeploymentSettings.Operation.OIDC.Azure.ClientID)
	} else {
		clientID, err = cmdutil.ReadConsole("Azure client id")
	}
	if err != nil {
		return err
	}
	if clientID == "" {
		return errors.New("Client id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.Azure.TenantID != "" {
		tenantID, err = cmdutil.ReadConsoleWithDefault("Azure tenant id", sd.DeploymentSettings.Operation.OIDC.Azure.TenantID)
	} else {
		tenantID, err = cmdutil.ReadConsole("Azure tenant id")
	}
	if err != nil {
		return err
	}
	if tenantID == "" {
		return errors.New("Tenant id is required")
	}

	if sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID != "" {
		subscriptionID, err = cmdutil.ReadConsoleWithDefault("Azure subscription id",
			sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID)
	} else {
		subscriptionID, err = cmdutil.ReadConsole("Azure subscription id")
	}
	if err != nil {
		return err
	}
	if subscriptionID == "" {
		return errors.New("Subscription id is required")
	}

	sd.DeploymentSettings.Operation.OIDC.Azure.ClientID = clientID
	sd.DeploymentSettings.Operation.OIDC.Azure.TenantID = tenantID
	sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID = subscriptionID

	return nil
}

func configureAdvancedSettings(displayOpts *display.Options, sd *workspace.ProjectStackDeployment) error {
	var options []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: "Advanced settings",
		Options: []string{
			optSkipIntermediateDeployments,
			optSkipDepsInstall,
		},
		Default: []string{
			optSkipIntermediateDeployments,
		},
	}, &options, surveyIcons(displayOpts.Color)); err != nil {
		return errors.New("selection cancelled")
	}

	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.Options == nil {
		sd.DeploymentSettings.Operation.Options = &apitype.OperationContextOptions{}
	}

	if slices.Contains(options, optSkipDepsInstall) {
		sd.DeploymentSettings.Operation.Options.SkipInstallDependencies = true
	}

	if slices.Contains(options, optSkipIntermediateDeployments) {
		sd.DeploymentSettings.Operation.Options.SkipIntermediateDeployments = true
	}

	return nil
}

func configureImageRepository(ctx context.Context, displayOpts *display.Options,
	be backend.Backend, s backend.Stack, sd *workspace.ProjectStackDeployment,
) error {
	var imageReference string
	var username string
	var password string
	var err error

	confirm, err := askForConfirmation("Do you want to use a custom executor image?", displayOpts.Color, false)
	if err != nil {
		return err
	}

	if !confirm {
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
		imageReference, err = cmdutil.ReadConsoleWithDefault("Enter the image reference",
			sd.DeploymentSettings.Executor.ExecutorImage.Reference)
	} else {
		imageReference, err = cmdutil.ReadConsole("Enter the image reference")
	}
	if err != nil {
		return err
	}

	if imageReference == "" {
		return errors.New("Invalid empty image reference")
	}

	if sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Username != "" {
		imageReference, err = cmdutil.ReadConsoleWithDefault("(Optional) Enter the image repository username",
			sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Username)
	} else {
		username, err = cmdutil.ReadConsole("(Optional) Enter the image repository username")
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
		return errors.New("Invalid empty password")
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

func askForConfirmation(prompt string, color colors.Colorization, defaultValue bool) (bool, error) {
	var option string
	def := optNo
	if defaultValue {
		def = optYes
	}
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: []string{optYes, optNo},
		Default: def,
	}, &option, surveyIcons(color)); err != nil {
		return false, errors.New("selection cancelled")
	}

	return option == optYes, nil
}
