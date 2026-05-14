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

// AI Generated - needs human review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// deploymentSettingsGetClient is the narrow API surface this command depends
// on. Defined per-command so tests can stub it without implementing the full
// cloud client.
type deploymentSettingsGetClient interface {
	GetStackDeploymentSettings(
		ctx context.Context, stack client.StackIdentifier,
	) (*apitype.DeploymentSettings, error)
}

// deploymentSettingsGetClientFactory resolves a client and StackIdentifier
// for the get command. stackFlag carries the raw `--stack` value (empty means
// "use the current stack").
type deploymentSettingsGetClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentSettingsGetClient, client.StackIdentifier, error)

// deploymentSettingsGetArgs collects the resolved flag values so Run can be
// driven directly from tests.
type deploymentSettingsGetArgs struct {
	stack  string
	output string
}

// newDeploymentSettingsGetCmd builds `pulumi deployment settings get` wired to
// the real cloud client factory.
func newDeploymentSettingsGetCmd() *cobra.Command {
	return newDeploymentSettingsGetCmdWith(defaultDeploymentSettingsGetClientFactory)
}

func newDeploymentSettingsGetCmdWith(factory deploymentSettingsGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "deploymentSettingsGetClientFactory must not be nil")
	var args deploymentSettingsGetArgs

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "get",
		Short:  "[EXPERIMENTAL] Retrieve the deployment settings for a stack",
		Long: "[EXPERIMENTAL] Retrieve the deployment settings for a stack.\n" +
			"\n" +
			"Returns the saved Pulumi Deployments configuration for the selected stack,\n" +
			"including source context (git repository, branch, commit, working directory),\n" +
			"executor context (image), operation context (environment variable names and\n" +
			"pre-run commands), GitHub integration triggers, and the agent pool ID if set.\n" +
			"\n" +
			"Secret material (git credentials and environment variable values) is never\n" +
			"emitted by this command.\n" +
			"\n" +
			"Wraps the `GetDeploymentSettings` Pulumi Cloud REST endpoint. Default output\n" +
			"is a human-readable summary; pass --output=json for the raw response as JSON.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runDeploymentSettingsGet(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&args.output, "output", "o", "default",
		"Output format. One of: default, json")

	return cmd
}

// defaultDeploymentSettingsGetClientFactory mirrors the production wiring used
// by `pulumi deployment get`: resolve the stack, ensure we're on the Pulumi
// Cloud backend, and hand back the underlying *client.Client.
func defaultDeploymentSettingsGetClientFactory(
	ctx context.Context, stackFlag string,
) (deploymentSettingsGetClient, client.StackIdentifier, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stackFlag, cmdStack.LoadOnly, opts)
	if err != nil {
		return nil, client.StackIdentifier{}, fmt.Errorf("resolving stack: %w", err)
	}

	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, client.StackIdentifier{},
			errors.New("getting deployment settings requires the Pulumi Cloud backend; run `pulumi login`")
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
			errors.New("getting deployment settings requires the Pulumi Cloud backend; run `pulumi login`")
	}
	return be.Client(), stackID, nil
}

// runDeploymentSettingsGet is the cobra-decoupled entry point so tests can
// drive the command without parsing flags.
func runDeploymentSettingsGet(
	ctx context.Context, w io.Writer,
	factory deploymentSettingsGetClientFactory, args deploymentSettingsGetArgs,
) error {
	render, err := deploymentSettingsGetRenderer(args.output)
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, args.stack)
	if err != nil {
		return err
	}

	resp, err := c.GetStackDeploymentSettings(ctx, stackID)
	if err != nil {
		return fmt.Errorf("getting deployment settings: %w", err)
	}
	if resp == nil {
		resp = &apitype.DeploymentSettings{}
	}

	return render(w, *resp)
}

type deploymentSettingsGetRenderFunc func(w io.Writer, settings apitype.DeploymentSettings) error

func deploymentSettingsGetRenderer(format string) (deploymentSettingsGetRenderFunc, error) {
	switch format {
	case "", "default":
		return renderDeploymentSettingsGetText, nil
	case "json":
		return renderDeploymentSettingsGetJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q (must be 'default' or 'json')", format)
	}
}

// renderDeploymentSettingsGetText prints a human-readable summary as aligned
// key/value pairs. Secrets (git auth, environment variable values) are never
// emitted; only counts and keys.
func renderDeploymentSettingsGetText(w io.Writer, s apitype.DeploymentSettings) error {
	executorImage := "-"
	if s.Executor != nil && s.Executor.ExecutorImage != nil && s.Executor.ExecutorImage.Reference != "" {
		executorImage = s.Executor.ExecutorImage.Reference
	}
	workingDir := "-"
	if s.Executor != nil && s.Executor.WorkingDirectory != "" {
		workingDir = s.Executor.WorkingDirectory
	}

	repoURL, branch, commit, repoDir := "-", "-", "-", "-"
	if s.SourceContext != nil && s.SourceContext.Git != nil {
		git := s.SourceContext.Git
		if git.RepoURL != "" {
			repoURL = git.RepoURL
		}
		if git.Branch != "" {
			branch = git.Branch
		}
		if git.Commit != "" {
			commit = git.Commit
		}
		if git.RepoDir != "" {
			repoDir = git.RepoDir
		}
	}

	agentPool := "-"
	if s.AgentPoolID != nil && *s.AgentPoolID != "" {
		agentPool = *s.AgentPoolID
	}

	githubRepo := "-"
	deployCommits, previewPRs, prTemplate := false, false, false
	var githubPaths []string
	if s.GitHub != nil {
		if s.GitHub.Repository != "" {
			githubRepo = s.GitHub.Repository
		}
		deployCommits = s.GitHub.DeployCommits
		previewPRs = s.GitHub.PreviewPullRequests
		prTemplate = s.GitHub.PullRequestTemplate
		githubPaths = s.GitHub.Paths
	}

	envVarCount := 0
	preRunCount := 0
	if s.Operation != nil {
		envVarCount = len(s.Operation.EnvironmentVariables)
		preRunCount = len(s.Operation.PreRunCommands)
	}

	fmt.Fprintf(w, "%-24s %s\n", "Executor image:", executorImage)
	fmt.Fprintf(w, "%-24s %s\n", "Working directory:", workingDir)
	fmt.Fprintf(w, "%-24s %s\n", "Source repo URL:", repoURL)
	fmt.Fprintf(w, "%-24s %s\n", "Source branch:", branch)
	fmt.Fprintf(w, "%-24s %s\n", "Source commit:", commit)
	fmt.Fprintf(w, "%-24s %s\n", "Source repo dir:", repoDir)
	fmt.Fprintf(w, "%-24s %s\n", "GitHub repository:", githubRepo)
	fmt.Fprintf(w, "%-24s %t\n", "GitHub deploy commits:", deployCommits)
	fmt.Fprintf(w, "%-24s %t\n", "GitHub preview PRs:", previewPRs)
	fmt.Fprintf(w, "%-24s %t\n", "GitHub PR template:", prTemplate)
	fmt.Fprintf(w, "%-24s %d\n", "GitHub paths:", len(githubPaths))
	fmt.Fprintf(w, "%-24s %s\n", "Agent pool ID:", agentPool)
	fmt.Fprintf(w, "%-24s %d\n", "Env var keys:", envVarCount)
	fmt.Fprintf(w, "%-24s %d\n", "Pre-run commands:", preRunCount)
	return nil
}

// getDeploymentSettingsJSON is the JSON envelope for
// `pulumi deployment settings get --output=json`. It mirrors
// apitype.DeploymentSettings but normalizes nil slices to empty arrays so
// scripts can rely on the keys always being JSON arrays.
type getDeploymentSettingsJSON struct {
	Tag           string                              `json:"tag,omitempty"`
	Executor      *apitype.ExecutorContext            `json:"executorContext,omitempty"`
	SourceContext *apitype.SourceContext              `json:"sourceContext,omitempty"`
	GitHub        *getDeploymentSettingsGitHubJSON    `json:"gitHub,omitempty"`
	Operation     *getDeploymentSettingsOperationJSON `json:"operationContext,omitempty"`
	AgentPoolID   *string                             `json:"agentPoolID,omitempty"`
}

type getDeploymentSettingsGitHubJSON struct {
	Repository          string   `json:"repository,omitempty"`
	PullRequestTemplate bool     `json:"pullRequestTemplate"`
	DeployCommits       bool     `json:"deployCommits"`
	PreviewPullRequests bool     `json:"previewPullRequests"`
	DeployPullRequest   *int64   `json:"deployPullRequest,omitempty"`
	Paths               []string `json:"paths"`
}

type getDeploymentSettingsOperationJSON struct {
	OIDC                 *apitype.OperationContextOIDCConfiguration `json:"oidc,omitempty"`
	PreRunCommands       []string                                   `json:"preRunCommands"`
	Operation            apitype.PulumiOperation                    `json:"operation"`
	EnvironmentVariables map[string]apitype.SecretValue             `json:"environmentVariables"`
	Options              *apitype.OperationContextOptions           `json:"options,omitempty"`
}

func toGetDeploymentSettingsJSON(s apitype.DeploymentSettings) getDeploymentSettingsJSON {
	out := getDeploymentSettingsJSON{
		Tag:           s.Tag,
		Executor:      s.Executor,
		SourceContext: s.SourceContext,
		AgentPoolID:   s.AgentPoolID,
	}
	if s.GitHub != nil {
		paths := s.GitHub.Paths
		if paths == nil {
			paths = []string{}
		}
		out.GitHub = &getDeploymentSettingsGitHubJSON{
			Repository:          s.GitHub.Repository,
			PullRequestTemplate: s.GitHub.PullRequestTemplate,
			DeployCommits:       s.GitHub.DeployCommits,
			PreviewPullRequests: s.GitHub.PreviewPullRequests,
			DeployPullRequest:   s.GitHub.DeployPullRequest,
			Paths:               paths,
		}
	}
	if s.Operation != nil {
		preRun := s.Operation.PreRunCommands
		if preRun == nil {
			preRun = []string{}
		}
		envVars := s.Operation.EnvironmentVariables
		if envVars == nil {
			envVars = map[string]apitype.SecretValue{}
		}
		out.Operation = &getDeploymentSettingsOperationJSON{
			OIDC:                 s.Operation.OIDC,
			PreRunCommands:       preRun,
			Operation:            s.Operation.Operation,
			EnvironmentVariables: envVars,
			Options:              s.Operation.Options,
		}
	}
	return out
}

func renderDeploymentSettingsGetJSON(w io.Writer, s apitype.DeploymentSettings) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toGetDeploymentSettingsJSON(s))
}
