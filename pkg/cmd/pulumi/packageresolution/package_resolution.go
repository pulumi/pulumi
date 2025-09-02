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
// This differs from registry.ResolvePackageFromName which specifically queries
// the Pulumi registry. This package determines the resolution strategy first,
// then may delegate to registry functions, local file operations, or external
// source handling as appropriate.
package packageresolution

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Options struct {
	DisableRegistryResolve bool
	Experimental           bool
}

type Result interface {
	isResult()
}

type RegistryResult struct {
	Metadata apitype.PackageMetadata
}

func (RegistryResult) isResult() {}

type LocalPathResult struct {
	LocalPluginPathAbs string
}

func (LocalPathResult) isResult() {}

type ExternalSourceResult struct{}

func (ExternalSourceResult) isResult() {}

func Resolve(
	ctx context.Context,
	reg registry.Registry,
	pluginSpec workspace.PluginSpec,
	options Options,
	projectRoot string, // Pass "" for 'not in a project context'
) (Result, error) {
	sourceToCheck := pluginSpec.Name

	if projectRoot != "" {
		localSource := getLocalProjectPackageSource(projectRoot, pluginSpec.Name)
		if localSource != "" {
			sourceToCheck = localSource
		}
	}

	if plugin.IsLocalPluginPath(ctx, sourceToCheck) {
		return LocalPathResult{LocalPluginPathAbs: sourceToCheck}, nil
	}

	if workspace.IsExternalURL(sourceToCheck) || pluginSpec.IsGitPlugin() {
		return ExternalSourceResult{}, nil
	}

	var registryErr error
	if !options.DisableRegistryResolve && options.Experimental {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, pluginSpec.Name, pluginSpec.Version)
		if err == nil {
			return RegistryResult{Metadata: metadata}, nil
		}
		if !errors.Is(err, registry.ErrNotFound) {
			registryErr = err
		}
	}

	if registry.IsPreRegistryPackage(pluginSpec.Name) {
		return ExternalSourceResult{}, nil
	}

	if registryErr != nil {
		return RegistryResult{}, registryErr
	}

	return ExternalSourceResult{}, fmt.Errorf("package %s not found", pluginSpec.Name)
}

func getLocalProjectPackageSource(
	projectRoot string,
	packageName string,
) string {
	projPath := filepath.Join(projectRoot, "Pulumi.yaml")
	project, err := workspace.LoadProject(projPath)
	if err != nil {
		return ""
	}

	packages := project.GetPackageSpecs()
	if packages == nil {
		return ""
	}

	if packageSpec, exists := packages[packageName]; exists {
		return packageSpec.Source
	}
	return ""
}
