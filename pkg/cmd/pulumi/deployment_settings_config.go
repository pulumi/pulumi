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
	"path"
	"path/filepath"
	"slices"

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
	var yes bool

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

	cmd.PersistentFlags().BoolVarP(
		&yes, "yes", "y", false,
		"Automatically confirm every confirmation prompt")

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
	Yes            bool
	Ctx            context.Context
}

func initializeDeploymentSettingsCmd(cmd *cobra.Command, stack string) (*deploymentSettingsCommandDependencies, error) {
	yes := false
	if cmd.Flag("yes") != nil {
		yes = cmd.Flag("yes").Value.String() == "true"
	}
	return initializeDeploymentSettings(cmd.Context(), stack, yes)
}

func initializeDeploymentSettings(
	ctx context.Context, stack string, yes bool,
) (*deploymentSettingsCommandDependencies, error) {
	interactive := cmdutil.Interactive()

	if !interactive && !yes {
		return nil, errors.New("--yes must be passed in to proceed when running in non-interactive mode")
	}

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
		unsupportedBackendMsg := fmt.Sprintf("Backends of type %q do not support managed deployments.\n\n"+
			"Create a Pulumi Cloud account to get started, learn more about pulumi deployments here: "+
			"https://www.pulumi.com/docs/pulumi-cloud/deployments/",
			be.Name())

		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg,
			fmt.Sprintf("Backends of type %q do not support managed deployments", be.Name()),
			colors.Red+colors.Bold)
		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg, "Pulumi Cloud", colors.BrightCyan+colors.Bold)
		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg,
			"https://www.pulumi.com/docs/pulumi-cloud/deployments/", colors.BrightBlue+colors.Underline+colors.Bold)

		fmt.Println()
		fmt.Println(displayOpts.Color.Colorize(unsupportedBackendMsg))
		fmt.Println()

		return nil, fmt.Errorf("unable to manage stack deployments for backend type: %s",
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
		Yes:            yes,
		Ctx:            ctx,
	}, nil
}

func newDeploymentSettingsInitCmd() *cobra.Command {
	var force bool
	var stack string
	var gitSSHPrivateKeyPath string

	cmd := &cobra.Command{
		Use:        "init",
		SuggestFor: []string{"new", "create"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Initialize the stack's deployment.yaml file",
		Long:       "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			d, err := initializeDeploymentSettingsCmd(cmd, stack)
			if err != nil {
				return err
			}

			if d.Deployment != nil && !force {
				return fmt.Errorf("Deployment settings already configured for stack %q. Rerun for a "+
					"different stack by using --stack, update it by using the \"configure\" command or by "+
					"editing the file manually; or use --force", d.Stack.Ref())
			}

			return initStackDeploymentCmd(d, gitSSHPrivateKeyPath)
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&gitSSHPrivateKeyPath, "git-ssh-private-key", "k", "",
		"Private key path")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces content to be generated even if it is already configured")

	return cmd
}

func initStackDeploymentCmd(d *deploymentSettingsCommandDependencies, gitSSHPrivateKeyPath string) error {
	d.Deployment = &workspace.ProjectStackDeployment{
		DeploymentSettings: apitype.DeploymentSettings{},
	}

	err := configureGit(d, gitSSHPrivateKeyPath)
	if err != nil {
		return err
	}

	err = configureImageRepository(d)
	if err != nil {
		return err
	}

	err = configureAdvancedSettings(d)
	if err != nil {
		return err
	}

	option := promptUserSkippable(
		d.Yes,
		"Do you want to configure an OpenID Connect integration?",
		[]string{
			optNo,
			optOidcAws,
			optOidcAzure,
			optOidcGcp,
		},
		optNo,
		d.DisplayOptions.Color)

	switch option {
	case optOidcAws:
		err = configureOidcAws(d)
	case optOidcAzure:
		err = configureOidcAzure(d)
	case optOidcGcp:
		err = configureOidcGCP(d)
	}

	if err != nil {
		return err
	}

	return saveProjectStackDeployment(d.Deployment, d.Stack)
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
			d, err := initializeDeploymentSettingsCmd(cmd, stack)
			if err != nil {
				return err
			}

			if d.Deployment == nil {
				return errors.New("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			option := promptUserSkippable(
				d.Yes,
				"Configure",
				[]string{
					optGit,
					optExecutorImage,
					optAdvancedSettings,
					optOidcAws,
					optOidcAzure,
					optOidcGcp,
				},
				optGit,
				d.DisplayOptions.Color)

			switch option {
			case optGit:
				err = configureGit(d, gitSSHPrivateKeyPath)
			case optExecutorImage:
				err = configureImageRepository(d)
			case optAdvancedSettings:
				err = configureAdvancedSettings(d)
			case optOidcAws:
				err = configureOidcAws(d)
			case optOidcAzure:
				err = configureOidcAzure(d)
			case optOidcGcp:
				err = configureOidcGCP(d)
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

func newRepoLookup(wd string) (repoLookup, error) {
	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
	switch {
	case errors.Is(err, git.ErrRepositoryNotExists):
		return &noRepoLookupImpl{}, nil
	case err != nil:
		return nil, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	h, err := repo.Head()
	if err != nil {
		return nil, err
	}

	return &repoLookupImpl{
		RepoRoot: worktree.Filesystem.Root(),
		Repo:     repo,
		Head:     h,
	}, nil
}

type repoLookup interface {
	GetRootDirectory(wd string) (string, error)
	GetBranchName() string
	RemoteURL() (string, error)
	GetRepoRoot() string
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
	if r.Head == nil {
		return ""
	}
	return r.Head.Name().String()
}

func (r *repoLookupImpl) RemoteURL() (string, error) {
	if r.Repo == nil {
		return "", nil
	}
	return gitutil.GetGitRemoteURL(r.Repo, "origin")
}

func (r *repoLookupImpl) GetRepoRoot() string {
	return r.RepoRoot
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

func (r *noRepoLookupImpl) GetRepoRoot() string {
	return ""
}

func configureGit(d *deploymentSettingsCommandDependencies, gitSSHPrivateKeyPath string,
) error {
	sd := d.Deployment
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

	rl, err := newRepoLookup(wd)
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

	repoDir, err := promptForValue(d.Yes, "repository directory", defaultRepoDir, false,
		ValidateRelativeDirectory(rl.GetRepoRoot()), *d.DisplayOptions)
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

	branchName, err = promptForValue(d.Yes, "branch name", branchName, false, ValidateBranchName, *d.DisplayOptions)
	if err != nil {
		return err
	}
	sd.DeploymentSettings.SourceContext.Git.Branch = branchName

	remoteURL, err := rl.RemoteURL()
	if err != nil {
		return err
	}

	remoteURL, err = promptForValue(d.Yes, "repository URL", remoteURL, false, ValidateGitURL, *d.DisplayOptions)
	if err != nil {
		return err
	}

	vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL)

	var useGitHub bool
	if err == nil {
		useGitHub = vcsInfo.Kind == gitutil.GitHubHostName
	} else {
		// we failed to parse the remote URL, we default to a non-github repo
		useGitHub = false
	}

	if useGitHub {
		useGitHub = askForConfirmation("A GitHub repository was detected, do you want to use the Pulumi GitHub App?",
			d.DisplayOptions.Color, true, d.Yes)
	}

	if useGitHub {
		err := configureGitHubRepo(d, vcsInfo)
		if err != nil {
			return err
		}
	} else {
		err := configureBareGitRepo(d, remoteURL, gitSSHPrivateKeyPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func configureGitHubRepo(d *deploymentSettingsCommandDependencies, vcsInfo *gitutil.VCSInfo) error {
	sd := d.Deployment

	sd.DeploymentSettings.GitHub = &apitype.DeploymentSettingsGitHub{
		Repository: fmt.Sprintf("%s/%s", vcsInfo.Owner, vcsInfo.Repo),
	}

	options := promptUserMultiSkippable(
		d.Yes,
		"GitHub configuration",
		[]string{
			optPreviewPr,
			optUpdatePushes,
			optPrTemplate,
		},
		[]string{
			optPreviewPr,
			optUpdatePushes,
		},
		d.DisplayOptions.Color)

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

func configureBareGitRepo(d *deploymentSettingsCommandDependencies,
	remoteURL string, gitSSHPrivateKeyPath string,
) error {
	sd := d.Deployment

	sd.DeploymentSettings.SourceContext.Git.RepoURL = remoteURL

	option := promptUserSkippable(
		d.Yes,
		"What kind of authentication does the repository use?",
		[]string{
			optUserPass,
			optSSH,
		},
		optUserPass,
		d.DisplayOptions.Color)

	switch option {
	case optUserPass:
		return configureGitPassword(d)
	case optSSH:
		return configureGitSSH(d, gitSSHPrivateKeyPath)
	}

	return nil
}

func configureGitPassword(d *deploymentSettingsCommandDependencies) error {
	var username string
	var password string
	var err error

	sd := d.Deployment

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
		username = sd.DeploymentSettings.SourceContext.Git.GitAuth.BasicAuth.UserName.Value
	}

	username, err = promptForValue(d.Yes, "git username", username, false, ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	password, err = promptForValue(d.Yes, "git password", password, true, ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	secret, err := d.Backend.EncryptStackDeploymentSettingsSecret(d.Ctx, d.Stack, password)
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

func configureGitSSH(d *deploymentSettingsCommandDependencies, gitSSHPrivateKeyPath string) error {
	if gitSSHPrivateKeyPath == "" {
		configureMsg := "No SSH private key was provided, run `pulumi deployment settings " +
			"configure` with the `--git-ssh-private-key` flag set"
		configureMsg = colors.Highlight(configureMsg, "No SSH private key was provided", colors.Red+colors.Bold)
		configureMsg = colors.Highlight(configureMsg, "pulumi deployment settings configure", colors.BrightBlue+colors.Bold)
		configureMsg = colors.Highlight(configureMsg, "git-ssh-private-key", colors.BrightBlue+colors.Bold)
		fmt.Println()
		fmt.Println(d.DisplayOptions.Color.Colorize(configureMsg))
		fmt.Println()
		return nil
	}

	privateKey, err := os.ReadFile(gitSSHPrivateKeyPath)
	if err != nil {
		return err
	}

	secret, err := d.Backend.EncryptStackDeploymentSettingsSecret(d.Ctx, d.Stack, string(privateKey))
	if err != nil {
		return err
	}

	sd := d.Deployment

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

	password, err = promptForValue(d.Yes, "(optional) private key password", password, true,
		ValidateGenericInput, *d.DisplayOptions)
	if err != nil {
		return err
	}

	if password != "" {
		secret, err := d.Backend.EncryptStackDeploymentSettingsSecret(d.Ctx, d.Stack, password)
		if err != nil {
			return err
		}

		sd.DeploymentSettings.SourceContext.Git.GitAuth.SSHAuth.Password = secret
	}

	return nil
}

func configureOidcAws(d *deploymentSettingsCommandDependencies) error {
	sd := d.Deployment

	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.OIDC == nil {
		sd.DeploymentSettings.Operation.OIDC = &apitype.OperationContextOIDCConfiguration{}
	}

	if sd.DeploymentSettings.Operation.OIDC.AWS == nil {
		sd.DeploymentSettings.Operation.OIDC.AWS = &apitype.OperationContextAWSOIDCConfiguration{}
	}

	roleARN, err := promptForValue(d.Yes, "AWS role ARN", sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN,
		false, ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sessionName, err := promptForValue(d.Yes, "AWS session name",
		sd.DeploymentSettings.Operation.OIDC.AWS.SessionName, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN = roleARN
	sd.DeploymentSettings.Operation.OIDC.AWS.SessionName = sessionName

	return nil
}

func configureOidcGCP(d *deploymentSettingsCommandDependencies) error {
	sd := d.Deployment

	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.OIDC == nil {
		sd.DeploymentSettings.Operation.OIDC = &apitype.OperationContextOIDCConfiguration{}
	}

	if sd.DeploymentSettings.Operation.OIDC.GCP == nil {
		sd.DeploymentSettings.Operation.OIDC.GCP = &apitype.OperationContextGCPOIDCConfiguration{}
	}

	projectID, err := promptForValue(d.Yes, "GCP project id", sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	providerID, err := promptForValue(d.Yes, "GCP provider id", sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	workloadPoolID, err := promptForValue(d.Yes, "GCP identity provider id",
		sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	serviceAccount, err := promptForValue(d.Yes, "GCP service account email address",
		sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID = projectID
	sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID = providerID
	sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID = workloadPoolID
	sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount = serviceAccount

	return nil
}

func configureOidcAzure(d *deploymentSettingsCommandDependencies) error {
	sd := d.Deployment

	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.OIDC == nil {
		sd.DeploymentSettings.Operation.OIDC = &apitype.OperationContextOIDCConfiguration{}
	}

	if sd.DeploymentSettings.Operation.OIDC.Azure == nil {
		sd.DeploymentSettings.Operation.OIDC.Azure = &apitype.OperationContextAzureOIDCConfiguration{}
	}

	clientID, err := promptForValue(d.Yes, "Azure client ID", sd.DeploymentSettings.Operation.OIDC.Azure.ClientID, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	tenantID, err := promptForValue(d.Yes, "Azure tenant ID", sd.DeploymentSettings.Operation.OIDC.Azure.TenantID, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	subscriptionID, err := promptForValue(d.Yes, "Azure subscription ID",
		sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Operation.OIDC.Azure.ClientID = clientID
	sd.DeploymentSettings.Operation.OIDC.Azure.TenantID = tenantID
	sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID = subscriptionID

	return nil
}

func configureAdvancedSettings(d *deploymentSettingsCommandDependencies) error {
	sd := d.Deployment
	options := promptUserMultiSkippable(
		d.Yes,
		"Advanced settings",
		[]string{
			optSkipIntermediateDeployments,
			optSkipDepsInstall,
		},
		[]string{
			optSkipIntermediateDeployments,
		},
		d.DisplayOptions.Color)

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

func configureImageRepository(d *deploymentSettingsCommandDependencies) error {
	sd := d.Deployment

	confirm := askForConfirmation("Do you want to use a custom executor image?", d.DisplayOptions.Color, false, d.Yes)

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

	imageReference, err := promptForValue(d.Yes, "image reference",
		sd.DeploymentSettings.Executor.ExecutorImage.Reference, false,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	username, err := promptForValue(d.Yes, "(optional) image repository username",
		sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Username, false,
		ValidateGenericInput, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Executor.ExecutorImage = &apitype.DockerImage{
		Reference: imageReference,
	}

	if username == "" {
		return nil
	}

	password, err := promptForValue(d.Yes, "image repository password", "", true,
		ValidateGenericInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	secret, err := d.Backend.EncryptStackDeploymentSettingsSecret(d.Ctx, d.Stack, password)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Executor.ExecutorImage.Credentials = &apitype.DockerImageCredentials{
		Username: username,
		Password: *secret,
	}

	return nil
}

func askForConfirmation(prompt string, color colors.Colorization, defaultValue bool, yes bool) bool {
	def := optNo
	if defaultValue {
		def = optYes
	}
	options := []string{optYes, optNo}
	response := promptUserSkippable(yes, prompt, options, def, color)
	return response == optYes
}

// ValidateRelativeDirectory ensures a relative path points to a valid directory
func ValidateRelativeDirectory(rootDir string) func(string) error {
	return func(s string) error {
		if rootDir == "" {
			return nil
		}

		dir := path.Join(rootDir, s)
		info, err := os.Stat(dir)

		switch {
		case os.IsNotExist(err):
			return fmt.Errorf("invalid relative path %s", s)
		case err != nil:
			return err
		}

		if !info.IsDir() {
			return fmt.Errorf("invalid relative path %s, is not a directory", s)
		}

		return nil
	}
}

func ValidateBranchName(s string) error {
	const maxTagValueLength = 256

	if s == "" {
		return errors.New("should not be empty")
	}

	if len(s) > maxTagValueLength {
		return errors.New("must be 256 characters or less")
	}

	return nil
}

func ValidateGitURL(s string) error {
	_, _, err := gitutil.ParseGitRepoURL(s)

	return err
}

func ValidateGenericInputNonEmpty(s string) error {
	if s == "" {
		return errors.New("should not be empty")
	}

	return ValidateGenericInput(s)
}

func ValidateGenericInput(s string) error {
	const maxTagValueLength = 256

	if len(s) > maxTagValueLength {
		return errors.New("must be 256 characters or less")
	}

	return nil
}
