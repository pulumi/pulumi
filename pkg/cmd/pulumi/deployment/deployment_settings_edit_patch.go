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

// Helpers that turn the parsed cobra flags into the JSON patch body sent to the Pulumi Cloud PATCH
// /deployments/settings endpoint. The endpoint applies a deep merge: keys present in the patch overwrite the stored
// value, a literal null deletes the key, and absent keys are preserved.

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

var editFlagNames = []string{
	flagGitHubRepo, flagRepo, flagVCSProvider, flagGitURL, flagBranch, flagCommit, flagFolder,
	flagPreviewPRs, flagPushToDeploy, flagPRTemplate, flagPathFilter, flagClearPathFilters,
	flagDeployTags, flagTagFilter, flagClearTagFilters, flagReviewStackLabel, flagClearReviewStackLabel,
	flagInstallationID, flagDeployPullRequest,
	flagRunnerPool, flagExecutorImage, flagExecutorRootPath,
	flagPreRunCommand, flagClearPreRunCommands, flagEnv, flagSecretEnv, flagRemoveEnv, flagClearEnv,
	flagSkipInstallDeps, flagSkipIntermediate, flagShell, flagDeleteAfterDestroy,
	flagOIDCAWSRoleARN, flagOIDCAWSSessionName, flagOIDCAWSDuration, flagOIDCAWSPolicyARN, flagOIDCAWSClear,
	flagOIDCAzureClientID, flagOIDCAzureTenantID, flagOIDCAzureSubscriptionID, flagOIDCAzureClear,
	flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID, flagOIDCGCPProviderID,
	flagOIDCGCPServiceAccount, flagOIDCGCPRegion, flagOIDCGCPTokenLifetime, flagOIDCGCPClear,
}

// vcsEditFlags write the vcs object rather than a deep-merged key path, so setting any of them
// forces a GET before the PATCH. The provider-neutral --branch / --commit / --folder / --git-url
// write sourceContext.git and are deliberately absent.
var vcsEditFlags = []string{
	flagGitHubRepo, flagRepo, flagVCSProvider,
	flagPreviewPRs, flagPushToDeploy, flagPRTemplate,
	flagPathFilter, flagClearPathFilters,
	flagDeployTags, flagTagFilter, flagClearTagFilters,
	flagReviewStackLabel, flagClearReviewStackLabel,
	flagInstallationID, flagDeployPullRequest,
}

// clearEditFlags are presence-only: passing one with an explicit false value is rejected rather
// than silently ignored.
var clearEditFlags = []string{
	flagClearPathFilters, flagClearTagFilters, flagClearReviewStackLabel,
	flagClearPreRunCommands, flagClearEnv,
	flagOIDCAWSClear, flagOIDCAzureClear, flagOIDCGCPClear,
}

// oidcProviderFlags lists the field-setter flags for each OIDC provider so
// validateEditArgs can refuse combining --oidc-<provider>-clear with any of them.
var oidcProviderFlags = map[string][]string{
	flagOIDCAWSClear: {
		flagOIDCAWSRoleARN, flagOIDCAWSSessionName, flagOIDCAWSDuration, flagOIDCAWSPolicyARN,
	},
	flagOIDCAzureClear: {
		flagOIDCAzureClientID, flagOIDCAzureTenantID, flagOIDCAzureSubscriptionID,
	},
	flagOIDCGCPClear: {
		flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID, flagOIDCGCPProviderID,
		flagOIDCGCPServiceAccount, flagOIDCGCPRegion, flagOIDCGCPTokenLifetime,
	},
}

func anyEditFlagSet(args deploymentSettingsEditArgs) bool {
	if args.flagsChanged == nil {
		return false
	}
	return slices.ContainsFunc(editFlagNames, args.flagsChanged)
}

func anyVCSEditFlagSet(args deploymentSettingsEditArgs) bool {
	if args.flagsChanged == nil {
		return false
	}
	return slices.ContainsFunc(vcsEditFlags, args.flagsChanged)
}

func clearFlagValue(args deploymentSettingsEditArgs, flag string) bool {
	switch flag {
	case flagClearPathFilters:
		return args.clearPathFilters
	case flagClearTagFilters:
		return args.clearTagFilters
	case flagClearReviewStackLabel:
		return args.clearReviewStackLabels
	case flagClearPreRunCommands:
		return args.clearPreRunCommands
	case flagClearEnv:
		return args.clearEnv
	case flagOIDCAWSClear:
		return args.oidcAWSClear
	case flagOIDCAzureClear:
		return args.oidcAzureClear
	case flagOIDCGCPClear:
		return args.oidcGCPClear
	}
	return false
}

func parseVCSProvider(s string) (apitype.VCSProvider, error) {
	known := []apitype.VCSProvider{
		apitype.VCSProviderGitHub, apitype.VCSProviderGitLab, apitype.VCSProviderAzureDevOps,
		apitype.VCSProviderBitbucket, apitype.VCSProviderCustom,
	}
	if slices.Contains(known, apitype.VCSProvider(s)) {
		return apitype.VCSProvider(s), nil
	}
	names := make([]string, len(known))
	for i, p := range known {
		names[i] = string(p)
	}
	return "", fmt.Errorf("--%s must be one of %s, got %q", flagVCSProvider, strings.Join(names, ", "), s)
}

// resolveEditVCS applies the flags that were set on top of the stored vcs object. The whole object
// is sent because the service replaces it wholesale, so a key left out of the patch is a key erased.
func resolveEditVCS(
	args deploymentSettingsEditArgs, stored *apitype.DeploymentSettings,
) (*apitype.DeploymentSettingsVCS, error) {
	if !anyVCSEditFlagSet(args) {
		return nil, nil
	}
	changed := args.flagsChanged

	var vcs apitype.DeploymentSettingsVCS
	switch {
	case stored != nil && stored.VCS != nil:
		vcs = *stored.VCS
	case stored != nil && stored.GitHub != nil:
		g := stored.GitHub
		vcs = apitype.DeploymentSettingsVCS{
			Provider:            apitype.VCSProviderGitHub,
			Repository:          g.Repository,
			DeployCommits:       g.DeployCommits,
			DeployTags:          g.DeployTags,
			Paths:               g.Paths,
			TagFilters:          g.TagFilters,
			InstallationID:      g.InstallationID,
			PullRequestTemplate: g.PullRequestTemplate,
			PreviewPullRequests: g.PreviewPullRequests,
			DeployPullRequest:   g.DeployPullRequest,
			ReviewStackLabels:   g.ReviewStackLabels,
		}
	}

	var requested apitype.VCSProvider
	switch {
	case changed(flagVCSProvider):
		p, err := parseVCSProvider(args.vcsProvider)
		if err != nil {
			return nil, err
		}
		requested = p
	case changed(flagGitHubRepo):
		requested = apitype.VCSProviderGitHub
	}

	switch {
	case vcs.Provider == "" && requested == "":
		return nil, fmt.Errorf(
			"this stack has no version control source configured; pass --%s to say which provider to configure",
			flagVCSProvider)
	case vcs.Provider == "":
		vcs.Provider = requested
	case requested != "" && requested != vcs.Provider:
		return nil, fmt.Errorf(
			"this stack's deployment source is %s, and %s would replace it; "+
				"change the provider in the Pulumi Cloud console instead",
			vcs.Provider, requestedProviderOrigin(args, requested))
	}

	for _, f := range []string{flagReviewStackLabel, flagClearReviewStackLabel} {
		if changed(f) && vcs.Provider != apitype.VCSProviderGitHub {
			return nil, fmt.Errorf("--%s is only supported on github sources, and this stack's source is %s",
				f, vcs.Provider)
		}
	}

	if changed(flagRepo) {
		vcs.Repository = args.repo
	}
	if changed(flagGitHubRepo) {
		vcs.Repository = args.githubRepo
	}
	if changed(flagPreviewPRs) {
		vcs.PreviewPullRequests = args.previewPRs
	}
	if changed(flagPushToDeploy) {
		vcs.DeployCommits = args.pushToDeploy
	}
	if changed(flagPRTemplate) {
		vcs.PullRequestTemplate = args.prTemplate
	}
	if changed(flagPathFilter) {
		vcs.Paths = args.pathFilters
	}
	if changed(flagClearPathFilters) {
		vcs.Paths = nil
	}
	if changed(flagDeployTags) {
		vcs.DeployTags = args.deployTags
	}
	if changed(flagTagFilter) {
		vcs.TagFilters = args.tagFilters
	}
	if changed(flagClearTagFilters) {
		vcs.TagFilters = nil
	}
	if changed(flagReviewStackLabel) {
		vcs.ReviewStackLabels = args.reviewStackLabels
	}
	if changed(flagClearReviewStackLabel) {
		vcs.ReviewStackLabels = nil
	}
	if changed(flagInstallationID) {
		vcs.InstallationID = args.installationID
	}
	if changed(flagDeployPullRequest) {
		vcs.DeployPullRequest = nil
		if args.deployPullRequest > 0 {
			pr := args.deployPullRequest
			vcs.DeployPullRequest = &pr
		}
	}

	// Both checks run against the merged object rather than the flags, so they also catch a flag that
	// conflicts with what the stack already stores. The messages name the stored setting in that case,
	// since naming a flag the user never passed sends them looking for it.
	if vcs.DeployCommits && vcs.DeployTags {
		switch {
		case changed(flagPushToDeploy) && !changed(flagDeployTags):
			return nil, fmt.Errorf("this stack deploys on tags; pass --%s=false to deploy on commits instead",
				flagDeployTags)
		case changed(flagDeployTags) && !changed(flagPushToDeploy):
			return nil, fmt.Errorf("this stack deploys on commits; pass --%s=false to deploy on tags instead",
				flagPushToDeploy)
		default:
			return nil, fmt.Errorf("--%s and --%s are mutually exclusive", flagPushToDeploy, flagDeployTags)
		}
	}

	// The service silently discards deployPullRequest when any of the three standard triggers is on.
	// Asking for a pull request number that would be discarded is refused; turning a trigger on for a
	// stack that merely stores one drops it, which is what the service does with it anyway.
	if vcs.DeployPullRequest != nil &&
		(vcs.DeployCommits || vcs.PreviewPullRequests || vcs.PullRequestTemplate) {
		if changed(flagDeployPullRequest) && args.deployPullRequest > 0 {
			return nil, fmt.Errorf(
				"--%s is only honored when --%s, --%s and --%s are all off; the service discards it otherwise",
				flagDeployPullRequest, flagPushToDeploy, flagPreviewPRs, flagPRTemplate)
		}
		vcs.DeployPullRequest = nil
	}

	return &vcs, nil
}

func requestedProviderOrigin(args deploymentSettingsEditArgs, requested apitype.VCSProvider) string {
	if args.flagsChanged(flagVCSProvider) {
		return fmt.Sprintf("--%s %s", flagVCSProvider, requested)
	}
	return fmt.Sprintf("--%s (which implies a github source)", flagGitHubRepo)
}

// validateEditArgs catches conflicts that cobra can't express on its own
// (e.g. setting and removing the same env var)
func validateEditArgs(args deploymentSettingsEditArgs) error {
	envKeys := map[string]string{}
	check := func(spec, flag string) error {
		key, _, ok := strings.Cut(spec, "=")
		if !ok {
			return fmt.Errorf("--%s expects KEY=VALUE, got %q", flag, spec)
		}
		if key == "" {
			return fmt.Errorf("--%s key must not be empty", flag)
		}
		if prev, dup := envKeys[key]; dup {
			return fmt.Errorf("--env / --secret-env / --remove-env set %q multiple times (previously via --%s)", key, prev)
		}
		envKeys[key] = flag
		return nil
	}
	for _, s := range args.envVars {
		if err := check(s, flagEnv); err != nil {
			return err
		}
	}
	for _, s := range args.secretEnvVars {
		if err := check(s, flagSecretEnv); err != nil {
			return err
		}
	}
	for _, k := range args.removeEnv {
		if k == "" {
			return fmt.Errorf("--%s key must not be empty", flagRemoveEnv)
		}
		if prev, dup := envKeys[k]; dup {
			return fmt.Errorf("--env / --secret-env / --remove-env set %q multiple times (previously via --%s)", k, prev)
		}
		envKeys[k] = flagRemoveEnv
	}
	if args.flagsChanged == nil {
		return nil
	}
	for clearFlag, fieldFlags := range oidcProviderFlags {
		if !args.flagsChanged(clearFlag) {
			continue
		}
		for _, f := range fieldFlags {
			if args.flagsChanged(f) {
				return fmt.Errorf("--%s cannot be combined with --%s", clearFlag, f)
			}
		}
	}
	for _, clearFlag := range clearEditFlags {
		if args.flagsChanged(clearFlag) && !clearFlagValue(args, clearFlag) {
			return fmt.Errorf("--%s does not accept a false value; omit it to leave the setting alone", clearFlag)
		}
	}
	if args.flagsChanged(flagVCSProvider) {
		if _, err := parseVCSProvider(args.vcsProvider); err != nil {
			return err
		}
	}
	if args.flagsChanged(flagDeployPullRequest) && args.deployPullRequest < 0 {
		return fmt.Errorf("--%s must not be negative; pass 0 to clear it", flagDeployPullRequest)
	}
	return nil
}

// buildSecretEnvVars converts each "KEY=VALUE" --secret-env entry into the plaintext-secret
// wire form ({"secret": VALUE}), which the server encrypts when handling the PATCH.
func buildSecretEnvVars(specs []string) map[string]map[string]any {
	if len(specs) == 0 {
		return nil
	}
	out := map[string]map[string]any{}
	for _, spec := range specs {
		key, value, _ := strings.Cut(spec, "=")
		out[key] = map[string]any{"secret": value}
	}
	return out
}

// buildEditFlagPatch turns the parsed flag values into a JSON-shaped map that mirrors apitype.DeploymentSettings.
// vcs, when non-nil, is the complete replacement object resolved by resolveEditVCS. The deprecated gitHub block is
// never written alongside it: the service gives vcs priority and clears gitHub when both are present.
func buildEditFlagPatch(
	args deploymentSettingsEditArgs,
	secretEnv map[string]map[string]any,
	vcs *apitype.DeploymentSettingsVCS,
) map[string]any {
	patch := map[string]any{}
	changed := args.flagsChanged
	if changed == nil {
		changed = func(string) bool { return false }
	}

	if vcs != nil {
		patch["vcs"] = vcs
	}
	if changed(flagGitURL) {
		setNested(patch, []string{"sourceContext", "git", "repoUrl"}, args.gitURL)
	}
	// A branch and a commit cannot both be set: the service validates the merged object and rejects
	// the pair. Setting either one therefore has to null the other rather than leave it stored.
	if changed(flagBranch) {
		setNested(patch, []string{"sourceContext", "git", "branch"}, nullIfEmpty(args.branch))
		if args.branch != "" {
			setNested(patch, []string{"sourceContext", "git", "commit"}, nil)
		}
	}
	if changed(flagCommit) {
		setNested(patch, []string{"sourceContext", "git", "commit"}, nullIfEmpty(args.commit))
		if args.commit != "" {
			setNested(patch, []string{"sourceContext", "git", "branch"}, nil)
		}
	}
	if changed(flagFolder) {
		setNested(patch, []string{"sourceContext", "git", "repoDir"}, args.folder)
	}

	if changed(flagRunnerPool) {
		// Map empty string back to null so the server clears the field. `--runner-pool ""` means "go back to the
		// Pulumi-hosted pool".
		var v any = args.runnerPool
		if args.runnerPool == "" {
			v = nil
		}
		patch["agentPoolID"] = v
	}
	if changed(flagExecutorImage) {
		// Only the keys the user set are emitted: a bare-string image decodes server-side to an
		// object whose credentials are explicitly null, which erases the stored registry
		// credentials. Empty string still clears the whole image, per the --runner-pool convention.
		var v any
		if args.executorImage != "" {
			v = map[string]any{"reference": args.executorImage}
		}
		setNested(patch, []string{"executorContext", "executorImage"}, v)
	}
	if changed(flagExecutorRootPath) {
		var v any = args.executorRootPath
		if args.executorRootPath == "" {
			v = nil
		}
		setNested(patch, []string{"executorContext", "executorRootPath"}, v)
	}

	switch {
	case changed(flagClearPreRunCommands):
		setNested(patch, []string{"operationContext", "preRunCommands"}, nil)
	case changed(flagPreRunCommand):
		setNested(patch, []string{"operationContext", "preRunCommands"}, args.preRunCommands)
	}

	envEntries := map[string]any{}
	for _, spec := range args.envVars {
		key, value, _ := strings.Cut(spec, "=")
		envEntries[key] = value
	}
	for key, wire := range secretEnv {
		envEntries[key] = wire
	}
	for _, key := range args.removeEnv {
		envEntries[key] = nil
	}
	switch {
	case changed(flagClearEnv):
		// An empty map would be a no-op: the server copies through every stored key the patch does
		// not mention.
		setNested(patch, []string{"operationContext", "environmentVariables"}, nil)
	case len(envEntries) > 0:
		setNested(patch, []string{"operationContext", "environmentVariables"}, envEntries)
	}

	if changed(flagSkipInstallDeps) {
		setNested(patch, []string{"operationContext", "options", "skipInstallDependencies"}, args.skipInstallDeps)
	}
	if changed(flagSkipIntermediate) {
		setNested(patch, []string{"operationContext", "options", "skipIntermediateDeployments"}, args.skipIntermediate)
	}
	if changed(flagShell) {
		setNested(patch, []string{"operationContext", "options", "shell"}, args.shell)
	}
	if changed(flagDeleteAfterDestroy) {
		setNested(patch, []string{"operationContext", "options", "deleteAfterDestroy"}, args.deleteAfterDestroy)
	}

	// OIDC - AWS
	if changed(flagOIDCAWSClear) {
		setNested(patch, []string{"operationContext", "oidc", "aws"}, nil)
	}
	if changed(flagOIDCAWSRoleARN) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "roleArn"}, args.oidcAWSRoleARN)
	}
	if changed(flagOIDCAWSSessionName) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "sessionName"}, args.oidcAWSSessionName)
	}
	if changed(flagOIDCAWSDuration) {
		var v any = args.oidcAWSDuration
		if args.oidcAWSDuration == "" {
			v = nil
		}
		setNested(patch, []string{"operationContext", "oidc", "aws", "duration"}, v)
	}
	if changed(flagOIDCAWSPolicyARN) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "policyArns"}, args.oidcAWSPolicyARNs)
	}

	// OIDC - Azure
	if changed(flagOIDCAzureClear) {
		setNested(patch, []string{"operationContext", "oidc", "azure"}, nil)
	}
	if changed(flagOIDCAzureClientID) {
		setNested(patch, []string{"operationContext", "oidc", "azure", "clientId"}, args.oidcAzureClientID)
	}
	if changed(flagOIDCAzureTenantID) {
		setNested(patch, []string{"operationContext", "oidc", "azure", "tenantId"}, args.oidcAzureTenantID)
	}
	if changed(flagOIDCAzureSubscriptionID) {
		setNested(patch, []string{"operationContext", "oidc", "azure", "subscriptionId"}, args.oidcAzureSubscriptionID)
	}

	// OIDC - GCP
	if changed(flagOIDCGCPClear) {
		setNested(patch, []string{"operationContext", "oidc", "gcp"}, nil)
	}
	if changed(flagOIDCGCPProjectNumber) {
		setNested(patch, []string{"operationContext", "oidc", "gcp", "projectId"}, args.oidcGCPProjectNumber)
	}
	if changed(flagOIDCGCPWorkloadPoolID) {
		setNested(patch, []string{"operationContext", "oidc", "gcp", "workloadPoolId"}, args.oidcGCPWorkloadPoolID)
	}
	if changed(flagOIDCGCPProviderID) {
		setNested(patch, []string{"operationContext", "oidc", "gcp", "providerId"}, args.oidcGCPProviderID)
	}
	if changed(flagOIDCGCPServiceAccount) {
		setNested(patch, []string{"operationContext", "oidc", "gcp", "serviceAccount"}, args.oidcGCPServiceAccount)
	}
	if changed(flagOIDCGCPRegion) {
		setNested(patch, []string{"operationContext", "oidc", "gcp", "region"}, args.oidcGCPRegion)
	}
	if changed(flagOIDCGCPTokenLifetime) {
		var v any = args.oidcGCPTokenLifetime
		if args.oidcGCPTokenLifetime == "" {
			v = nil
		}
		setNested(patch, []string{"operationContext", "oidc", "gcp", "tokenLifetime"}, v)
	}

	return patch
}

// nullIfEmpty maps an empty flag value to a JSON null so the server clears the stored field, since
// an empty string is a value the server would store as-is.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func setNested(m map[string]any, path []string, value any) {
	cur := m
	for i, k := range path {
		if i == len(path)-1 {
			cur[k] = value
			return
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[k] = next
		}
		cur = next
	}
}
