package engine

import engine "github.com/pulumi/pulumi/sdk/v3/pkg/engine"

// A PluginManager handles plugin installation.
type PluginManager = engine.PluginManager

// PluginSet represents a set of plugins.
type PluginSet = engine.PluginSet

// PackageSet represents a set of packages.
type PackageSet = engine.PackageSet

// A PackageUpdate represents an update from one version of a package to another.
type PackageUpdate = engine.PackageUpdate

// NewPluginSet creates a new PluginSet from the specified PluginSpecs.
func NewPluginSet(plugins ...workspace.PluginSpec) PluginSet {
	return engine.NewPluginSet(plugins...)
}

// NewPackageSet creates a new PackageSet from the specified PackageDescriptors.
func NewPackageSet(pkgs ...workspace.PackageDescriptor) PackageSet {
	return engine.NewPackageSet(pkgs...)
}

// GetRequiredPlugins lists a full set of plugins that will be required by the given program.
func GetRequiredPlugins(host plugin.Host, runtime string, info plugin.ProgramInfo) ([]workspace.PluginSpec, error) {
	return engine.GetRequiredPlugins(host, runtime, info)
}

// EnsurePluginsAreInstalled inspects all plugins in the plugin set and, if any plugins are not currently installed,
// uses the given backend client to install them. Installations are processed in parallel, though
// ensurePluginsAreInstalled does not return until all installations are completed.
func EnsurePluginsAreInstalled(ctx context.Context, opts *deploymentOptions, d diag.Sink, plugins PluginSet, projectPlugins []workspace.ProjectPlugin, reinstall, explicitInstall bool) error {
	return engine.EnsurePluginsAreInstalled(ctx, opts, d, plugins, projectPlugins, reinstall, explicitInstall)
}

