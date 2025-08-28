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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryResolvePackageWithFallback(t *testing.T) {
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
		packageName      string
		registryResponse func() (*backend.MockCloudRegistry, error)
		setupProject     bool
		expectedFound    bool
		expectedFallback PackageFallbackType
		expectError      bool
	}{
		{
			name:        "found in IDP registry",
			packageName: "found-pkg",
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
			expectedFound:    true,
			expectedFallback: NoFallback,
			expectError:      false,
		},
		{
			name:        "not found + pre-GitHub registry package",
			packageName: "aws", // aws is in the pre-registry whitelist
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expectedFound:    false,
			expectedFallback: PreGitHubRegistryFallback,
			expectError:      false,
		},
		{
			name:        "not found + local project package",
			packageName: "my-local-pkg",
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			setupProject:     true,
			expectedFound:    false,
			expectedFallback: LocalProjectFallback,
			expectError:      false,
		},
		{
			name:        "not found + no fallback available",
			packageName: "unknown-pkg",
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {} // empty - not found
					},
				}, nil
			},
			expectedFound:    false,
			expectedFallback: NoFallback,
			expectError:      true,
		},
		{
			name:        "registry error (non-NotFound)",
			packageName: "any-pkg",
			registryResponse: func() (*backend.MockCloudRegistry, error) {
				return &backend.MockCloudRegistry{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{}, errors.New("network error"))
						}
					},
				}, nil
			},
			expectedFound:    false,
			expectedFallback: NoFallback,
			expectError:      true,
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

			result := TryResolvePackageWithFallback(
				context.Background(),
				reg,
				tt.packageName,
				nil,
				projectRoot,
				diagtest.LogSink(t),
			)

			assert.Equal(t, tt.expectedFound, result.Found)
			assert.Equal(t, tt.expectedFallback, result.FallbackType)

			if tt.expectError {
				assert.Error(t, result.Error)
			} else {
				require.NoError(t, result.Error)
			}

			if result.Found {
				require.NotNil(t, result.Metadata)
				assert.Equal(t, tt.packageName, result.Metadata.Name)
			} else {
				assert.Nil(t, result.Metadata)
			}
		})
	}
}

func TestTryResolvePackageWithFallback_WithVersion(t *testing.T) {
	t.Parallel()

	version := semver.Version{Major: 2, Minor: 1, Patch: 0}
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

	result := TryResolvePackageWithFallback(
		context.Background(),
		reg,
		"versioned-pkg",
		&version,
		t.TempDir(),
		diagtest.LogSink(t),
	)

	assert.True(t, result.Found)
	assert.Equal(t, NoFallback, result.FallbackType)
	require.NoError(t, result.Error)
	require.NotNil(t, result.Metadata)
	assert.Equal(t, version, result.Metadata.Version)
}

func TestIsLocalProjectPackage(t *testing.T) {
	t.Parallel()

	createTestProject := func(t *testing.T) string {
		tmpDir := t.TempDir()
		pulumiYaml := `name: test-project
runtime: go
packages:
  local-pkg: https://github.com/example/local-pkg
  another-pkg: ./local-path`

		pulumiYamlPath := filepath.Join(tmpDir, "Pulumi.yaml")
		err := os.WriteFile(pulumiYamlPath, []byte(pulumiYaml), 0o600)
		require.NoError(t, err)

		return tmpDir
	}

	testCases := []struct {
		name        string
		setupFunc   func(t *testing.T) string
		packageName string
		expected    bool
	}{
		{"package exists in project", createTestProject, "local-pkg", true},
		{"another package exists", createTestProject, "another-pkg", true},
		{"package does not exist", createTestProject, "nonexistent-pkg", false},
		{"invalid project root", func(t *testing.T) string { return "/nonexistent/path" }, "local-pkg", false},
		{"empty package name", createTestProject, "", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			projectRoot := tc.setupFunc(t)
			result := IsLocalProjectPackage(projectRoot, tc.packageName, nil)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsLocalProjectPackageForInstall(t *testing.T) {
	t.Parallel()

	origDir, err := os.Getwd()
	require.NoError(t, err)

	tmpDir := t.TempDir()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	pulumiYaml := `name: test-project
runtime: nodejs
packages:
  install-pkg: ./local-path`

	err = os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte(pulumiYaml), 0o600)
	require.NoError(t, err)

	result := IsLocalProjectPackageForInstall("install-pkg", nil)
	assert.True(t, result)

	result = IsLocalProjectPackageForInstall("nonexistent", nil)
	assert.False(t, result)
}

func TestFallbackTypePrecedence(t *testing.T) {
	t.Parallel()

	// Test that pre-GitHub registry packages take precedence over local packages
	// when both conditions are true
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

	result := TryResolvePackageWithFallback(
		context.Background(),
		reg,
		"aws", // This is both pre-GitHub AND defined locally
		nil,
		tmpDir,
		diagtest.LogSink(t),
	)

	// Should prefer pre-GitHub registry fallback over local project fallback
	assert.False(t, result.Found)
	assert.Equal(t, PreGitHubRegistryFallback, result.FallbackType)
	require.NoError(t, result.Error)
}
