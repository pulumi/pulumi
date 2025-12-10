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
	"iter"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageresolution"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestInstallAlreadyInstalledPackage(t *testing.T) {
	t.Parallel()

	testReplay(t, testReplayArgs{
		spec: workspace.PluginSpec{
			Name: "already-installed",
			Kind: apitype.ResourcePlugin,
		},
		options: packageinstallation.Options{
			Options: packageresolution.Options{
				IncludeInstalledInWorkspace: true,
			},
			Concurrency: 1,
		},
	},
		HasPluginGTE(func(spec workspace.PluginSpec) (bool, error) {
			return true, nil
		}),
		GetPluginPath(func(ctx context.Context, spec workspace.PluginSpec) (string, error) {
			return "/path/to/plugin", nil
		}),
		RunPackage(func(
			ctx context.Context, pluginDir string,
			params plugin.ParameterizeParameters,
		) (plugin.Provider, error) {
			return nil, nil
		}),
	)
}

func TestInstallExternalBinaryPackage(t *testing.T) {
	t.Parallel()

	testReplay(t, testReplayArgs{
		spec: workspace.PluginSpec{
			Name:              "external-package",
			Kind:              apitype.ResourcePlugin,
			PluginDownloadURL: "https://example.com/external-package.tar.gz",
		},
		options: packageinstallation.Options{
			Options: packageresolution.Options{
				IncludeInstalledInWorkspace: true,
			},
			Concurrency: 1,
		},
	},
		HasPluginGTE(func(spec workspace.PluginSpec) (bool, error) {
			return false, nil
		}),
		IsExternalURL(func(source string) bool {
			return true
		}),
		StandardDownloadPlugin,
		DetectPluginPathAt(func(ctx context.Context, path string) (string, error) {
			return "", workspace.ErrPluginNotFound
		}),
		IsExecutable(func(ctx context.Context, binaryPath string) (bool, error) {
			return true, nil
		}),
		StandardRunPackage,
	)
}

func TestInstallPluginWithParameterizedDependency(t *testing.T) {
	t.Parallel()

	testReplay(t, testReplayArgs{
		spec: workspace.PluginSpec{
			Name:              "plugin-a",
			Kind:              apitype.ResourcePlugin,
			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
		},
		options: packageinstallation.Options{
			Options: packageresolution.Options{
				IncludeInstalledInWorkspace: true,
			},
			Concurrency: 1,
		},
	},
		HasPluginGTE(func(spec workspace.PluginSpec) (bool, error) {
			return false, nil
		}),
		IsExternalURL(func(source string) bool {
			return true
		}),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{
				"plugin-b": {
					Source:     "plugin-b",
					Parameters: []string{"param1", "param2"},
				},
			},
		),
		HasPluginGTE(func(spec workspace.PluginSpec) (bool, error) {
			return false, nil
		}),
		IsExternalURL(func(source string) bool {
			return true
		}),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{},
		),
		StandardInstallPluginAt,
		StandardLinkPackage,
		StandardInstallPluginAt,
		StandardRunPackage,
	)
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

	testReplay(t, testReplayArgs{
		spec: workspace.PluginSpec{
			Name:              "plugin-a",
			Kind:              apitype.ResourcePlugin,
			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
		},
		options: packageinstallation.Options{
			Concurrency: 1,
		},
	},
		IsExternalURL(func(source string) bool {
			return true
		}),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{
				"plugin-b": {Source: "plugin-b"},
				"plugin-c": {Source: "plugin-c"},
			},
		),
		IsExternalURL(func(source string) bool {
			return true
		}),
		IsExternalURL(func(source string) bool {
			return true
		}),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{
				"plugin-d": {Source: "plugin-d"},
			},
		),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{
				"plugin-d": {Source: "plugin-d"},
			},
		),
		IsExternalURL(func(source string) bool {
			return true
		}),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{},
		),
		StandardInstallPluginAt,
		StandardLinkPackage,
		StandardInstallPluginAt,
		StandardLinkPackage,
		StandardLinkPackage,
		StandardInstallPluginAt,
		StandardLinkPackage,
		StandardInstallPluginAt,
		StandardRunPackage,
	)
}

func TestDeduplicateRegistryBasedPlugin(t *testing.T) {
	t.Parallel()
	t.Skip(`TODO: Add a test showing that if we have packages

	A -> B -> C
	A -> D

And both C and D are the same underlying package after registry resolution, that we
correctly handle C/D: Only downloading and installing once.`)
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
// The cycle is detected and reported as: B -> A -> C -> B (starting from B, the first
// dependency encountered).
func TestInstallPluginWithCyclicDependency(t *testing.T) {
	t.Parallel()

	ws := replayWorkspace{
		t: t,
		steps: []replayStep{
			IsExternalURL(func(source string) bool {
				return true
			}),
			StandardDownloadPlugin,
			StandardDetectPluginPathAt,
			StandardLoadPluginProject(
				workspace.NewProjectRuntimeInfo("go", nil),
				map[string]workspace.PackageSpec{
					"plugin-b": {Source: "plugin-b"},
				},
			),
			IsExternalURL(func(source string) bool {
				return true
			}),
			StandardDownloadPlugin,
			StandardDetectPluginPathAt,
			StandardLoadPluginProject(
				workspace.NewProjectRuntimeInfo("go", nil),
				map[string]workspace.PackageSpec{
					"plugin-c": {Source: "plugin-c"},
				},
			),
			IsExternalURL(func(source string) bool {
				return true
			}),
			StandardDownloadPlugin,
			StandardDetectPluginPathAt,
			StandardLoadPluginProject(
				workspace.NewProjectRuntimeInfo("go", nil),
				map[string]workspace.PackageSpec{
					"plugin-a": {Source: "plugin-a"},
				},
			),
			IsExternalURL(func(source string) bool {
				return true
			}),
			StandardDownloadPlugin,
			StandardDetectPluginPathAt,
			StandardLoadPluginProject(
				workspace.NewProjectRuntimeInfo("go", nil),
				map[string]workspace.PackageSpec{
					"plugin-b": {Source: "plugin-b"},
				},
			),
		},
	}

	_, err := packageinstallation.Install(
		t.Context(),
		workspace.PluginSpec{
			Name:              "plugin-a",
			Kind:              apitype.ResourcePlugin,
			PluginDownloadURL: "https://example.com/plugin-a.tar.gz",
		},
		nil,
		"",
		packageinstallation.Options{
			Concurrency: 1,
		},
		nil,
		&ws,
	)

	var cyclicErr packageinstallation.ErrorCyclicDependencies
	require.ErrorAs(t, err, &cyclicErr)

	require.Equal(t, []workspace.PluginSpec{
		{Name: "plugin-b", Kind: apitype.ResourcePlugin},
		{Name: "plugin-a", Kind: apitype.ResourcePlugin},
		{Name: "plugin-c", Kind: apitype.ResourcePlugin},
		{Name: "plugin-b", Kind: apitype.ResourcePlugin},
	}, cyclicErr.Cycle)
}

func TestInstallRegistryPackage(t *testing.T) {
	t.Parallel()

	testReplay(t, testReplayArgs{
		spec: workspace.PluginSpec{
			Name: "registry-package",
			Kind: apitype.ResourcePlugin,
		},
		options: packageinstallation.Options{
			Options: packageresolution.Options{
				Experimental: true,
			},
			Concurrency: 1,
		},
	},
		IsExternalURL(func(source string) bool {
			return false
		}),
		ListPackages(func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				yield(apitype.PackageMetadata{
					Source:            "pulumi",
					Publisher:         "pulumi",
					Name:              "registry-package",
					Version:           semver.Version{Major: 1},
					PluginDownloadURL: "https://registry.example.com/registry-package-1.0.0.tar.gz",
				}, nil)
			}
		}),
		StandardDownloadPlugin,
		StandardDetectPluginPathAt,
		StandardLoadPluginProject(
			workspace.NewProjectRuntimeInfo("go", nil),
			map[string]workspace.PackageSpec{},
		),
		StandardInstallPluginAt,
		StandardRunPackage,
	)
}
