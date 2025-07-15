// Copyright 2023-2024, Pulumi Corporation.
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

package plugin

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test for https://github.com/pulumi/pulumi/issues/11703, check we give an error when trying to install a
// bundled plugin
func TestBundledError(t *testing.T) {
	t.Parallel()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		env: env.NewEnv(
			env.MapStore{"PULUMI_DEV": "false"},
		),
	}

	err := cmd.Run(context.Background(), []string{"language", "nodejs"})
	assert.EqualError(t, err,
		"the nodejs language plugin is bundled with Pulumi, "+
			"and cannot be directly installed with this command. "+
			"If you need to reinstall this plugin, reinstall Pulumi via your package manager or install script.")
}

// Test for https://github.com/pulumi/pulumi/issues/11703, check we still try to install bundled plugins if
// PULUMI_DEV is set.
func TestBundledDev(t *testing.T) {
	t.Parallel()

	var getLatestVersionCalled bool
	defer func() {
		assert.True(t, getLatestVersionCalled, "GetLatestVersion should have been called")
	}()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		env: env.NewEnv(
			env.MapStore{"PULUMI_DEV": "true"},
		),
		pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
			getLatestVersionCalled = true
			assert.Equal(t, "nodejs", ps.Name)
			assert.Equal(t, apitype.LanguagePlugin, ps.Kind)
			return nil, errors.New("404 HTTP error fetching plugin")
		},
	}

	err := cmd.Run(context.Background(), []string{"language", "nodejs"})
	assert.ErrorContains(t, err, "404 HTTP error fetching plugin")
}

func TestGetLatestPluginIncludedVersion(t *testing.T) {
	t.Parallel()

	var pluginWasInstalled bool
	defer func() {
		assert.True(t, pluginWasInstalled, "installPluginSpec should have been called")
	}()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
			assert.Fail(t, "GetLatestVersion should not have been called")
			return nil, nil
		},
		installPluginSpec: func(
			_ context.Context, _ string,
			install workspace.PluginSpec, file string,
			_ diag.Sink, _ colors.Colorization, _ bool,
		) error {
			pluginWasInstalled = true
			assert.Empty(t, file)
			assert.Equal(t, workspace.PluginSpec{
				Name: "aws",
				Kind: apitype.ResourcePlugin,
				Version: &semver.Version{
					Major: 1000,
					Minor: 78,
				},
			}, install)
			return nil
		},
	}

	err := cmd.Run(context.Background(), []string{"resource", "aws@1000.78.0"})
	assert.NoError(t, err)
}

func TestGetPluginDownloadURLFromRegistry(t *testing.T) {
	t.Parallel()

	var pluginWasInstalled bool
	defer func() {
		assert.True(t, pluginWasInstalled, "installPluginSpec should have been called")
	}()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
			assert.Fail(t, "GetLatestVersion should not have been called")
			return nil, nil
		},
		registry: &backend.MockCloudRegistry{
			ListPackagesF: func(
				ctx context.Context, name *string,
			) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(apitype.PackageMetadata{
						Name:              "foo",
						Publisher:         "pulumi",
						Source:            "pulumi",
						Version:           semver.Version{Major: 2},
						PluginDownloadURL: "http://example.com/download",
					}, nil)
				}
			},
		},
		installPluginSpec: func(
			_ context.Context, _ string,
			install workspace.PluginSpec, _ string,
			_ diag.Sink, _ colors.Colorization, _ bool,
		) error {
			pluginWasInstalled = true
			assert.Equal(t, workspace.PluginSpec{
				Name: "foo",
				Kind: apitype.ResourcePlugin,
				Version: &semver.Version{
					Major: 2,
				},
				PluginDownloadURL: "http://example.com/download",
			}, install)
			return nil
		},
	}

	err := cmd.Run(context.Background(), []string{"resource", "foo@2.0.0"})
	assert.NoError(t, err)
}

// We need to check that we don't break installs of versions that are not published into the registry.
func TestGetPluginDownloadFromKnownUnpublishedPackage(t *testing.T) {
	t.Parallel()

	var pluginWasInstalled bool
	defer func() {
		assert.True(t, pluginWasInstalled, "installPluginSpec should have been called")
	}()

	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
			assert.Fail(t, "GetLatestVersion should not have been called")
			return nil, nil
		},
		registry: &backend.MockCloudRegistry{
			GetPackageF: func(
				_ context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				switch version.String() {
				case "1.48.0":
					return apitype.PackageMetadata{}, registry.ErrNotFound
				default:
					require.Failf(t, "unknown version requested", "found %s", version)
					return apitype.PackageMetadata{}, nil
				}
			},
			ListPackagesF: func(
				ctx context.Context, name *string,
			) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(apitype.PackageMetadata{
						Name:      "random",
						Publisher: "pulumi",
						Source:    "pulumi",
						Version:   semver.Version{Major: 2},
					}, nil)
				}
			},
		},
		installPluginSpec: func(
			_ context.Context, _ string,
			install workspace.PluginSpec, _ string,
			_ diag.Sink, _ colors.Colorization, _ bool,
		) error {
			pluginWasInstalled = true
			assert.Equal(t, workspace.PluginSpec{
				Name: "random",
				Kind: apitype.ResourcePlugin,
				Version: &semver.Version{
					Major: 1,
					Minor: 48,
				},
			}, install)
			return nil
		},
	}

	err := cmd.Run(context.Background(), []string{"resource", "random", "1.48.0"})
	assert.NoError(t, err)
}

func TestGetPluginDownloadForMissingPackage(t *testing.T) {
	t.Parallel()

	t.Run("with version", func(t *testing.T) {
		t.Parallel()

		cmd := &pluginInstallCmd{
			diag: diagtest.LogSink(t),
			pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
				assert.Fail(t, "GetLatestVersion should not have been called")
				return nil, nil
			},
			registry: &backend.MockCloudRegistry{
				ListPackagesF: func(
					ctx context.Context, name *string,
				) iter.Seq2[apitype.PackageMetadata, error] {
					return func(yield func(apitype.PackageMetadata, error) bool) {}
				},
			},
		}

		err := cmd.Run(context.Background(), []string{"resource", "unknown", "1.48.0"})
		assert.ErrorContains(t, err,
			"Unable to resolve package from name: not found: unknown@1.48.0 does not match a registry package")
	})

	t.Run("without version", func(t *testing.T) {
		t.Parallel()

		cmd := &pluginInstallCmd{
			diag: diagtest.LogSink(t),
			pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
				assert.Fail(t, "GetLatestVersion should not have been called")
				return nil, nil
			},
			registry: &backend.MockCloudRegistry{
				ListPackagesF: func(
					ctx context.Context, name *string,
				) iter.Seq2[apitype.PackageMetadata, error] {
					return func(yield func(apitype.PackageMetadata, error) bool) {}
				},
			},
		}

		err := cmd.Run(context.Background(), []string{"resource", "unknown"})
		assert.ErrorContains(t, err,
			"Unable to resolve package from name: not found: unknown does not match a registry package")
	})
}

func TestRegistryIsNotUsedWhenAFileIsSpecified(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var wasInstalled bool
	defer func() { assert.True(t, wasInstalled) }()
	cmd := &pluginInstallCmd{
		diag: diagtest.LogSink(t),
		pluginGetLatestVersion: func(ps workspace.PluginSpec, ctx context.Context) (*semver.Version, error) {
			require.Fail(t, "GetLatestVersion should not have been called")
			return nil, nil
		},
		registry: &backend.MockCloudRegistry{ /* empty registry will fail when used */ },
		file:     "./pulumi-resource-some-file.tar.gz", // This is a flag: --file
		installPluginSpec: func(
			_ context.Context, _ string,
			install workspace.PluginSpec, file string,
			sink diag.Sink, color colors.Colorization, reinstall bool,
		) error {
			wasInstalled = true
			assert.Equal(t, "./pulumi-resource-some-file.tar.gz", file)
			assert.Equal(t, workspace.PluginSpec{
				Name:    "some-file",
				Kind:    "resource",
				Version: &semver.Version{Major: 1},
			}, install)
			return nil
		},
	}

	assert.NoError(t, cmd.Run(ctx, []string{"resource", "some-file", "v1.0.0"}))
}
