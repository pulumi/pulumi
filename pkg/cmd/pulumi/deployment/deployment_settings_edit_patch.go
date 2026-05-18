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
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
)

var editFlagNames = []string{
	flagGitHubRepo, flagGitURL, flagBranch, flagCommit, flagFolder,
	flagPreviewPRs, flagPushToDeploy, flagPRTemplate, flagPathFilter,
	flagRunnerPool, flagExecutorImage, flagExecutorRootPath,
	flagPreRunCommand, flagEnv, flagSecretEnv, flagRemoveEnv,
	flagSkipInstallDeps, flagSkipIntermediate, flagShell, flagDeleteAfterDestroy,
	flagOIDCAWSRoleARN, flagOIDCAWSSessionName, flagOIDCAWSDuration, flagOIDCAWSPolicyARN, flagOIDCAWSClear,
	flagOIDCAzureClientID, flagOIDCAzureTenantID, flagOIDCAzureSubscriptionID, flagOIDCAzureClear,
	flagOIDCGCPProjectNumber, flagOIDCGCPWorkloadPoolID, flagOIDCGCPProviderID,
	flagOIDCGCPServiceAccount, flagOIDCGCPRegion, flagOIDCGCPTokenLifetime, flagOIDCGCPClear,
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
	if args.flagsChanged != nil {
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
	}
	return nil
}

// encryptSecretEnvVars converts each "KEY=VALUE" --secret-env entry into a {"ciphertext": ...} JSON object by calling
// the cloud encrypt endpoint per value.
func encryptSecretEnvVars(
	ctx context.Context, c deploymentSettingsEditClient, stack client.StackIdentifier,
	specs []string,
) (map[string]map[string]any, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	out := map[string]map[string]any{}
	for _, spec := range specs {
		key, value, _ := strings.Cut(spec, "=")
		sv, err := c.EncryptStackDeploymentSettingsSecret(ctx, stack, value)
		if err != nil {
			return nil, fmt.Errorf("encrypting %q: %w", key, err)
		}
		if sv == nil || sv.Ciphertext == "" {
			return nil, fmt.Errorf("encrypting %q: server returned empty ciphertext", key)
		}
		out[key] = map[string]any{"ciphertext": sv.Ciphertext}
	}
	return out, nil
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
	if changed(flagBranch) {
		setNested(patch, []string{"sourceContext", "git", "branch"}, args.branch)
	}
	if changed(flagCommit) {
		setNested(patch, []string{"sourceContext", "git", "commit"}, args.commit)
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
		// Map empty string to null so `--executor-image ""` clears the field
		// back to the default image. Matches the --runner-pool convention.
		var v any = args.executorImage
		if args.executorImage == "" {
			v = nil
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
	if len(envEntries) > 0 {
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

	// OIDC — AWS
	if changed(flagOIDCAWSClear) && args.oidcAWSClear {
		setNested(patch, []string{"operationContext", "oidc", "aws"}, nil)
	}
	if changed(flagOIDCAWSRoleARN) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "roleArn"}, args.oidcAWSRoleARN)
	}
	if changed(flagOIDCAWSSessionName) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "sessionName"}, args.oidcAWSSessionName)
	}
	if changed(flagOIDCAWSDuration) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "duration"}, args.oidcAWSDuration)
	}
	if changed(flagOIDCAWSPolicyARN) {
		setNested(patch, []string{"operationContext", "oidc", "aws", "policyArns"}, args.oidcAWSPolicyARNs)
	}

	// OIDC — Azure
	if changed(flagOIDCAzureClear) && args.oidcAzureClear {
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
	if changed(flagOIDCGCPClear) && args.oidcGCPClear {
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
		setNested(patch, []string{"operationContext", "oidc", "gcp", "tokenLifetime"}, args.oidcGCPTokenLifetime)
	}

	return patch
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
