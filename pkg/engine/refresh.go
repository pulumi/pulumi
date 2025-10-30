package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

func Refresh(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (*deploy.Plan, display.ResourceChanges, error) {
	return engine.Refresh(u, ctx, opts, dryRun)
}

// RefreshV2 is a version of Refresh that uses the normal update source (i.e. it runs the user program) and
// runs the step generator in "refresh" mode. This allows it to get up-to-date configuration for provider
// resources.
func RefreshV2(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (*deploy.Plan, display.ResourceChanges, error) {
	return engine.RefreshV2(u, ctx, opts, dryRun)
}

