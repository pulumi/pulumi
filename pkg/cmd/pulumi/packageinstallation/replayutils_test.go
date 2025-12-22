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

package packageinstallation_test

import (
	"context"
	"maps"
	"path"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var StandardDownloadPlugin DownloadPlugin = func(
	_ context.Context, plugin workspace.PluginDescriptor,
) (string, func(bool), error) {
	return "/tmp/plugins/" + plugin.Dir(), func(bool) {}, nil
}

var StandardDetectPluginPathAt DetectPluginPathAt = func(ctx context.Context, dir string) (string, error) {
	return path.Join(dir, "PulumiPlugin.yaml"), nil
}

var StandardRunPackage RunPackage = func(
	ctx context.Context, rootDir, pluginDir string, params plugin.ParameterizeParameters,
) (plugin.Provider, error) {
	return nil, nil
}

var StandardInstallPluginAt InstallPluginAt = func(
	ctx context.Context, dirPath string, project *workspace.PluginProject,
) error {
	return nil
}

var StandardLinkPackage LinkPackage = func(
	ctx context.Context,
	project *workspace.ProjectRuntimeInfo, projectDir string,
	packageName string, pluginDir string, params plugin.ParameterizeParameters,
	originalSpec workspace.PackageSpec,
) error {
	return nil
}

func StandardLoadPluginProject(
	runtime workspace.ProjectRuntimeInfo, packages map[string]workspace.PackageSpec,
) LoadPluginProject {
	return func(ctx context.Context, path string) (*workspace.PluginProject, error) {
		proj := &workspace.PluginProject{Runtime: runtime}

		for _, name := range slices.Sorted(maps.Keys(packages)) {
			proj.AddPackage(name, packages[name])
		}

		return proj, nil
	}
}
