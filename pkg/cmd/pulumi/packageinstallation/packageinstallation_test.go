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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

// Check that [packageworkspace.Workspace] implements [packageinstallation.Workspace]
// without importing [packageworkspace] from [packageinstallation].
var _ packageinstallation.Workspace = packageworkspace.Workspace{}

func TestInstallAlreadyInstalledPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, invariantPlugin{
		d: workspace.PluginDescriptor{
			Name: "already-installed",
			Kind: apitype.ResourcePlugin,
		},
		downloaded: true,
		installed:  true,
		hasBinary:  true,
	})
	rws := &recordingWorkspace{ws, nil}

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
	run(t.Context(), "/tmp")

	rws.save(t)
}

func TestInstallExternalBinaryPackage(t *testing.T) {
	t.Parallel()

	ws := newInvariantWorkspace(t, invariantPlugin{
		d: workspace.PluginDescriptor{
			Name: "external-package",
			Kind: apitype.ResourcePlugin,
		},
		hasBinary: true,
	})
	rws := &recordingWorkspace{ws, nil}

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
	run(t.Context(), "/tmp")

	rws.save(t)
}

// func TestInstallPluginWithParameterizedDependency(t *testing.T) {
// 	t.Parallel()

// 	replayInstallPlugin(t, replayInstallPluginArgs{
// 		spec: workspace.PackageSpec{
// 			Source:            "plugin-a",
// 			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
// 		},
// 		options: packageinstallation.Options{
// 			Options: packageresolution.Options{
// 				ResolveVersionWithLocalWorkspace:           true,
// 				AllowNonInvertableLocalWorkspaceResolution: true,
// 			},
// 			Concurrency: 1,
// 		},
// 	},
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		HasPluginGTE(func(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
// 			return false, nil, nil
// 		}),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		StandardGetLatestVersion(&semver.Version{Major: 4}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-b": {
// 					Source:     "plugin-b",
// 					Parameters: []string{"param1", "param2"},
// 					Version:    "3.0.0",
// 				},
// 			},
// 		),
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{},
// 		),
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardRunPackage,
// 	)
// }

// // TestInstallPluginWithDiamondDependency tests that we de-duplicate dependencies that are
// // seen multiple times in the graph.
// //
// // Here, our graph is:
// //
// //	A -> B -> D
// //	A -> C -> D
// //
// // We test that we only install D once, and that we do so before both B and C (and A).
// func TestInstallPluginWithDiamondDependency(t *testing.T) {
// 	t.Parallel()

// 	replayInstallPlugin(t, replayInstallPluginArgs{
// 		spec: workspace.PackageSpec{
// 			Source:            "plugin-a",
// 			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
// 		},
// 		options: packageinstallation.Options{
// 			Concurrency: 1,
// 		},
// 	},
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 1}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-b": {Source: "plugin-b"},
// 				"plugin-c": {Source: "plugin-c"},
// 			},
// 		),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 2}),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 3}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-d": {Source: "plugin-d"},
// 			},
// 		),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-d": {Source: "plugin-d"},
// 			},
// 		),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 3}),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(spec workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 3}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{},
// 		),
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardRunPackage,
// 	)
// }

// // TestDeduplicateRegistryBasedPlugin tests that we correctly deduplicate packages when
// // two different package names resolve to the same underlying package via registry resolution.
// //
// // Here, our graph is:
// //
// //	A -> B -> C
// //	A -> D
// //
// // Where C and D are different package names that both resolve to the same underlying package
// // (pulumi/shared-plugin v1.0.0) via registry resolution. We verify that we only download and
// // install the shared plugin once.
// func TestDeduplicateRegistryBasedPlugin(t *testing.T) {
// 	t.Parallel()

// 	sharedPluginURL := "https://registry.example.com/shared-plugin-1.0.0.tar.gz"

// 	replayInstallPlugin(t, replayInstallPluginArgs{
// 		spec: workspace.PackageSpec{
// 			Source:            "plugin-a",
// 			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
// 		},
// 		options: packageinstallation.Options{
// 			Options: packageresolution.Options{
// 				ResolveWithRegistry: true,
// 			},
// 			Concurrency: 1,
// 		},
// 	},
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 1}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject( // A
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-b": {Source: "plugin-b"},
// 				"plugin-d": {Source: "plugin-d"},
// 			},
// 		),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 2}),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		HasPlugin(func(workspace.PluginDescriptor) bool { return false }),
// 		StandardGetLatestVersion(&semver.Version{Major: 2}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-c": {Source: "plugin-c"},
// 			},
// 		),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{},
// 		),

// 		IsExternalURL(func(source string) bool {
// 			return false
// 		}),
// 		ListPackages(func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
// 			assert.Equal(t, "plugin-c", *name)
// 			return func(yield func(apitype.PackageMetadata, error) bool) {
// 				yield(apitype.PackageMetadata{
// 					Source:            "pulumi",
// 					Publisher:         "pulumi",
// 					Name:              "shared-plugin",
// 					Version:           semver.Version{Major: 1},
// 					PluginDownloadURL: sharedPluginURL,
// 				}, nil)
// 			}
// 		}),
// 		HasPluginGTE(func(spec workspace.PluginDescriptor) (bool, *semver.Version, error) { return false, nil, nil }),
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		// StandardLinkPackage,
// 		// StandardInstallPluginAt,
// 		// StandardLinkPackage,
// 		// StandardInstallPluginAt,
// 		// StandardRunPackage,
// 	)
// }

// // TestInstallPluginWithCyclicDependency tests that we correctly detect and report cyclic
// // dependencies in the plugin graph.
// //
// // Here, we set up dependencies:
// //
// //	A -> B
// //	B -> C
// //	C -> A
// //
// // The cycle is detected and reported as: B -> A -> C -> B (starting from B, the first
// // dependency encountered).
// func TestInstallPluginWithCyclicDependency(t *testing.T) {
// 	t.Parallel()
// 	t.Skip("TODO")

// 	ws := replayWorkspace{
// 		t: t,
// 		steps: []replayStep{
// 			IsExternalURL(func(source string) bool {
// 				return true
// 			}),
// 			StandardDownloadPlugin,
// 			StandardDetectPluginPathAt,
// 			StandardLoadPluginProject(
// 				workspace.NewProjectRuntimeInfo("go", nil),
// 				map[string]workspace.PackageSpec{
// 					"plugin-b": {Source: "plugin-b"},
// 				},
// 			),
// 			IsExternalURL(func(source string) bool {
// 				return true
// 			}),
// 			StandardDownloadPlugin,
// 			StandardDetectPluginPathAt,
// 			StandardLoadPluginProject(
// 				workspace.NewProjectRuntimeInfo("go", nil),
// 				map[string]workspace.PackageSpec{
// 					"plugin-c": {Source: "plugin-c"},
// 				},
// 			),
// 			IsExternalURL(func(source string) bool {
// 				return true
// 			}),
// 			StandardDownloadPlugin,
// 			StandardDetectPluginPathAt,
// 			StandardLoadPluginProject(
// 				workspace.NewProjectRuntimeInfo("go", nil),
// 				map[string]workspace.PackageSpec{
// 					"plugin-a": {Source: "plugin-a"},
// 				},
// 			),
// 			IsExternalURL(func(source string) bool {
// 				return true
// 			}),
// 			StandardDownloadPlugin,
// 			StandardDetectPluginPathAt,
// 			StandardLoadPluginProject(
// 				workspace.NewProjectRuntimeInfo("go", nil),
// 				map[string]workspace.PackageSpec{
// 					"plugin-b": {Source: "plugin-b"},
// 				},
// 			),
// 			IsExternalURL(func(source string) bool {
// 				return true
// 			}),
// 		},
// 	}

// 	_, err := packageinstallation.InstallPlugin(
// 		t.Context(),
// 		workspace.PackageSpec{
// 			Source:            "plugin-a",
// 			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
// 		},
// 		nil,
// 		"",
// 		packageinstallation.Options{
// 			Concurrency: 1,
// 		},
// 		nil,
// 		&ws,
// 	)

// 	var cyclicErr packageinstallation.ErrorCyclicDependencies
// 	require.ErrorAs(t, err, &cyclicErr)

// 	require.Equal(t, []workspace.PluginDescriptor{
// 		{Name: "plugin-b", Kind: apitype.ResourcePlugin},
// 		{Name: "plugin-a", Kind: apitype.ResourcePlugin},
// 		{Name: "plugin-c", Kind: apitype.ResourcePlugin},
// 		{Name: "plugin-b", Kind: apitype.ResourcePlugin},
// 	}, cyclicErr.Cycle)
// }

// func TestInstallRegistryPackage(t *testing.T) {
// 	t.Parallel()
// 	t.Skip("TODO")

// 	replayInstallPlugin(t, replayInstallPluginArgs{
// 		spec: workspace.PackageSpec{
// 			Source: "registry-package",
// 		},
// 		options: packageinstallation.Options{
// 			Options: packageresolution.Options{
// 				ResolveWithRegistry: true,
// 			},
// 			Concurrency: 1,
// 		},
// 	},
// 		IsExternalURL(func(source string) bool {
// 			return false
// 		}),
// 		ListPackages(func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
// 			return func(yield func(apitype.PackageMetadata, error) bool) {
// 				yield(apitype.PackageMetadata{
// 					Source:            "pulumi",
// 					Publisher:         "pulumi",
// 					Name:              "registry-package",
// 					Version:           semver.Version{Major: 1},
// 					PluginDownloadURL: "https://registry.example.com/registry-package-1.0.0.tar.gz",
// 				}, nil)
// 			}
// 		}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{},
// 		),
// 		StandardInstallPluginAt,
// 		StandardRunPackage,
// 	)
// }

// // TestInstallInProjectWithSharedDependency tests installing dependencies in a project where
// // the project depends on both a git-based plugin and a binary plugin, and the git-based
// // plugin also depends on the same binary plugin.
// //
// // Dependency graph:
// //
// //	P -> A -> B
// //	P -> B
// //
// // Where P is the project, A is a git-based plugin, and B is a binary plugin.
// func TestInstallInProjectWithSharedDependency(t *testing.T) {
// 	t.Parallel()
// 	t.Skip("TODO")

// 	replayInstallInProject(t, replayInstallInProjectArgs{
// 		project: &workspace.Project{
// 			Name:    "test-project",
// 			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
// 		},
// 		projectDir: "/project",
// 		options: packageinstallation.Options{
// 			Concurrency: 1,
// 		},
// 		packages: map[string]workspace.PackageSpec{
// 			"plugin-a": {Source: "plugin-a"},
// 			"plugin-b": {Source: "plugin-b"},
// 		},
// 	},
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		StandardDownloadPlugin,
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"plugin-b": {Source: "plugin-b"},
// 			},
// 		),
// 		StandardDownloadPlugin,
// 		DetectPluginPathAt(func(ctx context.Context, path string) (string, error) {
// 			return "", workspace.ErrPluginNotFound
// 		}),
// 		IsExecutable(func(ctx context.Context, binaryPath string) (bool, error) {
// 			return true, nil
// 		}),
// 		StandardLinkPackage,
// 		IsExternalURL(func(source string) bool {
// 			return true
// 		}),
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 	)
// }

// // TestInstallInProjectWithRelativePaths tests that InstallPluginAt is called with paths
// // correctly resolved relative to projectDir for packages in various relative locations.
// //
// // Dependency graph:
// //
// //	P -> A -> C
// //	P -> B -> C
// //
// // Where:
// //   - P is the project at /work/project
// //   - A is a package at ".." (resolves to /work)
// //   - B is a package at "./pkg-b" (resolves to /work/project/pkg-b)
// //   - C is a shared dependency /work/pkg-c
// func TestInstallInProjectWithRelativePaths(t *testing.T) {
// 	t.Parallel()
// 	t.Skip("TODO")

// 	replayInstallInProject(t, replayInstallInProjectArgs{
// 		project: &workspace.Project{
// 			Name:    "test-project",
// 			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
// 		},
// 		projectDir: "/work/project",
// 		options: packageinstallation.Options{
// 			Concurrency: 1,
// 		},
// 		packages: map[string]workspace.PackageSpec{
// 			"a": {Source: ".."},
// 			"b": {Source: "./pkg-b"},
// 		},
// 	},
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("go", nil),
// 			map[string]workspace.PackageSpec{
// 				"pkg-c": {Source: "./pkg-c"},
// 			},
// 		),
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("nodejs", nil),
// 			map[string]workspace.PackageSpec{
// 				"pkg-c": {Source: "../../pkg-c"},
// 			},
// 		),
// 		StandardDetectPluginPathAt,
// 		StandardLoadPluginProject(
// 			workspace.NewProjectRuntimeInfo("python", nil),
// 			nil,
// 		),
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 		StandardLinkPackage,
// 		StandardInstallPluginAt,
// 		StandardLinkPackage,
// 	)
// }
