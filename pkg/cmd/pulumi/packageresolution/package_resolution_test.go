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
		env              *Env
		pluginSpec       workspace.PluginSpec
		registryResponse func() (*backend.MockCloudRegistry, error)
		setupProject     bool
		expected         Result
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
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expected: ExternalSourceResult{},
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
			setupProject: true,
			expected:     LocalPathResult{LocalPluginPathAbs: "./local-path"},
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
			setupProject: true,
			expected:     ExternalSourceResult{},
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
			expected: ExternalSourceResult{},
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
			expected: ErrorResult{},
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
			expected: ErrorResult{},
		},

		// Environment combination tests for pre-registry packages
		{
			name:       "pre-registry package with registry disabled",
			env:        &Env{DisableRegistryResolve: true, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "aws"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expected: ExternalSourceResult{},
		},
		{
			name:       "registry disabled ignores available registry package",
			env:        &Env{DisableRegistryResolve: true, Experimental: true},
			pluginSpec: workspace.PluginSpec{Name: "aws"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expected: ExternalSourceResult{},
		},

		// Environment combination tests for unknown packages
		{
			name:       "unknown package with registry disabled",
			env:        &Env{DisableRegistryResolve: true, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "unknown-package"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when disabled")
					},
				}, nil
			},
			expected: ErrorResult{},
		},
		{
			name:       "unknown package with experimental off",
			env:        &Env{DisableRegistryResolve: false, Experimental: false},
			pluginSpec: workspace.PluginSpec{Name: "unknown-package"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty
					},
				}, nil
			},
			expected: ErrorResult{},
		},
		{
			name:       "project source takes precedence over plugin name",
			pluginSpec: workspace.PluginSpec{Name: "my-local-pkg", PluginDownloadURL: "git://github.com/should-not-use/this"},
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						panic("Registry should not be queried when project source is available")
					},
				}, nil
			},
			setupProject: true,
			expected:     LocalPathResult{LocalPluginPathAbs: "./local-path"},
		},
	}

	for _, tt := range tests {
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

			env := Env{
				DisableRegistryResolve: false,
				Experimental:           true,
			}
			if tt.env != nil {
				env = *tt.env
			}

			var projectRootArg string
			if tt.setupProject {
				projectRootArg = projectRoot
			}
			result := Resolve(
				context.Background(),
				reg,
				tt.pluginSpec,
				diagtest.LogSink(t),
				env,
				projectRootArg,
			)

			switch tt.expected.(type) {
			case ErrorResult:
				errorRes, ok := result.(ErrorResult)
				require.True(t, ok, "Expected ErrorResult")
				assert.Error(t, errorRes.Error)
			default:
				assert.Equal(t, tt.expected, result)
			}

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

	result := Resolve(
		context.Background(),
		reg,
		pluginSpec,
		diagtest.LogSink(t),
		Env{
			DisableRegistryResolve: false,
			Experimental:           true,
		},
		t.TempDir(),
	)

	res, ok := result.(RegistryResult)
	require.True(t, ok, "Expected RegistryResult but got %T", result)
	assert.Equal(t, version, res.Metadata.Version)
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
	result := Resolve(
		context.Background(),
		reg,
		pluginSpec, // This is both pre-registry AND defined locally
		diagtest.LogSink(t),
		Env{
			DisableRegistryResolve: true,
			Experimental:           false,
		},
		tmpDir,
	)

	assert.Equal(t, LocalPathResult{LocalPluginPathAbs: "./local-aws-override"}, result)
}

func TestGetLocalProjectPackageSource_NilPackages(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pulumiYaml := `name: test-project
runtime: nodejs`

	err := os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o600)
	require.NoError(t, err)

	source := getLocalProjectPackageSource(tmpDir, "some-package")
	assert.Empty(t, source)
}

func TestGetLocalProjectPackageSource_PackageNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pulumiYaml := `name: test-project
runtime: nodejs
packages:
  existing-package: ./local-path`

	err := os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o600)
	require.NoError(t, err)

	source := getLocalProjectPackageSource(tmpDir, "non-existent-package")
	assert.Empty(t, source)
}
