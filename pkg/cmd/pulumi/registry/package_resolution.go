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
	"errors"
	"os"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type PackageFallbackType int

const (
	NoFallback                PackageFallbackType = iota
	PreGitHubRegistryFallback                     // Use GitHub releases for pre-IDP Registry packages
	LocalProjectFallback                          // Use local Pulumi.yaml package definition
)

type PackageResolutionResult struct {
	Found        bool
	Metadata     *apitype.PackageMetadata
	FallbackType PackageFallbackType
	Error        error
}

// TryResolvePackageWithFallback attempts IDP Registry resolution first,
// then determines appropriate fallback strategy if not found.
func TryResolvePackageWithFallback(
	ctx context.Context,
	reg registry.Registry,
	packageName string,
	version *semver.Version,
	projectRoot string,
	diagSink diag.Sink,
) PackageResolutionResult {
	metadata, err := registry.ResolvePackageFromName(ctx, reg, packageName, version)
	if err == nil {
		return PackageResolutionResult{
			Found:        true,
			Metadata:     &metadata,
			FallbackType: NoFallback,
			Error:        nil,
		}
	}

	if errors.Is(err, registry.ErrNotFound) {
		if registry.IsPreGitHubRegistryPackage(packageName) {
			return PackageResolutionResult{
				Found:        false,
				Metadata:     nil,
				FallbackType: PreGitHubRegistryFallback,
				Error:        nil,
			}
		}

		if IsLocalProjectPackage(projectRoot, packageName, diagSink) {
			return PackageResolutionResult{
				Found:        false,
				Metadata:     nil,
				FallbackType: LocalProjectFallback,
				Error:        nil,
			}
		}

		return PackageResolutionResult{
			Found:        false,
			Metadata:     nil,
			FallbackType: NoFallback,
			Error:        err,
		}
	}

	return PackageResolutionResult{
		Found:        false,
		Metadata:     nil,
		FallbackType: NoFallback,
		Error:        err,
	}
}

func IsLocalProjectPackage(projectRoot, packageName string, diagSink diag.Sink) bool {
	projPath := filepath.Join(projectRoot, "Pulumi.yaml")
	proj, err := workspace.LoadProject(projPath)
	if err != nil {
		if diagSink != nil {
			diagSink.Infof(
				diag.Message("", "Could not read project file %s when checking for local package %s: %v"),
				projPath, packageName, err)
		}
		return false
	}

	packages := proj.GetPackageSpecs()
	if packages == nil {
		return false
	}

	_, exists := packages[packageName]
	return exists
}

func IsLocalProjectPackageForInstall(packageName string, diagSink diag.Sink) bool {
	cwd, err := os.Getwd()
	if err != nil {
		if diagSink != nil {
			diagSink.Warningf(
				diag.Message("", "Unable to determine current directory while checking for local packages: %v"), err)
		}
		return false
	}
	return IsLocalProjectPackage(cwd, packageName, diagSink)
}
