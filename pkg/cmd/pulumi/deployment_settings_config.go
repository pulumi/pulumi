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

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TODO: hack/pulumirc

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
	optNoAuthentication            = "No authentication"
	optUserPass                    = "Username/Password"
	optSSH                         = "SSH key"
	optSkipDepsInstall             = "Skip automatic dependency installation step"
	optSkipIntermediateDeployments = "Skip intermediate deployments"
)

//nolint:lll
type DeploymentArgs struct {
	StackDeploymentConfigFile string `args:"config-file" argsUsage:"Override the file name where the deployment settings are specified. Default is Pulumi.[stack].deploy.yaml"`
}

func newDeploymentCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
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
			config := UnmarshalArgs[DeploymentArgs](v, cmd)
			// TODO: hack/pulumirc
			stackDeploymentConfigFile = config.StackDeploymentConfigFile

			return cmd.Help()
		}),
	}

	newDeploymentSettingsCmd(v, cmd)
	newDeploymentRunCmd(v, cmd)

	return cmd
}

func newDeploymentSettingsCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Args:  cmdutil.NoArgs,
		Short: "Manage stack deployment settings",
		Long: "Manage stack deployment settings\n" +
			"\n" +
			"Use this command to manage a stack's deployment settings like\n" +
			"generating the deployment file, updating secrets or pushing the\n" +
			"updated settings to Pulumi Cloud.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		}),
	}

	parentCmd.AddCommand(cmd)

	newDeploymentSettingsInitCmd(v, cmd)
	newDeploymentSettingsPullCmd(v, cmd)
	newDeploymentSettingsUpdateCmd(v, cmd)
	newDeploymentSettingsDestroyCmd(v, cmd)
	newDeploymentSettingsEnvCmd(v, cmd)
	cmd.AddCommand(newDeploymentSettingsConfigureCmd())

	return cmd
}

type deploymentSettingsCommandDependencies struct {
	DisplayOptions *display.Options
	Stack          backend.Stack
	Deployment     *workspace.ProjectStackDeployment
	Backend        backend.Backend
	Interactive    bool
	Ctx            context.Context
	Prompts        prompts
	WorkDir        string
}

func initializeDeploymentSettingsCmd(
	ctx context.Context, stack string,
) (*deploymentSettingsCommandDependencies, error) {
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
		unsupportedBackendMsg := fmt.Sprintf("Backends of type %q do not support managed deployments.\n\n"+
			"Create a Pulumi Cloud account to get started, learn more about pulumi deployments here: "+
			"https://www.pulumi.com/docs/pulumi-cloud/deployments/",
			be.Name())

		unsupportedBackendMsg = colors.Highlight(unsupportedBackendMsg,
			fmt.Sprintf("Backends of type %q do not support managed deployments", be.Name()),
			colors.SpecError+colors.Bold)
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

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &deploymentSettingsCommandDependencies{
		DisplayOptions: &displayOpts,
		Stack:          s,
		Deployment:     sd,
		Backend:        be,
		Interactive:    interactive,
		Ctx:            ctx,
		Prompts:        promptHandlers{},
		WorkDir:        wd,
	}, nil
}

//nolint:lll
type DeploymentSettingsInitArgs struct {
	Force                 bool   `argsShort:"f" argsUsage:"Forces content to be generated even if it is already configured"`
	Stack                 string `argsShort:"s" argsUsage:"The name of the stack to operate on. Defaults to the current stack"`
	GitSSHPrivateKeyPath  string `args:"git-auth-ssh-private-key-path" argsUsage:"Git SSH private key path"`
	GitSSHPrivateKeyValue string `args:"git-auth-ssh-private-key" argsUsage:"Git SSH private key"`
}

func newDeploymentSettingsInitCmd(v *viper.Viper, parentCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "init",
		SuggestFor: []string{"new", "create"},
		Args:       cmdutil.ExactArgs(0),
		Short:      "Initialize the stack's deployment.yaml file",
		Long:       "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			config := UnmarshalArgs[DeploymentSettingsInitArgs](v, cmd)
			d, err := initializeDeploymentSettingsCmd(cmd.Context(), config.Stack)
			if err != nil {
				return err
			}

			if d.Deployment != nil && !config.Force {
				return fmt.Errorf("Deployment settings already configured for stack %q. Rerun for a "+
					"different stack by using --stack, update it by using the \"configure\" command or by "+
					"editing the file manually; or use --force", d.Stack.Ref())
			}

			return initStackDeploymentCmd(d, config.GitSSHPrivateKeyPath, config.GitSSHPrivateKeyValue)
		}),
	}

	parentCmd.AddCommand(cmd)
	BindFlags[DeploymentSettingsInitArgs](v, cmd)

	return cmd
}

func initStackDeploymentCmd(
	d *deploymentSettingsCommandDependencies, gitSSHPrivateKeyPath string, gitSSHPrivateKeyValue string,
) error {
	d.Deployment = &workspace.ProjectStackDeployment{
		DeploymentSettings: apitype.DeploymentSettings{},
	}

	err := configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)
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

	// For non interactive execution, we skip oidc configuration
	option := d.Prompts.PromptUserSkippable(
		!d.Interactive,
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
	var gitSSHPrivateKeyValue string

	cmd := &cobra.Command{
		Use:   "configure",
		Args:  cmdutil.ExactArgs(0),
		Short: "Updates stack's deployment settings secrets",
		Long:  "",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			if !cmdutil.Interactive() {
				return errors.New("configure command is only supported in interactive mode")
			}

			d, err := initializeDeploymentSettingsCmd(cmd.Context(), stack)
			if err != nil {
				return err
			}

			if d.Deployment == nil {
				return errors.New("Deployment file not initialized, please run `pulumi deployment settings init` instead")
			}

			option := d.Prompts.PromptUser(
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
				err = configureGit(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)
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
		&gitSSHPrivateKeyPath, "git-auth-ssh-private-key-path", "k", "",
		"Private key path")

	cmd.PersistentFlags().StringVar(
		&gitSSHPrivateKeyValue, "git-auth-ssh-private-key", "",
		"Git SSH private key")

	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")

	return cmd
}

func configureGit(d *deploymentSettingsCommandDependencies, gitSSHPrivateKeyPath string, gitSSHPrivateKeyValue string,
) error {
	sd := d.Deployment
	if sd.DeploymentSettings.SourceContext == nil {
		sd.DeploymentSettings.SourceContext = &apitype.SourceContext{}
	}
	if sd.DeploymentSettings.SourceContext.Git == nil {
		sd.DeploymentSettings.SourceContext.Git = &apitype.SourceContextGit{}
	}

	rl, err := newRepoLookup(d.WorkDir)
	if err != nil {
		return err
	}

	var defaultRepoDir string
	if sd.DeploymentSettings.SourceContext.Git.RepoDir != "" {
		// we convert the directory to use the native path separator to keep it consistent with the environment
		defaultRepoDir = filepath.FromSlash(sd.DeploymentSettings.SourceContext.Git.RepoDir)
	} else {
		defaultRepoDir, err = rl.GetRootDirectory(d.WorkDir)
		if err != nil {
			return err
		}
	}

	repoDir, err := d.Prompts.PromptForValue(!d.Interactive, "Repository directory",
		defaultRepoDir, false, ValidateRelativeDirectory(rl.GetRepoRoot()), *d.DisplayOptions)
	if err != nil {
		return err
	}

	// we have to convert non unix to use the unix path separator
	sd.DeploymentSettings.SourceContext.Git.RepoDir = filepath.ToSlash(repoDir)

	var branchName string
	if sd.DeploymentSettings.SourceContext.Git.Branch != "" {
		branchName = sd.DeploymentSettings.SourceContext.Git.Branch
	} else {
		branchName = rl.GetBranchName()
	}

	branchName, err = d.Prompts.PromptForValue(!d.Interactive, "Branch name",
		branchName, false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}
	sd.DeploymentSettings.SourceContext.Git.Branch = branchName

	remoteURL, err := rl.RemoteURL()
	if err != nil {
		return err
	}

	remoteURL, err = d.Prompts.PromptForValue(!d.Interactive, "Repository URL",
		remoteURL, false, ValidateGitURL, *d.DisplayOptions)
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
		useGitHub = d.Prompts.AskForConfirmation(
			"A GitHub repository was detected, do you want to use the Pulumi GitHub App?",
			d.DisplayOptions.Color, true, !d.Interactive)
	}

	if useGitHub {
		err := configureGitHubRepo(d, vcsInfo)
		if err != nil {
			return err
		}
	} else {
		err := configureBareGitRepo(d, remoteURL, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func configureGitHubRepo(d *deploymentSettingsCommandDependencies, vcsInfo *gitutil.VCSInfo) error {
	sd := d.Deployment

	if sd.DeploymentSettings.GitHub == nil {
		sd.DeploymentSettings.GitHub = &apitype.DeploymentSettingsGitHub{}
	}

	sd.DeploymentSettings.GitHub.Repository = fmt.Sprintf("%s/%s", vcsInfo.Owner, vcsInfo.Repo)

	var defaults []string

	if sd.DeploymentSettings.GitHub.PreviewPullRequests {
		defaults = append(defaults, optPreviewPr)
	}

	if sd.DeploymentSettings.GitHub.DeployCommits {
		defaults = append(defaults, optUpdatePushes)
	}

	if sd.DeploymentSettings.GitHub.PullRequestTemplate {
		defaults = append(defaults, optPrTemplate)
	}

	if len(defaults) == 0 {
		defaults = []string{
			optPreviewPr,
			optUpdatePushes,
		}
	}

	// For non interactive execution, it automatically accepts the default values
	options := d.Prompts.PromptUserMultiSkippable(
		!d.Interactive,
		"GitHub configuration",
		[]string{
			optPreviewPr,
			optUpdatePushes,
			optPrTemplate,
		},
		defaults,
		d.DisplayOptions.Color)

	sd.DeploymentSettings.GitHub.PreviewPullRequests = slices.Contains(options, optPreviewPr)
	sd.DeploymentSettings.GitHub.DeployCommits = slices.Contains(options, optUpdatePushes)
	sd.DeploymentSettings.GitHub.PullRequestTemplate = slices.Contains(options, optPrTemplate)

	return nil
}

func configureBareGitRepo(d *deploymentSettingsCommandDependencies,
	remoteURL string, gitSSHPrivateKeyPath string, gitSSHPrivateKeyValue string,
) error {
	sd := d.Deployment

	sd.DeploymentSettings.SourceContext.Git.RepoURL = remoteURL

	option := d.Prompts.PromptUser(
		"What kind of authentication does the repository use?",
		[]string{
			optUserPass,
			optSSH,
			optNoAuthentication,
		},
		optUserPass,
		d.DisplayOptions.Color)

	switch option {
	case optUserPass:
		return configureGitPassword(d)
	case optSSH:
		return configureGitSSH(d, gitSSHPrivateKeyPath, gitSSHPrivateKeyValue)
	case optNoAuthentication:
		sd.DeploymentSettings.SourceContext.Git.GitAuth = nil
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

	username, err = d.Prompts.PromptForValue(false, "Git username",
		username, false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	password, err = d.Prompts.PromptForValue(false, "Git password",
		password, true, ValidateShortInputNonEmpty, *d.DisplayOptions)
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

func configureGitSSH(
	d *deploymentSettingsCommandDependencies, gitSSHPrivateKeyPath string, gitSSHPrivateKeyValue string,
) error {
	if gitSSHPrivateKeyPath == "" && gitSSHPrivateKeyValue == "" {
		configureMsg := "No SSH private key was provided, run `pulumi deployment settings " +
			"configure` with the `--git-auth-ssh-private-key` or `--git-auth-ssh-private-key-path` flag set"
		configureMsg = colors.Highlight(configureMsg, "No SSH private key was provided", colors.SpecError+colors.Bold)
		configureMsg = colors.Highlight(configureMsg, "pulumi deployment settings configure", colors.BrightBlue+colors.Bold)
		configureMsg = colors.Highlight(configureMsg, "--git-auth-ssh-private-key", colors.BrightBlue+colors.Bold)
		configureMsg = colors.Highlight(configureMsg, "--git-auth-ssh-private-key-path", colors.BrightBlue+colors.Bold)
		fmt.Println()
		fmt.Println(d.DisplayOptions.Color.Colorize(configureMsg))
		fmt.Println()
		return nil
	}

	privateKey := gitSSHPrivateKeyValue
	if privateKey == "" {
		value, err := os.ReadFile(gitSSHPrivateKeyPath)
		if err != nil {
			return err
		}
		privateKey = string(value)
	}

	secret, err := d.Backend.EncryptStackDeploymentSettingsSecret(d.Ctx, d.Stack, privateKey)
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

	password, err = d.Prompts.PromptForValue(false, "(Optional) Private key password", password, true,
		ValidateShortInput, *d.DisplayOptions)
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

	roleARN, err := d.Prompts.PromptForValue(false, "AWS role ARN", sd.DeploymentSettings.Operation.OIDC.AWS.RoleARN,
		false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sessionName, err := d.Prompts.PromptForValue(false, "AWS session name",
		sd.DeploymentSettings.Operation.OIDC.AWS.SessionName, false,
		ValidateShortInputNonEmpty, *d.DisplayOptions)
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

	projectID, err := d.Prompts.PromptForValue(false, "GCP project id",
		sd.DeploymentSettings.Operation.OIDC.GCP.ProjectID, false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	providerID, err := d.Prompts.PromptForValue(false, "GCP provider id",
		sd.DeploymentSettings.Operation.OIDC.GCP.ProviderID, false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	workloadPoolID, err := d.Prompts.PromptForValue(false, "GCP identity provider id",
		sd.DeploymentSettings.Operation.OIDC.GCP.WorkloadPoolID, false,
		ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	serviceAccount, err := d.Prompts.PromptForValue(false, "GCP service account email address",
		sd.DeploymentSettings.Operation.OIDC.GCP.ServiceAccount, false,
		ValidateShortInputNonEmpty, *d.DisplayOptions)
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

	clientID, err := d.Prompts.PromptForValue(false, "Azure client ID",
		sd.DeploymentSettings.Operation.OIDC.Azure.ClientID, false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	tenantID, err := d.Prompts.PromptForValue(false, "Azure tenant ID",
		sd.DeploymentSettings.Operation.OIDC.Azure.TenantID, false, ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	subscriptionID, err := d.Prompts.PromptForValue(false, "Azure subscription ID",
		sd.DeploymentSettings.Operation.OIDC.Azure.SubscriptionID, false,
		ValidateShortInputNonEmpty, *d.DisplayOptions)
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

	var defaults []string

	if sd.DeploymentSettings.Operation == nil {
		sd.DeploymentSettings.Operation = &apitype.OperationContext{}
	}

	if sd.DeploymentSettings.Operation.Options == nil {
		sd.DeploymentSettings.Operation.Options = &apitype.OperationContextOptions{}
	}

	if sd.DeploymentSettings.Operation.Options.SkipInstallDependencies {
		defaults = append(defaults, optSkipDepsInstall)
	}

	if sd.DeploymentSettings.Operation.Options.SkipIntermediateDeployments {
		defaults = append(defaults, optSkipIntermediateDeployments)
	}

	if len(defaults) == 0 {
		defaults = []string{
			optSkipIntermediateDeployments,
		}
	}

	// For non interactive execution, it automatically accepts the default values
	options := d.Prompts.PromptUserMultiSkippable(
		!d.Interactive,
		"Advanced settings",
		[]string{
			optSkipIntermediateDeployments,
			optSkipDepsInstall,
		},
		defaults,
		d.DisplayOptions.Color)

	sd.DeploymentSettings.Operation.Options.
		SkipInstallDependencies = slices.Contains(options, optSkipDepsInstall)
	sd.DeploymentSettings.Operation.Options.
		SkipIntermediateDeployments = slices.Contains(options, optSkipIntermediateDeployments)

	return nil
}

func configureImageRepository(d *deploymentSettingsCommandDependencies) error {
	sd := d.Deployment

	// for non interactive runs, we default to false
	confirm := d.Prompts.AskForConfirmation("Do you want to use a custom executor image?",
		d.DisplayOptions.Color, false, !d.Interactive)

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

	imageReference, err := d.Prompts.PromptForValue(false, "Image reference",
		sd.DeploymentSettings.Executor.ExecutorImage.Reference, false,
		ValidateShortInputNonEmpty, *d.DisplayOptions)
	if err != nil {
		return err
	}

	username, err := d.Prompts.PromptForValue(false, "(Optional) Image repository username",
		sd.DeploymentSettings.Executor.ExecutorImage.Credentials.Username, false,
		ValidateShortInput, *d.DisplayOptions)
	if err != nil {
		return err
	}

	sd.DeploymentSettings.Executor.ExecutorImage = &apitype.DockerImage{
		Reference: imageReference,
	}

	if username == "" {
		return nil
	}

	password, err := d.Prompts.PromptForValue(false, "Image repository password", "", true,
		ValidateShortInputNonEmpty, *d.DisplayOptions)
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
