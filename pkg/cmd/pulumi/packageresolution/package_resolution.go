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
	Version     *semver.Version
	OriginalErr error
}

func (e PackageNotFoundError) Error() string {
	if e.Version != nil {
		return fmt.Sprintf("package %s@%s not found", e.Package, e.Version.String())
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
	DisableRegistryResolve      bool
	Experimental                bool
	IncludeInstalledInWorkspace bool
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

func (RegistryResult) isResult()             {}
func (LocalPathResult) isResult()            {}
func (ExternalSourceResult) isResult()       {}
func (InstalledInWorkspaceResult) isResult() {}

type (
	RegistryResult             struct{ Metadata apitype.PackageMetadata }
	LocalPathResult            struct{ LocalPath string }
	ExternalSourceResult       struct{ Spec workspace.PluginSpec }
	InstalledInWorkspaceResult struct{}
)

func Resolve(
	ctx context.Context,
	reg registry.Registry,
	ws PluginWorkspace,
	pluginSpec workspace.PluginSpec,
	options Options,
	projectOrPlugin workspace.BaseProject, // Pass nil for 'not in a project context'
) (Result, error) {
	sourceToCheck := pluginSpec.Name

	if options.IncludeInstalledInWorkspace {
		if pluginSpec.Version != nil && ws.HasPlugin(pluginSpec) {
			return InstalledInWorkspaceResult{}, nil
		}

		has, _ := ws.HasPluginGTE(pluginSpec)
		if pluginSpec.Version == nil && has {
			return InstalledInWorkspaceResult{}, nil
		}
	}

	if projectOrPlugin != nil {
		localSource, ok := projectOrPlugin.GetPackageSpecs()[pluginSpec.Name]
		if ok {
			sourceToCheck = localSource.Source
		}
	}

	if plugin.IsLocalPluginPath(ctx, sourceToCheck) {
		return LocalPathResult{LocalPath: sourceToCheck}, nil
	}

	if ws.IsExternalURL(sourceToCheck) || pluginSpec.IsGitPlugin() {
		return ExternalSourceResult{Spec: pluginSpec}, nil
	}

	var registryNotFoundErr error
	var registryQueryErr error

	if !options.DisableRegistryResolve && options.Experimental {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, pluginSpec.Name, pluginSpec.Version)
		if err == nil {
			return RegistryResult{Metadata: metadata}, nil
		}
		if errors.Is(err, registry.ErrNotFound) {
			registryNotFoundErr = err
		} else {
			registryQueryErr = fmt.Errorf("%w: %v", ErrRegistryQuery, err)
		}
	}

	if registry.IsPreRegistryPackage(pluginSpec.Name) {
		return ExternalSourceResult{Spec: pluginSpec}, nil
	}

	if registryQueryErr != nil {
		return nil, registryQueryErr
	}

	return nil, &PackageNotFoundError{
		Package:     pluginSpec.Name,
		Version:     pluginSpec.Version,
		OriginalErr: registryNotFoundErr,
	}
}

// PluginWorkspace dictates how resolution interacts with globally installed plugins.
type PluginWorkspace interface {
	HasPlugin(spec workspace.PluginSpec) bool
	HasPluginGTE(spec workspace.PluginSpec) (bool, error)
	IsExternalURL(source string) bool
}

type defaultWorkspace struct{}

func (defaultWorkspace) HasPlugin(spec workspace.PluginSpec) bool {
	return workspace.HasPlugin(spec)
}

func (defaultWorkspace) HasPluginGTE(spec workspace.PluginSpec) (bool, error) {
	return workspace.HasPluginGTE(spec)
}

func (defaultWorkspace) IsExternalURL(source string) bool {
	return workspace.IsExternalURL(source)
}

func DefaultWorkspace() PluginWorkspace {
	return defaultWorkspace{}
}
