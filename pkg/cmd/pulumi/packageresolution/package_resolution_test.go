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

package packageresolution

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWorkspace struct {
	hasPlugin    func(spec workspace.PluginSpec) bool
	hasPluginGTE func(spec workspace.PluginSpec) (bool, error)
}

func (m mockWorkspace) HasPlugin(spec workspace.PluginSpec) bool {
	if m.hasPlugin != nil {
		return m.hasPlugin(spec)
	}
	return false
}

func (m mockWorkspace) HasPluginGTE(spec workspace.PluginSpec) (bool, error) {
	if m.hasPluginGTE != nil {
		return m.hasPluginGTE(spec)
	}
	return false, nil
}

func (m mockWorkspace) IsExternalURL(source string) bool {
	return workspace.IsExternalURL(source)
}

func TestResolvePackage(t *testing.T) {
	t.Parallel()

	createTestProject := func(t *testing.T) workspace.BaseProject {
		pulumiYaml := `name: test-project
runtime: nodejs
packages:
  my-local-pkg: ./local-path
  another-local: https://github.com/example/another`
		bp, err := workspace.LoadProjectBytes([]byte(pulumiYaml), filepath.Join("test", "Pulumi.yaml"), encoding.YAML)
		require.NoError(t, err)
		return bp
	}

	tests := []struct {
		name             string
		env              *Options
		pluginSpec       workspace.PluginSpec
		workspace        PluginWorkspace
		registryResponse func() (registry.Registry, error)
		setupProject     bool
		expected         Result
		expectedErr      error
	}{
		{
			name:       "found in IDP registry",
			pluginSpec: workspace.PluginSpec{Name: "found-pkg"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{
								Name:              "found-pkg",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           semver.Version{Major: 1, Minor: 2, Patch: 3},
								PluginDownloadURL: "https://example.com/download",
							}, nil)
						}
					},
				}, nil
			},
			expected: RegistryResult{
				Metadata: apitype.PackageMetadata{
					Name:              "found-pkg",
					Publisher:         "pulumi",
					Source:            "pulumi",
					Version:           semver.Version{Major: 1, Minor: 2, Patch: 3},
					PluginDownloadURL: "https://example.com/download",
				},
			},
		},
		{
			name:       "not found + pre-registry package",
			pluginSpec: workspace.PluginSpec{Name: "aws"}, // aws is in the pre-registry allowlist
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expected: ExternalSourceResult{Spec: workspace.PluginSpec{Name: "aws"}},
		},
		{
			name:       "local project package resolves to local path",
			pluginSpec: workspace.PluginSpec{Name: "my-local-pkg"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			setupProject: true,
			expected:     LocalPathResult{LocalPath: "./local-path", RelativeToWorkspace: true},
		},
		{
			name:       "local path directly in spec (not from project)",
			pluginSpec: workspace.PluginSpec{Name: "./direct-local-path"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expected: LocalPathResult{LocalPath: "./direct-local-path", RelativeToWorkspace: false},
		},
		{
			name:       "local project package resolves to Git URL",
			pluginSpec: workspace.PluginSpec{Name: "another-local"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			setupProject: true,
			expected:     ExternalSourceResult{Spec: workspace.PluginSpec{Name: "another-local"}},
		},
		{
			name:       "Git URL plugin",
			pluginSpec: workspace.PluginSpec{Name: "example-plugin", PluginDownloadURL: "git://github.com/example/plugin"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expected: ExternalSourceResult{
				Spec: workspace.PluginSpec{
					Name:              "example-plugin",
					PluginDownloadURL: "git://github.com/example/plugin",
				},
			},
		},
		{
			name:       "not found + no fallback available",
			pluginSpec: workspace.PluginSpec{Name: "unknown-pkg"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expectedErr: &PackageNotFoundError{Package: "unknown-pkg"},
		},
		{
			name:       "registry error (non-NotFound)",
			pluginSpec: workspace.PluginSpec{Name: "any-pkg"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{}, errors.New("network error"))
						}
					},
				}, nil
			},
			expectedErr: fmt.Errorf("%w: %v", ErrRegistryQuery, errors.New("network error")),
		},

		// Environment combination tests for pre-registry packages
		{
			name:       "pre-registry package with registry disabled",
			env:        &Options{DisableRegistryResolve: true, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "aws"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expected: ExternalSourceResult{Spec: workspace.PluginSpec{Name: "aws"}},
		},
		{
			name:       "registry disabled ignores available registry package",
			env:        &Options{DisableRegistryResolve: true, Experimental: true},
			pluginSpec: workspace.PluginSpec{Name: "aws"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expected: ExternalSourceResult{Spec: workspace.PluginSpec{Name: "aws"}},
		},

		// Environment combination tests for unknown packages
		{
			name:       "unknown package with registry disabled",
			env:        &Options{DisableRegistryResolve: true, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "unknown-package"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expectedErr: &PackageNotFoundError{Package: "unknown-package"},
		},
		{
			name:       "unknown package with experimental off",
			env:        &Options{DisableRegistryResolve: false, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "unknown-package"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty
					},
				}, nil
			},
			expectedErr: &PackageNotFoundError{Package: "unknown-package"},
		},
		{
			name:       "project source takes precedence over plugin name",
			pluginSpec: workspace.PluginSpec{Name: "my-local-pkg", PluginDownloadURL: "git://github.com/should-not-use/this"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when project source is available")
					},
				}, nil
			},
			setupProject: true,
			expected:     LocalPathResult{LocalPath: "./local-path", RelativeToWorkspace: true},
		},
		{
			name:       "installed in workspace with exact version",
			env:        &Options{IncludeInstalledInWorkspace: true, Experimental: true},
			pluginSpec: workspace.PluginSpec{Name: "installed-pkg", Version: &semver.Version{Major: 1, Minor: 2, Patch: 3}},
			workspace: mockWorkspace{
				hasPlugin: func(spec workspace.PluginSpec) bool {
					return spec.Name == "installed-pkg" &&
						spec.Version != nil &&
						spec.Version.EQ(semver.Version{Major: 1, Minor: 2, Patch: 3})
				},
			},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty
					},
				}, nil
			},
			expected: InstalledInWorkspaceResult{},
		},
		{
			name:       "installed in workspace without version (GTE check)",
			env:        &Options{IncludeInstalledInWorkspace: true, Experimental: true},
			pluginSpec: workspace.PluginSpec{Name: "installed-pkg"},
			workspace: mockWorkspace{
				hasPluginGTE: func(spec workspace.PluginSpec) (bool, error) {
					return spec.Name == "installed-pkg", nil
				},
			},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty
					},
				}, nil
			},
			expected: InstalledInWorkspaceResult{},
		},
		{
			name:       "not installed in workspace, fallback to registry",
			env:        &Options{IncludeInstalledInWorkspace: true, Experimental: true},
			pluginSpec: workspace.PluginSpec{Name: "registry-pkg"},
			workspace: mockWorkspace{
				hasPlugin:    func(spec workspace.PluginSpec) bool { return false },
				hasPluginGTE: func(spec workspace.PluginSpec) (bool, error) { return false, nil },
			},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{
								Name:              "registry-pkg",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           semver.Version{Major: 1, Minor: 0, Patch: 0},
								PluginDownloadURL: "https://example.com/download",
							}, nil)
						}
					},
				}, nil
			},
			expected: RegistryResult{
				Metadata: apitype.PackageMetadata{
					Name:              "registry-pkg",
					Publisher:         "pulumi",
					Source:            "pulumi",
					Version:           semver.Version{Major: 1, Minor: 0, Patch: 0},
					PluginDownloadURL: "https://example.com/download",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var project workspace.BaseProject
			if tt.setupProject {
				project = createTestProject(t)
			}

			reg, expectedErr := tt.registryResponse()
			require.NoError(t, expectedErr)

			env := Options{
				DisableRegistryResolve: false,
				Experimental:           true,
			}
			if tt.env != nil {
				env = *tt.env
			}

			ws := tt.workspace
			if ws == nil {
				ws = DefaultWorkspace()
			}

			result, err := Resolve(
				context.Background(),
				reg,
				ws,
				tt.pluginSpec,
				env,
				project,
			)

			if tt.expectedErr != nil {
				if packageNotFoundErr, ok := tt.expectedErr.(*PackageNotFoundError); ok {
					actualErr, actualOk := err.(*PackageNotFoundError)
					require.True(t, actualOk, "Expected PackageNotFoundError but got %T", err)
					assert.Equal(t, packageNotFoundErr.Package, actualErr.Package)
					assert.Equal(t, packageNotFoundErr.Version, actualErr.Version)
				} else {
					assert.Equal(t, tt.expectedErr, err)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)

			switch res := result.(type) {
			case RegistryResult:
				assert.Equal(t, tt.pluginSpec.Name, res.Metadata.Name)
			}
		})
	}
}

func TestResolvePackage_WithVersion(t *testing.T) {
	t.Parallel()

	version := semver.Version{Major: 2, Minor: 1, Patch: 0}
	pluginSpec := workspace.PluginSpec{Name: "versioned-pkg", Version: &version}
	reg := registry.Mock{
		GetPackageF: func(
			ctx context.Context, source, publisher, name string, version *semver.Version,
		) (apitype.PackageMetadata, error) {
			return apitype.PackageMetadata{
				Name:      name,
				Publisher: publisher,
				Source:    source,
				Version:   *version,
			}, nil
		},
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				yield(apitype.PackageMetadata{
					Name:      *name,
					Publisher: "pulumi",
					Source:    "pulumi",
					Version:   version,
				}, nil)
			}
		},
	}

	result, err := Resolve(
		context.Background(),
		reg,
		DefaultWorkspace(),
		pluginSpec,
		Options{
			DisableRegistryResolve: false,
			Experimental:           true,
		},
		nil,
	)
	require.NoError(t, err)

	res, ok := result.(RegistryResult)
	require.True(t, ok, "Expected RegistryResult but got %T", result)
	assert.Equal(t, version, res.Metadata.Version)
}

func TestResolutionStrategyPrecedence(t *testing.T) {
	t.Parallel()

	// Test that local packages take precedence over pre-registry packages
	pulumiYaml := `name: test-project
runtime: nodejs
packages:
  aws: ./local-aws-override`

	project, err := workspace.LoadProjectBytes([]byte(pulumiYaml), filepath.Join("/test", "Pulumi.yaml"), encoding.YAML)
	require.NoError(t, err)

	reg := registry.Mock{
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				// Return empty - not found in registry
			}
		},
	}

	pluginSpec := workspace.PluginSpec{Name: "aws"}
	result, err := Resolve(
		context.Background(),
		reg,
		DefaultWorkspace(),
		pluginSpec, // This is both pre-registry AND defined locally
		Options{
			DisableRegistryResolve: true,
			Experimental:           false,
		},
		project,
	)
	require.NoError(t, err)

	assert.Equal(t, LocalPathResult{LocalPath: "./local-aws-override", RelativeToWorkspace: true}, result)
}
