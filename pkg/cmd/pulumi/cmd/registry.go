package cmd

import cmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/cmd"

func NewDefaultRegistry(ctx context.Context, workspace pkgWorkspace.Context, project *workspace.Project, diag diag.Sink, env env.Env) registry.Registry {
	return cmd.NewDefaultRegistry(ctx, workspace, project, diag, env)
}

