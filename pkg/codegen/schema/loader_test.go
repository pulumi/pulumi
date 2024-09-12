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
		ResolvePluginF: func(kind apitype.PluginKind, name string, version *semver.Version) (*workspace.PluginInfo, error) {
			assert.Equal(t, apitype.ResourcePlugin, kind)
			assert.Equal(t, "terraform-provider", name)
			assert.Equal(t, semver.MustParse("1.0.0"), *version)

			return &workspace.PluginInfo{
				Name:    "terraform-provider",
				Kind:    apitype.ResourcePlugin,
				Version: version,
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
