// Copyright 2026, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type deploymentSettingsEditClient interface {
	PatchStackDeploymentSettings(
		ctx context.Context, stack client.StackIdentifier, patch json.RawMessage,
	) error
	GetStackDeploymentSettings(
		ctx context.Context, stack client.StackIdentifier,
	) (*apitype.DeploymentSettings, error)
}

type deploymentSettingsEditClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentSettingsEditClient, client.StackIdentifier, error)

type deploymentSettingsEditArgs struct {
	stack        string
	outputFormat outputflag.OutputFlag[deploymentSettingsGetRenderFunc]

	// Source - repository and git URLs are mutually exclusive
	githubRepo  string
	repo        string
	vcsProvider string
	gitURL      string
	branch      string
	commit      string
	folder      string

	// Git authentication
	gitAuthToken                 string
	gitAuthSSHPrivateKey         string
	gitAuthSSHPrivateKeyPath     string
	gitAuthSSHPrivateKeyPassword string
	gitAuthUsername              string
	gitAuthPassword              string

	templateSourceURL string

	// VCS toggles
	previewPRs             bool
	pushToDeploy           bool
	prTemplate             bool
	pathFilters            []string
	clearPathFilters       bool
	deployTags             bool
	tagFilters             []string
	clearTagFilters        bool
	reviewStackLabels      []string
	clearReviewStackLabels bool
	installationID         string
	deployPullRequest      int64

	// Runner
	runnerPool       string
	executorImage    string
	executorRootPath string

	// Operation
	preRunCommands      []string
	clearPreRunCommands bool
	envVars             []string // each "KEY=VALUE"; plaintext.
	secretEnvVars       []string // each "KEY=VALUE"; encrypted before send.
	removeEnv           []string // each "KEY"; sent as null to delete.
	clearEnv            bool

	skipInstallDeps    bool
	skipIntermediate   bool
	shell              string
	deleteAfterDestroy bool
	remediateIfDrift   bool

	deploymentRoleID string
	cache            bool

	// OIDC - AWS
	oidcAWSRoleARN     string
	oidcAWSSessionName string
	oidcAWSDuration    string
	oidcAWSPolicyARNs  []string
	oidcAWSClear       bool

	// OIDC - Azure
	oidcAzureClientID       string
	oidcAzureTenantID       string
	oidcAzureSubscriptionID string
	oidcAzureClear          bool

	// OIDC - GCP
	oidcGCPProjectNumber  string
	oidcGCPWorkloadPoolID string
	oidcGCPProviderID     string
	oidcGCPServiceAccount string
	oidcGCPRegion         string
	oidcGCPTokenLifetime  string
	oidcGCPClear          bool

	flagsChanged func(name string) bool
}

const (
	flagGitHubRepo            = "github-repo"
	flagRepo                  = "repo"
	flagVCSProvider           = "vcs-provider"
	flagGitURL                = "git-url"
	flagBranch                = "branch"
	flagCommit                = "commit"
	flagFolder                = "folder"
	flagGitAuthToken          = "git-auth-token" //nolint:gosec // flag name, not a credential
	flagGitAuthSSHKey         = "git-auth-ssh-private-key"
	flagGitAuthSSHKeyPath     = "git-auth-ssh-private-key-path"
	flagGitAuthSSHKeyPassword = "git-auth-ssh-private-key-password" //nolint:gosec // flag name, not a credential
	flagGitAuthUsername       = "git-auth-username"
	flagGitAuthPassword       = "git-auth-password" //nolint:gosec // flag name, not a credential
	flagTemplateSourceURL     = "template-source-url"
	flagPreviewPRs            = "preview-prs"
	flagPushToDeploy          = "push-to-deploy"
	flagPRTemplate            = "pr-template"
	flagPathFilter            = "path-filter"
	flagClearPathFilters      = "clear-path-filters"
	flagDeployTags            = "deploy-tags"
	flagTagFilter             = "tag-filter"
	flagClearTagFilters       = "clear-tag-filters"
	flagReviewStackLabel      = "review-stack-label"
	flagClearReviewStackLabel = "clear-review-stack-labels"
	flagInstallationID        = "installation-id"
	flagDeployPullRequest     = "deploy-pull-request"
	flagRunnerPool            = "runner-pool"
	flagExecutorImage         = "executor-image"
	flagExecutorRootPath      = "executor-root-path"
	flagPreRunCommand         = "pre-run-command"
	flagClearPreRunCommands   = "clear-pre-run-commands"
	flagEnv                   = "env"
	flagSecretEnv             = "secret-env"
	flagRemoveEnv             = "remove-env"
	flagClearEnv              = "clear-env"
	flagSkipInstallDeps       = "skip-install-deps"
	flagSkipIntermediate      = "skip-intermediate-deployments"
	flagShell                 = "shell"
	flagDeleteAfterDestroy    = "delete-after-destroy"
	flagRemediateIfDrift      = "remediate-if-drift-detected"
	flagDeploymentRoleID      = "deployment-role-id"
	flagCache                 = "cache"

	flagOIDCAWSRoleARN     = "oidc-aws-role-arn"
	flagOIDCAWSSessionName = "oidc-aws-session-name"
	flagOIDCAWSDuration    = "oidc-aws-duration"
	flagOIDCAWSPolicyARN   = "oidc-aws-policy-arn"
	flagOIDCAWSClear       = "oidc-aws-clear"

	flagOIDCAzureClientID       = "oidc-azure-client-id"
	flagOIDCAzureTenantID       = "oidc-azure-tenant-id"
	flagOIDCAzureSubscriptionID = "oidc-azure-subscription-id"
	flagOIDCAzureClear          = "oidc-azure-clear"

	flagOIDCGCPProjectNumber  = "oidc-gcp-project-number"
	flagOIDCGCPWorkloadPoolID = "oidc-gcp-workload-pool-id" //nolint:gosec // flag name, not a credential
	flagOIDCGCPProviderID     = "oidc-gcp-provider-id"
	flagOIDCGCPServiceAccount = "oidc-gcp-service-account"
	flagOIDCGCPRegion         = "oidc-gcp-region"
	flagOIDCGCPTokenLifetime  = "oidc-gcp-token-lifetime" //nolint:gosec // flag name, not a credential
	flagOIDCGCPClear          = "oidc-gcp-clear"
)

func newDeploymentSettingsEditCmd() *cobra.Command {
	return newDeploymentSettingsEditCmdWith(defaultDeploymentSettingsEditClientFactory)
}

func newDeploymentSettingsEditCmdWith(factory deploymentSettingsEditClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "deploymentSettingsEditClientFactory must not be nil")
	var args deploymentSettingsEditArgs
	args.outputFormat = defaultDeploymentSettingsGetOutputFormat()

	cmd := &cobra.Command{
		Use:     "edit",
		Aliases: []string{"update", "modify"},
		Short:   "[EXPERIMENTAL] Create or update deployment settings for a stack",
		Long:    "[EXPERIMENTAL] Create or update deployment settings for a stack.",
		Example: "  # Switch the deployment source branch.\n" +
			"  pulumi deployment settings edit --branch feature-x\n\n" +
			"  # Configure a GitHub source.\n" +
			"  pulumi deployment settings edit \\\n" +
			"    --vcs-provider github --repo acme/infra --branch main --folder stacks/prod \\\n" +
			"    --preview-prs --push-to-deploy\n\n" +
			"  # Configure a GitLab source.\n" +
			"  pulumi deployment settings edit \\\n" +
			"    --vcs-provider gitlab --repo acme/infra --branch main --push-to-deploy\n\n" +
			"  # Authenticate against a private git repository with an access token.\n" +
			"  pulumi deployment settings edit \\\n" +
			"    --git-url https://git.acme.example/infra.git --git-auth-token \"$GIT_TOKEN\"\n\n" +
			"  # Authenticate against a private git repository with an SSH key.\n" +
			"  pulumi deployment settings edit \\\n" +
			"    --git-url git@git.acme.example:acme/infra.git \\\n" +
			"    --git-auth-ssh-private-key-path ~/.ssh/id_ed25519\n\n" +
			"  # Run deployments under a Pulumi Cloud role, whose id `pulumi org role ls` lists.\n" +
			"  pulumi deployment settings edit --deployment-role-id role-1a2b3c\n\n" +
			"  # Set environment variables (plaintext and encrypted).\n" +
			"  pulumi deployment settings edit --env LOG_LEVEL=info --secret-env API_KEY=s3cret\n\n" +
			"  # Remove an environment variable.\n" +
			"  pulumi deployment settings edit --remove-env STALE_VAR\n\n" +
			"  # Configure AWS OIDC.\n" +
			"  pulumi deployment settings edit \\\n" +
			"    --oidc-aws-role-arn arn:aws:iam::123:role/pulumi-deploy \\\n" +
			"    --oidc-aws-session-name pulumi-deploy --oidc-aws-duration 30m\n\n" +
			"  # Remove the AWS OIDC configuration entirely.\n" +
			"  pulumi deployment settings edit --oidc-aws-clear\n\n" +
			"  # Clear the agent pool back to the Pulumi-hosted default.\n" +
			"  pulumi deployment settings edit --runner-pool \"\"",
		RunE: func(cmd *cobra.Command, _ []string) error {
			args.flagsChanged = cmd.Flags().Changed
			return runDeploymentSettingsEdit(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	f := cmd.Flags()
	f.StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.VarP(f, &args.outputFormat)

	// Source
	f.StringVar(&args.githubRepo, flagGitHubRepo, "",
		"GitHub source: organization/repository")
	_ = f.MarkDeprecated(flagGitHubRepo,
		fmt.Sprintf("use --%s together with --%s github", flagRepo, flagVCSProvider))
	f.StringVar(&args.repo, flagRepo, "",
		"Version control source: repository reference, e.g. organization/repository "+
			"(mutually exclusive with --git-url)")
	f.StringVar(&args.vcsProvider, flagVCSProvider, "",
		"Version control provider: github, gitlab, azure_devops, bitbucket or custom")
	f.StringVar(&args.gitURL, flagGitURL, "",
		"Git source: full repository URL (mutually exclusive with --repo)")
	f.StringVar(&args.branch, flagBranch, "", "Source branch")
	f.StringVar(&args.commit, flagCommit, "", "Source commit hash")
	f.StringVar(&args.folder, flagFolder, "", "Path to the Pulumi.yaml folder within the source repo")
	f.BoolVar(&args.previewPRs, flagPreviewPRs, false, "Run previews for pull requests")
	f.BoolVar(&args.pushToDeploy, flagPushToDeploy, false,
		fmt.Sprintf("Run updates for pushed commits (cannot be enabled together with --%s)", flagDeployTags))
	f.BoolVar(&args.prTemplate, flagPRTemplate, false, "Use this stack as a template for PR review stacks")
	f.StringArrayVar(&args.pathFilters, flagPathFilter, nil,
		"Replace the path filter list (repeatable; pass once per filter)")
	f.BoolVar(&args.clearPathFilters, flagClearPathFilters, false, "Remove every path filter")
	f.BoolVar(&args.deployTags, flagDeployTags, false,
		fmt.Sprintf("Run updates for pushed tags (cannot be enabled together with --%s)", flagPushToDeploy))
	f.StringArrayVar(&args.tagFilters, flagTagFilter, nil,
		"Replace the tag filter list (repeatable; pass once per filter)")
	f.BoolVar(&args.clearTagFilters, flagClearTagFilters, false, "Remove every tag filter")
	f.StringArrayVar(&args.reviewStackLabels, flagReviewStackLabel, nil,
		"GitHub only: replace the labels that trigger a PR review stack (repeatable)")
	f.BoolVar(&args.clearReviewStackLabels, flagClearReviewStackLabel, false,
		"GitHub only: remove every PR review stack label")
	// No backquotes in a flag usage string: pflag reads the first backquoted span as the value
	// placeholder and strips it, so a quoted command name replaces the flag's type in the help.
	f.StringVar(&args.installationID, flagInstallationID, "",
		"Version control integration ID; only needed to choose between several integrations for "+
			"the same provider. List them with: pulumi api ListAllVCSIntegrations -F orgName=<org>")
	f.Int64Var(&args.deployPullRequest, flagDeployPullRequest, 0,
		fmt.Sprintf("Pull request number this review stack deploys; 0 clears it "+
			"(requires --%s, --%s and --%s to all be off)", flagPushToDeploy, flagPreviewPRs, flagPRTemplate))

	// Git authentication
	f.StringVar(&args.gitAuthToken, flagGitAuthToken, "",
		"Git source: personal access token; empty string removes the stored git credentials")
	f.StringVar(&args.gitAuthSSHPrivateKey, flagGitAuthSSHKey, "",
		"Git source: PEM-encoded SSH private key, key material and not a path; empty string removes "+
			"the stored git credentials")
	f.StringVar(&args.gitAuthSSHPrivateKeyPath, flagGitAuthSSHKeyPath, "",
		fmt.Sprintf("Git source: path to a PEM-encoded SSH private key file (mutually exclusive with --%s)",
			flagGitAuthSSHKey))
	f.StringVar(&args.gitAuthSSHPrivateKeyPassword, flagGitAuthSSHKeyPassword, "",
		"Git source: password for the SSH private key")
	f.StringVar(&args.gitAuthUsername, flagGitAuthUsername, "",
		"Git source: basic auth username; empty string removes the stored git credentials")
	f.StringVar(&args.gitAuthPassword, flagGitAuthPassword, "",
		"Git source: basic auth password")
	f.StringVar(&args.templateSourceURL, flagTemplateSourceURL, "",
		"Template source URL, e.g. registry://templates/source/acme/vpc; empty string clears it")

	// Runner
	f.StringVar(&args.runnerPool, flagRunnerPool, "",
		"Deployment runner pool ID; empty string clears it to the Pulumi-hosted pool")
	f.StringVar(&args.executorImage, flagExecutorImage, "",
		"Custom executor image; empty string clears it to the default image")
	f.StringVar(&args.executorRootPath, flagExecutorRootPath, "",
		"Executor root path; empty string clears it to the default (/)")

	// Operation
	f.StringArrayVar(&args.preRunCommands, flagPreRunCommand, nil,
		"Replace the pre-run command list (repeatable; pass once per command)")
	f.BoolVar(&args.clearPreRunCommands, flagClearPreRunCommands, false, "Remove every pre-run command")
	f.StringArrayVar(&args.envVars, flagEnv, nil,
		"Set a plaintext environment variable (repeatable, KEY=VALUE)")
	f.StringArrayVar(&args.secretEnvVars, flagSecretEnv, nil,
		"Set an encrypted environment variable (repeatable, KEY=VALUE)")
	f.StringSliceVar(&args.removeEnv, flagRemoveEnv, nil,
		"Delete an environment variable by key (repeatable or comma-separated)")
	f.BoolVar(&args.clearEnv, flagClearEnv, false, "Remove every environment variable")

	f.BoolVar(&args.skipInstallDeps, flagSkipInstallDeps, false,
		"Skip automatic dependency installation")
	f.BoolVar(&args.skipIntermediate, flagSkipIntermediate, false,
		"Skip intermediate deployments")
	f.StringVar(&args.shell, flagShell, "", "Shell to use for pre-run commands")
	f.BoolVar(&args.deleteAfterDestroy, flagDeleteAfterDestroy, false,
		"Delete the stack after a successful destroy")
	f.BoolVar(&args.remediateIfDrift, flagRemediateIfDrift, false,
		"Remediate the stack when a drift detection run finds drift")
	f.StringVar(&args.deploymentRoleID, flagDeploymentRoleID, "",
		"ID of the Pulumi Cloud role to run deployments under, as listed by pulumi org role ls; "+
			"empty string clears it back to stack-only access")
	f.BoolVar(&args.cache, flagCache, false, "Cache dependencies between deployments")

	// OIDC - AWS
	f.StringVar(&args.oidcAWSRoleARN, flagOIDCAWSRoleARN, "",
		"AWS OIDC: IAM role ARN to assume")
	f.StringVar(&args.oidcAWSSessionName, flagOIDCAWSSessionName, "",
		"AWS OIDC: assume-role session name")
	f.StringVar(&args.oidcAWSDuration, flagOIDCAWSDuration, "",
		"AWS OIDC: assume-role session duration (e.g. 30m, 1h)")
	f.StringSliceVar(&args.oidcAWSPolicyARNs, flagOIDCAWSPolicyARN, nil,
		"AWS OIDC: replace the session policy ARN list (repeatable or comma-separated)")
	f.BoolVar(&args.oidcAWSClear, flagOIDCAWSClear, false,
		"Remove the entire AWS OIDC configuration")

	// OIDC - Azure
	f.StringVar(&args.oidcAzureClientID, flagOIDCAzureClientID, "",
		"Azure OIDC: federated workload identity client ID")
	f.StringVar(&args.oidcAzureTenantID, flagOIDCAzureTenantID, "",
		"Azure OIDC: federated workload identity tenant ID")
	f.StringVar(&args.oidcAzureSubscriptionID, flagOIDCAzureSubscriptionID, "",
		"Azure OIDC: federated workload identity subscription ID")
	f.BoolVar(&args.oidcAzureClear, flagOIDCAzureClear, false,
		"Remove the entire Azure OIDC configuration")

	// OIDC - GCP
	f.StringVar(&args.oidcGCPProjectNumber, flagOIDCGCPProjectNumber, "",
		"GCP OIDC: numerical project number (e.g. 987654321)")
	f.StringVar(&args.oidcGCPWorkloadPoolID, flagOIDCGCPWorkloadPoolID, "",
		"GCP OIDC: workload identity pool ID")
	f.StringVar(&args.oidcGCPProviderID, flagOIDCGCPProviderID, "",
		"GCP OIDC: identity provider ID within the workload pool")
	f.StringVar(&args.oidcGCPServiceAccount, flagOIDCGCPServiceAccount, "",
		"GCP OIDC: service account email")
	f.StringVar(&args.oidcGCPRegion, flagOIDCGCPRegion, "",
		"GCP OIDC: region")
	f.StringVar(&args.oidcGCPTokenLifetime, flagOIDCGCPTokenLifetime, "",
		"GCP OIDC: lifetime of the temporary credentials (e.g. 30m, 1h)")
	f.BoolVar(&args.oidcGCPClear, flagOIDCGCPClear, false,
		"Remove the entire GCP OIDC configuration")

	cmd.MarkFlagsMutuallyExclusive(flagGitHubRepo, flagGitURL)
	cmd.MarkFlagsMutuallyExclusive(flagRepo, flagGitURL)
	cmd.MarkFlagsMutuallyExclusive(flagGitHubRepo, flagRepo)
	cmd.MarkFlagsMutuallyExclusive(flagBranch, flagCommit)
	cmd.MarkFlagsMutuallyExclusive(flagPathFilter, flagClearPathFilters)
	cmd.MarkFlagsMutuallyExclusive(flagTagFilter, flagClearTagFilters)
	cmd.MarkFlagsMutuallyExclusive(flagReviewStackLabel, flagClearReviewStackLabel)
	cmd.MarkFlagsMutuallyExclusive(flagPreRunCommand, flagClearPreRunCommands)
	cmd.MarkFlagsMutuallyExclusive(flagEnv, flagClearEnv)
	cmd.MarkFlagsMutuallyExclusive(flagSecretEnv, flagClearEnv)
	cmd.MarkFlagsMutuallyExclusive(flagRemoveEnv, flagClearEnv)
	cmd.MarkFlagsMutuallyExclusive(flagGitAuthToken, flagGitAuthSSHKey)
	cmd.MarkFlagsMutuallyExclusive(flagGitAuthToken, flagGitAuthSSHKeyPath)
	cmd.MarkFlagsMutuallyExclusive(flagGitAuthToken, flagGitAuthUsername)
	cmd.MarkFlagsMutuallyExclusive(flagGitAuthSSHKey, flagGitAuthSSHKeyPath)
	cmd.MarkFlagsMutuallyExclusive(flagGitAuthSSHKey, flagGitAuthUsername)
	cmd.MarkFlagsMutuallyExclusive(flagGitAuthSSHKeyPath, flagGitAuthUsername)

	return cmd
}

func defaultDeploymentSettingsEditClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentSettingsEditClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, cmdStack.LoadOnly, opts, "")
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("editing deployment settings requires the Pulumi Cloud backend; run `pulumi login`")
	}

	ref := cloudStack.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}
	stackID := client.StackIdentifier{
		Owner:   cloudStack.OrgName(),
		Project: project,
		Stack:   ref.Name(),
	}

	be, ok := cloudStack.Backend().(httpstate.Backend)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("editing deployment settings requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return be.Client(), stackID, nil
}

func runDeploymentSettingsEdit(
	ctx context.Context, w io.Writer,
	factory deploymentSettingsEditClientFactory, args deploymentSettingsEditArgs,
) error {
	if !anyEditFlagSet(args) {
		return errors.New("nothing to do: pass one of the edit flags (see --help)")
	}

	if err := validateEditArgs(args); err != nil {
		return err
	}

	// Resolved before the secrets are registered so the key material read from disk is filtered out
	// of the request dumps too.
	if err := resolveEditGitAuthSSHKey(&args); err != nil {
		return err
	}

	registerEditSecrets(args)

	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	// The service replaces the whole vcs object instead of merging it field by field, so a flag that
	// touches it has to be applied on top of the stored value rather than sent on its own.
	var stored *apitype.DeploymentSettings
	if anyVCSEditFlagSet(args) {
		stored, err = c.GetStackDeploymentSettings(ctx, stackID)
		switch {
		case isNotFound(err):
			// A stack with no deployment settings has nothing to read; the PATCH creates them.
			stored = nil
		case err != nil:
			return fmt.Errorf("reading deployment settings: %w", err)
		}
	}
	vcs, err := resolveEditVCS(args, stored)
	if err != nil {
		return err
	}

	// Secret env vars are sent in plaintext-secret wire form; the server encrypts them on PATCH.
	secretValues := buildSecretEnvVars(args.secretEnvVars)

	patch := buildEditFlagPatch(args, secretValues, vcs, stored)
	raw, err := marshalAndValidatePatch(patch)
	if err != nil {
		return fmt.Errorf("validating patch: %w", err)
	}

	if err := c.PatchStackDeploymentSettings(ctx, stackID, raw); err != nil {
		return fmt.Errorf("editing deployment settings: %w", err)
	}

	resp, err := c.GetStackDeploymentSettings(ctx, stackID)
	if err != nil {
		return fmt.Errorf("getting deployment settings: %w", err)
	}
	if resp == nil {
		resp = &apitype.DeploymentSettings{}
	}

	return args.outputFormat.Get()(w, *resp)
}

func isNotFound(err error) bool {
	errResp, ok := errors.AsType[*apitype.ErrorResponse](err)
	return ok && errResp.Code == http.StatusNotFound
}

// marshalAndValidatePatch turns the constructed map into bytes, then decodes those bytes into
// apitype.DeploymentSettings with DisallowUnknownFields for validation. Note that we don't pass a typed
// `apitype.DeploymentSettings` to the API client because we need to be able to send partial payloads, potentially will
// `null` fields, which we can't easily handle via Go's JSON struct tags.
func marshalAndValidatePatch(patch map[string]any) (json.RawMessage, error) {
	raw, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var probe apitype.DeploymentSettings
	if err := dec.Decode(&probe); err != nil {
		return nil, err
	}
	return raw, nil
}
