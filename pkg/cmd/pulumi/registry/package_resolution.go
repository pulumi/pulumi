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

package registry

import (
	"context"
	"path/filepath"
	"strings"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type PackageResolutionStrategy int

const (
	RegistryResolution PackageResolutionStrategy = iota
	LocalPluginPathResolution
	LegacyResolution
	UnknownPackage
)

type PackageResolutionEnv struct {
	DisableRegistryResolve bool
	Experimental           bool
}

type PackageResolutionProjectContext struct {
	Root string
}

// DetectProjectContext tries to detect the current project context.
// Returns nil if no project is found (e.g., when not in a project directory).
func DetectProjectContext() *PackageResolutionProjectContext {
	if _, root, err := pkgWorkspace.Instance.ReadProject(); err == nil {
		return &PackageResolutionProjectContext{Root: root}
	}
	return nil
}

type PackageResolutionResult struct {
	// Metadata is only set for RegistryResolution strategy
	// This avoids needing to call registry resolution again if the package is found
	Metadata *apitype.PackageMetadata
	Strategy PackageResolutionStrategy
	Error    error
}

func ResolvePackage(
	ctx context.Context,
	reg registry.Registry,
	pluginSpec workspace.PluginSpec,
	diagSink diag.Sink,
	env PackageResolutionEnv,
	proj *PackageResolutionProjectContext,
) PackageResolutionResult {
	if plugin.IsLocalPluginPath(ctx, pluginSpec.Name) {
		return PackageResolutionResult{
			Strategy: LocalPluginPathResolution,
		}
	}

	if pluginSpec.IsGitPlugin() {
		return PackageResolutionResult{
			Strategy: LegacyResolution,
		}
	}

	if proj != nil {
		localSource := getLocalProjectPackageSource(proj, pluginSpec.Name, diagSink)
		if localSource != "" {
			if plugin.IsLocalPluginPath(ctx, localSource) {
				return PackageResolutionResult{
					Strategy: LocalPluginPathResolution,
				}
			}

			if isGitURL(localSource) {
				return PackageResolutionResult{
					Strategy: LegacyResolution,
				}
			}
		}
	}

	var registryErr error
	if !env.DisableRegistryResolve && env.Experimental {
		metadata, err := registry.ResolvePackageFromName(ctx, reg, pluginSpec.Name, pluginSpec.Version)
		if err == nil {
			return PackageResolutionResult{
				Metadata: &metadata,
				Strategy: RegistryResolution,
			}
		}
		registryErr = err
	}

	if registry.IsPreRegistryPackage(pluginSpec.Name) {
		return PackageResolutionResult{
			Strategy: LegacyResolution,
		}
	}

	return PackageResolutionResult{
		Strategy: UnknownPackage,
		Error:    registryErr,
	}
}

func getLocalProjectPackageSource(
	proj *PackageResolutionProjectContext,
	packageName string,
	diagSink diag.Sink,
) string {
	projPath := filepath.Join(proj.Root, "Pulumi.yaml")
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
