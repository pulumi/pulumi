// Copyright 2026, Pulumi Corporation.
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

//go:build !js

package schema

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalSchemaJSON returns a minimal valid schema for the given package name and version.
func minimalSchemaJSON(name, version string) []byte {
	data, err := json.Marshal(PackageSpec{Name: name, Version: version})
	if err != nil {
		panic(err)
	}
	return data
}

// newCachingHost returns a mock host backed by the given provider and pluginInfo.
func newCachingHost(provider plugin.Provider, pluginInfo *workspace.PluginInfo) *plugin.MockHost {
	return &plugin.MockHost{
		ProviderF: func(workspace.PluginDescriptor, env.Env) (plugin.Provider, error) {
			return provider, nil
		},
		ResolvePluginF: func(workspace.PluginDescriptor) (*workspace.PluginInfo, error) {
			return pluginInfo, nil
		},
	}
}

// fileCacheLoader creates a loader with the in-memory caches disabled so that
// only the file-based cache is exercised.
func fileCacheLoader(host plugin.Host) ReferenceLoader {
	return newPluginLoaderWithOptions(host, pluginLoaderCacheOptions{
		disableEntryCache: true,
		disableMmap:       true,
	})
}

// TestSchemaFilePathFormat checks that schemaFilePath produces the expected
// filename structure in each descriptor combination.
func TestSchemaFilePathFormat(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PULUMI_HOME", homeDir)

	v := semver.MustParse("1.2.3")

	// Basic: name + version only.
	path, err := schemaFilePath("aws", &v, "", nil)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homeDir, "schemas", "aws-1.2.3.json"), path)

	// A download URL appends a hash component.
	pathURL, err := schemaFilePath("aws", &v, "https://example.com/plugin", nil)
	require.NoError(t, err)
	assert.NotEqual(t, path, pathURL)
	base := filepath.Base(pathURL)
	assert.Equal(t, "aws-1.2.3-", base[:10])

	// Same URL → identical path (deterministic).
	pathURL2, err := schemaFilePath("aws", &v, "https://example.com/plugin", nil)
	require.NoError(t, err)
	assert.Equal(t, pathURL, pathURL2)

	// Different URL → different path.
	pathOtherURL, err := schemaFilePath("aws", &v, "https://other.com/plugin", nil)
	require.NoError(t, err)
	assert.NotEqual(t, pathURL, pathOtherURL)

	// Parameterization appends a hash component.
	param1 := &ParameterizationDescriptor{
		Name:    "terraform-aws",
		Version: semver.MustParse("5.0.0"),
		Value:   []byte("aws"),
	}
	pathParam, err := schemaFilePath("terraform-bridge", &v, "", param1)
	require.NoError(t, err)
	assert.NotEqual(t, path, pathParam)

	// Different parameterization → different path.
	param2 := &ParameterizationDescriptor{
		Name:    "terraform-aws",
		Version: semver.MustParse("6.0.0"),
		Value:   []byte("aws"),
	}
	pathParam2, err := schemaFilePath("terraform-bridge", &v, "", param2)
	require.NoError(t, err)
	assert.NotEqual(t, pathParam, pathParam2)

	// Both URL and parameterization produce a path with two hash components.
	pathBoth, err := schemaFilePath("aws", &v, "https://example.com/plugin", param1)
	require.NoError(t, err)
	assert.NotEqual(t, pathURL, pathBoth)
	assert.NotEqual(t, pathParam, pathBoth)
}

// TestSchemaCacheMissWritesFile verifies that on a cache miss the plugin is
// called and the schema is written to ~/.pulumi/schemas/.
func TestSchemaCacheMissWritesFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PULUMI_HOME", homeDir)

	version := semver.MustParse("1.0.0")
	schema := minimalSchemaJSON("aws", "1.0.0")

	getCalled := 0
	provider := &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			getCalled++
			return plugin.GetSchemaResponse{Schema: schema}, nil
		},
	}

	pluginInfo := &workspace.PluginInfo{
		Name:        "aws",
		Kind:        apitype.ResourcePlugin,
		Version:     &version,
		InstallTime: time.Now().Add(-time.Hour),
	}

	loader := fileCacheLoader(newCachingHost(provider, pluginInfo))

	ref, err := loader.LoadPackageReferenceV2(t.Context(), &PackageDescriptor{
		Name:    "aws",
		Version: &version,
	})
	require.NoError(t, err)
	assert.Equal(t, "aws", ref.Name())
	assert.Equal(t, 1, getCalled)

	// Schema file must exist at the expected path.
	schemaPath, err := schemaFilePath("aws", &version, "", nil)
	require.NoError(t, err)
	assert.FileExists(t, schemaPath)
	stored, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	assert.Equal(t, schema, stored)
}

// TestSchemaCacheHitSkipsPlugin verifies that a fresh cache file means the
// plugin's GetSchema is never called.
func TestSchemaCacheHitSkipsPlugin(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PULUMI_HOME", homeDir)

	version := semver.MustParse("1.0.0")
	schema := minimalSchemaJSON("aws", "1.0.0")

	// Write the cache file before creating the loader.
	schemaPath, err := schemaFilePath("aws", &version, "", nil)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(schemaPath), 0o700))
	require.NoError(t, os.WriteFile(schemaPath, schema, 0o600))

	getCalled := 0
	provider := &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			getCalled++
			return plugin.GetSchemaResponse{Schema: schema}, nil
		},
	}

	// Plugin install time is before the cache file was written.
	pluginInfo := &workspace.PluginInfo{
		Name:        "aws",
		Kind:        apitype.ResourcePlugin,
		Version:     &version,
		InstallTime: time.Now().Add(-time.Hour),
	}

	loader := fileCacheLoader(newCachingHost(provider, pluginInfo))

	ref, err := loader.LoadPackageReferenceV2(t.Context(), &PackageDescriptor{
		Name:    "aws",
		Version: &version,
	})
	require.NoError(t, err)
	assert.Equal(t, "aws", ref.Name())
	assert.Equal(t, 0, getCalled, "GetSchema must not be called when the cache is fresh")
}

// TestSchemaCacheStaleOnReinstall verifies that a plugin reinstalled after the
// cache file was written causes the cache to be considered stale.
func TestSchemaCacheStaleOnReinstall(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PULUMI_HOME", homeDir)

	version := semver.MustParse("1.0.0")
	schema := minimalSchemaJSON("aws", "1.0.0")

	// Write the cache file and backdate its mtime to the past.
	schemaPath, err := schemaFilePath("aws", &version, "", nil)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(schemaPath), 0o700))
	require.NoError(t, os.WriteFile(schemaPath, schema, 0o600))
	pastTime := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(schemaPath, pastTime, pastTime))

	// Plugin was reinstalled after the cache was written.
	getCalled := 0
	provider := &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			getCalled++
			return plugin.GetSchemaResponse{Schema: schema}, nil
		},
	}

	pluginInfo := &workspace.PluginInfo{
		Name:        "aws",
		Kind:        apitype.ResourcePlugin,
		Version:     &version,
		InstallTime: time.Now(), // newer than the backdated cache file
	}

	loader := fileCacheLoader(newCachingHost(provider, pluginInfo))

	ref, err := loader.LoadPackageReferenceV2(t.Context(), &PackageDescriptor{
		Name:    "aws",
		Version: &version,
	})
	require.NoError(t, err)
	assert.Equal(t, "aws", ref.Name())
	assert.Equal(t, 1, getCalled, "GetSchema must be called when the plugin is newer than the cached schema")
}

// TestSchemaCacheNoVersionSkipsCache verifies that when a plugin has no version
// the file cache is not used (the schema cannot be keyed deterministically).
func TestSchemaCacheNoVersionSkipsCache(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PULUMI_HOME", homeDir)

	resolvedVersion := semver.MustParse("1.0.0")
	schema := minimalSchemaJSON("aws", "1.0.0")

	getCalled := 0
	provider := &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			getCalled++
			return plugin.GetSchemaResponse{Schema: schema}, nil
		},
		GetPluginInfoF: func(context.Context) (plugin.PluginInfo, error) {
			return plugin.PluginInfo{Version: &resolvedVersion}, nil
		},
	}

	// PluginInfo has no version — caching should be skipped.
	pluginInfo := &workspace.PluginInfo{
		Name: "aws",
		Kind: apitype.ResourcePlugin,
	}

	loader := fileCacheLoader(newCachingHost(provider, pluginInfo))

	for range 2 {
		_, err := loader.LoadPackageReferenceV2(t.Context(), &PackageDescriptor{Name: "aws"})
		require.NoError(t, err)
	}

	assert.Equal(t, 2, getCalled, "GetSchema must be called every time when no version is available for caching")

	// No cache files should have been written.
	schemasDir := filepath.Join(homeDir, "schemas")
	if entries, readErr := os.ReadDir(schemasDir); readErr == nil {
		assert.Empty(t, entries, "no schema files should be written when version is unknown")
	}
}

// TestSchemaCacheParameterized verifies that parameterized schemas are cached
// and that different parameterizations use distinct cache files.
func TestSchemaCacheParameterized(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("PULUMI_HOME", homeDir)

	bridgeVersion := semver.MustParse("1.0.0")
	awsParamVersion := semver.MustParse("5.0.0")
	azureParamVersion := semver.MustParse("3.0.0")

	awsParam := &ParameterizationDescriptor{
		Name:    "terraform-aws",
		Version: awsParamVersion,
		Value:   []byte("aws"),
	}
	azureParam := &ParameterizationDescriptor{
		Name:    "terraform-azure",
		Version: azureParamVersion,
		Value:   []byte("azure"),
	}

	// Verify the two parameterizations land in different cache files.
	awsPath, err := schemaFilePath("terraform-bridge", &bridgeVersion, "", awsParam)
	require.NoError(t, err)
	azurePath, err := schemaFilePath("terraform-bridge", &bridgeVersion, "", azureParam)
	require.NoError(t, err)
	assert.NotEqual(t, awsPath, azurePath, "different parameterizations must use distinct cache files")

	installTime := time.Now().Add(-time.Hour)
	pluginInfo := &workspace.PluginInfo{
		Name:        "terraform-bridge",
		Kind:        apitype.ResourcePlugin,
		Version:     &bridgeVersion,
		InstallTime: installTime,
	}

	// Helper: build a loader backed by a provider that always returns the given schema.
	makeLoader := func(schema []byte) ReferenceLoader {
		p := &plugin.MockProvider{
			ParameterizeF: func(_ context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
				v := req.Parameters.(*plugin.ParameterizeValue)
				return plugin.ParameterizeResponse{Name: v.Name, Version: v.Version}, nil
			},
			GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
				return plugin.GetSchemaResponse{Schema: schema}, nil
			},
		}
		return fileCacheLoader(newCachingHost(p, pluginInfo))
	}

	awsSchema := minimalSchemaJSON("terraform-aws", "5.0.0")
	azureSchema := minimalSchemaJSON("terraform-azure", "3.0.0")

	awsDescriptor := &PackageDescriptor{
		Name:             "terraform-bridge",
		Version:          &bridgeVersion,
		Parameterization: awsParam,
	}
	azureDescriptor := &PackageDescriptor{
		Name:             "terraform-bridge",
		Version:          &bridgeVersion,
		Parameterization: azureParam,
	}

	// Prime the cache for both parameterizations.
	_, err = makeLoader(awsSchema).LoadPackageReferenceV2(t.Context(), awsDescriptor)
	require.NoError(t, err)
	_, err = makeLoader(azureSchema).LoadPackageReferenceV2(t.Context(), azureDescriptor)
	require.NoError(t, err)

	// A loader that must never call GetSchema (cache must be used).
	neverCallProvider := &plugin.MockProvider{
		ParameterizeF: func(_ context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			v := req.Parameters.(*plugin.ParameterizeValue)
			return plugin.ParameterizeResponse{Name: v.Name, Version: v.Version}, nil
		},
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			t.Error("GetSchema called unexpectedly — cache should have been used")
			return plugin.GetSchemaResponse{}, nil
		},
	}
	cachedLoader := fileCacheLoader(newCachingHost(neverCallProvider, pluginInfo))

	awsRef, err := cachedLoader.LoadPackageReferenceV2(t.Context(), awsDescriptor)
	require.NoError(t, err)
	assert.Equal(t, "terraform-aws", awsRef.Name(), "aws parameterized schema should be served from cache")

	azureRef, err := cachedLoader.LoadPackageReferenceV2(t.Context(), azureDescriptor)
	require.NoError(t, err)
	assert.Equal(t, "terraform-azure", azureRef.Name(), "azure parameterized schema should be served from cache")
}
