// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package packageresolution provides functionality for resolving package sources
// when the source location is unknown beforehand. This is used throughout the
// CLI to determine where to fetch packages from (registry, local paths, or external sources).
//
// This differs from [registry.ResolvePackageFromName] which specifically queries
// the Pulumi registry. This package determines the resolution strategy first,
// then may delegate to registry functions, local file operations, or external
// source handling as appropriate.
package packageresolution

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrRegistryQuery   = errors.New("registry query error")
)

type PackageNotFoundError struct {
	Package     string
	Version     string
	OriginalErr error
}

func (e PackageNotFoundError) Error() string {
	if e.Version != "" {
		return fmt.Sprintf("package %s@%s not found", e.Package, e.Version)
	}
	return fmt.Sprintf("package %s not found", e.Package)
}

func (e PackageNotFoundError) Is(target error) bool {
	return target == ErrPackageNotFound
}

func (e PackageNotFoundError) Unwrap() error {
	return e.OriginalErr
}

func (e PackageNotFoundError) Suggestions() []apitype.PackageMetadata {
	return registry.GetSuggestedPackages(e)
}

type Options struct {
	DisableRegistryResolve bool
	Experimental           bool

	// If the resolution should use already installed plugins when resolving
	// plugins without specific versions provided.
	//
	// Concretely, when resolving a package like aws, if a version of aws is
	// already on disk, then that version will be preferred over latest.
	ResolveVersionWithLocalWorkspace bool

	// Resolve the source directly against the local workspace if possible.
	AllowNonInvertableLocalWorkspaceResolution bool
}

// The result of running [Resolve].
//
// Result will be one of 4 types:
//
// - [RegistryResult]: The package was resolved using Pulumi's Registry.
// - [LocalPathResult]: The package is local and already on disk.
// - [ExternalSourceResult]: The package is external, and should be downloaded normally.
type Result interface {
	isResult()
}

func (RegistryResult) isResult()       {}
func (LocalPathResult) isResult()      {}
func (ExternalSourceResult) isResult() {}

type (
	// The package should be downloaded with information from the registry.
	RegistryResult struct {
		// The full metadata from the registry lookup
		Metadata apitype.PackageMetadata
		// The package descriptor that corresponds with Metadata
		Pkg workspace.PackageDescriptor

		// If the package is already installed in the workspace.
		InstalledInWorkspace bool
	}
	// The package is referenced by a local path.
	//
	// For example:
	//
	//	/a/nice/absolute/path/to/pulumi-resource-example
	LocalPathResult struct {
		// The path to the plugin on disk.
		LocalPath string
	}
	ExternalSourceResult struct {
		Spec workspace.UnresolvedPackageDescriptor

		// If the package is already installed in the workspace.
		InstalledInWorkspace bool
	}
)

func naivePackageDescriptor(
	ctx context.Context, spec workspace.PackageSpec,
) (workspace.UnresolvedPackageDescriptor, error) {
	pluginSpecSource := spec.Source
	var version *semver.Version
	if spec.Version != "" {
		if v, err := semver.ParseTolerant(spec.Version); err != nil {
			pluginSpecSource += "@" + spec.Version
		} else {
			version = &v
		}
	}
	pluginDesc, err := workspace.NewPluginDescriptor(ctx, pluginSpecSource, apitype.ResourcePlugin,
		version, spec.PluginDownloadURL, spec.Checksums)
	return workspace.UnresolvedPackageDescriptor{
		PluginDescriptor:     pluginDesc,
		ParameterizationArgs: spec.Parameters,
	}, err
}

func Resolve(
	ctx context.Context,
	reg registry.Registry,
	ws PluginWorkspace,
	spec workspace.PackageSpec,
	options Options,
) (Result, error) {
	logging.V(3).Infof("Resolving package from %#v\n", spec)
	if plugin.IsLocalPluginPath(ctx, spec.Source) {
		return LocalPathResult{
			LocalPath: spec.Source,
		}, nil
	}

	naivePackageDescriptor, err := naivePackageDescriptor(ctx, spec)
	if err != nil {
		return nil, err
	}

	if options.AllowNonInvertableLocalWorkspaceResolution {
		if ws.HasPlugin(naivePackageDescriptor.PluginDescriptor) {
			return ExternalSourceResult{Spec: naivePackageDescriptor, InstalledInWorkspace: true}, nil
		}

		if naivePackageDescriptor.Version == nil {
			has, version, err := ws.HasPluginGTE(naivePackageDescriptor.PluginDescriptor)
			if err != nil {
				return nil, err
			}
			if has {
				naivePackageDescriptor.Version = version
				return ExternalSourceResult{Spec: naivePackageDescriptor, InstalledInWorkspace: true}, nil
			}
		}
	}

	if ws.IsExternalURL(spec.Source) || naivePackageDescriptor.IsGitPlugin() {
		logging.V(3).Infof("Resolved package %#v to an external source %#v\n",
			spec, naivePackageDescriptor)
		// If we have the exact version installed, then use that
		if ws.HasPlugin(naivePackageDescriptor.PluginDescriptor) {
			return ExternalSourceResult{Spec: naivePackageDescriptor, InstalledInWorkspace: true}, nil
		}
		// If we don't have a version specified and we are referencing the local workspace
		if naivePackageDescriptor.Version == nil && options.ResolveVersionWithLocalWorkspace {
			has, version, err := ws.HasPluginGTE(naivePackageDescriptor.PluginDescriptor)
			if err != nil {
				return nil, err
			}
			if has {
				naivePackageDescriptor.Version = version
				return ExternalSourceResult{Spec: naivePackageDescriptor, InstalledInWorkspace: true}, nil
			}
		}

		// We still don't have a version, so let's look up the latest version.
		if naivePackageDescriptor.Version == nil {
			naivePackageDescriptor.Version, err = ws.GetLatestVersion(ctx, naivePackageDescriptor.PluginDescriptor)
			if err != nil && !errors.Is(err, workspace.ErrGetLatestVersionNotSupported) {
				return nil, fmt.Errorf("unable to get the latest version of %q: %w",
					naivePackageDescriptor.Name, err)
			}
		}

		// At this point, we either have a version or aren't going to have one
		return ExternalSourceResult{Spec: naivePackageDescriptor}, nil
	}

	var registryNotFoundErr error
	var registryQueryErr error

	if options.includeRegistryResolve() {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, spec.Source, naivePackageDescriptor.Version)
		if err == nil {
			pkgDescriptor := pkgDescriptorFromMetadata(metadata, spec.Checksums)

			// Now that we've resolved to a version, we need to check if we have a good-enough version
			// already installed.

			// If the version was specified in the request, then the only good-enough version is the correct version
			if naivePackageDescriptor.Version != nil {
				installed := ws.HasPlugin(pkgDescriptor.PluginDescriptor)
				logging.V(3).Infof("Resolved package %#v via the registry to %#v (installedInWorkspace=%t)\n",
					spec.Source, pkgDescriptor, installed)
				return RegistryResult{
					Metadata: metadata, Pkg: pkgDescriptor,
					InstalledInWorkspace: installed,
				}, nil
			}

			// If the version wasn't specified in the request, then good enough is any plugin with the same
			// *major version* as what the registry gave us.
			has, version, err := ws.HasPluginGTE(func(s workspace.PluginDescriptor) workspace.PluginDescriptor {
				s.Version = &semver.Version{Major: s.Version.Major}
				return s
			}(pkgDescriptor.PluginDescriptor))
			if err != nil {
				return nil, err
			}

			// There is no local version of this plugin that meets our version requirements, so we just
			// request the latest.
			if !has || version == nil {
				logging.V(3).Infof("Resolved package %#v via the registry to %#v (installedInWorkspace=%t)\n",
					spec.Source, pkgDescriptor, has)
				return RegistryResult{Metadata: metadata, Pkg: pkgDescriptor, InstalledInWorkspace: has}, nil
			}

			// We have a version that's already installed at the right major version, so we should use
			// that... if it's valid in the registry. We need to check.

			newMetadata, err := registry.ResolvePackageFromName(
				ctx, reg, path.Join(metadata.Source, metadata.Publisher, metadata.Name), version)
			if errors.Is(err, registry.ErrNotFound) {
				// The version we have isn't in the registry, so request latest that *is* in the
				// registry.
				return RegistryResult{Metadata: metadata, Pkg: pkgDescriptor, InstalledInWorkspace: has}, nil
			}
			if err != nil {
				return nil, err
			}

			pkgDescriptor = pkgDescriptorFromMetadata(newMetadata, spec.Checksums)
			logging.V(3).Infof("Resolved package %#v via the registry to %#v (installedInWorkspace=true)\n",
				spec.Source, pkgDescriptor)
			return RegistryResult{
				Metadata: newMetadata, Pkg: pkgDescriptor,
				InstalledInWorkspace: true,
			}, nil
		}
		if errors.Is(err, registry.ErrNotFound) {
			registryNotFoundErr = err
		} else {
			registryQueryErr = fmt.Errorf("%w: %v", ErrRegistryQuery, err)
		}
	}

	if registry.IsPreRegistryPackage(spec.Source) {
		logging.V(3).Infof("Resolved package %#v to an external source %#v (installedInWorkspace=false)\n",
			spec, naivePackageDescriptor)
		return ExternalSourceResult{Spec: naivePackageDescriptor}, nil
	}

	if registryQueryErr != nil {
		logging.V(3).Infof("Failed to resolve package %#v\n", spec)
		return nil, registryQueryErr
	}

	logging.V(3).Infof("Failed to resolve package %#v\n", spec)
	return nil, &PackageNotFoundError{
		Package:     spec.Source,
		Version:     spec.Version,
		OriginalErr: registryNotFoundErr,
	}
}

func pkgDescriptorFromMetadata(
	metadata apitype.PackageMetadata, checksums map[string][]byte,
) workspace.PackageDescriptor {
	pkgDescriptor := workspace.PackageDescriptor{
		PluginDescriptor: workspace.PluginDescriptor{
			Name:              metadata.Name,
			Kind:              apitype.ResourcePlugin,
			Version:           &metadata.Version,
			PluginDownloadURL: metadata.PluginDownloadURL,
			Checksums:         checksums,
		},
	}
	if metadata.Parameterization != nil {
		pkgDescriptor.Parameterization = &workspace.Parameterization{
			Name:    metadata.Name,
			Version: metadata.Version,
			Value:   metadata.Parameterization.Parameter,
		}
		pkgDescriptor.Name = metadata.Parameterization.BaseProvider.Name
		pkgDescriptor.Version = &metadata.Parameterization.BaseProvider.Version
	}
	return pkgDescriptor
}

func (o Options) includeRegistryResolve() bool { return !o.DisableRegistryResolve && o.Experimental }

// PluginWorkspace dictates how resolution interacts with globally installed plugins.
type PluginWorkspace interface {
	HasPlugin(spec workspace.PluginDescriptor) bool
	HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error)
	IsExternalURL(source string) bool
	GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)
}

type defaultWorkspace struct{}

func (defaultWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	return workspace.HasPlugin(spec)
}

func (defaultWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	return workspace.HasPluginGTE(spec)
}

func (defaultWorkspace) IsExternalURL(source string) bool {
	return workspace.IsExternalURL(source)
}

func (defaultWorkspace) GetLatestVersion(
	ctx context.Context, spec workspace.PluginDescriptor,
) (*semver.Version, error) {
	return spec.GetLatestVersion(ctx)
}

func DefaultWorkspace() PluginWorkspace {
	return defaultWorkspace{}
}
