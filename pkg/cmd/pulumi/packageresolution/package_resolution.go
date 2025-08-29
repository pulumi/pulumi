// Copyright 2024, Pulumi Corporation.
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
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Env struct {
	DisableRegistryResolve bool
	Experimental           bool
}

type RegistryResult struct {
	Metadata apitype.PackageMetadata
}

type LocalPathResult struct {
	LocalPluginPathAbs string
}

type ExternalSourceResult struct{}

type UnknownResult struct {
	Error error
}

func ResolvePackage(
	ctx context.Context,
	reg registry.Registry,
	pluginSpec workspace.PluginSpec,
	diagSink diag.Sink,
	env Env,
	projectRoot string, // Pass "" for 'not in a project context'
) any {
	sourceToCheck := pluginSpec.Name

	if projectRoot != "" {
		localSource := getLocalProjectPackageSource(projectRoot, pluginSpec.Name, diagSink)
		if localSource != "" {
			sourceToCheck = localSource
		}
	}

	if plugin.IsLocalPluginPath(ctx, sourceToCheck) {
		return LocalPathResult{
			LocalPluginPathAbs: sourceToCheck,
		}
	}

	if isGitURL(sourceToCheck) || pluginSpec.IsGitPlugin() {
		return ExternalSourceResult{}
	}

	var registryErr error
	if !env.DisableRegistryResolve && env.Experimental {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, pluginSpec.Name, pluginSpec.Version)
		if err == nil {
			return RegistryResult{Metadata: metadata}
		}
		registryErr = err
	}

	if registry.IsPreRegistryPackage(pluginSpec.Name) {
		return ExternalSourceResult{}
	}

	return UnknownResult{
		Error: registryErr,
	}
}

func getLocalProjectPackageSource(
	projectRoot string,
	packageName string,
	diagSink diag.Sink,
) string {
	projPath := filepath.Join(projectRoot, "Pulumi.yaml")
	project, err := workspace.LoadProject(projPath)
	if err != nil {
		if diagSink != nil {
			diagSink.Infof(
				diag.Message("", "Could not read project file %s when checking for local package %s: %v"),
				projPath, packageName, err)
		}
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

func isGitURL(source string) bool {
	return strings.HasPrefix(source, "https://github.com/") ||
		strings.HasPrefix(source, "git://") ||
		strings.HasPrefix(source, "ssh://git@")
}
