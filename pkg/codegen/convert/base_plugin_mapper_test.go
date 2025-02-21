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

package convert

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Tests that the base plugin mapper will return data from passed mapping entries if available.
func TestBasePluginMapper_UsesEntries(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{}
	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		t.Fatal("should not be called")
		return nil, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	tempDir := t.TempDir()
	mappingFile := tempDir + "/provider.json"

	err := os.WriteFile(mappingFile, []byte("entrydata"), 0o600)
	assert.NoError(t, err)

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		[]string{mappingFile},
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "provider", nil /*hint*/)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte("entrydata"), data)
}

// Tests that the base plugin mapper will find and use an already installed plugin.
func TestBasePluginMapper_InstalledPluginMatches(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "provider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProvider := &testProvider{
		pkg: "provider",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "provider", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")
		return testProvider, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "provider", nil /*hint*/)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

// Tests that the base plugin mapper will try all plugins when hunting for a mapping if it can't find or install any
// whose name matches the provider requested.
func TestBasePluginMapper_MappedNameDiffersFromPulumiName(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProvider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProvider := &testProvider{
		pkg: "pulumiProvider",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "otherProvider", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")
		return testProvider, nil
	}

	installCalled := false
	installPlugin := func(pluginName string) *semver.Version {
		// GetMapping will fail to find a plugin matching "otherProvider", and so will try to install it. We are going to
		// set that up to fail here, so that it will instead try each installed plugin one by one. It's possible for a
		// plugin A to provide mappings for some name B, and that's what we've set up in the testProvider, so that path
		// should be taken and succeed.
		assert.Equal(t, "otherProvider", pluginName)
		installCalled = true

		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "otherProvider", nil /*hint*/)

	// Assert.
	assert.True(t, installCalled, "installPlugin should have been called")
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

// Tests that a base plugin mapper will try to install a plugin if it can't find one that matches the provider
// requested, and if that installation succeeds, it will use the version reported.
func TestBasePluginMapper_NoPluginMatches_ButCanBeInstalled(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProvider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	// This provider will be returned by the factory, but since it's not specified in the workspace we'll expect an
	// "install" to be requested beforehand.
	testProvider := &testProvider{
		pkg: "yetAnotherProvider",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "yetAnotherProvider", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")
		return testProvider, nil
	}

	installCalled := false
	installPlugin := func(pluginName string) *semver.Version {
		assert.Equal(t, "yetAnotherProvider", pluginName)
		installCalled = true

		ver := semver.MustParse("1.0.0")
		return &ver
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "yetAnotherProvider", nil /*hint*/)

	// Assert.
	assert.True(t, installCalled, "installPlugin should have been called")
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

// Tests that when a base plugin mapper has multiple plugins available, it prioritises those whose name matches the
// requested provider name.
func TestBasePluginMapper_UseMatchingNameFirst(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "otherProvider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
			{
				Name:    "provider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProvider := &testProvider{
		pkg: "provider",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "provider", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")
		return testProvider, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "provider", nil /*hint*/)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

// Tests that when a base plugin mapper has multiple plugins available, and none of them matches the requested name,
// that in the absence of "hints" about which to pick it will simply try each of them in turn.
func TestBasePluginMapper_MappedNamesDifferFromPulumiName(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProviderAws",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
			{
				Name:    "pulumiProviderGcp",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProviderAws := &testProvider{
		pkg: "pulumiProviderAws",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("dataaws"), "aws", nil
		},
	}

	testProviderGcp := &testProvider{
		pkg: "pulumiProviderGcp",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("datagcp"), "gcp", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		if descriptor.Name == testProviderAws.pkg {
			return testProviderAws, nil
		}

		if descriptor.Name == testProviderGcp.pkg {
			return testProviderGcp, nil
		}

		assert.Fail(t, "unexpected package %s", descriptor.Name)
		return nil, fmt.Errorf("unexpected package %s", descriptor.Name)
	}

	installCalls := 0
	installPlugin := func(pluginName string) *semver.Version {
		// Our first request will be for the gcp provider. Since the workspace doesn't have a plugin with that name, and
		// there's no hint, we should see an installation attempt for it. The second request will be for aws, which will
		// trigger the same behaviour since nothing will match in the workspace.
		if installCalls == 0 {
			assert.Equal(t, "gcp", pluginName)
		} else if installCalls == 1 {
			assert.Equal(t, "aws", pluginName)
		}

		installCalls++
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "gcp", nil /*hint*/)

	// Assert.
	assert.Equal(t, 1, installCalls, "installPlugin should have been called once")
	assert.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)

	// Act.
	data, err = mapper.GetMapping(context.Background(), "aws", nil /*hint*/)

	// Assert.
	assert.Equal(t, 2, installCalls, "installPlugin should have been called twice")
	assert.NoError(t, err)
	assert.Equal(t, []byte("dataaws"), data)
}

// Tests that when a base plugin mapper has multiple plugins available, and none of them matches the requested name,
// that it will use supplied "hints" to prioritize which plugin to use.
func TestBasePluginMapper_MappedNamesDifferFromPulumiNameWithHint(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProviderAws",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
			{
				Name:    "pulumiProviderGcp",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProvider := &testProvider{
		pkg: "pulumiProviderGcp",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("datagcp"), "gcp", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")
		return testProvider, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "gcp", &MapperPackageHint{
		PluginName: "pulumiProviderGcp",
	})

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)
}

// Tests that when a base plugin mapper has multiple plugins available, and none of them matches the requested name,
// that it will use parameterized "hints" to request parameterized plugins whose names match that in the hint, before
// asking for mappings.
func TestBasePluginMapper_MappedNamesDifferFromPulumiNameWithParameterizedHint(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProviderAws",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
			{
				Name:    "terraform-provider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProvider := &testProvider{
		pkg: "terraform-provider",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)

			return []byte("datagcp"), "gcp", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")

		assert.Equal(t, descriptor.Parameterization.Name, "gcp")
		assert.Equal(t, descriptor.Parameterization.Version, semver.MustParse("2.0.0"))
		assert.Equal(t, descriptor.Parameterization.Value, []byte("value"))

		return testProvider, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "gcp", &MapperPackageHint{
		PluginName: "terraform-provider",
		Parameterization: &workspace.Parameterization{
			Name:    "gcp",
			Version: semver.MustParse("2.0.0"),
			Value:   []byte("value"),
		},
	})

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)
}

// Tests that when a base plugin mapper has multiple plugins available, and it has been given a parameterized hint, that
// it will not use that hint when plugin names do not match.
func TestBasePluginMapper_MappedNamesDifferFromPulumiNameWithUnusableParameterizedHint(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProviderAws",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProvider := &testProvider{
		pkg: "pulumiProviderAws",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)

			return []byte("dataaws"), "aws", nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		assert.Equal(t, descriptor.Name, testProvider.pkg, "unexpected package")
		assert.Nil(t, descriptor.Parameterization)

		return testProvider, nil
	}

	installPlugin := func(pluginName string) *semver.Version {
		assert.Equal(t, "aws", pluginName)
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.
	data, err := mapper.GetMapping(context.Background(), "aws", &MapperPackageHint{
		PluginName: "aws",
		Parameterization: &workspace.Parameterization{
			Name:    "aws",
			Version: semver.MustParse("2.0.0"),
			Value:   []byte("value"),
		},
	})

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte("dataaws"), data)
}

// Tests that mapping lookup terminates when there is no mapping to be found.
func TestBasePluginMapper_InfiniteLoopRegression(t *testing.T) {
	t.Parallel()

	// Arrange.
	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProviderAws",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	testProviderAws := &testProvider{
		pkg: "pulumiProviderAws",
		GetMappingF: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return nil, "", nil
		},
		GetMappingsF: func(key string) ([]string, error) {
			assert.Equal(t, "key", key)
			return []string{"aws"}, nil
		},
	}

	providerFactory := func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
		if descriptor.Name == testProviderAws.pkg {
			return testProviderAws, nil
		}

		assert.Fail(t, "unexpected package %s", descriptor.Name)
		return nil, fmt.Errorf("unexpected package %s", descriptor.Name)
	}

	installPlugin := func(pluginName string) *semver.Version {
		assert.Equal(t, "gcp", pluginName)
		return nil
	}

	mapper, err := NewBasePluginMapper(
		ws,
		"key", /*conversionKey*/
		providerFactory,
		installPlugin,
		nil, /*mappings*/
	)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	// Act.

	// Attempt to get the mapping for the GCP provider, which we don't have a plugin for.
	data, err := mapper.GetMapping(context.Background(), "gcp", nil /*hint*/)

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, data)
}

// testWorkspace implements the Workspace interface with a fixed set of plugins.
type testWorkspace struct {
	infos []workspace.PluginInfo
}

func (ws *testWorkspace) GetPlugins() ([]workspace.PluginInfo, error) {
	return ws.infos, nil
}

// testProvider implements the Provider interface with a fixed package name and mocked mapping functions.
type testProvider struct {
	plugin.UnimplementedProvider

	pkg string

	GetMappingF  func(key, provider string) ([]byte, string, error)
	GetMappingsF func(key string) ([]string, error)
}

func (prov *testProvider) Pkg() tokens.Package {
	return tokens.Package(prov.pkg)
}

func (prov *testProvider) GetMapping(
	_ context.Context,
	req plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	data, provider, err := prov.GetMappingF(req.Key, req.Provider)
	return plugin.GetMappingResponse{
		Data:     data,
		Provider: provider,
	}, err
}

func (prov *testProvider) GetMappings(
	_ context.Context,
	req plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	if prov.GetMappingsF == nil {
		return plugin.GetMappingsResponse{}, nil
	}
	keys, err := prov.GetMappingsF(req.Key)
	return plugin.GetMappingsResponse{Keys: keys}, err
}

func semverMustParse(s string) *semver.Version {
	v := semver.MustParse(s)
	return &v
}
