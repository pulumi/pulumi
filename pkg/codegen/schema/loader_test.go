// Copyright 2022-2024, Pulumi Corporation.
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

package schema

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initLoader(b *testing.B, options pluginLoaderCacheOptions) ReferenceLoader {
	cwd, err := os.Getwd()
	require.NoError(b, err)
	sink := diagtest.LogSink(b)
	ctx, err := plugin.NewContext(sink, sink, nil, nil, cwd, nil, true, nil)
	require.NoError(b, err)
	loader := newPluginLoaderWithOptions(ctx.Host, options)

	return loader
}

func BenchmarkLoadPackageReference(b *testing.B) {
	cacheWarmingLoader := initLoader(b, pluginLoaderCacheOptions{})
	// ensure the file cache exists for later tests:
	_, err := cacheWarmingLoader.LoadPackageReference("azure-native", nil)
	require.NoError(b, err)

	b.Run("full-load", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			loader := initLoader(b, pluginLoaderCacheOptions{})

			_, err := loader.LoadPackageReference("azure-native", nil)
			require.NoError(b, err)
		}
	})

	b.Run("full-cache", func(b *testing.B) {
		loader := initLoader(b, pluginLoaderCacheOptions{})

		b.StopTimer()
		_, err := loader.LoadPackageReference("azure-native", nil)
		require.NoError(b, err)
		b.StartTimer()

		for n := 0; n < b.N; n++ {
			_, err := loader.LoadPackageReference("azure-native", nil)
			require.NoError(b, err)
		}
	})

	b.Run("mmap-cache", func(b *testing.B) {
		// Disables in-memory cache (single instancing), retains mmap of files:
		loader := initLoader(b, pluginLoaderCacheOptions{
			disableEntryCache: true,
		})

		b.StopTimer()
		_, err := loader.LoadPackageReference("azure-native", nil)
		require.NoError(b, err)
		b.StartTimer()

		for n := 0; n < b.N; n++ {
			_, err := loader.LoadPackageReference("azure-native", nil)
			require.NoError(b, err)
		}
	})

	b.Run("file-cache", func(b *testing.B) {
		// Disables in-memory cache and mmaping of files:
		loader := initLoader(b, pluginLoaderCacheOptions{
			disableEntryCache: true,
			disableMmap:       true,
		})

		b.StopTimer()
		_, err := loader.LoadPackageReference("azure-native", nil)
		require.NoError(b, err)
		b.StartTimer()

		for n := 0; n < b.N; n++ {
			_, err := loader.LoadPackageReference("azure-native", nil)
			require.NoError(b, err)
		}
	})

	b.Run("no-cache", func(b *testing.B) {
		// Disables in-memory cache, mmaping, and using schema files:
		loader := initLoader(b, pluginLoaderCacheOptions{
			disableEntryCache: true,
			disableMmap:       true,
			disableFileCache:  true,
		})

		b.StopTimer()
		_, err := loader.LoadPackageReference("azure-native", nil)
		require.NoError(b, err)
		b.StartTimer()

		for n := 0; n < b.N; n++ {
			_, err := loader.LoadPackageReference("azure-native", nil)
			require.NoError(b, err)
		}
	})
}

func TestLoadParameterized(t *testing.T) {
	t.Parallel()

	mockProvider := &plugin.MockProvider{
		ParameterizeF: func(_ context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			assert.Equal(t, &plugin.ParameterizeValue{
				Name:    "aws",
				Version: semver.MustParse("3.0.0"),
				Value:   []byte("testdata"),
			}, req.Parameters)

			return plugin.ParameterizeResponse{
				Name:    "aws",
				Version: semver.MustParse("3.0.0"),
			}, nil
		},

		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			schema := PackageSpec{
				Name:    "aws",
				Version: "3.0.0",
			}

			data, err := json.Marshal(schema)
			if err != nil {
				return plugin.GetSchemaResponse{}, err
			}

			return plugin.GetSchemaResponse{
				Schema: data,
			}, nil
		},
	}

	host := &plugin.MockHost{
		ProviderF: func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
			assert.Equal(t, "terraform-provider", descriptor.Name)
			assert.Equal(t, semver.MustParse("1.0.0"), *descriptor.Version)
			return mockProvider, nil
		},
		ResolvePluginF: func(spec workspace.PluginSpec) (*workspace.PluginInfo, error) {
			assert.Equal(t, apitype.ResourcePlugin, spec.Kind)
			assert.Equal(t, "terraform-provider", spec.Name)
			assert.Equal(t, semver.MustParse("1.0.0"), *spec.Version)

			return &workspace.PluginInfo{
				Name:    "terraform-provider",
				Kind:    apitype.ResourcePlugin,
				Version: spec.Version,
			}, nil
		},
	}

	loader := newPluginLoaderWithOptions(host, pluginLoaderCacheOptions{
		disableEntryCache: true,
		disableMmap:       true,
		disableFileCache:  true,
	})

	version := semver.MustParse("1.0.0")
	ref, err := loader.LoadPackageReferenceV2(context.Background(), &PackageDescriptor{
		Name:    "terraform-provider",
		Version: &version,
		Parameterization: &ParameterizationDescriptor{
			Name:    "aws",
			Version: semver.MustParse("3.0.0"),
			Value:   []byte("testdata"),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "aws", ref.Name())
}

// Tests that a PackageReferenceNameMismatchError is returned when the name in the descriptor does not match the name in
// the schema returned by the loaded plugin.
func TestLoadNameMismatch(t *testing.T) {
	t.Parallel()

	// Arrange.
	pkg := "aws"
	notPkg := "not-" + pkg

	version := semver.MustParse("3.0.0")

	provider := &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			schema := PackageSpec{
				Name:    notPkg,
				Version: version.String(),
			}

			data, err := json.Marshal(schema)
			if err != nil {
				return plugin.GetSchemaResponse{}, err
			}

			return plugin.GetSchemaResponse{
				Schema: data,
			}, nil
		},
	}

	host := &plugin.MockHost{
		ProviderF: func(workspace.PackageDescriptor) (plugin.Provider, error) {
			return provider, nil
		},
		ResolvePluginF: func(workspace.PluginSpec) (*workspace.PluginInfo, error) {
			return &workspace.PluginInfo{
				Name:    notPkg,
				Kind:    apitype.ResourcePlugin,
				Version: &version,
			}, nil
		},
	}

	loader := newPluginLoaderWithOptions(host, pluginLoaderCacheOptions{
		disableEntryCache: true,
		disableMmap:       true,
		disableFileCache:  true,
	})

	// Act.
	ref, err := LoadPackageReferenceV2(context.Background(), loader, &PackageDescriptor{
		Name:    pkg,
		Version: &version,
	})

	// Assert.

	// We should still get a reference back, even though the version doesn't match.
	require.NotNil(t, ref)

	var expectedErr *PackageReferenceNameMismatchError
	require.ErrorAsf(t, err, &expectedErr, "expected PackageReferenceNameMismatchError, got %T", err)

	require.Equal(t, pkg, expectedErr.RequestedName)
	require.Equal(t, &version, expectedErr.RequestedVersion)

	require.Equal(t, notPkg, expectedErr.LoadedName)
	require.Equal(t, &version, expectedErr.LoadedVersion)

	require.Equal(t, ref.Name(), expectedErr.LoadedName)
	require.Equal(t, ref.Version(), expectedErr.LoadedVersion)
}

// Tests that a PackageReferenceVersionMismatchError is returned when the version in the descriptor does not match the
// version in the schema returned by the loaded plugin.
func TestLoadVersionMismatch(t *testing.T) {
	t.Parallel()

	// Arrange.
	pkg := "aws"
	requestVersion := semver.MustParse("3.0.0")
	loadVersion := semver.MustParse("3.0.1")

	provider := &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			schema := PackageSpec{
				Name:    pkg,
				Version: loadVersion.String(),
			}

			data, err := json.Marshal(schema)
			if err != nil {
				return plugin.GetSchemaResponse{}, err
			}

			return plugin.GetSchemaResponse{
				Schema: data,
			}, nil
		},
	}

	host := &plugin.MockHost{
		ProviderF: func(workspace.PackageDescriptor) (plugin.Provider, error) {
			return provider, nil
		},
		ResolvePluginF: func(workspace.PluginSpec) (*workspace.PluginInfo, error) {
			return &workspace.PluginInfo{
				Name:    pkg,
				Kind:    apitype.ResourcePlugin,
				Version: &loadVersion,
			}, nil
		},
	}

	loader := newPluginLoaderWithOptions(host, pluginLoaderCacheOptions{
		disableEntryCache: true,
		disableMmap:       true,
		disableFileCache:  true,
	})

	// Act.
	ref, err := LoadPackageReferenceV2(context.Background(), loader, &PackageDescriptor{
		Name:    pkg,
		Version: &requestVersion,
	})

	// Assert.

	// We should still get a reference back, even though the version doesn't match.
	require.NotNil(t, ref)

	var expectedErr *PackageReferenceVersionMismatchError
	require.ErrorAsf(t, err, &expectedErr, "expected PackageReferenceVersionMismatchError, got %T", err)

	require.Equal(t, pkg, expectedErr.RequestedName)
	require.Equal(t, &requestVersion, expectedErr.RequestedVersion)

	require.Equal(t, pkg, expectedErr.LoadedName)
	require.Equal(t, &loadVersion, expectedErr.LoadedVersion)

	require.Equal(t, ref.Name(), expectedErr.LoadedName)
	require.Equal(t, ref.Version(), expectedErr.LoadedVersion)
}

// Simple test to ensure that the string representation of a PackageDescriptor is as expected. Both with and
// without parameterisation.
func TestPackageDescriptorString(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("3.0.0")

	cases := []struct {
		desc     PackageDescriptor
		expected string
	}{
		{
			PackageDescriptor{
				Name: "aws",
			}, "aws@nil",
		},
		{
			PackageDescriptor{
				Name:    "aws",
				Version: &version,
			}, "aws@3.0.0",
		},
		{
			PackageDescriptor{
				Name:    "base",
				Version: &version,
				Parameterization: &ParameterizationDescriptor{
					Name:    "gcp",
					Version: semver.MustParse("6.0.0"),
				},
			}, "gcp@6.0.0 (base@3.0.0)",
		},
		{
			PackageDescriptor{
				Name: "base",
				Parameterization: &ParameterizationDescriptor{
					Name:    "gcp",
					Version: semver.MustParse("6.0.0"),
				},
			}, "gcp@6.0.0 (base@nil)",
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, c.desc.String())
	}
}
