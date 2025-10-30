package tracing

import tracing "github.com/pulumi/pulumi/sdk/v3/pkg/util/tracing"

// TracingOptions describes the set of options available for configuring tracing on a per-request basis.
type Options = tracing.Options

// ContextWithOptions returns a new context.Context with the indicated tracing options.
func ContextWithOptions(ctx context.Context, opts Options) context.Context {
	return tracing.ContextWithOptions(ctx, opts)
}

// OptionsFromContext retrieves any tracing options present in the given context. If no options are present,
// this function returns the zero value.
func OptionsFromContext(ctx context.Context) Options {
	return tracing.OptionsFromContext(ctx)
}

