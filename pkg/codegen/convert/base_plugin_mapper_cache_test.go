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

package convert

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
)

// Tests that a mapping served by a plugin is cached on disk and that a subsequent mapper serves the same request
// from that cache without instantiating the plugin.
func TestBasePluginMapper_DiskCacheWarmHit(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "provider",
				Kind:    apitype.ResourcePlugin,
				Version: new(semver.MustParse("1.0.0")),
				Path:    t.TempDir(),
			},
		},
	}

	factoryCalls := 0
	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		factoryCalls++
		return &testProvider{
			pkg: "provider",
			GetMappingF: func(key, provider string) ([]byte, string, error) {
				return []byte("data"), "provider", nil
			},
		}, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	newMapper := func() Mapper {
		mapper, err := NewBasePluginMapper(
			ws,
			"key", /*conversionKey*/
			providerFactory,
			installPlugin,
			nil, /*mappings*/
		)
		require.NoError(t, err)
		return mapper
	}

	// Act.
	data, err := newMapper().GetMapping(t.Context(), "provider", nil /*hint*/, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
	assert.Equal(t, 1, factoryCalls)

	// Act.
	//
	// A fresh mapper models a new plan in a new process: the mapping must come from the disk cache without the
	// plugin being instantiated.
	data, err = newMapper().GetMapping(t.Context(), "provider", nil /*hint*/, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
	assert.Equal(t, 1, factoryCalls)
}

// Tests that cached mappings for parameterized plugins are keyed by their parameterization: the same
// parameterization is served from the disk cache, while a different one instantiates the plugin again.
func TestBasePluginMapper_DiskCacheParameterization(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "terraform-provider",
				Kind:    apitype.ResourcePlugin,
				Version: new(semver.MustParse("1.0.0")),
				Path:    t.TempDir(),
			},
		},
	}

	factoryCalls := 0
	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		factoryCalls++
		return &testProvider{
			pkg: "terraform-provider",
			GetMappingF: func(key, provider string) ([]byte, string, error) {
				return []byte("datagcp"), "gcp", nil
			},
		}, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	newMapper := func() Mapper {
		mapper, err := NewBasePluginMapper(
			ws,
			"key", /*conversionKey*/
			providerFactory,
			installPlugin,
			nil, /*mappings*/
		)
		require.NoError(t, err)
		return mapper
	}

	hint := &MapperPackageHint{
		PluginName: "terraform-provider",
		Parameterization: &workspace.Parameterization{
			Name:    "gcp",
			Version: semver.MustParse("2.0.0"),
			Value:   []byte("value"),
		},
	}

	// Act.
	data, err := newMapper().GetMapping(t.Context(), "gcp", hint, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)
	assert.Equal(t, 1, factoryCalls)

	// Act.
	//
	// The same parameterization must be served from the disk cache.
	data, err = newMapper().GetMapping(t.Context(), "gcp", hint, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)
	assert.Equal(t, 1, factoryCalls)

	// Act.
	//
	// A different parameterization must not hit the cache entry written for the first one.
	otherHint := &MapperPackageHint{
		PluginName: "terraform-provider",
		Parameterization: &workspace.Parameterization{
			Name:    "gcp",
			Version: semver.MustParse("3.0.0"),
			Value:   []byte("value"),
		},
	}
	data, err = newMapper().GetMapping(t.Context(), "gcp", otherHint, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)
	assert.Equal(t, 2, factoryCalls)
}

// Tests that a cached mapping that predates the plugin's installation is discarded and refreshed.
func TestBasePluginMapper_DiskCacheStaleAfterReinstall(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "provider",
				Kind:    apitype.ResourcePlugin,
				Version: new(semver.MustParse("1.0.0")),
				Path:    t.TempDir(),
			},
		},
	}

	payload := []byte("data1")
	factoryCalls := 0
	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		factoryCalls++
		return &testProvider{
			pkg: "provider",
			GetMappingF: func(key, provider string) ([]byte, string, error) {
				return payload, "provider", nil
			},
		}, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	newMapper := func() Mapper {
		mapper, err := NewBasePluginMapper(
			ws,
			"key", /*conversionKey*/
			providerFactory,
			installPlugin,
			nil, /*mappings*/
		)
		require.NoError(t, err)
		return mapper
	}

	// Act.
	data, err := newMapper().GetMapping(t.Context(), "provider", nil /*hint*/, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("data1"), data)
	assert.Equal(t, 1, factoryCalls)

	// Arrange.
	//
	// Backdate the cache file so that it predates the plugin's installation, as if the plugin had been reinstalled
	// since the mapping was cached.
	cachePath, err := mappingFilePath("key", "provider", "provider", semver.MustParse("1.0.0"), nil)
	require.NoError(t, err)
	past := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(cachePath, past, past))
	payload = []byte("data2")

	// Act.
	data, err = newMapper().GetMapping(t.Context(), "provider", nil /*hint*/, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("data2"), data)
	assert.Equal(t, 2, factoryCalls)

	content, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	assert.Equal(t, []byte("data2"), content)
}

// Tests that mappings from plugins whose install time cannot be determined are still written to the disk cache, but
// never read back from it, matching the behaviour of the schema cache.
func TestBasePluginMapper_DiskCacheUnknownInstallTime(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	// Arrange.
	//
	// The plugin has no path on disk, so its install time is unknown.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "provider",
				Kind:    apitype.ResourcePlugin,
				Version: new(semver.MustParse("1.0.0")),
			},
		},
	}

	factoryCalls := 0
	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		factoryCalls++
		return &testProvider{
			pkg: "provider",
			GetMappingF: func(key, provider string) ([]byte, string, error) {
				return []byte("data"), "provider", nil
			},
		}, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	newMapper := func() Mapper {
		mapper, err := NewBasePluginMapper(
			ws,
			"key", /*conversionKey*/
			providerFactory,
			installPlugin,
			nil, /*mappings*/
		)
		require.NoError(t, err)
		return mapper
	}

	// Act.
	data, err := newMapper().GetMapping(t.Context(), "provider", nil /*hint*/, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
	assert.Equal(t, 1, factoryCalls)

	cachePath, err := mappingFilePath("key", "provider", "provider", semver.MustParse("1.0.0"), nil)
	require.NoError(t, err)
	content, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), content)

	// Act.
	//
	// With no install time to validate freshness against, the cache must not be read back.
	data, err = newMapper().GetMapping(t.Context(), "provider", nil /*hint*/, "" /*ecosystem*/)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
	assert.Equal(t, 2, factoryCalls)
}
