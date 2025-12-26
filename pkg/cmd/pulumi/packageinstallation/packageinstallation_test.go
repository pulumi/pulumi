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
	"errors"
	"iter"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

// Check that [packageworkspace.Workspace] implements [packageinstallation.Workspace]
// without importing [packageworkspace] from [packageinstallation].
var _ packageinstallation.Workspace = packageworkspace.Workspace{}

func TestInstallAlreadyInstalledPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "already-installed",
				Kind: apitype.ResourcePlugin,
			},
			downloaded: true,
			installed:  true,
			hasBinary:  true,
		},
	})
	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source: "already-installed",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

func TestInstallExternalBinaryPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "external-package",
				Kind:    apitype.ResourcePlugin,
				Version: &semver.Version{Major: 6},
			},
			hasBinary: true,
		},
	})
	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:            "external-package",
		PluginDownloadURL: "https://example.com/external-package.tar.gz",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

func TestInstallPluginWithParameterizedDependency(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-a",
				Version: &semver.Version{Major: 4},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-b": {
						Source:            "plugin-b",
						Parameters:        []string{"param1", "param2"},
						Version:           "3.0.0",
						PluginDownloadURL: "example.com/plugin-b",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-b",
				Version: &semver.Version{Major: 3},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:            "plugin-a",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

// TestInstallPluginWithDiamondDependency tests that we de-duplicate dependencies that are
// seen multiple times in the graph.
//
// Here, our graph is:
//
//	A -> B -> D
//	A -> C -> D
//
// We test that we only install D once, and that we do so before both B and C (and A).
func TestInstallPluginWithDiamondDependency(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-a",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-b": {
						Source:            "plugin-b",
						PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
					},
					"plugin-c": {
						Source:            "plugin-c",
						PluginDownloadURL: "https://example.com/plugin-c.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-b",
				Version: &semver.Version{Major: 2},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-d": {
						Source:            "plugin-d",
						PluginDownloadURL: "https://example.com/plugin-d.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-c",
				Version: &semver.Version{Major: 3},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-d": {
						Source:            "plugin-d",
						PluginDownloadURL: "https://example.com/plugin-d.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-d",
				Version: &semver.Version{Major: 3},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:            "plugin-a",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

// TestDeduplicateRegistryBasedPlugin tests that we correctly deduplicate packages when
// two different package names resolve to the same underlying package via registry resolution.
//
// Here, our graph is:
//
//	A -> B -> C
//	A -> D
//
// Where C and D are different package names that both resolve to the same underlying package
// (pulumi/shared-plugin v1.0.0) via registry resolution. We verify that we only download and
// install the shared plugin once.
func TestDeduplicateRegistryBasedPlugin(t *testing.T) {
	t.Parallel()

	sharedPluginURL := "https://registry.example.com/shared-plugin-1.0.0.tar.gz"

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-a",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-b": {
						Source:            "plugin-b",
						PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
					},
					"plugin-d": {Source: "plugin-d"},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-b",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-c": {Source: "plugin-c"},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:              "shared-plugin",
				Version:           &semver.Version{Major: 1},
				Kind:              apitype.ResourcePlugin,
				PluginDownloadURL: sharedPluginURL,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	mockRegistry := registry.Mock{
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				if name != nil && (*name == "plugin-c" || *name == "plugin-d") {
					yield(apitype.PackageMetadata{
						Source:            "pulumi",
						Publisher:         "pulumi",
						Name:              "shared-plugin",
						Version:           semver.Version{Major: 1},
						PluginDownloadURL: sharedPluginURL,
					}, nil)
				}
			}
		},
	}

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:            "plugin-a",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
			ResolveWithRegistry:                        true,
		},
		Concurrency: 1,
	}, mockRegistry, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

// TestInstallPluginWithCyclicDependency tests that we correctly detect and report cyclic
// dependencies in the plugin graph.
//
// Here, we set up dependencies:
//
//	A -> B
//	B -> C
//	C -> A
//
// The cycle is detected and reported as: A -> C -> B -> A.
func TestInstallPluginWithCyclicDependency(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-a",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-b": {
						Source:            "plugin-b",
						PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-b",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-c": {
						Source:            "plugin-c",
						PluginDownloadURL: "https://example.com/plugin-c.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-c",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-a": {
						Source:            "plugin-a",
						PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
					},
				},
			},
		},
	})

	_, err := packageinstallation.InstallPlugin(
		t.Context(),
		workspace.PackageSpec{
			Source:            "plugin-a",
			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
		},
		nil,
		"",
		packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		},
		nil,
		ws,
	)

	var cyclicErr packageinstallation.ErrorCyclicDependencies
	require.ErrorAs(t, err, &cyclicErr)

	require.Equal(t, []workspace.PluginDescriptor{
		{Name: "plugin-a", Kind: apitype.ResourcePlugin, PluginDownloadURL: "https://example.com/plugin-a.tar.gz"},
		{Name: "plugin-c", Kind: apitype.ResourcePlugin, PluginDownloadURL: "https://example.com/plugin-c.tar.gz"},
		{Name: "plugin-b", Kind: apitype.ResourcePlugin, PluginDownloadURL: "https://example.com/plugin-b.tar.gz"},
		{Name: "plugin-a", Kind: apitype.ResourcePlugin, PluginDownloadURL: "https://example.com/plugin-a.tar.gz"},
	}, cyclicErr.Cycle)
}

func TestInstallRegistryPackage(t *testing.T) {
	t.Parallel()

	registryPackageURL := "https://registry.example.com/registry-package-1.0.0.tar.gz"

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:              "registry-package",
				Version:           &semver.Version{Major: 1},
				Kind:              apitype.ResourcePlugin,
				PluginDownloadURL: registryPackageURL,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	mockRegistry := registry.Mock{
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				if name != nil && *name == "registry-package" {
					yield(apitype.PackageMetadata{
						Source:            "pulumi",
						Publisher:         "pulumi",
						Name:              "registry-package",
						Version:           semver.Version{Major: 1},
						PluginDownloadURL: registryPackageURL,
					}, nil)
				}
			}
		},
	}

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source: "registry-package",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
			ResolveWithRegistry:                        true,
		},
		Concurrency: 1,
	}, mockRegistry, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

// TestInstallInProjectWithSharedDependency tests installing dependencies in a project where
// the project depends on both a git-based plugin and a binary plugin, and the git-based
// plugin also depends on the same binary plugin.
//
// Dependency graph:
//
//	P -> A -> B
//	P -> B
//
// Where P is the project, A is a git-based plugin, and B is a binary plugin.
func TestInstallInProjectWithSharedDependency(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-a",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-b": {
						Source:            "plugin-b",
						PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-b",
				Kind: apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	err := packageinstallation.InstallInProject(t.Context(), &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Packages: map[string]workspace.PackageSpec{
			"plugin-a": {
				Source:            "plugin-a",
				PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
			},
			"plugin-b": {
				Source:            "plugin-b",
				PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
			},
		},
	}, "/project", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)
}

// TestInstallInProjectWithRelativePaths tests that InstallPluginAt is called with paths
// correctly resolved relative to projectDir for packages in various relative locations.
//
// Dependency graph:
//
//	P -> A -> C
//	P -> B -> C
//
// Where:
//   - P is the project at /work/project
//   - A is a package at ".." (resolves to /work)
//   - B is a package at "./pkg-b" (resolves to /work/project/pkg-b)
//   - C is a shared dependency /work/pkg-c
func TestInstallInProjectWithRelativePaths(t *testing.T) {
	t.Parallel()

	// Create a custom workspace with plugins at specific local paths
	ws := &invariantWorkspace{
		t:  t,
		rw: new(sync.RWMutex),
		plugins: map[string]*invariantPlugin{
			"/work": {
				d: workspace.PluginDescriptor{
					Name: "a",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"pkg-c": {Source: "./pkg-c"},
					},
				},
				downloaded:      true,
				pathVisible:     true,
				projectDetected: true,
			},
			"/work/project/pkg-b": {
				d: workspace.PluginDescriptor{
					Name: "b",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
					Packages: map[string]workspace.PackageSpec{
						"pkg-c": {Source: "../../pkg-c"},
					},
				},
				downloaded:      true,
				pathVisible:     true,
				projectDetected: true,
			},
			"/work/pkg-c": {
				d: workspace.PluginDescriptor{
					Name: "pkg-c",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("python", nil),
				},
				downloaded:      true,
				pathVisible:     true,
				projectDetected: true,
			},
		},
		binaryPaths: map[string]string{},
		downloadedWorkspace: map[string]*invariantWorkDir{
			"/work/project": {},
		},
	}

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	err := packageinstallation.InstallInProject(t.Context(), &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Packages: map[string]workspace.PackageSpec{
			"a": {Source: ".."},
			"b": {Source: "./pkg-b"},
		},
	}, "/work/project", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)
}

// TestInstallPluginWithMultipleVersions tests that when two dependencies require
// different versions of the same plugin, both versions are installed side-by-side
// and each dependent gets linked to its requested version.
//
// Here, our graph is:
//
//	Root -> plugin-a -> shared-plugin v1.0.0
//	Root -> plugin-b -> shared-plugin v2.0.0
//
// The system installs both shared-plugin v1.0.0 and v2.0.0 in separate directories
// (resource-shared-plugin-v1.0.0 and resource-shared-plugin-v2.0.0) and links
// plugin-a to v1.0.0 and plugin-b to v2.0.0.
//
// This validates that multiple versions of the same plugin can coexist and that
// each dependent gets its requested version.
func TestInstallPluginWithMultipleVersions(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-a",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"shared-plugin": {
						Source:            "shared-plugin",
						Version:           "1.0.0",
						PluginDownloadURL: "https://example.com/shared-plugin-1.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-b",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"shared-plugin": {
						Source:            "shared-plugin",
						Version:           "2.0.0",
						PluginDownloadURL: "https://example.com/shared-plugin-2.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:    "shared-plugin",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name:    "shared-plugin",
				Version: &semver.Version{Major: 2},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "root",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"plugin-a": {
						Source:            "plugin-a",
						PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
					},
					"plugin-b": {
						Source:            "plugin-b",
						PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
					},
				},
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, err := packageinstallation.InstallPlugin(
		t.Context(),
		workspace.PackageSpec{
			Source:            "root",
			PluginDownloadURL: "https://example.com/root.tar.gz",
		},
		nil,
		"",
		packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		},
		nil,
		rws,
	)
	require.NoError(t, err)
	_, runErr := run(t.Context(), "/tmp")
	require.NoError(t, runErr)

	sharedV1Path := "$HOME/.pulumi/plugins/resource-shared-plugin-v1.0.0"
	sharedV2Path := "$HOME/.pulumi/plugins/resource-shared-plugin-v2.0.0"
	pluginAPath := "$HOME/.pulumi/plugins/resource-plugin-a"
	pluginBPath := "$HOME/.pulumi/plugins/resource-plugin-b"

	require.True(t, ws.plugins[sharedV1Path].downloaded, "shared-plugin v1.0.0 should be downloaded")
	require.True(t, ws.plugins[sharedV1Path].installed, "shared-plugin v1.0.0 should be installed")
	require.True(t, ws.plugins[sharedV2Path].downloaded, "shared-plugin v2.0.0 should be downloaded")
	require.True(t, ws.plugins[sharedV2Path].installed, "shared-plugin v2.0.0 should be installed")

	// Verify that plugin-a is linked to v1.0.0 and plugin-b is linked to v2.0.0
	require.Contains(t, ws.plugins[pluginAPath].linked, sharedV1Path,
		"plugin-a should be linked to shared-plugin v1.0.0")
	require.Contains(t, ws.plugins[pluginBPath].linked, sharedV2Path,
		"plugin-b should be linked to shared-plugin v2.0.0")
}

// TestDuplicateParameterizationSources tests error handling when a plugin receives
// parameterization from multiple sources (both registry metadata and local Parameters field).
func TestDuplicateParameterizationSources(t *testing.T) {
	t.Parallel()

	parameterizedPluginURL := "https://registry.example.com/param-plugin-1.0.0.tar.gz"

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:              "param-plugin",
				Version:           &semver.Version{Major: 1},
				Kind:              apitype.ResourcePlugin,
				PluginDownloadURL: parameterizedPluginURL,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "root",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"param-plugin": {
						Source: "param-plugin",
						// Local parameters specified here
						Parameters: []string{"local-param1", "local-param2"},
					},
				},
			},
		},
	})

	// Mock registry that returns parameterization metadata
	mockRegistry := registry.Mock{
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				if name != nil && *name == "param-plugin" {
					yield(apitype.PackageMetadata{
						Source:            "pulumi",
						Publisher:         "pulumi",
						Name:              "param-plugin",
						Version:           semver.Version{Major: 1},
						PluginDownloadURL: parameterizedPluginURL,
						Parameterization: &apitype.PackageParameterization{
							BaseProvider: apitype.ArtifactVersionNameSpec{
								Name:    "base-provider",
								Version: semver.Version{Major: 1},
							},
						},
					}, nil)
				}
			}
		},
	}

	_, err := packageinstallation.InstallPlugin(
		t.Context(),
		workspace.PackageSpec{
			Source:            "root",
			PluginDownloadURL: "https://example.com/root.tar.gz",
		},
		nil,
		"",
		packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveWithRegistry: true,
			},
			Concurrency: 1,
		},
		mockRegistry,
		ws,
	)

	require.ErrorContains(t, err,
		"unable to resolve: unable to resolve package: "+
			"resolved plugin to pulumi/pulumi/param-plugin, which is already parameterized")
}

// TestMissingBinaryAndProject tests error handling when a plugin directory exists but
// contains neither a PulumiPlugin.yaml file nor a valid plugin binary.
func TestMissingBinaryAndProject(t *testing.T) {
	t.Parallel()

	// Create a plugin that will be downloaded but has neither a PulumiPlugin.yaml
	// nor a valid binary executable
	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "invalid-plugin",
				Kind: apitype.ResourcePlugin,
			},
		},
	})

	_, err := packageinstallation.InstallPlugin(
		t.Context(),
		workspace.PackageSpec{
			Source:            "invalid-plugin",
			PluginDownloadURL: "https://example.com/invalid-plugin.tar.gz",
		},
		nil,
		"",
		packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		},
		nil,
		ws,
	)

	require.ErrorContains(t, err, `expected "invalid-plugin" to have an executable`+
		` named "pulumi-resource-invalid-plugin" or a PulumiPlugin file`)
}

// TestRegistryLookupFailure tests error handling when registry operations fail.
func TestRegistryLookupFailure(t *testing.T) {
	t.Parallel()

	t.Run("generic error", func(t *testing.T) {
		t.Parallel()

		ws := newInvariantWorkspace(t, nil, []invariantPlugin{})

		registryError := errors.New("registry API error")

		// Mock registry that immediately returns an error
		mockRegistry := registry.Mock{
			ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					if name != nil && *name == "unavailable-package" {
						// Simulate registry API error (500, network timeout, etc.)
						yield(apitype.PackageMetadata{}, registryError)
					}
				}
			},
		}

		_, err := packageinstallation.InstallPlugin(
			t.Context(),
			workspace.PackageSpec{
				Source: "unavailable-package",
			},
			nil,
			"",
			packageinstallation.Options{
				Options: packageresolution.Options{
					ResolveWithRegistry: true,
				},
				Concurrency: 1,
			},
			mockRegistry,
			ws,
		)

		require.ErrorIs(t, err, registryError)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		ws := newInvariantWorkspace(t, nil, []invariantPlugin{})

		// Mock registry that returns empty results (package doesn't exist)
		mockRegistry := registry.Mock{
			ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return nothing - package not found
				}
			},
		}

		_, err := packageinstallation.InstallPlugin(
			t.Context(),
			workspace.PackageSpec{
				Source: "nonexistent-package",
			},
			nil,
			"",
			packageinstallation.Options{
				Options: packageresolution.Options{
					ResolveWithRegistry: true,
				},
				Concurrency: 1,
			},
			mockRegistry,
			ws,
		)

		require.ErrorIs(t, err, registry.ErrNotFound)
	})
}

func TestInstallParameterizedProviderFromRegistry(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "terraform-provider",
				Version: &semver.Version{Major: 1, Patch: 2},
				Kind:    apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	mockRegistry := registry.Mock{
		GetPackageF: func(
			ctx context.Context, source, publisher, name string, version *semver.Version,
		) (apitype.PackageMetadata, error) {
			if name == "airbyte" {
				return apitype.PackageMetadata{
					Source:    "opentofu",
					Publisher: "airbytehq",
					Name:      "airbyte",
					Version:   semver.Version{Major: 0, Minor: 13, Patch: 0},
					Parameterization: &apitype.PackageParameterization{
						BaseProvider: apitype.ArtifactVersionNameSpec{
							Name:    "terraform-provider",
							Version: semver.Version{Major: 1, Patch: 2},
						},
						Parameter: []byte("opentofu/airbytehq/airbyte"),
					},
				}, nil
			}
			return apitype.PackageMetadata{}, registry.ErrNotFound
		},
	}

	run, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:  "opentofu/airbytehq/airbyte",
		Version: "0.13.0",
	}, nil, "", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveWithRegistry: true,
		},
		Concurrency: 1,
	}, mockRegistry, rws)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)
}

func TestConcurrency(t *testing.T) {
	t.Parallel()

	// Create a complex dependency graph to test concurrent installation:
	//
	//   root -> A -> D -> G
	//        -> B -> E -> H
	//        -> C -> F -> I
	//
	// This creates 3 parallel chains that can be installed concurrently.

	createWorkspace := func() *invariantWorkspace {
		return newInvariantWorkspace(t, nil, []invariantPlugin{
			// Level 0: root
			{
				d: workspace.PluginDescriptor{
					Name: "root",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-a": {
							Source:            "plugin-a",
							PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
						},
						"plugin-b": {
							Source:            "plugin-b",
							PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
						},
						"plugin-c": {
							Source:            "plugin-c",
							PluginDownloadURL: "https://example.com/plugin-c.tar.gz",
						},
					},
				},
			},
			// Level 1: A, B, C
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-a",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-d": {
							Source:            "plugin-d",
							PluginDownloadURL: "https://example.com/plugin-d.tar.gz",
						},
					},
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-b",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-e": {
							Source:            "plugin-e",
							PluginDownloadURL: "https://example.com/plugin-e.tar.gz",
						},
					},
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-c",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-f": {
							Source:            "plugin-f",
							PluginDownloadURL: "https://example.com/plugin-f.tar.gz",
						},
					},
				},
			},
			// Level 2: D, E, F
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-d",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-g": {
							Source:            "plugin-g",
							PluginDownloadURL: "https://example.com/plugin-g.tar.gz",
						},
					},
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-e",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-h": {
							Source:            "plugin-h",
							PluginDownloadURL: "https://example.com/plugin-h.tar.gz",
						},
					},
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-f",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-i": {
							Source:            "plugin-i",
							PluginDownloadURL: "https://example.com/plugin-i.tar.gz",
						},
					},
				},
			},
			// Level 3: G, H, I (leaf nodes)
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-g",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-h",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-i",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				},
			},
		})
	}

	baselineWs := createWorkspace()
	run, err := packageinstallation.InstallPlugin(
		t.Context(),
		workspace.PackageSpec{
			Source:            "root",
			PluginDownloadURL: "https://example.com/root.tar.gz",
		},
		nil,
		"",
		packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		},
		nil,
		baselineWs,
	)
	require.NoError(t, err)
	_, err = run(t.Context(), "/tmp")
	require.NoError(t, err)

	for range 100 {
		ws := createWorkspace()
		run, err := packageinstallation.InstallPlugin(
			t.Context(),
			workspace.PackageSpec{
				Source:            "root",
				PluginDownloadURL: "https://example.com/root.tar.gz",
			},
			nil,
			"",
			packageinstallation.Options{
				Options: packageresolution.Options{
					ResolveVersionWithLocalWorkspace:           true,
					AllowNonInvertableLocalWorkspaceResolution: true,
				},
			},
			nil,
			ws,
		)
		require.NoError(t, err)
		_, err = run(t.Context(), "/tmp")
		require.NoError(t, err)

		assertInvariantWorkspaceEqual(t, *baselineWs, *ws)
	}
}
