package newcmd

import newcmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/newcmd"

// InstallDependencies will install dependencies for the project, e.g. by running `npm install` for nodejs projects.
func InstallDependencies(ctx *plugin.Context, runtime *workspace.ProjectRuntimeInfo, main string) error {
	return newcmd.InstallDependencies(ctx, runtime, main)
}

