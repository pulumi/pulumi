package packageresolution

import packageresolution "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/packageresolution"

type PackageNotFoundError = packageresolution.PackageNotFoundError

type Options = packageresolution.Options

type Result = packageresolution.Result

type RegistryResult = packageresolution.RegistryResult

type LocalPathResult = packageresolution.LocalPathResult

type ExternalSourceResult = packageresolution.ExternalSourceResult

type InstalledInWorkspaceResult = packageresolution.InstalledInWorkspaceResult

type PluginWorkspace = packageresolution.PluginWorkspace

var ErrPackageNotFound = packageresolution.ErrPackageNotFound

var ErrRegistryQuery = packageresolution.ErrRegistryQuery

func Resolve(ctx context.Context, reg registry.Registry, ws PluginWorkspace, pluginSpec workspace.PluginSpec, options Options, projectRoot string) (Result, error) {
	return packageresolution.Resolve(ctx, reg, ws, pluginSpec, options, projectRoot)
}

func DefaultWorkspace() PluginWorkspace {
	return packageresolution.DefaultWorkspace()
}

