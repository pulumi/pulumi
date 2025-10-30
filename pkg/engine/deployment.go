package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// ProjectInfoContext returns information about the current project, including its pwd, main, and plugin context.
func ProjectInfoContext(projinfo *Projinfo, host plugin.Host, diag, statusDiag diag.Sink, debugging plugin.DebugContext, disableProviderPreview bool, tracingSpan opentracing.Span, config map[config.Key]string) (string, string, *plugin.Context, error) {
	return engine.ProjectInfoContext(projinfo, host, diag, statusDiag, debugging, disableProviderPreview, tracingSpan, config)
}

