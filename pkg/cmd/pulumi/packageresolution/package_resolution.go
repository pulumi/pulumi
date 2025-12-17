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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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
}

// The result of running [Resolve].
//
// Result will be one of 4 types:
//
// - [RegistryResult]: The package was resolved using Pulumi's Registry.
// - [LocalPathResult]: The package is local and already on disk.
// - [ExternalSourceResult]: The package is external, and should be downloaded normally.
// - [InstalledInWorkspaceResult]: The package is already installed.
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
	if plugin.IsLocalPluginPath(ctx, spec.Source) {
		return LocalPathResult{
			LocalPath: spec.Source,
		}, nil
	}

	naivePackageDescriptor, err := naivePackageDescriptor(ctx, spec)
	if err != nil {
		return nil, err
	}

	installed, err := isAlreadyInstalled(ws, naivePackageDescriptor.PluginDescriptor)
	if err != nil {
		return nil, err
	}

	if ws.IsExternalURL(spec.Source) || naivePackageDescriptor.IsGitPlugin() || installed {
		return ExternalSourceResult{Spec: naivePackageDescriptor, InstalledInWorkspace: installed}, nil
	}

	var registryNotFoundErr error
	var registryQueryErr error

	if options.includeRegistryResolve() {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, spec.Source, naivePackageDescriptor.Version)
		if err == nil {
			pkgDescriptor := workspace.PackageDescriptor{
				PluginDescriptor: workspace.PluginDescriptor{
					Name:              metadata.Name,
					Kind:              apitype.ResourcePlugin,
					Version:           &metadata.Version,
					PluginDownloadURL: metadata.PluginDownloadURL,
					Checksums:         spec.Checksums,
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

			installed, err := isAlreadyInstalled(ws, pkgDescriptor.PluginDescriptor)
			if err != nil {
				return nil, err
			}

			return RegistryResult{Metadata: metadata, Pkg: pkgDescriptor, InstalledInWorkspace: installed}, nil
		}
		if errors.Is(err, registry.ErrNotFound) {
			registryNotFoundErr = err
		} else {
			registryQueryErr = fmt.Errorf("%w: %v", ErrRegistryQuery, err)
		}
	}

	if registry.IsPreRegistryPackage(spec.Source) {
		return ExternalSourceResult{Spec: naivePackageDescriptor}, nil
	}

	if registryQueryErr != nil {
		return nil, registryQueryErr
	}

	return nil, &PackageNotFoundError{
		Package:     spec.Source,
		Version:     spec.Version,
		OriginalErr: registryNotFoundErr,
	}
}

func (o Options) includeRegistryResolve() bool { return !o.DisableRegistryResolve && o.Experimental }

func isAlreadyInstalled(ws PluginWorkspace, spec workspace.PluginDescriptor) (bool, error) {
	if spec.Version != nil {
		return ws.HasPlugin(spec), nil
	}
	return ws.HasPluginGTE(spec)
}

// PluginWorkspace dictates how resolution interacts with globally installed plugins.
type PluginWorkspace interface {
	HasPlugin(spec workspace.PluginDescriptor) bool
	HasPluginGTE(spec workspace.PluginDescriptor) (bool, error)
	IsExternalURL(source string) bool
}

type defaultWorkspace struct{}

func (defaultWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	return workspace.HasPlugin(spec)
}

func (defaultWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, error) {
	return workspace.HasPluginGTE(spec)
}

func (defaultWorkspace) IsExternalURL(source string) bool {
	return workspace.IsExternalURL(source)
}

func DefaultWorkspace() PluginWorkspace {
	return defaultWorkspace{}
}
