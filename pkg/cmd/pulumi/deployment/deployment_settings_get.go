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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

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

type deploymentSettingsGetClient interface {
	GetStackDeploymentSettings(
		ctx context.Context, stack client.StackIdentifier,
	) (*apitype.DeploymentSettings, error)
}

type deploymentSettingsGetClientFactory func(
	ctx context.Context, stackFlag string,
) (deploymentSettingsGetClient, client.StackIdentifier, error)

type deploymentSettingsGetArgs struct {
	stack        string
	outputFormat outputflag.OutputFlag[deploymentSettingsGetRenderFunc]
}

func defaultDeploymentSettingsGetOutputFormat() outputflag.OutputFlag[deploymentSettingsGetRenderFunc] {
	return outputflag.OutputFlag[deploymentSettingsGetRenderFunc]{
		RenderForTerminal: renderDeploymentSettingsGetText,
		RenderJSON:        renderDeploymentSettingsGetJSON,
	}
}

func newDeploymentSettingsGetCmd() *cobra.Command {
	return newDeploymentSettingsGetCmdWith(defaultDeploymentSettingsGetClientFactory)
}

func newDeploymentSettingsGetCmdWith(factory deploymentSettingsGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "deploymentSettingsGetClientFactory must not be nil")
	var args deploymentSettingsGetArgs
	args.outputFormat = defaultDeploymentSettingsGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "get",
		Short: "[EXPERIMENTAL] Retrieve the deployment settings for a stack",
		Long:  "[EXPERIMENTAL] Retrieve the deployment settings for a stack.",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runDeploymentSettingsGet(cmd.Context(), cmd.OutOrStdout(), factory, args)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&args.stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

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

func runDeploymentSettingsGet(
	ctx context.Context, w io.Writer,
	factory deploymentSettingsGetClientFactory, args deploymentSettingsGetArgs,
) error {
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

	return args.outputFormat.Get()(w, *resp)
}

type deploymentSettingsGetRenderFunc func(w io.Writer, settings apitype.DeploymentSettings) error

// deploymentSettingsView is the shared shape for both text and JSON output.
// Secret material (env var values, git auth) is intentionally not surfaced.
type deploymentSettingsView struct {
	Tag                  string        `json:"tag,omitempty"`
	Source               *sourceView   `json:"source,omitempty"`
	Runner               *runnerView   `json:"runner,omitempty"`
	PreRunCommands       []string      `json:"preRunCommands,omitempty"`
	EnvironmentVariables []string      `json:"environmentVariables,omitempty"`
	OIDC                 *oidcView     `json:"oidc,omitempty"`
	Advanced             *advancedView `json:"advanced,omitempty"`
}

type sourceView struct {
	Kind                string   `json:"kind"`
	Repository          string   `json:"repository,omitempty"`
	Branch              string   `json:"branch,omitempty"`
	Commit              string   `json:"commit,omitempty"`
	Folder              string   `json:"folder,omitempty"`
	PreviewPullRequests *bool    `json:"previewPullRequests,omitempty"`
	RunUpdatesOnPush    *bool    `json:"runUpdatesOnPush,omitempty"`
	PullRequestTemplate *bool    `json:"pullRequestTemplate,omitempty"`
	PathFilters         []string `json:"pathFilters,omitempty"`
}

type runnerView struct {
	Pool             string `json:"pool,omitempty"`
	ExecutorImage    string `json:"executorImage,omitempty"`
	WorkingDirectory string `json:"workingDirectory,omitempty"`
}

type oidcView struct {
	AWS   *oidcAWSView   `json:"aws,omitempty"`
	Azure *oidcAzureView `json:"azure,omitempty"`
	GCP   *oidcGCPView   `json:"gcp,omitempty"`
}

type oidcAWSView struct {
	RoleARN         string   `json:"roleArn"`
	SessionName     string   `json:"sessionName,omitempty"`
	SessionDuration string   `json:"sessionDuration,omitempty"`
	PolicyARNs      []string `json:"policyArns,omitempty"`
}

type oidcAzureView struct {
	ClientID       string `json:"clientId,omitempty"`
	TenantID       string `json:"tenantId,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

type oidcGCPView struct {
	ProjectNumber  string `json:"projectNumber,omitempty"`
	WorkloadPool   string `json:"workloadPoolId,omitempty"`
	Provider       string `json:"providerId,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
	Region         string `json:"region,omitempty"`
	TokenLifetime  string `json:"tokenLifetime,omitempty"`
}

type advancedView struct {
	SkipInstallDependencies     bool   `json:"skipInstallDependencies,omitempty"`
	SkipIntermediateDeployments bool   `json:"skipIntermediateDeployments,omitempty"`
	Shell                       string `json:"shell,omitempty"`
	DeleteAfterDestroy          bool   `json:"deleteAfterDestroy,omitempty"`
}

func toDeploymentSettingsView(s apitype.DeploymentSettings) deploymentSettingsView {
	v := deploymentSettingsView{Tag: s.Tag}
	v.Source = buildSourceView(s)
	v.Runner = buildRunnerView(s)
	if s.Operation != nil {
		if len(s.Operation.PreRunCommands) > 0 {
			v.PreRunCommands = s.Operation.PreRunCommands
		}
		if len(s.Operation.EnvironmentVariables) > 0 {
			names := make([]string, 0, len(s.Operation.EnvironmentVariables))
			for k := range s.Operation.EnvironmentVariables {
				names = append(names, k)
			}
			sort.Strings(names)
			v.EnvironmentVariables = names
		}
		v.OIDC = buildOIDCView(s.Operation.OIDC)
		v.Advanced = buildAdvancedView(s.Operation.Options)
	}
	return v
}

func buildSourceView(s apitype.DeploymentSettings) *sourceView {
	hasGitHub := s.GitHub != nil && s.GitHub.Repository != ""
	var git *apitype.SourceContextGit
	if s.SourceContext != nil {
		git = s.SourceContext.Git
	}
	hasGit := git != nil && (git.RepoURL != "" || git.Branch != "" || git.Commit != "" || git.RepoDir != "")
	if !hasGitHub && !hasGit {
		return nil
	}
	out := &sourceView{}
	if hasGitHub {
		out.Kind = "github"
		out.Repository = s.GitHub.Repository
		preview := s.GitHub.PreviewPullRequests
		push := s.GitHub.DeployCommits
		template := s.GitHub.PullRequestTemplate
		out.PreviewPullRequests = &preview
		out.RunUpdatesOnPush = &push
		out.PullRequestTemplate = &template
		if len(s.GitHub.Paths) > 0 {
			out.PathFilters = s.GitHub.Paths
		}
	} else {
		out.Kind = "git"
		if git != nil {
			out.Repository = git.RepoURL
		}
	}
	if git != nil {
		out.Branch = git.Branch
		out.Commit = git.Commit
		out.Folder = git.RepoDir
	}
	return out
}

func buildRunnerView(s apitype.DeploymentSettings) *runnerView {
	out := &runnerView{}
	if s.AgentPoolID != nil {
		out.Pool = *s.AgentPoolID
	}
	if s.Executor != nil {
		if s.Executor.ExecutorImage != nil {
			out.ExecutorImage = s.Executor.ExecutorImage.Reference
		}
		out.WorkingDirectory = s.Executor.WorkingDirectory
	}
	if out.Pool == "" && out.ExecutorImage == "" && out.WorkingDirectory == "" {
		return nil
	}
	return out
}

func buildOIDCView(o *apitype.OperationContextOIDCConfiguration) *oidcView {
	if o == nil {
		return nil
	}
	out := &oidcView{}
	if o.AWS != nil {
		out.AWS = &oidcAWSView{
			RoleARN:         o.AWS.RoleARN,
			SessionName:     o.AWS.SessionName,
			SessionDuration: formatDuration(o.AWS.Duration),
			PolicyARNs:      o.AWS.PolicyARNs,
		}
	}
	if o.Azure != nil {
		out.Azure = &oidcAzureView{
			ClientID:       o.Azure.ClientID,
			TenantID:       o.Azure.TenantID,
			SubscriptionID: o.Azure.SubscriptionID,
		}
	}
	if o.GCP != nil {
		out.GCP = &oidcGCPView{
			ProjectNumber:  o.GCP.ProjectID,
			WorkloadPool:   o.GCP.WorkloadPoolID,
			Provider:       o.GCP.ProviderID,
			ServiceAccount: o.GCP.ServiceAccount,
			Region:         o.GCP.Region,
			TokenLifetime:  formatDuration(o.GCP.TokenLifetime),
		}
	}
	if out.AWS == nil && out.Azure == nil && out.GCP == nil {
		return nil
	}
	return out
}

func formatDuration(d apitype.DeploymentDuration) string {
	if d == 0 {
		return ""
	}
	return time.Duration(d).String()
}

func buildAdvancedView(o *apitype.OperationContextOptions) *advancedView {
	if o == nil {
		return nil
	}
	if !o.SkipInstallDependencies && !o.SkipIntermediateDeployments &&
		o.Shell == "" && !o.DeleteAfterDestroy {
		return nil
	}
	return &advancedView{
		SkipInstallDependencies:     o.SkipInstallDependencies,
		SkipIntermediateDeployments: o.SkipIntermediateDeployments,
		Shell:                       o.Shell,
		DeleteAfterDestroy:          o.DeleteAfterDestroy,
	}
}

// renderDeploymentSettingsGetText prints a sectioned summary.
// Empty sections are skipped entirely.
func renderDeploymentSettingsGetText(w io.Writer, s apitype.DeploymentSettings) error {
	v := toDeploymentSettingsView(s)

	// Value column is the same across all sections
	const valueColumn = 29
	kv := func(indent int, label, value string) {
		prefix := strings.Repeat(" ", indent) + label + ":"
		fmt.Fprintf(w, "%-*s %s\n", valueColumn-2, prefix, value)
	}
	first := true
	section := func(title string) {
		if !first {
			fmt.Fprintln(w)
		}
		first = false
		fmt.Fprintln(w, title)
	}
	yesno := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}

	if v.Tag != "" {
		kv(0, "Tag", v.Tag)
		first = false
	}

	if v.Source != nil {
		switch v.Source.Kind {
		case "github":
			section("Source: GitHub")
		case "git":
			section("Source: Git")
		default:
			section("Source")
		}
		if v.Source.Repository != "" {
			kv(2, "Repository", v.Source.Repository)
		}
		if v.Source.Branch != "" {
			kv(2, "Branch", v.Source.Branch)
		}
		if v.Source.Commit != "" {
			kv(2, "Commit", v.Source.Commit)
		}
		if v.Source.Folder != "" {
			kv(2, "Pulumi.yaml folder", v.Source.Folder)
		}
		if v.Source.PreviewPullRequests != nil {
			kv(2, "Run previews for PRs", yesno(*v.Source.PreviewPullRequests))
		}
		if v.Source.RunUpdatesOnPush != nil {
			kv(2, "Run updates on push", yesno(*v.Source.RunUpdatesOnPush))
		}
		if v.Source.PullRequestTemplate != nil {
			kv(2, "PR stack template", yesno(*v.Source.PullRequestTemplate))
		}
		if len(v.Source.PathFilters) > 0 {
			kv(2, "Path filters", strings.Join(v.Source.PathFilters, ", "))
		}
	}

	if v.Runner != nil {
		section("Deployment runner")
		if v.Runner.Pool != "" {
			kv(2, "Runner pool", v.Runner.Pool)
		}
		if v.Runner.ExecutorImage != "" {
			kv(2, "Executor image", v.Runner.ExecutorImage)
		}
		if v.Runner.WorkingDirectory != "" {
			kv(2, "Working directory", v.Runner.WorkingDirectory)
		}
	}

	if len(v.PreRunCommands) > 0 {
		section("Pre-run commands")
		for _, c := range v.PreRunCommands {
			fmt.Fprintf(w, "  %s\n", c)
		}
	}

	if len(v.EnvironmentVariables) > 0 {
		section("Environment variables")
		for _, n := range v.EnvironmentVariables {
			fmt.Fprintf(w, "  %s\n", n)
		}
	}

	if v.OIDC != nil {
		section("OIDC")
		if v.OIDC.AWS != nil {
			fmt.Fprintln(w, "  AWS")
			if v.OIDC.AWS.RoleARN != "" {
				kv(4, "Role ARN", v.OIDC.AWS.RoleARN)
			}
			if v.OIDC.AWS.SessionName != "" {
				kv(4, "Session name", v.OIDC.AWS.SessionName)
			}
			if v.OIDC.AWS.SessionDuration != "" {
				kv(4, "Session duration", v.OIDC.AWS.SessionDuration)
			}
			if len(v.OIDC.AWS.PolicyARNs) > 0 {
				kv(4, "Policy ARNs", strings.Join(v.OIDC.AWS.PolicyARNs, ", "))
			}
		}
		if v.OIDC.Azure != nil {
			fmt.Fprintln(w, "  Azure")
			if v.OIDC.Azure.ClientID != "" {
				kv(4, "Client ID", v.OIDC.Azure.ClientID)
			}
			if v.OIDC.Azure.TenantID != "" {
				kv(4, "Tenant ID", v.OIDC.Azure.TenantID)
			}
			if v.OIDC.Azure.SubscriptionID != "" {
				kv(4, "Subscription ID", v.OIDC.Azure.SubscriptionID)
			}
		}
		if v.OIDC.GCP != nil {
			fmt.Fprintln(w, "  GCP")
			if v.OIDC.GCP.ProjectNumber != "" {
				kv(4, "Project number", v.OIDC.GCP.ProjectNumber)
			}
			if v.OIDC.GCP.WorkloadPool != "" {
				kv(4, "Workload pool", v.OIDC.GCP.WorkloadPool)
			}
			if v.OIDC.GCP.Provider != "" {
				kv(4, "Provider", v.OIDC.GCP.Provider)
			}
			if v.OIDC.GCP.ServiceAccount != "" {
				kv(4, "Service account", v.OIDC.GCP.ServiceAccount)
			}
			if v.OIDC.GCP.Region != "" {
				kv(4, "Region", v.OIDC.GCP.Region)
			}
			if v.OIDC.GCP.TokenLifetime != "" {
				kv(4, "Token lifetime", v.OIDC.GCP.TokenLifetime)
			}
		}
	}

	if v.Advanced != nil {
		section("Advanced")
		if v.Advanced.SkipInstallDependencies {
			kv(2, "Skip install dependencies", "yes")
		}
		if v.Advanced.SkipIntermediateDeployments {
			kv(2, "Skip intermediate", "yes")
		}
		if v.Advanced.Shell != "" {
			kv(2, "Shell", v.Advanced.Shell)
		}
		if v.Advanced.DeleteAfterDestroy {
			kv(2, "Delete after destroy", "yes")
		}
	}

	return nil
}

func renderDeploymentSettingsGetJSON(w io.Writer, s apitype.DeploymentSettings) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toDeploymentSettingsView(s))
}
