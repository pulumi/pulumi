// Copyright 2016-2023, Pulumi Corporation.
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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type testWorkspace struct {
	infos []workspace.PluginInfo
}

func (ws *testWorkspace) GetPlugins() ([]workspace.PluginInfo, error) {
	return ws.infos, nil
}

type testProvider struct {
	plugin.UnimplementedProvider
	pkg      tokens.Package
	mapping  func(key, provider string) ([]byte, string, error)
	mappings func(key string) ([]string, error)
}

func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}

func (prov *testProvider) GetMapping(
	_ context.Context, req plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	data, provider, err := prov.mapping(req.Key, req.Provider)
	return plugin.GetMappingResponse{
		Data:     data,
		Provider: provider,
	}, err
}

func (prov *testProvider) GetMappings(
	_ context.Context, req plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	if prov.mappings == nil {
		return plugin.GetMappingsResponse{}, nil
	}
	keys, err := prov.mappings(req.Key)
	return plugin.GetMappingsResponse{Keys: keys}, err
}

func semverMustParse(s string) *semver.Version {
	v := semver.MustParse(s)
	return &v
}

func TestPluginMapper_InstalledPluginMatches(t *testing.T) {
	t.Parallel()

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
		pkg: tokens.Package("provider"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "provider", nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
		return testProvider, nil
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	data, err := mapper.GetMapping(ctx, "provider", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

func TestPluginMapper_MappedNameDiffersFromPulumiName(t *testing.T) {
	t.Parallel()

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
		pkg: tokens.Package("pulumiProvider"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "otherProvider", nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
		return testProvider, nil
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		// GetMapping will try to install "yetAnotherProvider", but for this test were testing the case where
		// that doesn't match and can't be installed, but we should still return the mapping because
		// "pulumiProvider" is already installed and will return a mapping for "otherProvider".
		assert.Equal(t, "otherProvider", string(pkg))
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	data, err := mapper.GetMapping(ctx, "otherProvider", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

func TestPluginMapper_NoPluginMatches(t *testing.T) {
	t.Parallel()

	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProvider",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	t.Run("Available to install", func(t *testing.T) {
		t.Parallel()

		testProvider := &testProvider{
			pkg: tokens.Package("yetAnotherProvider"),
			mapping: func(key, provider string) ([]byte, string, error) {
				assert.Equal(t, "key", key)
				assert.Equal(t, "", provider)
				return []byte("data"), "yetAnotherProvider", nil
			},
		}

		providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
			assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
			return testProvider, nil
		}

		installPlugin := func(pkg tokens.Package) *semver.Version {
			ver := semver.MustParse("1.0.0")
			return &ver
		}

		mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
		assert.NoError(t, err)
		assert.NotNil(t, mapper)

		ctx := context.Background()

		data, err := mapper.GetMapping(ctx, "yetAnotherProvider", "")
		assert.NoError(t, err)
		assert.Equal(t, []byte("data"), data)
	})

	t.Run("Not available to install", func(t *testing.T) {
		t.Parallel()

		testProvider := &testProvider{
			pkg: tokens.Package("pulumiProvider"),
			mapping: func(key, provider string) ([]byte, string, error) {
				assert.Equal(t, "key", key)
				assert.Equal(t, "", provider)
				return []byte("data"), "otherProvider", nil
			},
		}

		providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
			assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
			return testProvider, nil
		}

		installPlugin := func(pkg tokens.Package) *semver.Version {
			// GetMapping will try to install "yetAnotherProvider", but for this test were testing the case
			// where that can't be installed
			assert.Equal(t, "yetAnotherProvider", string(pkg))
			return nil
		}

		mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
		assert.NoError(t, err)
		assert.NotNil(t, mapper)

		ctx := context.Background()

		data, err := mapper.GetMapping(ctx, "yetAnotherProvider", "")
		assert.NoError(t, err)
		assert.Equal(t, []byte{}, data)
	})
}

func TestPluginMapper_UseMatchingNameFirst(t *testing.T) {
	t.Parallel()

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
		pkg: tokens.Package("provider"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("data"), "provider", nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
		return testProvider, nil
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	data, err := mapper.GetMapping(ctx, "provider", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("data"), data)
}

func TestPluginMapper_MappedNamesDifferFromPulumiName(t *testing.T) {
	t.Parallel()

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
		pkg: tokens.Package("pulumiProviderAws"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("dataaws"), "aws", nil
		},
	}
	testProviderGcp := &testProvider{
		pkg: tokens.Package("pulumiProviderGcp"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("datagcp"), "gcp", nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		if pkg == testProviderAws.pkg {
			return testProviderAws, nil
		} else if pkg == testProviderGcp.pkg {
			return testProviderGcp, nil
		}
		assert.Fail(t, "unexpected package %s", pkg)
		return nil, fmt.Errorf("unexpected package %s", pkg)
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		// This will want to install the "gcp" package, but we're calling the pulumi name "pulumiProviderGcp"
		// for this test.
		assert.Equal(t, "gcp", string(pkg))
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	// Get the mapping for the GCP provider.
	data, err := mapper.GetMapping(ctx, "gcp", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)

	// Now get the mapping for the AWS provider, it should be cached.
	data, err = mapper.GetMapping(ctx, "aws", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("dataaws"), data)
}

func TestPluginMapper_MappedNamesDifferFromPulumiNameWithHint(t *testing.T) {
	t.Parallel()

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
		pkg: tokens.Package("pulumiProviderGcp"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("datagcp"), "gcp", nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
		return testProvider, nil
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		t.Fatal("should not be called")
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	// Get the mapping for the GCP provider, telling the mapper that it's pulumi name is "pulumiProviderGcp".
	data, err := mapper.GetMapping(ctx, "gcp", "pulumiProviderGcp")
	assert.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)
}

// Regression test for https://github.com/pulumi/pulumi/issues/13105
func TestPluginMapper_MissingProviderOnlyTriesToInstallOnce(t *testing.T) {
	t.Parallel()

	ws := &testWorkspace{}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		t.Fatal("should not be called")
		return nil, nil
	}

	called := 0
	installPlugin := func(pkg tokens.Package) *semver.Version {
		called++
		assert.Equal(t, "pulumiProviderGcp", string(pkg))
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	// Try to get the mapping for the GCP provider, telling the mapper that it's pulumi name is "pulumiProviderGcp".
	data, err := mapper.GetMapping(ctx, "gcp", "pulumiProviderGcp")
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, data)
	// Try and get the mapping again
	data, err = mapper.GetMapping(ctx, "gcp", "pulumiProviderGcp")
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, data)
	// Install should have only been called once
	assert.Equal(t, 1, called)
}

func TestPluginMapper_GetMappingsIsUsed(t *testing.T) {
	t.Parallel()

	// Test that if the provider supports it that GetMappings is used, and will fetch multiple mappings from the same
	// provider.

	ws := &testWorkspace{
		infos: []workspace.PluginInfo{
			{
				Name:    "pulumiProviderK8s",
				Kind:    apitype.ResourcePlugin,
				Version: semverMustParse("1.0.0"),
			},
		},
	}

	var mappingCalls []string

	testProvider := &testProvider{
		pkg: tokens.Package("pulumiProviderK8s"),
		mapping: func(key, provider string) ([]byte, string, error) {
			mappingCalls = append(mappingCalls, provider)

			assert.Equal(t, "key", key)
			if provider == "kubernetes" {
				return []byte("datakubernetes"), "kubernetes", nil
			} else if provider == "helm" {
				return []byte("datahelm"), "helm", nil
			}
			return nil, "", fmt.Errorf("unexpected provider key %s", provider)
		},
		mappings: func(key string) ([]string, error) {
			assert.Equal(t, "key", key)
			return []string{"kubernetes", "helm"}, nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		assert.Equal(t, pkg, testProvider.pkg, "unexpected package %s", pkg)
		return testProvider, nil
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		assert.Contains(t, []string{"kubernetes", "helm"}, string(pkg))
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	// Get the mapping for the kubernetes provider.
	data, err := mapper.GetMapping(ctx, "kubernetes", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("datakubernetes"), data)
	// This should only have called getMapping once
	assert.Equal(t, []string{"kubernetes"}, mappingCalls)

	// Now get the mapping for the helm provider.
	data, err = mapper.GetMapping(ctx, "helm", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("datahelm"), data)
	// This should have called getMapping again
	assert.Equal(t, []string{"kubernetes", "helm"}, mappingCalls)
}

func TestPluginMapper_GetMappingIsntCalledOnValidMappings(t *testing.T) {
	t.Parallel()

	// Test that if the provider supports GetMappings that we don't call GetMapping("") on it.

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
		pkg: tokens.Package("pulumiProviderAws"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "aws", provider)
			return []byte("dataaws"), "aws", nil
		},
		mappings: func(key string) ([]string, error) {
			assert.Equal(t, "key", key)
			return []string{"aws"}, nil
		},
	}
	testProviderGcp := &testProvider{
		pkg: tokens.Package("pulumiProviderGcp"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "", provider)
			return []byte("datagcp"), "gcp", nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		if pkg == testProviderAws.pkg {
			return testProviderAws, nil
		} else if pkg == testProviderGcp.pkg {
			return testProviderGcp, nil
		}
		assert.Fail(t, "unexpected package %s", pkg)
		return nil, fmt.Errorf("unexpected package %s", pkg)
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		assert.Contains(t, []string{"aws", "gcp"}, string(pkg))
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	// Get the mapping for the GCP provider.
	data, err := mapper.GetMapping(ctx, "gcp", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("datagcp"), data)

	// Now get the mapping for the AWS provider.
	data, err = mapper.GetMapping(ctx, "aws", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("dataaws"), data)
}

func TestPluginMapper_InfiniteLoopRegression(t *testing.T) {
	t.Parallel()

	// Test that the mapping loop doesn't end up in an infinite loop in some cases where no mapping is found.

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
		pkg: tokens.Package("pulumiProviderAws"),
		mapping: func(key, provider string) ([]byte, string, error) {
			assert.Equal(t, "key", key)
			assert.Equal(t, "aws", provider)
			return []byte("dataaws"), "aws", nil
		},
		mappings: func(key string) ([]string, error) {
			assert.Equal(t, "key", key)
			return []string{"aws"}, nil
		},
	}

	providerFactory := func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
		if pkg == testProviderAws.pkg {
			return testProviderAws, nil
		}
		assert.Fail(t, "unexpected package %s", pkg)
		return nil, fmt.Errorf("unexpected package %s", pkg)
	}

	installPlugin := func(pkg tokens.Package) *semver.Version {
		assert.Contains(t, []string{"gcp"}, string(pkg))
		return nil
	}

	mapper, err := NewPluginMapper(ws, providerFactory, "key", nil, installPlugin)
	assert.NoError(t, err)
	assert.NotNil(t, mapper)

	ctx := context.Background()

	// Get the mapping for the GCP provider, which we don't have a plugin for.
	data, err := mapper.GetMapping(ctx, "gcp", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, data)
}
