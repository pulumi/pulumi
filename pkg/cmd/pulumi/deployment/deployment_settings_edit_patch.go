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
	"os"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

var editFlagNames = []string{
	flagGitHubRepo, flagRepo, flagVCSProvider, flagGitURL, flagBranch, flagCommit, flagFolder,
	flagGitAuthToken, flagGitAuthSSHKey, flagGitAuthSSHKeyPath, flagGitAuthSSHKeyPassword,
	flagGitAuthUsername, flagGitAuthPassword, flagClearGitAuth, flagTemplateSourceURL,
	flagPreviewPRs, flagPushToDeploy, flagPRTemplate, flagPathFilter,
	flagDeployTags, flagTagFilter, flagReviewStackLabel,
	flagInstallationID, flagDeployPullRequest,
	flagRunnerPool, flagExecutorImage, flagExecutorRootPath,
	flagPreRunCommand, flagEnv, flagSecretEnv, flagRemoveEnv, flagRemoveAllEnv,
	flagSkipInstallDeps, flagSkipIntermediate, flagShell, flagDeleteAfterDestroy, flagRemediateIfDrift,
	flagDeploymentRoleID, flagCache,
	flagOIDCAWSRoleARN, flagOIDCAWSSessionName, flagOIDCAWSDuration, flagOIDCAWSPolicyARN, flagRemoveOIDCAWS,
	flagOIDCAzureClientID, flagOIDCAzureTenantID, flagOIDCAzureSubscriptionID, flagRemoveOIDCAzure,
	flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID, flagOIDCGCPProviderID,
	flagOIDCGCPServiceAccount, flagOIDCGCPRegion, flagOIDCGCPTokenLifetime, flagRemoveOIDCGCP,
}

// vcsEditFlags write the vcs object rather than a deep-merged key path, so setting any of them
// forces a GET before the PATCH. The provider-neutral --branch / --commit / --folder / --git-url
// write sourceContext.git and are deliberately absent.
var vcsEditFlags = []string{
	flagGitHubRepo, flagRepo, flagVCSProvider,
	flagPreviewPRs, flagPushToDeploy, flagPRTemplate,
	flagPathFilter,
	flagDeployTags, flagTagFilter, flagReviewStackLabel,
	flagInstallationID, flagDeployPullRequest,
}

// presenceOnlyEditFlags reject an explicit false value rather than silently ignoring it.
var presenceOnlyEditFlags = []string{
	flagRemoveAllEnv, flagClearGitAuth,
	flagRemoveOIDCAWS, flagRemoveOIDCAzure, flagRemoveOIDCGCP,
}

// oidcProviderFlags lists the field-setter flags for each OIDC provider so
// validateEditArgs can refuse combining --oidc-<provider>-clear with any of them.
var oidcProviderFlags = map[string][]string{
	flagRemoveOIDCAWS: {
		flagOIDCAWSRoleARN, flagOIDCAWSSessionName, flagOIDCAWSDuration, flagOIDCAWSPolicyARN,
	},
	flagRemoveOIDCAzure: {
		flagOIDCAzureClientID, flagOIDCAzureTenantID, flagOIDCAzureSubscriptionID,
	},
	flagRemoveOIDCGCP: {
		flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID, flagOIDCGCPProviderID,
		flagOIDCGCPServiceAccount, flagOIDCGCPRegion, flagOIDCGCPTokenLifetime,
	},
}

// clearedByEmptyString applies the empty-string clear convention to a repeatable list flag. The
// service copies through every key a patch does not mention, so an empty list is a no-op where null
// is a clear.
func clearedByEmptyString(values []string) []string {
	if len(values) == 1 && values[0] == "" {
		return nil
	}
	return values
}

func validateListFlag(values []string, flag string) error {
	if len(values) > 1 && slices.Contains(values, "") {
		return fmt.Errorf("--%s \"\" clears the list and cannot be combined with other values", flag)
	}
	return nil
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

func presenceOnlyFlagValue(args deploymentSettingsEditArgs, flag string) bool {
	switch flag {
	case flagRemoveAllEnv:
		return args.removeAllEnv
	case flagClearGitAuth:
		return args.clearGitAuth
	case flagRemoveOIDCAWS:
		return args.oidcAWSClear
	case flagRemoveOIDCAzure:
		return args.oidcAzureClear
	case flagRemoveOIDCGCP:
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

	if changed(flagReviewStackLabel) && vcs.Provider != apitype.VCSProviderGitHub {
		return nil, fmt.Errorf("--%s is only supported on github sources, and this stack's source is %s",
			flagReviewStackLabel, vcs.Provider)
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
		vcs.Paths = clearedByEmptyString(args.pathFilters)
	}
	if changed(flagDeployTags) {
		vcs.DeployTags = args.deployTags
	}
	if changed(flagTagFilter) {
		vcs.TagFilters = clearedByEmptyString(args.tagFilters)
	}
	if changed(flagReviewStackLabel) {
		vcs.ReviewStackLabels = clearedByEmptyString(args.reviewStackLabels)
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
		case changed(flagPushToDeploy) && changed(flagDeployTags):
			return nil, fmt.Errorf("--%s and --%s are mutually exclusive", flagPushToDeploy, flagDeployTags)
		case changed(flagPushToDeploy):
			return nil, fmt.Errorf("this stack deploys on tags; pass --%s=false to deploy on commits instead",
				flagDeployTags)
		case changed(flagDeployTags):
			return nil, fmt.Errorf("this stack deploys on commits; pass --%s=false to deploy on tags instead",
				flagPushToDeploy)
		default:
			// Neither flag was passed, so this edit did not cause the conflict. The object still
			// cannot be sent, because it replaces the stored one wholesale.
			return nil, fmt.Errorf(
				"this stack stores both commit and tag triggers, which the service rejects; "+
					"pass --%s=false or --%s=false to resolve it",
				flagPushToDeploy, flagDeployTags)
		}
	}

	// The service discards deployPullRequest when any of the three standard triggers is on, so asking
	// for a number that would be discarded is refused. A stored number is left alone rather than
	// deleted: the stack is already in that state, so sending it back cannot be newly invalid, and
	// dropping it here would lose a setting the user never mentioned.
	if changed(flagDeployPullRequest) && args.deployPullRequest > 0 &&
		(vcs.DeployCommits || vcs.PreviewPullRequests || vcs.PullRequestTemplate) {
		return nil, fmt.Errorf(
			"--%s is only honored when --%s, --%s and --%s are all off; the service discards it otherwise",
			flagDeployPullRequest, flagPushToDeploy, flagPreviewPRs, flagPRTemplate)
	}

	// The vcs object replaces the stored one wholesale, so an empty repository here would erase the
	// stack's source rather than leave it alone.
	if vcs.Repository == "" {
		return nil, fmt.Errorf("a %s source needs a repository; pass --%s owner/name",
			vcs.Provider, flagRepo)
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
	for flag, values := range map[string][]string{
		flagPreRunCommand:    args.preRunCommands,
		flagPathFilter:       args.pathFilters,
		flagTagFilter:        args.tagFilters,
		flagReviewStackLabel: args.reviewStackLabels,
	} {
		if err := validateListFlag(values, flag); err != nil {
			return err
		}
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
	for _, clearFlag := range presenceOnlyEditFlags {
		if args.flagsChanged(clearFlag) && !presenceOnlyFlagValue(args, clearFlag) {
			return fmt.Errorf("--%s does not accept a false value; omit it to leave the setting alone", clearFlag)
		}
	}
	if args.flagsChanged(flagVCSProvider) {
		if _, err := parseVCSProvider(args.vcsProvider); err != nil {
			return err
		}
	}
	if args.flagsChanged(flagGitAuthSSHKeyPassword) && !gitAuthSSHKeyEdited(args) {
		return fmt.Errorf("--%s requires --%s or --%s",
			flagGitAuthSSHKeyPassword, flagGitAuthSSHKey, flagGitAuthSSHKeyPath)
	}
	// Setting a key without this flag already stores it without a passphrase, so an empty value is
	// an unset shell variable rather than a request to drop one.
	if args.flagsChanged(flagGitAuthSSHKeyPassword) && args.gitAuthSSHPrivateKeyPassword == "" {
		return fmt.Errorf("--%s must not be empty; omit it to store the key without a passphrase",
			flagGitAuthSSHKeyPassword)
	}
	// A credential flag given an empty value is an unset shell variable far more often than a
	// request to delete, and the stored value can never be read back, so clearing needs its own flag.
	for _, f := range []struct {
		name  string
		value string
	}{
		{flagGitAuthToken, args.gitAuthToken},
		{flagGitAuthSSHKey, args.gitAuthSSHPrivateKey},
		{flagGitAuthSSHKeyPath, args.gitAuthSSHPrivateKeyPath},
		{flagGitAuthUsername, args.gitAuthUsername},
		{flagGitAuthPassword, args.gitAuthPassword},
	} {
		if args.flagsChanged(f.name) && f.value == "" {
			return fmt.Errorf("--%s must not be empty; pass --%s to remove the stored credentials",
				f.name, flagClearGitAuth)
		}
	}
	if args.flagsChanged(flagGitAuthUsername) && !args.flagsChanged(flagGitAuthPassword) {
		return fmt.Errorf("--%s requires --%s", flagGitAuthUsername, flagGitAuthPassword)
	}
	if args.flagsChanged(flagGitAuthPassword) && !args.flagsChanged(flagGitAuthUsername) {
		return fmt.Errorf("--%s requires --%s", flagGitAuthPassword, flagGitAuthUsername)
	}
	if args.flagsChanged(flagDeployPullRequest) && args.deployPullRequest < 0 {
		return fmt.Errorf("--%s must not be negative; pass 0 to clear it", flagDeployPullRequest)
	}
	return nil
}

// gitAuthSSHKeyEdited reports whether the SSH key was given inline or as a path, which
// resolveEditGitAuthSSHKey has already folded into the one field.
func gitAuthSSHKeyEdited(args deploymentSettingsEditArgs) bool {
	return args.flagsChanged(flagGitAuthSSHKey) || args.flagsChanged(flagGitAuthSSHKeyPath)
}

// resolveEditGitAuthSSHKey reads the key file so the rest of the command has a single field to
// consult. A truncated key file would otherwise be sent as an empty key, wiping every stored
// authentication mode while reporting success.
func resolveEditGitAuthSSHKey(args *deploymentSettingsEditArgs) error {
	if !args.flagsChanged(flagGitAuthSSHKeyPath) {
		return nil
	}
	key, err := os.ReadFile(args.gitAuthSSHPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("reading SSH private key %q: %w", args.gitAuthSSHPrivateKeyPath, err)
	}
	if strings.TrimSpace(string(key)) == "" {
		return fmt.Errorf("SSH private key %q holds no key material; pass --%s to remove the "+
			"stored git credentials", args.gitAuthSSHPrivateKeyPath, flagClearGitAuth)
	}
	args.gitAuthSSHPrivateKey = string(key)
	return nil
}

// registerEditSecrets keeps the credentials the flags carry out of the request bodies the HTTP
// client dumps at high verbosity.
func registerEditSecrets(args deploymentSettingsEditArgs) {
	var secrets []string
	for _, v := range []string{
		args.gitAuthToken, args.gitAuthSSHPrivateKey, args.gitAuthSSHPrivateKeyPassword,
		args.gitAuthUsername, args.gitAuthPassword,
	} {
		if v != "" {
			secrets = append(secrets, v)
		}
	}
	for _, spec := range args.secretEnvVars {
		if _, value, ok := strings.Cut(spec, "="); ok && value != "" {
			secrets = append(secrets, value)
		}
	}
	if len(secrets) > 0 {
		logging.AddGlobalSecretFilter(secrets, "[secret]")
	}
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
		out[key] = secretWireValue(value)
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
	// Branch, commit and tag are mutually exclusive: the service validates the merged object and
	// rejects any pair. Setting one therefore has to null the others rather than leave them stored.
	// Tag is service-set on a tag-triggered deployment, so a stack can hold one without ever having
	// been given one from here.
	if changed(flagBranch) {
		setNested(patch, []string{"sourceContext", "git", "branch"}, nullIfEmpty(args.branch))
		if args.branch != "" {
			setNested(patch, []string{"sourceContext", "git", "commit"}, nil)
			setNested(patch, []string{"sourceContext", "git", "tag"}, nil)
		}
	}
	if changed(flagCommit) {
		setNested(patch, []string{"sourceContext", "git", "commit"}, nullIfEmpty(args.commit))
		if args.commit != "" {
			setNested(patch, []string{"sourceContext", "git", "branch"}, nil)
			setNested(patch, []string{"sourceContext", "git", "tag"}, nil)
		}
	}
	if changed(flagFolder) {
		setNested(patch, []string{"sourceContext", "git", "repoDir"}, args.folder)
	}
	if gitAuth, ok := buildGitAuthPatch(args, changed); ok {
		setNested(patch, []string{"sourceContext", "git", "gitAuth"}, gitAuth)
	}
	if changed(flagTemplateSourceURL) {
		// Clearing only the url would leave an empty template object behind, which then takes
		// precedence over the git source and fails the deployment for a missing source url.
		if args.templateSourceURL == "" {
			setNested(patch, []string{"sourceContext", "template"}, nil)
		} else {
			setNested(patch, []string{"sourceContext", "template", "sourceUrl"}, args.templateSourceURL)
		}
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

	if changed(flagPreRunCommand) {
		setNested(patch, []string{"operationContext", "preRunCommands"},
			clearedByEmptyString(args.preRunCommands))
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
	case changed(flagRemoveAllEnv):
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
	if changed(flagRemediateIfDrift) {
		setNested(patch, []string{"operationContext", "options", "remediateIfDriftDetected"}, args.remediateIfDrift)
	}
	if changed(flagDeploymentRoleID) {
		// Only a null role unsets the assignment: the service looks up a role object that carries an
		// empty id and rejects it as invalid.
		if args.deploymentRoleID == "" {
			setNested(patch, []string{"operationContext", "role"}, nil)
		} else {
			setNested(patch, []string{"operationContext", "role", "id"}, args.deploymentRoleID)
		}
	}
	if changed(flagCache) {
		setNested(patch, []string{"cacheOptions", "enable"}, args.cache)
	}

	// OIDC — AWS
	if changed(flagRemoveOIDCAWS) {
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

	// OIDC — Azure
	if changed(flagRemoveOIDCAzure) {
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

	// OIDC — GCP
	if changed(flagRemoveOIDCGCP) {
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

// buildGitAuthPatch nulls the two git auth modes that were not selected: the executor picks an ssh
// key over an access token over basic auth, so a stored mode left in place would win over the one
// the user just set.
func buildGitAuthPatch(args deploymentSettingsEditArgs, changed func(string) bool) (any, bool) {
	switch {
	case changed(flagClearGitAuth):
		return nil, true

	case changed(flagGitAuthToken):
		return map[string]any{
			"accessToken": secretWireValue(args.gitAuthToken),
			"sshAuth":     nil,
			"basicAuth":   nil,
		}, true

	case gitAuthSSHKeyEdited(args):
		// The password is always written so that rotating to an unprotected key does not leave the
		// previous key's passphrase bound to it.
		sshAuth := map[string]any{
			"sshPrivateKey": secretWireValue(args.gitAuthSSHPrivateKey),
			"password":      nil,
		}
		if changed(flagGitAuthSSHKeyPassword) {
			sshAuth["password"] = secretWireValue(args.gitAuthSSHPrivateKeyPassword)
		}
		return map[string]any{
			"sshAuth":     sshAuth,
			"accessToken": nil,
			"basicAuth":   nil,
		}, true

	case changed(flagGitAuthUsername):
		return map[string]any{
			"basicAuth": map[string]any{
				"userName": secretWireValue(args.gitAuthUsername),
				"password": secretWireValue(args.gitAuthPassword),
			},
			"accessToken": nil,
			"sshAuth":     nil,
		}, true
	}
	return nil, false
}

func secretWireValue(v string) map[string]any {
	return map[string]any{"secret": v}
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
