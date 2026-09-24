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
	"strconv"
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
		stackFlag, cmdStack.LoadOnly, opts, "")
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
	switch {
	case isNotFound(err):
		// The factory has already resolved the stack, so a 404 here only means the stack has no
		// deployment settings rather than that the stack is missing.
		resp = nil
	case err != nil:
		return fmt.Errorf("getting deployment settings: %w", err)
	}
	if resp == nil {
		resp = &apitype.DeploymentSettings{}
	}

	return args.outputFormat.Get()(w, *resp)
}

type deploymentSettingsGetRenderFunc func(w io.Writer, settings apitype.DeploymentSettings) error

// deploymentSettingsView is the shared shape for both text and JSON output, and carries no secret
// material: an environment variable the service marks secret loses its value, and credentials
// survive only as an authentication mode or a presence flag.
type deploymentSettingsView struct {
	Tag                  string              `json:"tag,omitempty"`
	Version              int                 `json:"version,omitempty"`
	SettingsSource       string              `json:"settingsSource,omitempty"`
	DeploymentRole       *deploymentRoleView `json:"deploymentRole,omitempty"`
	CacheEnabled         *bool               `json:"cacheEnabled,omitempty"`
	Source               *sourceView         `json:"source,omitempty"`
	Runner               *runnerView         `json:"runner,omitempty"`
	PreRunCommands       []string            `json:"preRunCommands,omitempty"`
	EnvironmentVariables []envVarView        `json:"environmentVariables,omitempty"`
	OIDC                 *oidcView           `json:"oidc,omitempty"`
	Advanced             *advancedView       `json:"advanced,omitempty"`
}

// deploymentRoleView carries the id alongside the name because the id is what `edit` accepts, so
// dropping it would leave the rendered role unusable as an input.
type deploymentRoleView struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type envVarView struct {
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
	Secret bool   `json:"secret,omitempty"`
}

// Source kinds for a source that is not backed by a VCS integration. A sourceView.Kind that is not
// one of these is an apitype.VCSProvider value.
const (
	sourceKindGit      = "git"
	sourceKindHg       = "hg"
	sourceKindTemplate = "template"
)

type sourceView struct {
	Kind                     string   `json:"kind"`
	Repository               string   `json:"repository,omitempty"`
	InstallationID           string   `json:"installationId,omitempty"`
	Branch                   string   `json:"branch,omitempty"`
	Commit                   string   `json:"commit,omitempty"`
	Revision                 string   `json:"revision,omitempty"`
	GitTag                   string   `json:"gitTag,omitempty"`
	Folder                   string   `json:"folder,omitempty"`
	TemplateSourceURL        string   `json:"templateSourceUrl,omitempty"`
	ProjectTemplateSourceURL string   `json:"projectTemplateSourceUrl,omitempty"`
	Auth                     string   `json:"auth,omitempty"`
	PreviewPullRequests      *bool    `json:"previewPullRequests,omitempty"`
	RunUpdatesOnPush         *bool    `json:"runUpdatesOnPush,omitempty"`
	PullRequestTemplate      *bool    `json:"pullRequestTemplate,omitempty"`
	DeployPullRequest        *int64   `json:"deployPullRequest,omitempty"`
	DeployTags               bool     `json:"deployTags,omitempty"`
	TagFilters               []string `json:"tagFilters,omitempty"`
	PathFilters              []string `json:"pathFilters,omitempty"`
	ReviewStackLabels        []string `json:"reviewStackLabels,omitempty"`
}

type runnerView struct {
	Pool             string `json:"pool,omitempty"`
	ExecutorImage    string `json:"executorImage,omitempty"`
	DefaultImage     bool   `json:"defaultImage,omitempty"`
	ImageCredentials bool   `json:"imageCredentials,omitempty"`
	ExecutorRootPath string `json:"executorRootPath,omitempty"`
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
	RemediateIfDriftDetected    bool   `json:"remediateIfDriftDetected,omitempty"`
}

func toDeploymentSettingsView(s apitype.DeploymentSettings) deploymentSettingsView {
	v := deploymentSettingsView{Tag: s.Tag, Version: s.Version}
	if s.SettingsSource != nil {
		v.SettingsSource = string(*s.SettingsSource)
	}
	if s.CacheOptions != nil {
		enable := s.CacheOptions.Enable
		v.CacheEnabled = &enable
	}
	v.Source = buildSourceView(s)
	v.Runner = buildRunnerView(s)
	if s.Operation != nil {
		if len(s.Operation.PreRunCommands) > 0 {
			v.PreRunCommands = s.Operation.PreRunCommands
		}
		v.EnvironmentVariables = buildEnvVarViews(s.Operation.EnvironmentVariables)
		if s.Operation.Role != nil {
			v.DeploymentRole = &deploymentRoleView{ID: s.Operation.Role.ID, Name: s.Operation.Role.Name}
		}
		v.OIDC = buildOIDCView(s.Operation.OIDC)
		v.Advanced = buildAdvancedView(s.Operation.Options)
	}
	return v
}

func buildEnvVarViews(vars map[string]apitype.SecretValue) []envVarView {
	if len(vars) == 0 {
		return nil
	}
	names := make([]string, 0, len(vars))
	for k := range vars {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]envVarView, 0, len(names))
	for _, name := range names {
		v := vars[name]
		e := envVarView{Name: name, Secret: v.Secret}
		if !v.Secret {
			e.Value = v.Value
		}
		out = append(out, e)
	}
	return out
}

// deploymentRoleLabel renders a role as "name (id)". An assigned role reaches the CLI without a
// name only when the service fails to resolve it, so the id stands in rather than any wording that
// would imply no role is assigned.
func deploymentRoleLabel(r *deploymentRoleView) string {
	switch {
	case r == nil:
		return ""
	case r.Name == "":
		return r.ID
	case r.ID == "":
		return r.Name
	default:
		return fmt.Sprintf("%s (%s)", r.Name, r.ID)
	}
}

// refsHeadsPrefix is stripped for display only. The service prepends it on every enqueue and the
// executor normalizes again, so the CLI keeps writing bare branch names.
const refsHeadsPrefix = "refs/heads/"

func buildSourceView(s apitype.DeploymentSettings) *sourceView {
	var git *apitype.SourceContextGit
	var hg *apitype.SourceContextHg
	var template *apitype.SourceContextTemplate
	if s.SourceContext != nil {
		git, hg, template = s.SourceContext.Git, s.SourceContext.Hg, s.SourceContext.Template
	}
	hasGit := git != nil && (git.RepoURL != "" || git.Branch != "" || git.Commit != "" ||
		git.RepoDir != "" || git.Tag != "" || git.GitAuth != nil)
	hasHg := hg != nil && (hg.RepoURL != "" || hg.Branch != "" || hg.Revision != "" ||
		hg.RepoDir != "" || hg.HgAuth != nil)
	hasTemplate := template != nil && (template.SourceURL != "" || template.ProjectSourceURL != "")

	out := &sourceView{}
	switch {
	case s.VCS != nil:
		out.Kind = string(s.VCS.Provider)
		out.Repository = s.VCS.Repository
		out.InstallationID = s.VCS.InstallationID
		preview, push, prTemplate := s.VCS.PreviewPullRequests, s.VCS.DeployCommits, s.VCS.PullRequestTemplate
		out.PreviewPullRequests, out.RunUpdatesOnPush, out.PullRequestTemplate = &preview, &push, &prTemplate
		out.DeployPullRequest = s.VCS.DeployPullRequest
		out.DeployTags = s.VCS.DeployTags
		out.TagFilters = s.VCS.TagFilters
		out.PathFilters = s.VCS.Paths
		if s.VCS.Provider == apitype.VCSProviderGitHub {
			out.ReviewStackLabels = s.VCS.ReviewStackLabels
		}
	case s.GitHub != nil && s.GitHub.Repository != "":
		out.Kind = string(apitype.VCSProviderGitHub)
		out.Repository = s.GitHub.Repository
		out.InstallationID = s.GitHub.InstallationID
		preview, push, prTemplate := s.GitHub.PreviewPullRequests, s.GitHub.DeployCommits, s.GitHub.PullRequestTemplate
		out.PreviewPullRequests, out.RunUpdatesOnPush, out.PullRequestTemplate = &preview, &push, &prTemplate
		out.DeployPullRequest = s.GitHub.DeployPullRequest
		out.DeployTags = s.GitHub.DeployTags
		out.TagFilters = s.GitHub.TagFilters
		out.PathFilters = s.GitHub.Paths
		out.ReviewStackLabels = s.GitHub.ReviewStackLabels
	case hasGit:
		out.Kind = sourceKindGit
		out.Repository = git.RepoURL
	case hasHg:
		out.Kind = sourceKindHg
		out.Repository = hg.RepoURL
	case hasTemplate:
		out.Kind = sourceKindTemplate
		out.TemplateSourceURL = template.SourceURL
		if template.SourceURL == "" {
			out.ProjectTemplateSourceURL = template.ProjectSourceURL
		}
		out.Auth = gitAuthMode(template.GitAuth)
		return out
	default:
		return nil
	}

	switch {
	case hasGit:
		out.Branch = strings.TrimPrefix(git.Branch, refsHeadsPrefix)
		out.Commit = git.Commit
		out.GitTag = git.Tag
		out.Folder = git.RepoDir
		out.Auth = gitAuthMode(git.GitAuth)
	case hasHg:
		out.Branch = strings.TrimPrefix(hg.Branch, refsHeadsPrefix)
		out.Revision = hg.Revision
		out.Folder = hg.RepoDir
		out.Auth = gitAuthMode(hg.HgAuth)
	}
	return out
}

// gitAuthMode reports which credential the deployment will use, following the service's precedence
// when more than one is populated.
func gitAuthMode(a *apitype.GitAuthConfig) string {
	switch {
	case a == nil:
		return ""
	case a.SSHAuth != nil:
		return "SSH key"
	case a.PersonalAccessToken != nil:
		return "Access token"
	case a.BasicAuth != nil:
		return "Basic auth"
	default:
		return ""
	}
}

func sourceSectionTitle(kind string) string {
	switch kind {
	case string(apitype.VCSProviderGitHub):
		return "Source: GitHub"
	case string(apitype.VCSProviderGitLab):
		return "Source: GitLab"
	case string(apitype.VCSProviderAzureDevOps):
		return "Source: Azure DevOps"
	case string(apitype.VCSProviderBitbucket):
		return "Source: Bitbucket"
	case string(apitype.VCSProviderCustom):
		return "Source: Custom"
	case sourceKindGit:
		return "Source: Git"
	case sourceKindHg:
		return "Source: Mercurial"
	case sourceKindTemplate:
		return "Source: Template"
	default:
		return "Source"
	}
}

func buildRunnerView(s apitype.DeploymentSettings) *runnerView {
	out := &runnerView{}
	if s.AgentPoolID != nil {
		out.Pool = *s.AgentPoolID
	}
	if s.Executor != nil {
		if img := s.Executor.ExecutorImage; img != nil {
			out.ExecutorImage = img.Reference
			out.DefaultImage = img.IsDefault
			out.ImageCredentials = img.Credentials != nil
		}
		if s.Executor.ExecutorRootPath != nil {
			out.ExecutorRootPath = *s.Executor.ExecutorRootPath
		}
	}
	if out.Pool == "" && out.ExecutorImage == "" && out.ExecutorRootPath == "" &&
		!out.DefaultImage && !out.ImageCredentials {
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
		o.Shell == "" && !o.DeleteAfterDestroy && !o.RemediateIfDriftDetected {
		return nil
	}
	return &advancedView{
		SkipInstallDependencies:     o.SkipInstallDependencies,
		SkipIntermediateDeployments: o.SkipIntermediateDeployments,
		Shell:                       o.Shell,
		DeleteAfterDestroy:          o.DeleteAfterDestroy,
		RemediateIfDriftDetected:    o.RemediateIfDriftDetected,
	}
}

type settingsText struct {
	lines []settingsTextLine
}

type settingsTextLine struct {
	label string
	value string
}

func (t *settingsText) line(text string) {
	t.lines = append(t.lines, settingsTextLine{value: text})
}

func (t *settingsText) kv(indent int, label, value string) {
	t.lines = append(t.lines, settingsTextLine{
		label: strings.Repeat(" ", indent) + label + ":",
		value: value,
	})
}

func (t *settingsText) section(title string) {
	if len(t.lines) > 0 {
		t.line("")
	}
	t.line(title)
}

func (t *settingsText) empty() bool {
	return len(t.lines) == 0
}

// maxLabelWidth caps the column so one long environment variable name cannot indent every other
// value. It is the width output was padded to before, so nothing renders wider than it used to.
const maxLabelWidth = 32

func (t *settingsText) flush(w io.Writer) {
	width := 0
	for _, l := range t.lines {
		width = max(width, cmdutil.MeasureText(l.label))
	}
	width = min(width, maxLabelWidth)

	for _, l := range t.lines {
		if l.label == "" {
			fmt.Fprintln(w, l.value)
			continue
		}
		padding := strings.Repeat(" ", max(0, width-cmdutil.MeasureText(l.label)))
		fmt.Fprintf(w, "%s%s %s\n", l.label, padding, l.value)
	}
}

// renderDeploymentSettingsGetText prints a sectioned summary.
// Empty sections are skipped entirely.
func renderDeploymentSettingsGetText(w io.Writer, s apitype.DeploymentSettings) error {
	v := toDeploymentSettingsView(s)

	var out settingsText
	yesno := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}

	version := ""
	if v.Version != 0 {
		version = strconv.Itoa(v.Version)
	}
	cache := ""
	if v.CacheEnabled != nil {
		cache = "disabled"
		if *v.CacheEnabled {
			cache = "enabled"
		}
	}
	for _, entry := range []struct{ label, value string }{
		{"Tag", v.Tag},
		{"Version", version},
		{"Settings source", v.SettingsSource},
		{"Deployment role", deploymentRoleLabel(v.DeploymentRole)},
		{"Dependency cache", cache},
	} {
		if entry.value != "" {
			out.kv(0, entry.label, entry.value)
		}
	}

	if v.Source != nil {
		out.section(sourceSectionTitle(v.Source.Kind))
		if v.Source.Repository != "" {
			out.kv(2, "Repository", v.Source.Repository)
		}
		if v.Source.InstallationID != "" {
			out.kv(2, "Installation ID", v.Source.InstallationID)
		}
		if v.Source.Branch != "" {
			out.kv(2, "Branch", v.Source.Branch)
		}
		if v.Source.Commit != "" {
			out.kv(2, "Commit", v.Source.Commit)
		}
		if v.Source.Revision != "" {
			out.kv(2, "Revision", v.Source.Revision)
		}
		if v.Source.GitTag != "" {
			out.kv(2, "Git tag", v.Source.GitTag)
		}
		if v.Source.Folder != "" {
			out.kv(2, "Pulumi.yaml folder", v.Source.Folder)
		}
		if v.Source.TemplateSourceURL != "" {
			out.kv(2, "Template source", v.Source.TemplateSourceURL)
		}
		if v.Source.ProjectTemplateSourceURL != "" {
			out.kv(2, "Project template source", v.Source.ProjectTemplateSourceURL)
		}
		if v.Source.Auth != "" {
			out.kv(2, "Authentication", v.Source.Auth)
		}
		if v.Source.PreviewPullRequests != nil {
			out.kv(2, "Run previews for PRs", yesno(*v.Source.PreviewPullRequests))
		}
		if v.Source.RunUpdatesOnPush != nil {
			out.kv(2, "Run updates on push", yesno(*v.Source.RunUpdatesOnPush))
		}
		if v.Source.PullRequestTemplate != nil {
			out.kv(2, "PR stack template", yesno(*v.Source.PullRequestTemplate))
		}
		if v.Source.DeployPullRequest != nil {
			out.kv(2, "Deploy PR", strconv.FormatInt(*v.Source.DeployPullRequest, 10))
		}
		if v.Source.DeployTags {
			out.kv(2, "Deploy on tag", "yes")
		}
		if len(v.Source.TagFilters) > 0 {
			out.kv(2, "Tag filters", strings.Join(v.Source.TagFilters, ", "))
		}
		if len(v.Source.PathFilters) > 0 {
			out.kv(2, "Path filters", strings.Join(v.Source.PathFilters, ", "))
		}
		if len(v.Source.ReviewStackLabels) > 0 {
			out.kv(2, "Review stack labels", strings.Join(v.Source.ReviewStackLabels, ", "))
		}
	}

	if v.Runner != nil {
		out.section("Deployment runner")
		if v.Runner.Pool != "" {
			out.kv(2, "Runner pool", v.Runner.Pool)
		}
		if v.Runner.ExecutorImage != "" {
			out.kv(2, "Executor image", v.Runner.ExecutorImage)
		}
		if v.Runner.DefaultImage {
			out.kv(2, "Default image", "yes")
		}
		if v.Runner.ImageCredentials {
			out.kv(2, "Image credentials", "configured")
		}
		if v.Runner.ExecutorRootPath != "" {
			out.kv(2, "Executor root path", v.Runner.ExecutorRootPath)
		}
	}

	if len(v.PreRunCommands) > 0 {
		out.section("Pre-run commands")
		for _, c := range v.PreRunCommands {
			out.line("  " + c)
		}
	}

	if len(v.EnvironmentVariables) > 0 {
		out.section("Environment variables")
		for _, e := range v.EnvironmentVariables {
			value := e.Value
			if e.Secret {
				value = "[secret]"
			}
			out.kv(2, e.Name, value)
		}
	}

	if v.OIDC != nil {
		out.section("OIDC")
		if v.OIDC.AWS != nil {
			out.line("  AWS")
			if v.OIDC.AWS.RoleARN != "" {
				out.kv(4, "Role ARN", v.OIDC.AWS.RoleARN)
			}
			if v.OIDC.AWS.SessionName != "" {
				out.kv(4, "Session name", v.OIDC.AWS.SessionName)
			}
			if v.OIDC.AWS.SessionDuration != "" {
				out.kv(4, "Session duration", v.OIDC.AWS.SessionDuration)
			}
			if len(v.OIDC.AWS.PolicyARNs) > 0 {
				out.kv(4, "Policy ARNs", strings.Join(v.OIDC.AWS.PolicyARNs, ", "))
			}
		}
		if v.OIDC.Azure != nil {
			out.line("  Azure")
			if v.OIDC.Azure.ClientID != "" {
				out.kv(4, "Client ID", v.OIDC.Azure.ClientID)
			}
			if v.OIDC.Azure.TenantID != "" {
				out.kv(4, "Tenant ID", v.OIDC.Azure.TenantID)
			}
			if v.OIDC.Azure.SubscriptionID != "" {
				out.kv(4, "Subscription ID", v.OIDC.Azure.SubscriptionID)
			}
		}
		if v.OIDC.GCP != nil {
			out.line("  GCP")
			if v.OIDC.GCP.ProjectNumber != "" {
				out.kv(4, "Project number", v.OIDC.GCP.ProjectNumber)
			}
			if v.OIDC.GCP.WorkloadPool != "" {
				out.kv(4, "Workload pool", v.OIDC.GCP.WorkloadPool)
			}
			if v.OIDC.GCP.Provider != "" {
				out.kv(4, "Provider", v.OIDC.GCP.Provider)
			}
			if v.OIDC.GCP.ServiceAccount != "" {
				out.kv(4, "Service account", v.OIDC.GCP.ServiceAccount)
			}
			if v.OIDC.GCP.Region != "" {
				out.kv(4, "Region", v.OIDC.GCP.Region)
			}
			if v.OIDC.GCP.TokenLifetime != "" {
				out.kv(4, "Token lifetime", v.OIDC.GCP.TokenLifetime)
			}
		}
	}

	if v.Advanced != nil {
		out.section("Advanced")
		if v.Advanced.SkipInstallDependencies {
			out.kv(2, "Skip install dependencies", "yes")
		}
		if v.Advanced.SkipIntermediateDeployments {
			out.kv(2, "Skip intermediate deployments", "yes")
		}
		if v.Advanced.Shell != "" {
			out.kv(2, "Shell", v.Advanced.Shell)
		}
		if v.Advanced.DeleteAfterDestroy {
			out.kv(2, "Delete after destroy", "yes")
		}
		if v.Advanced.RemediateIfDriftDetected {
			out.kv(2, "Remediate on drift", "yes")
		}
	}
	if out.empty() {
		out.line("No deployment settings are configured for this stack.")
	}

	out.flush(w)
	return nil
}

func renderDeploymentSettingsGetJSON(w io.Writer, s apitype.DeploymentSettings) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toDeploymentSettingsView(s))
}
