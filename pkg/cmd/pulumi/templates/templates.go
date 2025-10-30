package templates

import templates "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/templates"

// Source provides access to a set of project templates, any set of which may be present on
// disk.
// 
// Source is responsible for cleaning up old templates, and should always be [Close]d when
// created.
type Source = templates.Source

type Template = templates.Template

// SearchScope dictates where [New] will search for templates.
type SearchScope = templates.SearchScope

var ScopeAll = templates.ScopeAll

var ScopeLocal = templates.ScopeLocal

// Create a new [Template] [Source] associated with a given [SearchScope].
func New(ctx context.Context, templateNamePathOrURL string, scope SearchScope, templateKind workspace.TemplateKind, e env.Env) *Source {
	return templates.New(ctx, templateNamePathOrURL, scope, templateKind, e)
}

