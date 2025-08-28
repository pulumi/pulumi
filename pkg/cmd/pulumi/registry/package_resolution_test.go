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
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePackage(t *testing.T) {
	t.Parallel()

	createTestProject := func(t *testing.T) string {
		tmpDir := t.TempDir()
		pulumiYaml := `name: test-project
runtime: nodejs
packages:
  my-local-pkg: ./local-path
  another-local: https://github.com/example/another`

		err := os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o600)
		require.NoError(t, err)
		return tmpDir
	}

	tests := []struct {
		name             string
		env              *PackageResolutionEnv
		pluginSpec       workspace.PluginSpec
		registryResponse func() (*backend.MockCloudRegistry, error)
		setupProject     bool
		expectedStrategy PackageResolutionStrategy
		expectError      bool
	}{
		{
			name:       "found in IDP registry",
			pluginSpec: workspace.PluginSpec{Name: "found-pkg"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
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
			expectedStrategy: RegistryResolution,
			expectError:      false,
		},
		{
			name:       "not found + pre-registry package",
			pluginSpec: workspace.PluginSpec{Name: "aws"}, // aws is in the pre-registry allowlist
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expectedStrategy: LegacyResolution,
			expectError:      false,
		},
		{
			name:       "local project package resolves to local path",
			pluginSpec: workspace.PluginSpec{Name: "my-local-pkg"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			setupProject:     true,
			expectedStrategy: LocalPluginPathResolution,
			expectError:      false,
		},
		{
			name:       "local project package resolves to Git URL",
			pluginSpec: workspace.PluginSpec{Name: "another-local"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			setupProject:     true,
			expectedStrategy: LegacyResolution,
			expectError:      false,
		},
		{
			name:       "Git URL plugin",
			pluginSpec: workspace.PluginSpec{Name: "example-plugin", PluginDownloadURL: "git://github.com/example/plugin"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expectedStrategy: LegacyResolution,
			expectError:      false,
		},
		{
			name:       "not found + no fallback available",
			pluginSpec: workspace.PluginSpec{Name: "unknown-pkg"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expectedStrategy: UnknownPackage,
			expectError:      true,
		},
		{
			name:       "registry error (non-NotFound)",
			pluginSpec: workspace.PluginSpec{Name: "any-pkg"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{}, errors.New("network error"))
						}
					},
				}, nil
			},
			expectedStrategy: UnknownPackage,
			expectError:      true,
		},

		// Environment combination tests for pre-registry packages
		{
			name:       "pre-registry package with registry disabled",
			env:        &PackageResolutionEnv{DisableRegistryResolve: true, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "aws"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expectedStrategy: LegacyResolution,
			expectError:      false,
		},
		{
			name:       "registry disabled ignores available registry package",
			env:        &PackageResolutionEnv{DisableRegistryResolve: true, Experimental: true},
			pluginSpec: workspace.PluginSpec{Name: "aws"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expectedStrategy: LegacyResolution,
			expectError:      false,
		},

		// Environment combination tests for unknown packages
		{
			name:       "unknown package with registry disabled",
			env:        &PackageResolutionEnv{DisableRegistryResolve: true, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "unknown-package"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expectedStrategy: UnknownPackage,
			expectError:      false,
		},
		{
			name:       "unknown package with experimental off",
			env:        &PackageResolutionEnv{DisableRegistryResolve: false, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "unknown-package"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty
					},
				}, nil
			},
			expectedStrategy: UnknownPackage,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var projectRoot string
			if tt.setupProject {
				projectRoot = createTestProject(t)
			} else {
				projectRoot = t.TempDir()
			}

			reg, expectedErr := tt.registryResponse()
			require.NoError(t, expectedErr)

			env := PackageResolutionEnv{
				DisableRegistryResolve: false,
				Experimental:           true,
			}
			if tt.env != nil {
				env = *tt.env
			}

			result := ResolvePackage(
				context.Background(),
				reg,
				tt.pluginSpec,
				projectRoot,
				diagtest.LogSink(t),
				env,
			)

			assert.Equal(t, tt.expectedStrategy, result.Strategy)

			if tt.expectError {
				assert.Error(t, result.Error)
			} else {
				require.NoError(t, result.Error)
			}

			if result.Strategy == RegistryResolution {
				require.NotNil(t, result.Metadata)
				assert.Equal(t, tt.pluginSpec.Name, result.Metadata.Name)
			} else {
				assert.Nil(t, result.Metadata)
			}
		})
	}
}

func TestResolvePackage_WithVersion(t *testing.T) {
	t.Parallel()

	version := semver.Version{Major: 2, Minor: 1, Patch: 0}
	pluginSpec := workspace.PluginSpec{Name: "versioned-pkg", Version: &version}
	reg := &backend.MockCloudRegistry{
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

	result := ResolvePackage(
		context.Background(),
		reg,
		pluginSpec,
		t.TempDir(),
		diagtest.LogSink(t),
		PackageResolutionEnv{
			DisableRegistryResolve: false,
			Experimental:           true,
		},
	)

	assert.Equal(t, RegistryResolution, result.Strategy)
	require.NoError(t, result.Error)
	require.NotNil(t, result.Metadata)
	assert.Equal(t, version, result.Metadata.Version)
}

func TestResolutionStrategyPrecedence(t *testing.T) {
	t.Parallel()

	// Test that local packages take precedence over pre-registry packages
	tmpDir := t.TempDir()
	pulumiYaml := `name: test-project
runtime: nodejs
packages:
  aws: ./local-aws-override`

	err := os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o600)
	require.NoError(t, err)

	reg := &backend.MockCloudRegistry{
		ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
			return func(yield func(apitype.PackageMetadata, error) bool) {
				// Return empty - not found in registry
			}
		},
	}

	pluginSpec := workspace.PluginSpec{Name: "aws"}
	result := ResolvePackage(
		context.Background(),
		reg,
		pluginSpec, // This is both pre-registry AND defined locally
		tmpDir,
		diagtest.LogSink(t),
		PackageResolutionEnv{
			DisableRegistryResolve: true,
			Experimental:           false,
		},
	)

	// Should prefer local project (local path) resolution over pre-registry resolution
	assert.Equal(t, LocalPluginPathResolution, result.Strategy)
	require.NoError(t, result.Error)
}
