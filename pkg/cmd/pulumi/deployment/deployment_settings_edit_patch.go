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
)

var editFlagNames = []string{
	flagGitHubRepo, flagGitURL, flagBranch, flagCommit, flagFolder,
	flagPreviewPRs, flagPushToDeploy, flagPRTemplate, flagPathFilter,
	flagRunnerPool, flagExecutorImage, flagExecutorRootPath,
	flagPreRunCommand, flagClearPreRunCommands, flagEnv, flagSecretEnv, flagRemoveEnv, flagClearEnv,
	flagSkipInstallDeps, flagSkipIntermediate, flagShell, flagDeleteAfterDestroy,
	flagOIDCAWSRoleARN, flagOIDCAWSSessionName, flagOIDCAWSDuration, flagOIDCAWSPolicyARN, flagOIDCAWSClear,
	flagOIDCAzureClientID, flagOIDCAzureTenantID, flagOIDCAzureSubscriptionID, flagOIDCAzureClear,
	flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID, flagOIDCGCPProviderID,
	flagOIDCGCPServiceAccount, flagOIDCGCPRegion, flagOIDCGCPTokenLifetime, flagOIDCGCPClear,
}

// clearEditFlags are presence-only: passing one with an explicit false value is rejected rather
// than silently ignored.
var clearEditFlags = []string{
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

func clearFlagValue(args deploymentSettingsEditArgs, flag string) bool {
	switch flag {
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
func buildEditFlagPatch(
	args deploymentSettingsEditArgs,
	secretEnv map[string]map[string]any,
) map[string]any {
	patch := map[string]any{}
	changed := args.flagsChanged
	if changed == nil {
		changed = func(string) bool { return false }
	}

	if changed(flagGitHubRepo) {
		setNested(patch, []string{"gitHub", "repository"}, args.githubRepo)
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
	if changed(flagPreviewPRs) {
		setNested(patch, []string{"gitHub", "previewPullRequests"}, args.previewPRs)
	}
	if changed(flagPushToDeploy) {
		setNested(patch, []string{"gitHub", "deployCommits"}, args.pushToDeploy)
	}
	if changed(flagPRTemplate) {
		setNested(patch, []string{"gitHub", "pullRequestTemplate"}, args.prTemplate)
	}
	if changed(flagPathFilter) {
		setNested(patch, []string{"gitHub", "paths"}, args.pathFilters)
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
