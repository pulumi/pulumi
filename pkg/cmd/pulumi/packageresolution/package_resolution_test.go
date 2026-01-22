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
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWorkspace struct {
	hasPlugin        func(spec workspace.PluginDescriptor) bool
	hasPluginGTE     func(spec workspace.PluginDescriptor) (bool, *semver.Version, error)
	getLatestVersion func(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error)
}

func (m mockWorkspace) GetLatestVersion(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
	if m.getLatestVersion != nil {
		return m.getLatestVersion(ctx, spec)
	}
	return nil, workspace.ErrGetLatestVersionNotSupported
}

func (m mockWorkspace) HasPlugin(spec workspace.PluginDescriptor) bool {
	if m.hasPlugin != nil {
		return m.hasPlugin(spec)
	}
	return false
}

func (m mockWorkspace) HasPluginGTE(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
	if m.hasPluginGTE != nil {
		return m.hasPluginGTE(spec)
	}
	return false, nil, nil
}

func (m mockWorkspace) IsExternalURL(source string) bool {
	return workspace.IsExternalURL(source)
}

func TestResolvePackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		options          *Options
		pluginSpec       workspace.PackageSpec
		workspace        PluginWorkspace
		registryResponse func() (registry.Registry, error)
		expected         Resolution
		expectedErr      error
	}{
		{
			name:       "found in IDP registry",
			pluginSpec: workspace.PackageSpec{Source: "found-pkg"},
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
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "found-pkg" &&
							version != nil && version.EQ(semver.Version{Major: 1, Minor: 2, Patch: 3}) {
							return apitype.PackageMetadata{
								Name:              "found-pkg",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           semver.Version{Major: 1, Minor: 2, Patch: 3},
								PluginDownloadURL: "https://example.com/download",
							}, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "pulumi/pulumi/found-pkg",
					Version: "1.2.3",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "found-pkg",
					Version:           &semver.Version{Major: 1, Minor: 2, Patch: 3},
					PluginDownloadURL: "https://example.com/download",
					Kind:              apitype.ResourcePlugin,
				}},
			},
		},
		{
			name:       "not found + pre-registry package",
			pluginSpec: workspace.PackageSpec{Source: "aws"}, // aws is in the pre-registry allowlist
			workspace: mockWorkspace{
				getLatestVersion: func(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
					if spec.Name == "aws" {
						return &semver.Version{Major: 7, Minor: 14}, nil
					}
					return nil, nil
				},
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "aws",
					Version: "7.14.0",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:    "aws",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 7, Minor: 14},
				}},
			},
		},
		{
			name:       "local path directly in spec",
			pluginSpec: workspace.PackageSpec{Source: "./direct-local-path"},
			expected: PathResolution{
				Path: "./direct-local-path",
				Spec: workspace.PackageSpec{Source: "./direct-local-path"},
			},
		},
		{
			name:       "git based pluginDownloadURL",
			pluginSpec: workspace.PackageSpec{Source: "example-plugin", PluginDownloadURL: "git://github.com/example/plugin"},
			workspace: mockWorkspace{
				getLatestVersion: func(_ context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
					require.Equal(t, "example-plugin", spec.Name)
					require.Equal(t, "git://github.com/example/plugin", spec.PluginDownloadURL)
					return &semver.Version{Pre: []semver.PRVersion{{VersionStr: "x123456"}}}, nil
				},
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:            "example-plugin",
					Version:           "0.0.0-x123456",
					PluginDownloadURL: "git://github.com/example/plugin",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "example-plugin",
					Kind:              apitype.ResourcePlugin,
					Version:           &semver.Version{Pre: []semver.PRVersion{{VersionStr: "x123456"}}},
					PluginDownloadURL: "git://github.com/example/plugin",
				}},
				InstalledInWorkspace: false,
			},
		},
		{
			// This needs to be a real repo, since workspace.NewPluginDescriptor doesn't facilitate mocking.
			name:       "git based source",
			pluginSpec: workspace.PackageSpec{Source: "github.com/pulumi/component-test-providers/test-provider"},
			workspace:  mockWorkspace{},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "github.com/pulumi/component-test-providers/test-provider",
					Version: "52a8a71555d964542b308da197755c64dbe63352",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name: "github.com_pulumi_component-test-providers.git_test-provider",
					Kind: apitype.ResourcePlugin,
					Version: &semver.Version{Pre: []semver.PRVersion{
						{VersionStr: "x52a8a71555d964542b308da197755c64dbe63352"},
					}},
					PluginDownloadURL: "git://github.com/pulumi/component-test-providers/test-provider",
				}},
			},
		},
		{
			name:        "not found + no fallback available",
			pluginSpec:  workspace.PackageSpec{Source: "unknown-pkg"},
			expectedErr: &PackageNotFoundError{Package: "unknown-pkg"},
		},
		{
			name:       "registry error (non-NotFound)",
			pluginSpec: workspace.PackageSpec{Source: "any-pkg"},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{}, errors.New("network error"))
						}
					},
				}, nil
			},
			expectedErr: fmt.Errorf("%w: %w", ErrRegistryQuery, errors.New("network error")),
		},
		{
			name:       "pre-registry package with registry disabled",
			options:    &Options{ResolveWithRegistry: false},
			pluginSpec: workspace.PackageSpec{Source: "aws"},
			workspace: mockWorkspace{
				getLatestVersion: func(ctx context.Context, spec workspace.PluginDescriptor) (*semver.Version, error) {
					if spec.Name != "aws" {
						panic(fmt.Sprintf("unexpected spec: %#v", spec))
					}
					return &semver.Version{Major: 7, Minor: 14}, nil
				},
			},
			expected: PackageResolution{
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:    "aws",
					Version: &semver.Version{Major: 7, Minor: 14},
					Kind:    apitype.ResourcePlugin,
				}},
				Spec: workspace.PackageSpec{
					Source:  "aws",
					Version: "7.14.0",
				},
			},
		},
		{
			name:       "unknown package with registry disabled",
			options:    &Options{ResolveWithRegistry: false},
			pluginSpec: workspace.PackageSpec{Source: "unknown-package"},
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
			name:        "unknown package with registry disabled",
			options:     &Options{ResolveWithRegistry: false},
			pluginSpec:  workspace.PackageSpec{Source: "unknown-package"},
			expectedErr: &PackageNotFoundError{Package: "unknown-package"},
		},
		{
			name: "installed in workspace with exact version",
			options: &Options{
				AllowNonInvertableLocalWorkspaceResolution: true,
				ResolveWithRegistry:                        true,
			},
			pluginSpec: workspace.PackageSpec{
				Source: "installed-pkg", Version: "1.2.3",
			},
			workspace: mockWorkspace{
				hasPlugin: func(spec workspace.PluginDescriptor) bool {
					return spec.Name == "installed-pkg" &&
						spec.Version != nil &&
						spec.Version.EQ(semver.Version{Major: 1, Minor: 2, Patch: 3})
				},
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "installed-pkg",
					Version: "1.2.3",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:    "installed-pkg",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 1, Minor: 2, Patch: 3},
				}},
				InstalledInWorkspace: true,
			},
		},
		{
			name: "installed in workspace without version (GTE check)",
			options: &Options{
				AllowNonInvertableLocalWorkspaceResolution: true,
				ResolveWithRegistry:                        true,
			},
			pluginSpec: workspace.PackageSpec{Source: "installed-pkg"},
			workspace: mockWorkspace{
				hasPluginGTE: func(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
					return spec.Name == "installed-pkg", &semver.Version{Major: 3}, nil
				},
				hasPlugin: func(spec workspace.PluginDescriptor) bool {
					return spec.Name == "installed-pkg" &&
						spec.Version != nil && spec.Version.EQ(semver.Version{Major: 3})
				},
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "installed-pkg",
					Version: "3.0.0",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:    "installed-pkg",
					Kind:    apitype.ResourcePlugin,
					Version: &semver.Version{Major: 3},
				}},
				InstalledInWorkspace: true,
			},
		},
		{
			name:       "not installed in workspace, fallback to registry",
			options:    &Options{ResolveWithRegistry: true},
			pluginSpec: workspace.PackageSpec{Source: "registry-pkg"},
			workspace:  mockWorkspace{},
			registryResponse: func() (registry.Registry, error) {
				meta := apitype.PackageMetadata{
					Name:              "registry-pkg",
					Publisher:         "pulumi",
					Source:            "pulumi",
					Version:           semver.Version{Major: 1},
					PluginDownloadURL: "https://example.com/download",
				}
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "registry-pkg" &&
							version.EQ(semver.Version{Major: 1}) {
							return meta, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(meta, nil)
						}
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "pulumi/pulumi/registry-pkg",
					Version: "1.0.0",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "registry-pkg",
					Kind:              apitype.ResourcePlugin,
					Version:           &semver.Version{Major: 1, Minor: 0, Patch: 0},
					PluginDownloadURL: "https://example.com/download",
				}},
			},
		},
		{
			name:       "registry name",
			options:    &Options{ResolveWithRegistry: true},
			pluginSpec: workspace.PackageSpec{Source: "org/registry-pkg"},
			workspace:  mockWorkspace{},
			registryResponse: func() (registry.Registry, error) {
				meta := apitype.PackageMetadata{
					Name:              "registry-pkg",
					Publisher:         "org",
					Source:            "private",
					Version:           semver.Version{Major: 2},
					PluginDownloadURL: "https://example.com/download",
				}
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "private" && publisher == "org" && name == "registry-pkg" &&
							(version == nil || version.EQ(semver.Version{Major: 2})) {
							return meta, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(meta, nil)
						}
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "private/org/registry-pkg",
					Version: "2.0.0",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "registry-pkg",
					Kind:              apitype.ResourcePlugin,
					Version:           &semver.Version{Major: 2, Minor: 0, Patch: 0},
					PluginDownloadURL: "https://example.com/download",
				}},
			},
		},
		{
			name: "spec with parameters returns PluginResolution",
			pluginSpec: workspace.PackageSpec{
				Source:     "param-pkg",
				Parameters: []string{"arg1"},
			},
			workspace: mockWorkspace{},
			registryResponse: func() (registry.Registry, error) {
				meta := apitype.PackageMetadata{
					Name:              "param-pkg",
					Publisher:         "pulumi",
					Source:            "pulumi",
					Version:           semver.Version{Major: 1, Minor: 0, Patch: 0},
					PluginDownloadURL: "https://example.com/param-pkg",
				}
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "param-pkg" {
							return meta, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(meta, nil)
						}
					},
				}, nil
			},
			expected: PluginResolution{
				Spec: workspace.PackageSpec{
					Source:     "pulumi/pulumi/param-pkg",
					Version:    "1.0.0",
					Parameters: []string{"arg1"},
				},
				Pkg: workspace.UnresolvedPackageDescriptor{
					PluginDescriptor: workspace.PluginDescriptor{
						Name:              "param-pkg",
						Kind:              apitype.ResourcePlugin,
						Version:           &semver.Version{Major: 1, Minor: 0, Patch: 0},
						PluginDownloadURL: "https://example.com/param-pkg",
					},
					ParameterizationArgs: []string{"arg1"},
				},
			},
		},
		{
			name: "spec with parameters + metadata with parameterization returns error",
			pluginSpec: workspace.PackageSpec{
				Source:     "already-param-pkg",
				Parameters: []string{"arg1"},
			},
			workspace: mockWorkspace{},
			registryResponse: func() (registry.Registry, error) {
				meta := apitype.PackageMetadata{
					Name:      "already-param-pkg",
					Publisher: "pulumi",
					Source:    "pulumi",
					Version:   semver.Version{Major: 1, Minor: 0, Patch: 0},
					Parameterization: &apitype.PackageParameterization{
						BaseProvider: apitype.ArtifactVersionNameSpec{
							Name:      "base-pkg",
							Publisher: "pulumi",
							Source:    "pulumi",
							Version:   semver.Version{Major: 1, Minor: 0, Patch: 0},
						},
						Parameter: []byte(`{"existing":"param"}`),
					},
					PluginDownloadURL: "https://example.com/already-param-pkg",
				}
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "already-param-pkg" {
							return meta, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(meta, nil)
						}
					},
				}, nil
			},
			expectedErr: fmt.Errorf(
				"unable to resolve package: resolved plugin to %s, which is already parameterized",
				"pulumi/pulumi/already-param-pkg",
			),
		},
		{
			name:       "metadata with parameterization populates Pkg.Parameterization",
			pluginSpec: workspace.PackageSpec{Source: "parameterized-pkg"},
			workspace:  mockWorkspace{},
			registryResponse: func() (registry.Registry, error) {
				meta := apitype.PackageMetadata{
					Name:      "parameterized-pkg",
					Publisher: "pulumi",
					Source:    "pulumi",
					Version:   semver.Version{Major: 2, Minor: 1, Patch: 0},
					Parameterization: &apitype.PackageParameterization{
						BaseProvider: apitype.ArtifactVersionNameSpec{
							Name:      "base-provider",
							Publisher: "pulumi",
							Source:    "pulumi",
							Version:   semver.Version{Major: 1, Minor: 5, Patch: 0},
						},
						Parameter: []byte(`{"region":"eu-west-1","tier":"premium"}`),
					},
					PluginDownloadURL: "https://example.com/parameterized-pkg",
				}
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "parameterized-pkg" {
							return meta, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(meta, nil)
						}
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "pulumi/pulumi/parameterized-pkg",
					Version: "2.1.0",
				},
				Pkg: workspace.PackageDescriptor{
					PluginDescriptor: workspace.PluginDescriptor{
						Name:              "base-provider",
						Kind:              apitype.ResourcePlugin,
						Version:           &semver.Version{Major: 1, Minor: 5, Patch: 0},
						PluginDownloadURL: "https://example.com/parameterized-pkg",
					},
					Parameterization: &workspace.Parameterization{
						Name:    "parameterized-pkg",
						Version: semver.Version{Major: 2, Minor: 1, Patch: 0},
						Value:   []byte(`{"region":"eu-west-1","tier":"premium"}`),
					},
				},
			},
		},
		{
			name:       "registry resolution with matching major version locally installed and in registry",
			options:    &Options{ResolveWithRegistry: true},
			pluginSpec: workspace.PackageSpec{Source: "major-match-pkg"},
			workspace: mockWorkspace{
				hasPluginGTE: func(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
					if spec.Name == "major-match-pkg" && spec.Version != nil && spec.Version.Major == 2 {
						return true, &semver.Version{Major: 2, Minor: 1, Patch: 0}, nil
					}
					return false, nil, nil
				},
				hasPlugin: func(spec workspace.PluginDescriptor) bool {
					return spec.Name == "major-match-pkg" &&
						spec.Version != nil &&
						spec.Version.EQ(semver.Version{Major: 2, Minor: 1, Patch: 0})
				},
			},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "major-match-pkg" {
							return apitype.PackageMetadata{
								Name:              "major-match-pkg",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           *version,
								PluginDownloadURL: "https://example.com/major-match-pkg",
							}, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{
								Name:              "major-match-pkg",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           semver.Version{Major: 2, Minor: 2, Patch: 0},
								PluginDownloadURL: "https://example.com/major-match-pkg",
							}, nil)
						}
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "pulumi/pulumi/major-match-pkg",
					Version: "2.1.0",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "major-match-pkg",
					Kind:              apitype.ResourcePlugin,
					Version:           &semver.Version{Major: 2, Minor: 1, Patch: 0},
					PluginDownloadURL: "https://example.com/major-match-pkg",
				}},
				InstalledInWorkspace: true,
			},
		},
		{
			name:       "registry resolution with matching major version locally but not in registry",
			options:    &Options{ResolveWithRegistry: true},
			pluginSpec: workspace.PackageSpec{Source: "major-match-not-in-registry"},
			workspace: mockWorkspace{
				hasPluginGTE: func(spec workspace.PluginDescriptor) (bool, *semver.Version, error) {
					if spec.Name == "major-match-not-in-registry" && spec.Version != nil && spec.Version.Major == 2 {
						return true, &semver.Version{Major: 2, Minor: 1, Patch: 0}, nil
					}
					return false, nil, nil
				},
			},
			registryResponse: func() (registry.Registry, error) {
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "major-match-not-in-registry" {
							if version != nil && version.EQ(semver.Version{Major: 2, Minor: 1, Patch: 0}) {
								return apitype.PackageMetadata{}, registry.ErrNotFound
							}
							return apitype.PackageMetadata{
								Name:              "major-match-not-in-registry",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           *version,
								PluginDownloadURL: "https://example.com/major-match-not-in-registry",
							}, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(apitype.PackageMetadata{
								Name:              "major-match-not-in-registry",
								Publisher:         "pulumi",
								Source:            "pulumi",
								Version:           semver.Version{Major: 2, Minor: 2, Patch: 0},
								PluginDownloadURL: "https://example.com/major-match-not-in-registry",
							}, nil)
						}
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "pulumi/pulumi/major-match-not-in-registry",
					Version: "2.2.0",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "major-match-not-in-registry",
					Kind:              apitype.ResourcePlugin,
					Version:           &semver.Version{Major: 2, Minor: 2, Patch: 0},
					PluginDownloadURL: "https://example.com/major-match-not-in-registry",
				}},
				InstalledInWorkspace: false,
			},
		},
		{
			name:    "checksums are preserved through resolution",
			options: &Options{ResolveWithRegistry: true},
			pluginSpec: workspace.PackageSpec{
				Source:  "checksum-pkg",
				Version: "1.0.0",
				Checksums: map[string][]byte{
					"linux-amd64":   []byte("abc123"),
					"darwin-amd64":  []byte("def456"),
					"windows-amd64": []byte("ghi789"),
				},
			},
			workspace: mockWorkspace{},
			registryResponse: func() (registry.Registry, error) {
				meta := apitype.PackageMetadata{
					Name:              "checksum-pkg",
					Publisher:         "pulumi",
					Source:            "pulumi",
					Version:           semver.Version{Major: 1, Minor: 0, Patch: 0},
					PluginDownloadURL: "https://example.com/checksum-pkg",
				}
				return registry.Mock{
					GetPackageF: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "pulumi" && publisher == "pulumi" && name == "checksum-pkg" {
							return meta, nil
						}
						return apitype.PackageMetadata{}, registry.ErrNotFound
					},
					ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
						return func(yield func(apitype.PackageMetadata, error) bool) {
							yield(meta, nil)
						}
					},
				}, nil
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:  "pulumi/pulumi/checksum-pkg",
					Version: "1.0.0",
					Checksums: map[string][]byte{
						"linux-amd64":   []byte("abc123"),
						"darwin-amd64":  []byte("def456"),
						"windows-amd64": []byte("ghi789"),
					},
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "checksum-pkg",
					Kind:              apitype.ResourcePlugin,
					Version:           &semver.Version{Major: 1, Minor: 0, Patch: 0},
					PluginDownloadURL: "https://example.com/checksum-pkg",
					Checksums: map[string][]byte{
						"linux-amd64":   []byte("abc123"),
						"darwin-amd64":  []byte("def456"),
						"windows-amd64": []byte("ghi789"),
					},
				}},
				InstalledInWorkspace: false,
			},
		},
		{
			name: "pluginDownloadURL prevents resolution errors",
			options: &Options{
				AllowNonInvertableLocalWorkspaceResolution: false,
			},
			workspace: mockWorkspace{},
			pluginSpec: workspace.PackageSpec{
				Source:            "some-pkg",
				PluginDownloadURL: "https://www.example.com/some-pkg.tar.gz",
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source:            "some-pkg",
					PluginDownloadURL: "https://www.example.com/some-pkg.tar.gz",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "some-pkg",
					Kind:              apitype.ResourcePlugin,
					PluginDownloadURL: "https://www.example.com/some-pkg.tar.gz",
				}},
			},
		},
		{
			name:      "handle pulumiverse packages",
			workspace: mockWorkspace{},
			pluginSpec: workspace.PackageSpec{
				Source: "aquasec", // In the pulumiverse
			},
			expected: PackageResolution{
				Spec: workspace.PackageSpec{
					Source: "aquasec",
				},
				Pkg: workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
					Name:              "aquasec",
					Kind:              apitype.ResourcePlugin,
					PluginDownloadURL: "github://api.github.com/pulumiverse",
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var reg registry.Registry = registry.Mock{
				ListPackagesF: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
					return func(yield func(apitype.PackageMetadata, error) bool) {} // empty
				},
			}
			if tt.registryResponse != nil {
				var err error
				reg, err = tt.registryResponse()
				require.NoError(t, err)
			}

			env := Options{
				ResolveWithRegistry: true,
			}
			if tt.options != nil {
				env = *tt.options
			}

			ws := tt.workspace
			if ws == nil {
				ws = DefaultWorkspace()
			}

			result, err := Resolve(
				t.Context(),
				reg,
				ws,
				tt.pluginSpec,
				env,
			)

			if tt.expectedErr != nil {
				if packageNotFoundErr, ok := tt.expectedErr.(*PackageNotFoundError); ok {
					actualErr, actualOk := err.(*PackageNotFoundError)
					require.True(t, actualOk, "Expected PackageNotFoundError but got %T", err)
					assert.Equal(t, packageNotFoundErr.Package, actualErr.Package)
					assert.Equal(t, packageNotFoundErr.Version, actualErr.Version)
					return
				}
				assert.Equal(t, tt.expectedErr, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, result)

			// Every resolution should return a spec that is guaranteed to
			// resolve to the same spec.

			var resolvedSpec workspace.PackageSpec
			switch result := result.(type) {
			case PathResolution:
				resolvedSpec = result.Spec
			case PluginResolution:
				resolvedSpec = result.Spec
			case PackageResolution:
				resolvedSpec = result.Spec
			default:
				require.Failf(t, "invalid Resolution type", "type %T", result)
			}
			resolution, err := Resolve(t.Context(), reg, ws, resolvedSpec, env)
			require.NoErrorf(t, err, "second resolution failed: resolving from %#v", resolvedSpec)
			assert.Equal(t, tt.expected, resolution, "second resolution should be the same")
		})
	}
}
