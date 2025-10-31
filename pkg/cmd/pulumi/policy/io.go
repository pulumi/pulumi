package policy

import policy "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/policy"

// ReadPolicyProject attempts to detect and read a Pulumi PolicyPack project for the current
// workspace. If the project is successfully detected and read, it is returned along with the path
// to its containing directory, which will be used as the root of the project's Pulumi program.
func ReadPolicyProject(pwd string) (*workspace.PolicyPackProject, string, string, error) {
	return policy.ReadPolicyProject(pwd)
}

func InstallPluginDependencies(ctx context.Context, root string, projRuntime workspace.ProjectRuntimeInfo) error {
	return policy.InstallPluginDependencies(ctx, root, projRuntime)
}

