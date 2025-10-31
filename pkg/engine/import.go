package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

func Import(u UpdateInfo, ctx *Context, opts UpdateOptions, imports []deploy.Import, dryRun bool) (*deploy.Plan, display.ResourceChanges, error) {
	return engine.Import(u, ctx, opts, imports, dryRun)
}

