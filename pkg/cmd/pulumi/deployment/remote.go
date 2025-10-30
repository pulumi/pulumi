package deployment

import deployment "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/deployment"

// Flags for remote operations.
type RemoteArgs = deployment.RemoteArgs

// RemoteSupported returns true if the CLI supports remote deployments.
func RemoteSupported() bool {
	return deployment.RemoteSupported()
}

// ValidateUnsupportedRemoteFlags returns an error if any unsupported flags are set when --remote is set.
func ValidateUnsupportedRemoteFlags(expectNop bool, configArray []string, configPath bool, client string, jsonDisplay bool, policyPackPaths []string, policyPackConfigPaths []string, refresh string, showConfig bool, showPolicyRemediations bool, showReplacementSteps bool, showSames bool, showReads bool, suppressOutputs bool, secretsProvider string, targets *[]string, excludes *[]string, replaces []string, targetReplaces []string, targetDependents bool, planFilePath string, stackConfigFile string, runProgram bool) error {
	return deployment.ValidateUnsupportedRemoteFlags(expectNop, configArray, configPath, client, jsonDisplay, policyPackPaths, policyPackConfigPaths, refresh, showConfig, showPolicyRemediations, showReplacementSteps, showSames, showReads, suppressOutputs, secretsProvider, targets, excludes, replaces, targetReplaces, targetDependents, planFilePath, stackConfigFile, runProgram)
}

func ValidateRemoteDeploymentFlags(url string, args RemoteArgs) error {
	return deployment.ValidateRemoteDeploymentFlags(url, args)
}

// RunDeployment kicks off a remote deployment.
func RunDeployment(ctx context.Context, ws pkgWorkspace.Context, cmd *cobra.Command, opts display.Options, operation apitype.PulumiOperation, stack, url string, args RemoteArgs) error {
	return deployment.RunDeployment(ctx, ws, cmd, opts, operation, stack, url, args)
}

