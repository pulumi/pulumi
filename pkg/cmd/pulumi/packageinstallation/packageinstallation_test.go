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
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check that [packageworkspace.Workspace] implements [packageinstallation.Context]
// without importing [packageworkspace] from [packageinstallation].
var _ packageinstallation.Context = packageworkspace.Workspace{}

func TestInstallAlreadyInstalledPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source: "already-installed",
	}, spec)
}

func TestInstallAlreadyInstalledPlugin(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			downloaded: true,
			installed:  true,
			hasBinary:  true,
		},
	})
	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, _, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source: "plugin", Version: "1.0.0",
		Parameters: []string{"parameterization"},
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

func TestDoNotInstallDependenciesOfAlreadyInstalledPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "already-installed",
				Kind: apitype.ResourcePlugin,
			},
			downloaded: true,
			installed:  true,
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
				Packages: map[string]workspace.PackageSpec{
					"dependency": {
						Source:            "dependency",
						PluginDownloadURL: "https://example.com/dependency.tar.gz",
					},
				},
			},
		},
		{
			d: workspace.PluginDescriptor{
				Name: "dependency",
				Kind: apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
		},
	})
	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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

	dependencyPath := "$HOME/.pulumi/plugins/resource-dependency"
	require.False(t, ws.plugins[dependencyPath].downloaded,
		"dependency should NOT be downloaded when parent is already installed")
	require.Equal(t, workspace.PackageSpec{Source: "already-installed"}, spec)
}

func TestInstallExternalBinaryPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "external-package",
		Version:           "6.0.0",
		PluginDownloadURL: "https://example.com/external-package.tar.gz",
	}, spec)
}

func TestInstallPluginWithParameterizedDependency(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "plugin-a",
		Version:           "4.0.0",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, spec)
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

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "plugin-a",
		Version:           "1.0.0",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, spec)
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

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "plugin-a",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, spec)
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

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	_, _, _, err := packageinstallation.InstallPlugin(
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

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source:  "pulumi/pulumi/registry-package",
		Version: "1.0.0",
	}, spec)
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

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
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

	_, err := packageinstallation.InstallProjectPlugins(t.Context(), &workspace.Project{
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

// TestInstallProjectWithSkipLink verifies that with Options.SkipLink set, local SDKs are
// still generated but not linked into the project's language manifest (package.json,
// requirements.txt, pyproject.toml). It uses the same project layout as
// TestInstallInProjectWithSharedDependency, whose golden file shows LinkIntoProject steps,
// so the difference in the golden file for this test is exactly the absence of linking.
func TestInstallProjectWithSkipLink(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
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

	_, err := packageinstallation.InstallProjectPlugins(t.Context(), &workspace.Project{
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
		SkipLink:    true,
	}, nil, rws)
	require.NoError(t, err)

	// Verify nothing was linked
	assert.Empty(t, ws.downloadedWorkspace["/project"].linked,
		"SkipLink must not link any package into the project")
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

	_, err := packageinstallation.InstallProjectPlugins(t.Context(), &workspace.Project{
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

// TestInstallInProjectWithBinaryPaths tests
func TestInstallPluginWithBinaryPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
	}{
		{name: "absolute", source: "/path/to/binary/pulumi-resource-test-provider"},
		{name: "relative", source: "./pulumi-resource-test-provider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ws := newInvariantWorkspace(t, nil, []string{tt.source}, []invariantPlugin{})
			rws := &recordingWorkspace{ws, nil}
			defer rws.save(t)

			runPlugin, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
				Source: tt.source,
			}, nil, "", packageinstallation.Options{
				Concurrency: 1,
			}, nil, rws)
			require.NoError(t, err)
			assert.Equal(t, workspace.PackageSpec{Source: tt.source}, spec)
			_, err = runPlugin(t.Context(), "/tmp")
			require.NoError(t, err)
		})
	}
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

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "root",
		PluginDownloadURL: "https://example.com/root.tar.gz",
	}, spec)

	sharedV1Path := "$HOME/.pulumi/plugins/resource-shared-plugin-v1.0.0"
	sharedV2Path := "$HOME/.pulumi/plugins/resource-shared-plugin-v2.0.0"
	pluginAPath := "$HOME/.pulumi/plugins/resource-plugin-a"
	pluginBPath := "$HOME/.pulumi/plugins/resource-plugin-b"

	require.True(t, ws.plugins[sharedV1Path].downloaded, "shared-plugin v1.0.0 should be downloaded")
	require.True(t, ws.plugins[sharedV1Path].installed, "shared-plugin v1.0.0 should be installed")
	require.True(t, ws.plugins[sharedV2Path].downloaded, "shared-plugin v2.0.0 should be downloaded")
	require.True(t, ws.plugins[sharedV2Path].installed, "shared-plugin v2.0.0 should be installed")

	// Verify that plugin-a is linked to v1.0.0 and plugin-b is linked to v2.0.0
	require.Contains(t, ws.plugins[pluginAPath].linked, sharedV1Path+"/sdk-<nil>",
		"plugin-a should be linked to shared-plugin v1.0.0")
	require.Contains(t, ws.plugins[pluginBPath].linked, sharedV2Path+"/sdk-<nil>",
		"plugin-b should be linked to shared-plugin v2.0.0")
}

// TestDuplicateParameterizationSources tests error handling when a plugin receives
// parameterization from multiple sources (both registry metadata and local Parameters field).
func TestDuplicateParameterizationSources(t *testing.T) {
	t.Parallel()

	parameterizedPluginURL := "https://registry.example.com/param-plugin-1.0.0.tar.gz"

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	_, _, _, err := packageinstallation.InstallPlugin(
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
	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "invalid-plugin",
				Kind: apitype.ResourcePlugin,
			},
		},
	})

	_, _, _, err := packageinstallation.InstallPlugin(
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

		ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{})

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

		_, _, _, err := packageinstallation.InstallPlugin(
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

		ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{})

		// Mock registry that returns empty results (package doesn't exist)
		mockRegistry := registry.Mock{
			ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return nothing - package not found
				}
			},
		}

		_, _, _, err := packageinstallation.InstallPlugin(
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

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
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
	assert.Equal(t, workspace.PackageSpec{
		Source:  "opentofu/airbytehq/airbyte",
		Version: "0.13.0",
	}, spec)
}

// TestInstallProjectWithMultiplePackagesSharingOnePlugin tests that when a project
// declares two parameterized packages that share the same underlying plugin, the plugin
// is downloaded and installed only once, while each package's SDK is generated
// separately using its own parameterization.
//
// Dependency graph:
//
//	project -> airbyte (parameterized) -> terraform-provider
//	project -> github  (parameterized) -> terraform-provider
func TestInstallProjectWithMultiplePackagesSharingOnePlugin(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
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
			switch name {
			case "airbyte":
				return apitype.PackageMetadata{
					Source:    "opentofu",
					Publisher: "airbytehq",
					Name:      "airbyte",
					Version:   semver.Version{Major: 0, Minor: 13},
					Parameterization: &apitype.PackageParameterization{
						BaseProvider: apitype.ArtifactVersionNameSpec{
							Name:    "terraform-provider",
							Version: semver.Version{Major: 1, Patch: 2},
						},
						Parameter: []byte("opentofu/airbytehq/airbyte"),
					},
				}, nil
			case "github":
				return apitype.PackageMetadata{
					Source:    "opentofu",
					Publisher: "integrations",
					Name:      "github",
					Version:   semver.Version{Major: 5},
					Parameterization: &apitype.PackageParameterization{
						BaseProvider: apitype.ArtifactVersionNameSpec{
							Name:    "terraform-provider",
							Version: semver.Version{Major: 1, Patch: 2},
						},
						Parameter: []byte("opentofu/integrations/github"),
					},
				}, nil
			}
			return apitype.PackageMetadata{}, registry.ErrNotFound
		},
	}

	_, err := packageinstallation.InstallProjectPlugins(t.Context(), &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Packages: map[string]workspace.PackageSpec{
			"airbyte": {
				Source:  "opentofu/airbytehq/airbyte",
				Version: "0.13.0",
			},
			"github": {
				Source:  "opentofu/integrations/github",
				Version: "5.0.0",
			},
		},
	}, "/project", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveWithRegistry: true,
		},
		Concurrency: 1,
	}, mockRegistry, rws)
	require.NoError(t, err)
}

// TestInstallPluginWithRequiredPackages tests that GetRequiredPackages is called and its
// results are used to install dependencies.
//
// Here, plugin-a has a PulumiPlugin.yaml and the language runtime reports that it requires
// plugin-b.
func TestInstallPluginWithRequiredPackages(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-a",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
			requiredPackages: []workspace.PackageDescriptor{
				{
					PluginDescriptor: workspace.PluginDescriptor{
						Name:              "plugin-b",
						Version:           &semver.Version{Major: 2},
						Kind:              apitype.ResourcePlugin,
						PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
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
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:            "plugin-a",
		Version:           "1.0.0",
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "plugin-a",
		Version:           "1.0.0",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, spec)

	pluginAPath := "$HOME/.pulumi/plugins/resource-plugin-a-v1.0.0"
	pluginBPath := "$HOME/.pulumi/plugins/resource-plugin-b-v2.0.0"

	require.True(t, ws.plugins[pluginAPath].downloaded, "plugin-a should be downloaded")
	require.True(t, ws.plugins[pluginAPath].installed, "plugin-a should be installed")
	require.True(t, ws.plugins[pluginBPath].downloaded, "plugin-b should be downloaded (as a required package)")
	require.True(t, ws.plugins[pluginBPath].installed, "plugin-b should be installed (as a required package)")
}

// TestInstallPluginWithRequiredPackageSpecs tests that the package specs returned as the
// second value of GetRequiredPackages are acted upon when gathering a plugin's dependencies.
//
// Here, plugin-a has a PulumiPlugin.yaml and the language runtime reports that it requires
// plugin-b, but reports it via the spec channel (a spec-aware language host) rather than as a
// resolved package descriptor. plugin-b should be downloaded and installed, and a local SDK for
// it should be generated and linked into plugin-a. Specs are discovered from the program, not
// declared by the user, so they are never written to the project's `packages` section.
func TestInstallPluginWithRequiredPackageSpecs(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, nil, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-a",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			project: &workspace.PluginProject{
				Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			},
			requiredSpecs: []workspace.PackageSpec{
				{
					Source:            "plugin-b",
					Version:           "2.0.0",
					PluginDownloadURL: "https://example.com/plugin-b.tar.gz",
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
			},
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	run, spec, _, err := packageinstallation.InstallPlugin(t.Context(), workspace.PackageSpec{
		Source:            "plugin-a",
		Version:           "1.0.0",
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "plugin-a",
		Version:           "1.0.0",
		PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
	}, spec)

	pluginAPath := "$HOME/.pulumi/plugins/resource-plugin-a-v1.0.0"
	pluginBPath := "$HOME/.pulumi/plugins/resource-plugin-b-v2.0.0"

	require.True(t, ws.plugins[pluginAPath].downloaded, "plugin-a should be downloaded")
	require.True(t, ws.plugins[pluginAPath].installed, "plugin-a should be installed")
	require.True(t, ws.plugins[pluginBPath].downloaded, "plugin-b should be downloaded (as a required spec)")
	require.True(t, ws.plugins[pluginBPath].installed, "plugin-b should be installed (as a required spec)")

	require.Contains(t, ws.plugins[pluginAPath].linked, pluginBPath+"/sdk-<nil>",
		"plugin-a should have a local SDK for plugin-b linked in (as a required spec)")
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
		return newInvariantWorkspace(t, nil, nil, []invariantPlugin{
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
	run, spec, _, err := packageinstallation.InstallPlugin(
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
	assert.Equal(t, workspace.PackageSpec{
		Source:            "root",
		PluginDownloadURL: "https://example.com/root.tar.gz",
	}, spec)

	for range 100 {
		ws := createWorkspace()
		ws.jitter = time.Millisecond * 5
		run, spec, _, err := packageinstallation.InstallPlugin(
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
		assert.Equal(t, workspace.PackageSpec{
			Source:            "root",
			PluginDownloadURL: "https://example.com/root.tar.gz",
		}, spec)

		assertInvariantWorkspaceEqual(t, *baselineWs, *ws)
	}
}

// TestInstallSharedDependencyInParallel tests the scenario where a project
// directly depends on component A, and also depends on component B which itself
// depends on A.
//
// Dependency graph:
//
//	Root -> component-a -> plugin-c
//	Root -> component-b -> component-a
//	                    -> plugin-d
func TestInstallSharedDependencyInParallel(t *testing.T) {
	t.Parallel()

	createWorkspace := func() *invariantWorkspace {
		return newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
			{
				d: workspace.PluginDescriptor{
					Name: "component-a",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("python", nil),
					Packages: map[string]workspace.PackageSpec{
						"plugin-c": {Source: "plugin-c", PluginDownloadURL: "https://example.com/plugin-c.tar.gz"},
					},
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "component-b",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("python", nil),
					Packages: map[string]workspace.PackageSpec{
						"component-a": {
							Source:            "component-a",
							PluginDownloadURL: "https://example.com/component-a.tar.gz",
						},
						"plugin-d": {Source: "plugin-d", PluginDownloadURL: "https://example.com/plugin-d.tar.gz"},
					},
				},
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-c",
					Kind: apitype.ResourcePlugin,
				},
				hasBinary: true,
			},
			{
				d: workspace.PluginDescriptor{
					Name: "plugin-d",
					Kind: apitype.ResourcePlugin,
				},
				hasBinary: true,
			},
		})
	}

	createProject := func() *workspace.Project {
		return &workspace.Project{
			Name:    "test-project",
			Runtime: workspace.NewProjectRuntimeInfo("python", nil),
			Packages: map[string]workspace.PackageSpec{
				"component-a": {
					Source:            "component-a",
					PluginDownloadURL: "https://example.com/component-a.tar.gz",
				},
				"component-b": {
					Source:            "component-b",
					PluginDownloadURL: "https://example.com/component-b.tar.gz",
				},
			},
		}
	}

	baselineWs := createWorkspace()
	_, err := packageinstallation.InstallProjectPlugins(t.Context(), createProject(),
		"/project", packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1, // Sequential
		}, nil, baselineWs)
	require.NoError(t, err)

	componentAPath := "$HOME/.pulumi/plugins/resource-component-a"
	componentBPath := "$HOME/.pulumi/plugins/resource-component-b"
	componentASDK := componentAPath + "/sdk-<nil>"
	pluginCSDK := "$HOME/.pulumi/plugins/resource-plugin-c/sdk-<nil>"
	pluginDSDK := "$HOME/.pulumi/plugins/resource-plugin-d/sdk-<nil>"

	require.True(t, baselineWs.plugins[componentAPath].installed,
		"component-a should be installed in baseline")
	require.True(t, baselineWs.plugins[componentBPath].installed,
		"component-b should be installed in baseline")

	require.Contains(t, baselineWs.plugins[componentAPath].linked, pluginCSDK,
		"component-a should be linked to plugin-c")
	require.Contains(t, baselineWs.plugins[componentBPath].linked, componentASDK,
		"component-b should be linked to component-a (the shared dependency)")
	require.Contains(t, baselineWs.plugins[componentBPath].linked, pluginDSDK,
		"component-b should be linked to plugin-d")

	for range 100 {
		ws := createWorkspace()
		ws.jitter = time.Millisecond * 10
		_, err := packageinstallation.InstallProjectPlugins(t.Context(), createProject(),
			"/project", packageinstallation.Options{
				Options: packageresolution.Options{
					ResolveVersionWithLocalWorkspace:           true,
					AllowNonInvertableLocalWorkspaceResolution: true,
				},
				Concurrency: 0, // unlimited parallelism
			}, nil, ws)
		require.NoError(t, err)
		assertInvariantWorkspaceEqual(t, *baselineWs, *ws)
	}
}

// TestRequiredPackagesDeclaredInProjectPackagesNotDownloaded tests that when
// GetRequiredPackages returns a package that is already declared in the
// component's packages section, it is not downloaded again.
//
// This is the scenario where a component provider (provider) depends on another
// local component provider (provider-nested) via its packages section. The
// language host's GetRequiredPackages also reports provider-nested as a
// dependency (because it's installed in the venv). Without the fix, the install
// code would try to download provider-nested from the registry and fail.
//
// Dependency graph:
//
//	project -> provider (local path) -> provider-nested (declared in packages AND returned by GetRequiredPackages)
func TestRequiredPackagesDeclaredInProjectPackagesNotDownloaded(t *testing.T) {
	t.Parallel()

	ws := &invariantWorkspace{
		t:  t,
		rw: new(sync.RWMutex),
		plugins: map[string]*invariantPlugin{
			"/work/provider": {
				d: workspace.PluginDescriptor{
					Name: "provider",
					Kind: apitype.ResourcePlugin,
				},
				project: &workspace.PluginProject{
					Runtime: workspace.NewProjectRuntimeInfo("python", nil),
					Packages: map[string]workspace.PackageSpec{
						"provider-nested": {Source: "../provider-nested"},
					},
				},
				downloaded:      true,
				pathVisible:     true,
				projectDetected: true,
				requiredPackages: []workspace.PackageDescriptor{
					{
						PluginDescriptor: workspace.PluginDescriptor{
							Name:    "provider-nested",
							Version: &semver.Version{},
							Kind:    apitype.ResourcePlugin,
						},
					},
				},
			},
			"/work/provider-nested": {
				d: workspace.PluginDescriptor{
					Name: "provider-nested",
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

	_, err := packageinstallation.InstallProjectPlugins(t.Context(), &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("python", nil),
		Packages: map[string]workspace.PackageSpec{
			"provider": {Source: "../provider"},
		},
	}, "/work/project", packageinstallation.Options{
		Options: packageresolution.Options{
			ResolveVersionWithLocalWorkspace:           true,
			AllowNonInvertableLocalWorkspaceResolution: true,
		},
		Concurrency: 1,
	}, nil, rws)
	require.NoError(t, err)

	providerPath := "/work/provider"
	providerNestedPath := "/work/provider-nested"

	require.True(t, ws.plugins[providerPath].installed,
		"provider should be installed")
	require.True(t, ws.plugins[providerNestedPath].installed,
		"provider-nested should be installed (via provider's packages)")
}

// TestInstallPluginSet tests that InstallPluginSet installs the resolved plugin
// descriptors directly while generating and linking local SDKs for the package specs.
func TestInstallPluginSet(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-descriptor",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-spec",
				Kind: apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	_, err := packageinstallation.InstallPluginSet(t.Context(),
		[]workspace.PackageDescriptor{
			{
				PluginDescriptor: workspace.PluginDescriptor{
					Name:              "plugin-descriptor",
					Version:           &semver.Version{Major: 1},
					Kind:              apitype.ResourcePlugin,
					PluginDownloadURL: "https://example.com/plugin-descriptor.tar.gz",
				},
			},
		},
		[]workspace.PackageSpec{
			{
				Source:            "plugin-spec",
				PluginDownloadURL: "https://example.com/plugin-spec.tar.gz",
			},
		},
		&workspace.Project{
			Name:    "test-project",
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		}, "/project", packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		}, nil, rws)
	require.NoError(t, err)

	descriptorPath := "$HOME/.pulumi/plugins/resource-plugin-descriptor-v1.0.0"
	specPath := "$HOME/.pulumi/plugins/resource-plugin-spec"

	require.True(t, ws.plugins[descriptorPath].downloaded, "descriptor plugin should be downloaded")
	require.True(t, ws.plugins[specPath].downloaded, "spec plugin should be downloaded")
}

// TestInstallPluginSetWithSkipLink verifies that Options.SkipLink also applies to the
// InstallPluginSet step: the local SDK should be generated but should not be linked into the project.
func TestInstallPluginSetWithSkipLink(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "plugin-descriptor",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-spec",
				Kind: apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	_, err := packageinstallation.InstallPluginSet(t.Context(),
		[]workspace.PackageDescriptor{
			{
				PluginDescriptor: workspace.PluginDescriptor{
					Name:              "plugin-descriptor",
					Version:           &semver.Version{Major: 1},
					Kind:              apitype.ResourcePlugin,
					PluginDownloadURL: "https://example.com/plugin-descriptor.tar.gz",
				},
			},
		},
		[]workspace.PackageSpec{
			{
				Source:            "plugin-spec",
				PluginDownloadURL: "https://example.com/plugin-spec.tar.gz",
			},
		},
		&workspace.Project{
			Name:    "test-project",
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		}, "/project", packageinstallation.Options{
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
			SkipLink:    true,
		}, nil, rws)
	require.NoError(t, err)

	// Verify nothing was linked
	assert.Empty(t, ws.downloadedWorkspace["/project"].linked,
		"SkipLink must not link any package into the project")
}

// TestInstallPluginSetRemotePackageOverride checks that a remote (registry)
// package in the project's `packages:` block - the kind written by `pulumi
// package add` - is installed via the normal descriptor download path rather
// than being diverted into local SDK generation. Only local-path packages
// should be diverted to the spec path, so a remote package must not be linked
// as a local SDK into the project.
func TestInstallPluginSetRemotePackageOverride(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name:    "aws",
				Version: &semver.Version{Major: 1},
				Kind:    apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	rws := &recordingWorkspace{ws, nil}
	defer rws.save(t)

	_, err := packageinstallation.InstallPluginSet(t.Context(),
		[]workspace.PackageDescriptor{
			{
				PluginDescriptor: workspace.PluginDescriptor{
					Name:              "aws",
					Version:           &semver.Version{Major: 1},
					Kind:              apitype.ResourcePlugin,
					PluginDownloadURL: "https://example.com/aws.tar.gz",
				},
			},
		},
		nil,
		&workspace.Project{
			Name:    "test-project",
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
			// A registry package whose name matches the descriptor, but whose
			// source is not a local path.
			Packages: map[string]workspace.PackageSpec{
				"aws": {
					Source:            "aws",
					Version:           "1.0.0",
					PluginDownloadURL: "https://example.com/aws.tar.gz",
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

	awsPath := "$HOME/.pulumi/plugins/resource-aws-v1.0.0"

	require.True(t, ws.plugins[awsPath].downloaded, "remote package plugin should be downloaded")
	require.Empty(t, ws.downloadedWorkspace["/project"].linked,
		"remote package must not be linked as a local SDK into the project")
}

func TestInstallContinuationSkipsDuplicateWork(t *testing.T) {
	t.Parallel()

	// Both installs run against the same workspace, modeling the shared plugin
	// storage that a real second install resumes from.
	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-spec",
				Kind: apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	specs := []workspace.PackageSpec{
		{
			Source:            "plugin-spec",
			PluginDownloadURL: "https://example.com/plugin-spec.tar.gz",
		},
	}
	proj := &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
	}
	options := func(c packageinstallation.State) packageinstallation.Options {
		return packageinstallation.Options{
			PriorState: c,
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		}
	}

	// First install performs the work and records a continuation.
	continuation, err := packageinstallation.InstallPluginSet(t.Context(),
		nil, specs, proj, "/project", options(packageinstallation.State{}), nil, ws)
	require.NoError(t, err)

	// Replay the same install against the same workspace, passing the continuation.
	// None of the previously performed work should be repeated.
	replay := &recordingWorkspace{ws, nil}
	defer replay.save(t)
	_, replayErr := packageinstallation.InstallPluginSet(t.Context(),
		nil, specs, proj, "/project", options(continuation), nil, replay)
	require.NoError(t, replayErr)
}

// TestInstallContinuationLinksDifferentParameterization verifies that the
// continuation does not over-dedup: a different package with an already
// recorded plugin in the continuation must still be linked.
func TestInstallContinuationLinksDifferentParameterization(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-spec",
				Kind: apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	specWithParam := func(param string) []workspace.PackageSpec {
		return []workspace.PackageSpec{
			{
				Source:            "plugin-spec",
				PluginDownloadURL: "https://example.com/plugin-spec.tar.gz",
				Parameters:        []string{param},
			},
		}
	}
	proj := &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
	}
	options := func(c packageinstallation.State) packageinstallation.Options {
		return packageinstallation.Options{
			PriorState: c,
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		}
	}

	// First install links the package parameterized with "paramA".
	continuation, err := packageinstallation.InstallPluginSet(t.Context(),
		nil, specWithParam("paramA"), proj, "/project", options(packageinstallation.State{}), nil, ws)
	require.NoError(t, err)

	// Replaying with a different parameterization must still link the package,
	// even though the underlying plugin is already installed.
	replay := &recordingWorkspace{ws, nil}
	defer replay.save(t)
	_, replayErr := packageinstallation.InstallPluginSet(t.Context(),
		nil, specWithParam("paramB"), proj, "/project", options(continuation), nil, replay)
	require.NoError(t, replayErr)

	require.Len(t, ws.downloadedWorkspace["/project"].linked, 2,
		"a package with a different parameterization must still be linked")
}

// TestInstallContinuationLinksDifferentProjectDir verifies that the continuation
// does not over-dedup across projects: the same package must still be linked into
// a different project directory than the one recorded in the continuation.
func TestInstallContinuationLinksDifferentProjectDir(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project1", "/project2"}, nil, []invariantPlugin{
		{
			d: workspace.PluginDescriptor{
				Name: "plugin-spec",
				Kind: apitype.ResourcePlugin,
			},
			hasBinary: true,
		},
	})

	specs := []workspace.PackageSpec{
		{
			Source:            "plugin-spec",
			PluginDownloadURL: "https://example.com/plugin-spec.tar.gz",
		},
	}
	proj := &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
	}
	options := func(c packageinstallation.State) packageinstallation.Options {
		return packageinstallation.Options{
			PriorState: c,
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			Concurrency: 1,
		}
	}

	// First install links the package into /project1.
	continuation, err := packageinstallation.InstallPluginSet(t.Context(),
		nil, specs, proj, "/project1", options(packageinstallation.State{}), nil, ws)
	require.NoError(t, err)

	// Replaying the same package into a different project dir must still link it
	// there, even though the plugin is already installed.
	replay := &recordingWorkspace{ws, nil}
	defer replay.save(t)
	_, replayErr := packageinstallation.InstallPluginSet(t.Context(),
		nil, specs, proj, "/project2", options(continuation), nil, replay)
	require.NoError(t, replayErr)

	require.Len(t, ws.downloadedWorkspace["/project2"].linked, 1,
		"the same package must still be linked into a different project dir")
}

// TestInstallContinuationAcrossInstallsDoesNotCycle reproduces a bug where the
// node recorded in a Continuation was reused in a subsequent install's DAG. Since
// a pdag.Node is just an index into the DAG that created it, reusing it in a new
// DAG references an unrelated node and can manufacture a spurious cycle.
func TestInstallContinuationAcrossInstallsDoesNotCycle(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, []string{"/project"}, nil, []invariantPlugin{
		{d: workspace.PluginDescriptor{Name: "spec", Kind: apitype.ResourcePlugin}, hasBinary: true},
		{d: workspace.PluginDescriptor{Name: "extra1", Kind: apitype.ResourcePlugin}, hasBinary: true},
		{d: workspace.PluginDescriptor{Name: "extra2", Kind: apitype.ResourcePlugin}, hasBinary: true},
	})

	specs := []workspace.PackageSpec{
		{Source: "spec", PluginDownloadURL: "https://example.com/spec.tar.gz"},
	}
	descriptors := []workspace.PackageDescriptor{
		{PluginDescriptor: workspace.PluginDescriptor{
			Name: "extra1", Kind: apitype.ResourcePlugin, PluginDownloadURL: "https://example.com/extra1.tar.gz",
		}},
		{PluginDescriptor: workspace.PluginDescriptor{
			Name: "extra2", Kind: apitype.ResourcePlugin, PluginDownloadURL: "https://example.com/extra2.tar.gz",
		}},
	}
	proj := &workspace.Project{
		Name:    "test-project",
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
	}
	options := func(c packageinstallation.State) packageinstallation.Options {
		return packageinstallation.Options{
			PriorState: c,
			Options: packageresolution.Options{
				ResolveVersionWithLocalWorkspace:           true,
				AllowNonInvertableLocalWorkspaceResolution: true,
			},
			// Concurrency 1 makes node-index assignment deterministic so this
			// reproduction is stable.
			Concurrency: 1,
		}
	}

	continuation, err := packageinstallation.InstallPluginSet(t.Context(),
		nil, specs, proj, "/project", options(packageinstallation.State{}), nil, ws)
	require.NoError(t, err)

	// The second install reuses the continuation. The spec's plugin is already
	// recorded there, so it must not be re-linked to a stale node from the first
	// install's DAG.
	_, err = packageinstallation.InstallPluginSet(t.Context(),
		descriptors, specs, proj, "/project", options(continuation), nil, ws)
	require.NoError(t, err)
}
