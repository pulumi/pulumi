package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

func Destroy(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (*deploy.Plan, display.ResourceChanges, error) {
	return engine.Destroy(u, ctx, opts, dryRun)
}

// DestroyV2 is a version of Destroy that uses the normal update source (i.e. it runs the user program) and
// runs the step generator in "destroy" mode. This allows it to get up-to-date configuration for provider
// resources.
func DestroyV2(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (*deploy.Plan, display.ResourceChanges, error) {
	return engine.DestroyV2(u, ctx, opts, dryRun)
}

