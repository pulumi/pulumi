package schema

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/require"
)

func initLoader(b *testing.B, options pluginLoaderCacheOptions) ReferenceLoader {
	b.Helper()

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
