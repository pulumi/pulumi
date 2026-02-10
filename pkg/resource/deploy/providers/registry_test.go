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

package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type testPluginHost struct {
	t             *testing.T
	provider      func(descriptor workspace.PluginDescriptor) (plugin.Provider, error)
	closeProvider func(provider plugin.Provider) error
}

func (host *testPluginHost) SignalCancellation() error {
	return nil
}

func (host *testPluginHost) Close() error {
	return nil
}

func (host *testPluginHost) ServerAddr() string {
	host.t.Fatalf("Host RPC address not available")
	return ""
}

func (host *testPluginHost) LoaderAddr() string {
	host.t.Fatalf("Loader RPC address not available")
	return ""
}

func (host *testPluginHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	host.t.Logf("[%v] %v@%v: %v", sev, urn, streamID, msg)
}

func (host *testPluginHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	host.t.Logf("[%v] %v@%v: %v", sev, urn, streamID, msg)
}

func (host *testPluginHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return nil, errors.New("unsupported")
}

func (host *testPluginHost) PolicyAnalyzer(name tokens.QName, path string,
	opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	return nil, errors.New("unsupported")
}

func (host *testPluginHost) ListAnalyzers() []plugin.Analyzer {
	return nil
}

func (host *testPluginHost) Provider(descriptor workspace.PluginDescriptor, e env.Env) (plugin.Provider, error) {
	return host.provider(descriptor)
}

func (host *testPluginHost) LanguageRuntime(root string) (plugin.LanguageRuntime, error) {
	return nil, errors.New("unsupported")
}

func (host *testPluginHost) EnsurePlugins(plugins []workspace.PluginDescriptor, kinds plugin.Flags) error {
	return nil
}

func (host *testPluginHost) ResolvePlugin(
	spec workspace.PluginDescriptor,
) (*workspace.PluginInfo, error) {
	return nil, nil
}

func (host *testPluginHost) GetProjectPlugins() []workspace.ProjectPlugin {
	return nil
}

func (host *testPluginHost) GetRequiredPlugins(project string, info plugin.ProgramInfo,
	kinds plugin.Flags,
) ([]workspace.PluginInfo, error) {
	return nil, nil
}

func (host *testPluginHost) StartDebugging(info plugin.DebuggingInfo) error {
	return nil
}

func (host *testPluginHost) AttachDebugger(_ plugin.DebugSpec) bool {
	return false
}

type testProvider struct {
	plugin.UnimplementedProvider

	pkg         tokens.Package
	version     semver.Version
	configured  bool
	checkConfig func(resource.URN, resource.PropertyMap,
		resource.PropertyMap, bool) (resource.PropertyMap, []plugin.CheckFailure, error)
	diffConfig func(resource.URN, resource.PropertyMap, resource.PropertyMap, bool, []string) (plugin.DiffResult, error)
	config     func(resource.PropertyMap) error
}

func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}

func (prov *testProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	return plugin.GetSchemaResponse{Schema: []byte("{}")}, nil
}

func (prov *testProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	props, failures, err := prov.checkConfig(req.URN, req.Olds, req.News, req.AllowUnknowns)
	return plugin.CheckConfigResponse{Properties: props, Failures: failures}, err
}

func (prov *testProvider) DiffConfig(
	_ context.Context, req plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return prov.diffConfig(req.URN, req.OldOutputs, req.NewInputs, req.AllowUnknowns, req.IgnoreChanges)
}

func (prov *testProvider) Configure(
	_ context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	if err := prov.config(req.Inputs); err != nil {
		return plugin.ConfigureResponse{}, err
	}
	prov.configured = true
	return plugin.ConfigureResponse{}, nil
}

func (prov *testProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: &prov.version,
	}, nil
}

func (prov *testProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (prov *testProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

type providerLoader struct {
	pkg     tokens.Package
	version semver.Version
	load    func() (plugin.Provider, error)
}

func newPluginHost(t *testing.T, loaders []*providerLoader) plugin.Host {
	return &testPluginHost{
		t: t,
		provider: func(descriptor workspace.PluginDescriptor) (plugin.Provider, error) {
			var best *providerLoader
			for _, l := range loaders {
				if string(l.pkg) != descriptor.Name {
					continue
				}

				if descriptor.Version != nil && l.version.LT(*descriptor.Version) {
					continue
				}
				if best == nil || l.version.GT(best.version) {
					best = l
				}
			}
			if best == nil {
				return nil, nil
			}
			return best.load()
		},
		closeProvider: func(provider plugin.Provider) error {
			return nil
		},
	}
}

func newLoader(t *testing.T, pkg, version string,
	load func(tokens.Package, semver.Version) (plugin.Provider, error),
) *providerLoader {
	var ver semver.Version
	if version != "" {
		v, err := semver.ParseTolerant(version)
		require.NoError(t, err)
		ver = v
	}
	return &providerLoader{
		pkg:     tokens.Package(pkg),
		version: ver,
		load: func() (plugin.Provider, error) {
			return load(tokens.Package(pkg), ver)
		},
	}
}

func newSimpleLoader(t *testing.T, pkg, version string, config func(resource.PropertyMap) error) *providerLoader {
	if config == nil {
		config = func(resource.PropertyMap) error {
			return nil
		}
	}
	return newLoader(t, pkg, version, func(pkg tokens.Package, ver semver.Version) (plugin.Provider, error) {
		return &testProvider{
			pkg:     pkg,
			version: ver,
			checkConfig: func(urn resource.URN, olds,
				news resource.PropertyMap, allowUnknowns bool,
			) (resource.PropertyMap, []plugin.CheckFailure, error) {
				return news, nil, nil
			},
			diffConfig: func(urn resource.URN, olds, news resource.PropertyMap,
				allowUnknowns bool, ignoreChanges []string,
			) (plugin.DiffResult, error) {
				return plugin.DiffResult{}, nil
			},
			config: config,
		}, nil
	})
}

func newProviderState(pkg, name, id string, del bool, inputs resource.PropertyMap) *resource.State {
	typ := providers.MakeProviderType(tokens.Package(pkg))
	urn := resource.NewURN("test", "test", "", typ, name)
	if inputs == nil {
		inputs = resource.PropertyMap{}
	}
	return &resource.State{
		Type:   typ,
		URN:    urn,
		Custom: true,
		Delete: del,
		ID:     resource.ID(id),
		Inputs: inputs,
	}
}

func TestNewRegistryNoOldState(t *testing.T) {
	t.Parallel()

	r := NewRegistry(&testPluginHost{}, false, nil)
	require.NotNil(t, r)

	r = NewRegistry(&testPluginHost{}, true, nil)
	require.NotNil(t, r)
}

func TestNewRegistryOldState(t *testing.T) {
	t.Parallel()

	olds := []*resource.State{
		// Two providers from package A, each with a unique name and ID
		newProviderState("pkgA", "a", "id1", false, nil),
		newProviderState("pkgA", "b", "id2", false, nil),
		// Two providers from package B, each with a unique name and ID
		newProviderState("pkgB", "a", "id1", false, nil),
		newProviderState("pkgB", "b", "id2", false, nil),
		// Two providers from package C, both with the same name but with unique IDs and one marked for deletion
		newProviderState("pkgC", "a", "id1", false, nil),
		newProviderState("pkgC", "a", "id2", true, nil),
		// One provider from package D with a version
		newProviderState("pkgD", "a", "id1", false, resource.PropertyMap{
			"version": resource.NewProperty("1.0.0"),
		}),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "", nil),
		newSimpleLoader(t, "pkgB", "", nil),
		newSimpleLoader(t, "pkgC", "", nil),
		newSimpleLoader(t, "pkgD", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	for _, old := range olds {
		ref, err := providers.NewReference(old.URN, old.ID)
		require.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.False(t, ok)
		assert.Nil(t, p)

		// "Same" the provider to add it to registry
		err = r.Same(context.Background(), old)
		require.NoError(t, err)

		// Now we should be able to get it
		p, ok = r.GetProvider(ref)
		assert.True(t, ok)
		require.NotNil(t, p)

		assert.True(t, p.(*testProvider).configured)

		assert.Equal(t, providers.GetProviderPackage(old.Type), p.Pkg())

		ver, err := GetProviderVersion(old.Inputs)
		require.NoError(t, err)
		if ver != nil {
			info, err := p.GetPluginInfo(context.Background())
			require.NoError(t, err)
			assert.True(t, info.Version.GTE(*ver))
		}
	}
}

func TestCRUD(t *testing.T) {
	t.Parallel()

	olds := []*resource.State{
		newProviderState("pkgA", "a", "id1", false, nil),
		newProviderState("pkgB", "a", "id1", false, nil),
		newProviderState("pkgC", "a", "id1", false, nil),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "", nil),
		newSimpleLoader(t, "pkgB", "", nil),
		newSimpleLoader(t, "pkgC", "", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	for _, old := range olds {
		ref, err := providers.NewReference(old.URN, old.ID)
		require.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.False(t, ok)
		assert.Nil(t, p)

		// "Same" the provider to add it to registry
		err = r.Same(context.Background(), old)
		require.NoError(t, err)

		// Now we should be able to get it
		p, ok = r.GetProvider(ref)
		assert.True(t, ok)
		require.NotNil(t, p)

		assert.Equal(t, providers.GetProviderPackage(old.Type), p.Pkg())
	}

	// Create a new provider for each package.
	for _, l := range loaders {
		typ := providers.MakeProviderType(l.pkg)
		urn := resource.NewURN("test", "test", "", typ, "b")
		olds, news := resource.PropertyMap{}, resource.PropertyMap{}
		timeout := float64(120)

		// Check
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: olds,
			News: news,
		})
		require.NoError(t, err)
		assert.Equal(t, news, check.Properties)
		assert.Empty(t, check.Failures)

		// Since this is not a preview, the provider should not yet be configured.
		p, ok := r.GetProvider(mustNewReference(urn, UnconfiguredID))
		assert.True(t, ok)
		assert.False(t, p.(*testProvider).configured)

		// Create
		create, err := r.Create(context.Background(), plugin.CreateRequest{
			URN:        urn,
			Name:       urn.Name(),
			Type:       urn.Type(),
			Properties: check.Properties,
			Timeout:    timeout,
		})
		require.NoError(t, err)
		assert.NotEqual(t, "", create.ID)
		assert.NotEqual(t, UnconfiguredID, create.ID)
		assert.NotEqual(t, UnknownID, create.ID)
		assert.Equal(t, resource.PropertyMap{}, create.Properties)
		assert.Equal(t, resource.StatusOK, create.Status)

		p2, ok := r.GetProvider(mustNewReference(urn, create.ID))
		assert.True(t, ok)
		assert.Equal(t, p, p2)
		assert.True(t, p2.(*testProvider).configured)
	}

	// Update the existing provider for the first entry in olds.
	{
		urn, id := olds[0].URN, olds[0].ID
		olds, news := olds[0].Inputs, olds[0].Inputs
		timeout := float64(120)

		// Fetch the old provider instance.
		old, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)

		// Check
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: olds,
			News: news,
		})
		require.NoError(t, err)
		assert.Equal(t, news, check.Properties)
		assert.Empty(t, check.Failures)

		// Since this is not a preview, the provider should not yet be configured.
		p, ok := r.GetProvider(mustNewReference(urn, UnconfiguredID))
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(context.Background(), plugin.DiffRequest{
			URN:        urn,
			ID:         id,
			OldOutputs: olds,
			NewInputs:  news,
		})
		require.NoError(t, err)
		assert.Equal(t, plugin.DiffResult{Changes: plugin.DiffNone}, diff)

		// The old provider should still be registered.
		p2, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)
		assert.Equal(t, old, p2)

		// Update
		update, err := r.Update(context.Background(), plugin.UpdateRequest{
			URN:        urn,
			ID:         id,
			OldOutputs: olds,
			NewInputs:  check.Properties,
			Timeout:    timeout,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{}, update.Properties)
		assert.Equal(t, resource.StatusOK, update.Status)

		p3, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)
		assert.True(t, p3 == p)
		assert.True(t, p3.(*testProvider).configured)
	}

	// Delete the existing provider for the last entry in olds.
	{
		urn, id := olds[len(olds)-1].URN, olds[len(olds)-1].ID
		timeout := float64(120)

		// Fetch the old provider instance.
		_, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)

		// Delete
		resp, err := r.Delete(context.Background(), plugin.DeleteRequest{
			URN:     urn,
			ID:      id,
			Inputs:  resource.PropertyMap{},
			Outputs: resource.PropertyMap{},
			Timeout: timeout,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.StatusOK, resp.Status)

		_, ok = r.GetProvider(mustNewReference(urn, id))
		assert.False(t, ok)
	}
}

func TestCRUDPreview(t *testing.T) {
	t.Parallel()

	olds := []*resource.State{
		newProviderState("pkgA", "a", "id1", false, nil),
		newProviderState("pkgB", "a", "id1", false, nil),
		newProviderState("pkgC", "a", "id1", false, nil),
		newProviderState("pkgD", "a", "id1", false, nil),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "", nil),
		newSimpleLoader(t, "pkgB", "", nil),
		newSimpleLoader(t, "pkgC", "", nil),
		newLoader(t, "pkgD", "", func(pkg tokens.Package, ver semver.Version) (plugin.Provider, error) {
			return &testProvider{
				pkg:     pkg,
				version: ver,
				checkConfig: func(urn resource.URN, olds,
					news resource.PropertyMap, allowUnknowns bool,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return news, nil, nil
				},
				diffConfig: func(urn resource.URN, olds, news resource.PropertyMap,
					allowUnknowns bool, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// Always reuquire replacement.
					return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"id"}}, nil
				},
				config: func(inputs resource.PropertyMap) error {
					return nil
				},
			}, nil
		}),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, true, nil)
	require.NotNil(t, r)

	for _, old := range olds {
		ref, err := providers.NewReference(old.URN, old.ID)
		require.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.False(t, ok)
		assert.Nil(t, p)

		// "Same" the provider to add it to registry
		err = r.Same(context.Background(), old)
		require.NoError(t, err)

		// Now we should be able to get it
		p, ok = r.GetProvider(ref)
		assert.True(t, ok)
		require.NotNil(t, p)

		assert.Equal(t, providers.GetProviderPackage(old.Type), p.Pkg())
	}

	// Create a new provider for each package.
	for _, l := range loaders {
		typ := providers.MakeProviderType(l.pkg)
		urn := resource.NewURN("test", "test", "", typ, "b")
		olds, news := resource.PropertyMap{}, resource.PropertyMap{}

		// Check
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: olds,
			News: news,
		})
		require.NoError(t, err)
		assert.Equal(t, news, check.Properties)
		assert.Empty(t, check.Failures)

		// The provider should not be configured: configuration will occur during the previewed Create.
		p, ok := r.GetProvider(mustNewReference(urn, UnconfiguredID))
		assert.True(t, ok)
		assert.False(t, p.(*testProvider).configured)
	}

	// Update the existing provider for the first entry in olds.
	{
		urn, id := olds[0].URN, olds[0].ID
		olds, news := olds[0].Inputs, olds[0].Inputs

		// Fetch the old provider instance.
		old, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)

		// Check
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: olds,
			News: news,
		})
		require.NoError(t, err)
		assert.Equal(t, news, check.Properties)
		assert.Empty(t, check.Failures)

		// The provider should remain unconfigured.
		p, ok := r.GetProvider(mustNewReference(urn, UnconfiguredID))
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(context.Background(), plugin.DiffRequest{
			URN:        urn,
			ID:         id,
			OldOutputs: olds,
			NewInputs:  news,
		})
		require.NoError(t, err)
		assert.Equal(t, plugin.DiffResult{Changes: plugin.DiffNone}, diff)

		// The original provider should be used because the config did not change.
		p2, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)
		assert.True(t, p2 == old)
		assert.False(t, p2 == p)
	}

	// Replace the existing provider for the last entry in olds.
	{
		urn, id := olds[len(olds)-1].URN, olds[len(olds)-1].ID
		olds, news := olds[len(olds)-1].Inputs, olds[len(olds)-1].Inputs

		// Fetch the old provider instance.
		old, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)

		// Check
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: olds,
			News: news,
		})
		require.NoError(t, err)
		assert.Equal(t, news, check.Properties)
		assert.Empty(t, check.Failures)

		// The provider should remain unconfigured.
		p, ok := r.GetProvider(mustNewReference(urn, UnconfiguredID))
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(context.Background(), plugin.DiffRequest{
			URN:        urn,
			ID:         id,
			OldOutputs: olds,
			NewInputs:  news,
		})
		require.NoError(t, err)
		assert.True(t, diff.Replace())

		// The new provider should be not be registered; the registered provider should still be the original.
		p2, ok := r.GetProvider(mustNewReference(urn, id))
		assert.True(t, ok)
		assert.True(t, p2 == old)
		assert.False(t, p2 == p)
	}
}

func TestCRUDNoProviders(t *testing.T) {
	t.Parallel()

	host := newPluginHost(t, []*providerLoader{})

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	typ := providers.MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{}

	// Check
	check, err := r.Check(context.Background(), plugin.CheckRequest{
		URN:  urn,
		Olds: olds,
		News: news,
	})
	assert.Error(t, err)
	assert.Empty(t, check.Failures)
	assert.Nil(t, check.Properties)
}

func TestCRUDWrongPackage(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgB", "", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	typ := providers.MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{}

	// Check
	check, err := r.Check(context.Background(), plugin.CheckRequest{
		URN:  urn,
		Olds: olds,
		News: news,
	})
	assert.Error(t, err)
	assert.Empty(t, check.Failures)
	assert.Nil(t, check.Properties)
}

func TestCRUDWrongVersion(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "0.5.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	typ := providers.MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewProperty("1.0.0")}

	// Check
	check, err := r.Check(context.Background(), plugin.CheckRequest{
		URN:  urn,
		Olds: olds,
		News: news,
	})
	assert.Error(t, err)
	assert.Empty(t, check.Failures)
	assert.Nil(t, check.Properties)
}

func TestCRUDBadVersionNotString(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	typ := providers.MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewProperty(true)}

	// Check
	check, err := r.Check(context.Background(), plugin.CheckRequest{
		URN:  urn,
		Olds: olds,
		News: news,
	})
	require.NoError(t, err)
	require.Len(t, check.Failures, 1)
	assert.Equal(t, "version", string(check.Failures[0].Property))
	assert.Nil(t, check.Properties)
}

func TestCRUDBadVersion(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	typ := providers.MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewProperty("foo")}

	// Check
	check, err := r.Check(context.Background(), plugin.CheckRequest{
		URN:  urn,
		Olds: olds,
		News: news,
	})
	require.NoError(t, err)
	require.Len(t, check.Failures, 1)
	assert.Equal(t, "version", string(check.Failures[0].Property))
	assert.Nil(t, check.Properties)
}

//nolint:paralleltest
func TestLoadProvider_missingError(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	version := semver.MustParse("1.2.3")
	loader := newLoader(t, "myplugin", "1.2.3",
		func(p tokens.Package, v semver.Version) (plugin.Provider, error) {
			return nil, workspace.NewMissingError(
				workspace.PluginDescriptor{
					Kind:    apitype.ResourcePlugin,
					Name:    "myplugin",
					Version: &version,
				}, false /* ambient */)
		})
	host := newPluginHost(t, []*providerLoader{loader})

	t.Run("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

		_, err := loadProvider(
			context.Background(),
			"myplugin", &version, srv.URL,
			nil, host, nil /* builtins */, nil)
		assert.ErrorContains(t, err,
			"no resource plugin 'pulumi-resource-myplugin' found in the workspace at version v1.2.3")
		assert.Equal(t, 0, count)
	})

	t.Run("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=false", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

		_, err := loadProvider(
			context.Background(),
			"myplugin", &version, srv.URL,
			nil, host, nil /* builtins */, nil)
		assert.ErrorContains(t, err,
			"Could not automatically download and install resource plugin 'pulumi-resource-myplugin' at version v1.2.3")
		assert.ErrorContains(t, err,
			fmt.Sprintf("install the plugin using `pulumi plugin install resource myplugin v1.2.3 --server %s`", srv.URL))
		assert.Equal(t, 5, count)
	})
}

func TestConcurrentRegistryUsage(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/13491, make sure we can use registry in
	// parallel.

	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	require.NotNil(t, r)

	// We're going to create a few thousand providers in parallel, registering a load of aliases for each of
	// them.
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			typ := providers.MakeProviderType("pkgA")
			providerURN := resource.NewURN("test", "test", "", typ, fmt.Sprintf("p%d", i))

			for j := 0; j < 1000; j++ {
				aliasURN := resource.NewURN("test", "test", "", typ, fmt.Sprintf("p%d_%d", i, j))
				r.RegisterAlias(providerURN, aliasURN)
			}

			// Now check that we can get the provider back.
			olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewProperty(true)}

			// Check
			check, err := r.Check(context.Background(), plugin.CheckRequest{
				URN:  providerURN,
				Olds: olds,
				News: news,
			})
			require.NoError(t, err)
			require.Len(t, check.Failures, 1)
			assert.Equal(t, "version", string(check.Failures[0].Property))
			assert.Nil(t, check.Properties)
		}(i)
	}
}

func TestRegistry(t *testing.T) {
	t.Parallel()
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Nil(t, r.Close())
		// Ensure idempotent.
		assert.Nil(t, r.Close())
	})
	t.Run("Pkg", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Equal(t, tokens.Package("pulumi"), r.Pkg())
	})
	t.Run("GetSchema", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.GetSchema(context.Background(), plugin.GetSchemaRequest{})
		})
	})
	t.Run("GetMapping", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.GetMapping(context.Background(), plugin.GetMappingRequest{})
		})
	})
	t.Run("GetMappings", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.GetMappings(context.Background(), plugin.GetMappingsRequest{})
		})
	})
	t.Run("CheckConfig", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.CheckConfig(context.Background(), plugin.CheckConfigRequest{AllowUnknowns: true})
		})
	})
	t.Run("DiffConfig", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.DiffConfig(context.Background(), plugin.DiffConfigRequest{AllowUnknowns: true})
		})
	})
	t.Run("Configure", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.Configure(context.Background(), plugin.ConfigureRequest{})
		})
	})
	t.Run("Read", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		_, err := r.Read(context.Background(), plugin.ReadRequest{})
		assert.ErrorContains(t, err, "provider resources may not be read")
	})
	t.Run("Construct", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		_, err := r.Construct(context.Background(), plugin.ConstructRequest{})
		assert.ErrorContains(t, err, "provider resources may not be constructed")
	})
	t.Run("Invoke", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.Invoke(context.Background(), plugin.InvokeRequest{})
		})
	})
	t.Run("Call", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Panics(t, func() {
			_, _ = r.Call(context.Background(), plugin.CallRequest{})
		})
	})
	t.Run("GetPluginInfo", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		_, err := r.GetPluginInfo(context.Background())
		assert.ErrorContains(t, err, "the provider registry does not report plugin info")
	})
	t.Run("SignalCancellation", func(t *testing.T) {
		t.Parallel()
		r := &Registry{}
		assert.Nil(t, r.SignalCancellation(context.Background()))
	})
}

func TestEnvironmentVariableMappings(t *testing.T) {
	t.Parallel()

	t.Run("SetAndGet", func(t *testing.T) {
		t.Parallel()

		inputs := resource.PropertyMap{}
		mappings := map[string]string{
			"MY_SPECIAL_VAR": "PROVIDER_VAR",
			"ANOTHER_VAR":    "OTHER_VAR",
		}

		SetEnvironmentVariableMappings(inputs, mappings)

		retrieved, err := GetEnvironmentVariableMappings(inputs)
		require.NoError(t, err)
		assert.Equal(t, mappings, retrieved)
	})

	t.Run("GetEmpty", func(t *testing.T) {
		t.Parallel()

		inputs := resource.PropertyMap{}
		retrieved, err := GetEnvironmentVariableMappings(inputs)
		require.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("GetWithNoInternal", func(t *testing.T) {
		t.Parallel()

		// Old state without __internal key should return nil
		inputs := resource.PropertyMap{
			"version": resource.NewProperty("1.0.0"),
		}
		retrieved, err := GetEnvironmentVariableMappings(inputs)
		require.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("ErrorWhenInternalNotObject", func(t *testing.T) {
		t.Parallel()

		// __internal is not an object (it's a string)
		inputs := resource.PropertyMap{
			"__internal": resource.NewProperty("not-an-object"),
		}
		retrieved, err := GetEnvironmentVariableMappings(inputs)
		assert.ErrorContains(t, err, "'__internal' must be an object")
		assert.Nil(t, retrieved)
	})

	t.Run("ErrorWhenEnvVarMappingsNotObject", func(t *testing.T) {
		t.Parallel()

		// envVarMappings is not an object (it's a string)
		inputs := resource.PropertyMap{
			"__internal": resource.NewProperty(resource.PropertyMap{
				"envVarMappings": resource.NewProperty("not-an-object"),
			}),
		}
		retrieved, err := GetEnvironmentVariableMappings(inputs)
		assert.ErrorContains(t, err, "'envVarMappings' must be an object")
		assert.Nil(t, retrieved)
	})

	t.Run("ErrorWhenMappingValueNotString", func(t *testing.T) {
		t.Parallel()

		// A value in envVarMappings is not a string (it's a number)
		inputs := resource.PropertyMap{
			"__internal": resource.NewProperty(resource.PropertyMap{
				"envVarMappings": resource.NewProperty(resource.PropertyMap{
					"VALID_KEY":   resource.NewProperty("valid-value"),
					"INVALID_KEY": resource.NewProperty(123.0),
				}),
			}),
		}
		retrieved, err := GetEnvironmentVariableMappings(inputs)
		assert.ErrorContains(t, err, "'envVarMappings[INVALID_KEY]' must be a string")
		assert.Nil(t, retrieved)
	})

	t.Run("ProviderWithEnvMappings", func(t *testing.T) {
		t.Parallel()

		// Create provider inputs with env mappings
		inputs := resource.PropertyMap{}
		mappings := map[string]string{"SOURCE_VAR": "TARGET_VAR"}
		SetEnvironmentVariableMappings(inputs, mappings)

		// Create provider state
		old := newProviderState("pkgA", "test-provider", "id1", false, inputs)

		loaders := []*providerLoader{
			newSimpleLoader(t, "pkgA", "", nil),
		}
		host := newPluginHost(t, loaders)

		r := NewRegistry(host, false, nil)
		require.NotNil(t, r)

		// Same the provider
		err := r.Same(context.Background(), old)
		require.NoError(t, err)

		// Verify provider is registered
		ref, err := providers.NewReference(old.URN, old.ID)
		require.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.True(t, ok)
		require.NotNil(t, p)

		// Verify the mappings can be retrieved from the original inputs
		retrieved, err := GetEnvironmentVariableMappings(old.Inputs)
		require.NoError(t, err)
		assert.Equal(t, mappings, retrieved)
	})

	t.Run("CheckWithEnvMappings", func(t *testing.T) {
		t.Parallel()

		loaders := []*providerLoader{
			newSimpleLoader(t, "testPackage", "", nil),
		}
		host := newPluginHost(t, loaders)

		r := NewRegistry(host, false, nil)
		require.NotNil(t, r)

		typ := providers.MakeProviderType(tokens.Package("testPackage"))
		urn := resource.NewURN("test", "test", "", typ, "test-provider")

		// Create news with env mappings
		news := resource.PropertyMap{}
		mappings := map[string]string{"MY_VAR": "PROVIDER_VAR"}
		SetEnvironmentVariableMappings(news, mappings)

		// Check should succeed and preserve the mappings
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: resource.PropertyMap{},
			News: news,
		})
		require.NoError(t, err)
		assert.Empty(t, check.Failures)

		// The returned properties should contain the mappings
		retrieved, err := GetEnvironmentVariableMappings(check.Properties)
		require.NoError(t, err)
		assert.Equal(t, mappings, retrieved)
	})

	t.Run("CreateWithEnvMappings", func(t *testing.T) {
		t.Parallel()

		loaders := []*providerLoader{
			newSimpleLoader(t, "testPackage", "", nil),
		}
		host := newPluginHost(t, loaders)

		r := NewRegistry(host, false, nil)
		require.NotNil(t, r)

		typ := providers.MakeProviderType(tokens.Package("testPackage"))
		urn := resource.NewURN("test", "test", "", typ, "test-provider")

		// Create inputs with env mappings
		inputs := resource.PropertyMap{}
		mappings := map[string]string{"MY_VAR": "PROVIDER_VAR"}
		SetEnvironmentVariableMappings(inputs, mappings)

		// Call Check first
		check, err := r.Check(context.Background(), plugin.CheckRequest{
			URN:  urn,
			Olds: resource.PropertyMap{},
			News: inputs,
		})
		require.NoError(t, err)

		create, err := r.Create(context.Background(), plugin.CreateRequest{
			URN:        urn,
			Name:       urn.Name(),
			Type:       urn.Type(),
			Properties: check.Properties,
			Timeout:    120,
		})
		require.NoError(t, err)
		assert.NotEqual(t, "", create.ID)
		assert.Equal(t, resource.StatusOK, create.Status)

		// Verify provider is registered and configured
		p, ok := r.GetProvider(mustNewReference(urn, create.ID))
		assert.True(t, ok)
		assert.True(t, p.(*testProvider).configured)
	})
}

// testPluginHostWithEnvCapture is a test host that captures the env passed to Provider()
type testPluginHostWithEnvCapture struct {
	testPluginHost
	capturedEnv env.Env
}

//nolint:lll
func (host *testPluginHostWithEnvCapture) Provider(descriptor workspace.PluginDescriptor, e env.Env) (plugin.Provider, error) {
	host.capturedEnv = e
	return host.provider(descriptor)
}

func TestEnvMappingsPassedToHost(t *testing.T) {
	// Set SOURCE_VAR in the environment so the mapping can be tested
	t.Setenv("CUSTOM_VAR", "use-this-value")

	// Create a host that captures the environment passed to Provider()
	customHost := &testPluginHostWithEnvCapture{
		testPluginHost: testPluginHost{
			t: t,
			provider: func(descriptor workspace.PluginDescriptor) (plugin.Provider, error) {
				return &testProvider{
					pkg:     tokens.Package(descriptor.Name),
					version: semver.MustParse("1.0.0"),
					//nolint:lll
					checkConfig: func(urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
						return news, nil, nil
					},
					//nolint:lll
					diffConfig: func(urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
						return plugin.DiffResult{}, nil
					},
					config: func(inputs resource.PropertyMap) error {
						return nil
					},
				}, nil
			},
		},
	}

	r := NewRegistry(customHost, false, nil)
	require.NotNil(t, r)

	typ := providers.MakeProviderType(tokens.Package("testPackage"))
	urn := resource.NewURN("test", "test", "", typ, "test-provider")
	inputs := resource.PropertyMap{}
	mappings := map[string]string{"CUSTOM_VAR": "PROVIDER_VAR"}
	SetEnvironmentVariableMappings(inputs, mappings)

	// Load the provider and pass env to host
	_, err := r.Check(context.Background(), plugin.CheckRequest{
		URN:  urn,
		Olds: resource.PropertyMap{},
		News: inputs,
	})
	require.NoError(t, err)

	// Verify that an env was passed to the host
	require.NotNil(t, customHost.capturedEnv, "Environment should be passed to host.Provider()")

	store := customHost.capturedEnv.GetStore()
	require.NotNil(t, store, "Environment should have a store")

	targetValue, ok := store.Raw("PROVIDER_VAR")
	assert.True(t, ok, "PROVIDER_VAR should exist in the environment")
	customValue, ok := store.Raw("CUSTOM_VAR")
	assert.True(t, ok, "CUSTOM_VAR should also still exist in the environment")
	assert.Equal(t, targetValue, customValue, "PROVIDER_VAR should have CUSTOM_VAR's value")
}
