package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"

// Workspace encapsulates an environment containing an enumerable set of plugins.
type Workspace = convert.Workspace

// DefaultWorkspace returns a default workspace implementation that uses the workspace module directly to get plugin
// info.
func DefaultWorkspace() Workspace {
	return convert.DefaultWorkspace()
}

// NewBasePluginMapper creates a new plugin mapper backed by the supplied workspace.
func NewBasePluginMapper(ws Workspace, conversionKey string, providerFactory ProviderFactory, installPlugin func(string) *semver.Version, mappings []string) (Mapper, error) {
	return convert.NewBasePluginMapper(ws, conversionKey, providerFactory, installPlugin, mappings)
}

