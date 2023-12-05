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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type testPluginHost struct {
	t             *testing.T
	provider      func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error)
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

func (host *testPluginHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	return host.provider(pkg, version)
}

func (host *testPluginHost) CloseProvider(provider plugin.Provider) error {
	return host.closeProvider(provider)
}

func (host *testPluginHost) LanguageRuntime(
	root, pwd, runtime string, options map[string]interface{},
) (plugin.LanguageRuntime, error) {
	return nil, errors.New("unsupported")
}

func (host *testPluginHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds plugin.Flags) error {
	return nil
}

func (host *testPluginHost) ResolvePlugin(
	kind workspace.PluginKind, name string, version *semver.Version,
) (*workspace.PluginInfo, error) {
	return nil, nil
}

func (host *testPluginHost) GetProjectPlugins() []workspace.ProjectPlugin {
	return nil
}

func (host *testPluginHost) GetRequiredPlugins(info plugin.ProgInfo,
	kinds plugin.Flags,
) ([]workspace.PluginInfo, error) {
	return nil, nil
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

func (prov *testProvider) GetSchema(version int) ([]byte, error) {
	return []byte("{}"), nil
}

func (prov *testProvider) CheckConfig(urn resource.URN, olds,
	news resource.PropertyMap, allowUnknowns bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.checkConfig(urn, olds, news, allowUnknowns)
}

func (prov *testProvider) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return prov.diffConfig(urn, oldOutputs, newInputs, allowUnknowns, ignoreChanges)
}

func (prov *testProvider) Configure(inputs resource.PropertyMap) error {
	if err := prov.config(inputs); err != nil {
		return err
	}
	prov.configured = true
	return nil
}

func (prov *testProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{
		Name:    "testProvider",
		Version: &prov.version,
	}, nil
}

func (prov *testProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", nil
}

func (prov *testProvider) GetMappings(key string) ([]string, error) {
	return []string{}, nil
}

type providerLoader struct {
	pkg     tokens.Package
	version semver.Version
	load    func() (plugin.Provider, error)
}

func newPluginHost(t *testing.T, loaders []*providerLoader) plugin.Host {
	return &testPluginHost{
		t: t,
		provider: func(pkg tokens.Package, ver *semver.Version) (plugin.Provider, error) {
			var best *providerLoader
			for _, l := range loaders {
				if l.pkg != pkg {
					continue
				}

				if ver != nil && l.version.LT(*ver) {
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
		assert.NoError(t, err)
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

func newProviderState(pkg, name, id string, delete bool, inputs resource.PropertyMap) *resource.State {
	typ := MakeProviderType(tokens.Package(pkg))
	urn := resource.NewURN("test", "test", "", typ, name)
	if inputs == nil {
		inputs = resource.PropertyMap{}
	}
	return &resource.State{
		Type:   typ,
		URN:    urn,
		Custom: true,
		Delete: delete,
		ID:     resource.ID(id),
		Inputs: inputs,
	}
}

func TestNewRegistryNoOldState(t *testing.T) {
	t.Parallel()

	r := NewRegistry(&testPluginHost{}, false, nil)
	assert.NotNil(t, r)

	r = NewRegistry(&testPluginHost{}, true, nil)
	assert.NotNil(t, r)
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
			"version": resource.NewStringProperty("1.0.0"),
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
	assert.NotNil(t, r)

	for _, old := range olds {
		ref, err := NewReference(old.URN, old.ID)
		assert.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.False(t, ok)
		assert.Nil(t, p)

		// "Same" the provider to add it to registry
		err = r.Same(old)
		assert.NoError(t, err)

		// Now we should be able to get it
		p, ok = r.GetProvider(ref)
		assert.True(t, ok)
		assert.NotNil(t, p)

		assert.True(t, p.(*testProvider).configured)

		assert.Equal(t, GetProviderPackage(old.Type), p.Pkg())

		ver, err := GetProviderVersion(old.Inputs)
		assert.NoError(t, err)
		if ver != nil {
			info, err := p.GetPluginInfo()
			assert.NoError(t, err)
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
	assert.NotNil(t, r)

	for _, old := range olds {
		ref, err := NewReference(old.URN, old.ID)
		assert.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.False(t, ok)
		assert.Nil(t, p)

		// "Same" the provider to add it to registry
		err = r.Same(old)
		assert.NoError(t, err)

		// Now we should be able to get it
		p, ok = r.GetProvider(ref)
		assert.True(t, ok)
		assert.NotNil(t, p)

		assert.Equal(t, GetProviderPackage(old.Type), p.Pkg())
	}

	// Create a new provider for each package.
	for _, l := range loaders {
		typ := MakeProviderType(l.pkg)
		urn := resource.NewURN("test", "test", "", typ, "b")
		olds, news := resource.PropertyMap{}, resource.PropertyMap{}
		timeout := float64(120)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is not a preview, the provider should not yet be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnconfiguredID})
		assert.True(t, ok)
		assert.False(t, p.(*testProvider).configured)

		// Create
		id, outs, status, err := r.Create(urn, inputs, timeout, false)
		assert.NoError(t, err)
		assert.NotEqual(t, "", id)
		assert.NotEqual(t, UnconfiguredID, id)
		assert.NotEqual(t, UnknownID, id)
		assert.Equal(t, resource.PropertyMap{}, outs)
		assert.Equal(t, resource.StatusOK, status)

		p2, ok := r.GetProvider(Reference{urn: urn, id: id})
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
		old, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is not a preview, the provider should not yet be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnconfiguredID})
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(urn, id, nil, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, plugin.DiffResult{Changes: plugin.DiffNone}, diff)

		// The old provider should still be registered.
		p2, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.Equal(t, old, p2)

		// Update
		outs, status, err := r.Update(urn, id, nil, olds, inputs, timeout, nil, false)
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{}, outs)
		assert.Equal(t, resource.StatusOK, status)

		p3, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.True(t, p3 == p)
		assert.True(t, p3.(*testProvider).configured)
	}

	// Delete the existing provider for the last entry in olds.
	{
		urn, id := olds[len(olds)-1].URN, olds[len(olds)-1].ID
		timeout := float64(120)

		// Fetch the old provider instance.
		_, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Delete
		status, err := r.Delete(urn, id, resource.PropertyMap{}, resource.PropertyMap{}, timeout)
		assert.NoError(t, err)
		assert.Equal(t, resource.StatusOK, status)

		_, ok = r.GetProvider(Reference{urn: urn, id: id})
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
	assert.NotNil(t, r)

	for _, old := range olds {
		ref, err := NewReference(old.URN, old.ID)
		assert.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.False(t, ok)
		assert.Nil(t, p)

		// "Same" the provider to add it to registry
		err = r.Same(old)
		assert.NoError(t, err)

		// Now we should be able to get it
		p, ok = r.GetProvider(ref)
		assert.True(t, ok)
		assert.NotNil(t, p)

		assert.Equal(t, GetProviderPackage(old.Type), p.Pkg())
	}

	// Create a new provider for each package.
	for _, l := range loaders {
		typ := MakeProviderType(l.pkg)
		urn := resource.NewURN("test", "test", "", typ, "b")
		olds, news := resource.PropertyMap{}, resource.PropertyMap{}

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// The provider should not be configured: configuration will occur during the previewed Create.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnconfiguredID})
		assert.True(t, ok)
		assert.False(t, p.(*testProvider).configured)
	}

	// Update the existing provider for the first entry in olds.
	{
		urn, id := olds[0].URN, olds[0].ID
		olds, news := olds[0].Inputs, olds[0].Inputs

		// Fetch the old provider instance.
		old, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// The provider should remain unconfigured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnconfiguredID})
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(urn, id, nil, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, plugin.DiffResult{Changes: plugin.DiffNone}, diff)

		// The original provider should be used because the config did not change.
		p2, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.True(t, p2 == old)
		assert.False(t, p2 == p)
	}

	// Replace the existing provider for the last entry in olds.
	{
		urn, id := olds[len(olds)-1].URN, olds[len(olds)-1].ID
		olds, news := olds[len(olds)-1].Inputs, olds[len(olds)-1].Inputs

		// Fetch the old provider instance.
		old, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false, nil)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// The provider should remain unconfigured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnconfiguredID})
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(urn, id, nil, olds, news, false, nil)
		assert.NoError(t, err)
		assert.True(t, diff.Replace())

		// The new provider should be not be registered; the registered provider should still be the original.
		p2, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.True(t, p2 == old)
		assert.False(t, p2 == p)
	}
}

func TestCRUDNoProviders(t *testing.T) {
	t.Parallel()

	host := newPluginHost(t, []*providerLoader{})

	r := NewRegistry(host, false, nil)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false, nil)
	assert.Error(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, inputs)
}

func TestCRUDWrongPackage(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgB", "", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false, nil)
	assert.Error(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, inputs)
}

func TestCRUDWrongVersion(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "0.5.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewStringProperty("1.0.0")}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false, nil)
	assert.Error(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, inputs)
}

func TestCRUDBadVersionNotString(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewBoolProperty(true)}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false, nil)
	assert.NoError(t, err)
	assert.Len(t, failures, 1)
	assert.Equal(t, "version", string(failures[0].Property))
	assert.Nil(t, inputs)
}

func TestCRUDBadVersion(t *testing.T) {
	t.Parallel()

	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r := NewRegistry(host, false, nil)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewStringProperty("foo")}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false, nil)
	assert.NoError(t, err)
	assert.Len(t, failures, 1)
	assert.Equal(t, "version", string(failures[0].Property))
	assert.Nil(t, inputs)
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
				workspace.ResourcePlugin, "myplugin", &version, false /* ambient */)
		})
	host := newPluginHost(t, []*providerLoader{loader})

	t.Run("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

		_, err := loadProvider(
			"myplugin", &version, srv.URL,
			nil, host, nil /* builtins */)
		assert.ErrorContains(t, err,
			"no resource plugin 'pulumi-resource-myplugin' found in the workspace at version v1.2.3")
		assert.Equal(t, 0, count)
	})

	t.Run("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=false", func(t *testing.T) {
		t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

		_, err := loadProvider(
			"myplugin", &version, srv.URL,
			nil, host, nil /* builtins */)
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
	assert.NotNil(t, r)

	// We're going to create a few thousand providers in parallel, registering a load of aliases for each of
	// them.
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			typ := MakeProviderType("pkgA")
			providerURN := resource.NewURN("test", "test", "", typ, fmt.Sprintf("p%d", i))

			for j := 0; j < 1000; j++ {
				aliasURN := resource.NewURN("test", "test", "", typ, fmt.Sprintf("p%d_%d", i, j))
				r.RegisterAlias(providerURN, aliasURN)
			}

			// Now check that we can get the provider back.
			olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewBoolProperty(true)}

			// Check
			inputs, failures, err := r.Check(providerURN, olds, news, false, nil)
			assert.NoError(t, err)
			assert.Len(t, failures, 1)
			assert.Equal(t, "version", string(failures[0].Property))
			assert.Nil(t, inputs)
		}(i)
	}
}
