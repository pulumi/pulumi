// Copyright 2016-2018, Pulumi Corporation.
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
	"testing"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
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
func (host *testPluginHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	return host.provider(pkg, version)
}
func (host *testPluginHost) CloseProvider(provider plugin.Provider) error {
	return host.closeProvider(provider)
}
func (host *testPluginHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	return nil, errors.New("unsupported")
}
func (host *testPluginHost) ListPlugins() []workspace.PluginInfo {
	return nil
}
func (host *testPluginHost) EnsurePlugins(plugins []workspace.PluginInfo, kinds plugin.Flags) error {
	return nil
}
func (host *testPluginHost) GetRequiredPlugins(info plugin.ProgInfo,
	kinds plugin.Flags) ([]workspace.PluginInfo, error) {
	return nil, nil
}

type testProvider struct {
	pkg         tokens.Package
	version     semver.Version
	configured  bool
	checkConfig func(resource.PropertyMap, resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
	diffConfig  func(resource.PropertyMap, resource.PropertyMap) (plugin.DiffResult, error)
	config      func(resource.PropertyMap) error
}

func (prov *testProvider) SignalCancellation() error {
	return nil
}
func (prov *testProvider) Close() error {
	return nil
}
func (prov *testProvider) Pkg() tokens.Package {
	return prov.pkg
}
func (prov *testProvider) CheckConfig(olds,
	news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return prov.checkConfig(olds, news)
}
func (prov *testProvider) DiffConfig(olds, news resource.PropertyMap) (plugin.DiffResult, error) {
	return prov.diffConfig(olds, news)
}
func (prov *testProvider) Configure(inputs resource.PropertyMap) error {
	if err := prov.config(inputs); err != nil {
		return err
	}
	prov.configured = true
	return nil
}
func (prov *testProvider) Check(urn resource.URN,
	olds, news resource.PropertyMap, _ bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return nil, nil, errors.New("unsupported")
}
func (prov *testProvider) Create(urn resource.URN, props resource.PropertyMap) (resource.ID,
	resource.PropertyMap, resource.Status, error) {
	return "", nil, resource.StatusOK, errors.New("unsupported")
}
func (prov *testProvider) Read(urn resource.URN, id resource.ID,
	props resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	return nil, resource.StatusUnknown, errors.New("unsupported")
}
func (prov *testProvider) Diff(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap, _ bool) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, errors.New("unsupported")
}
func (prov *testProvider) Update(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	return nil, resource.StatusOK, errors.New("unsupported")
}
func (prov *testProvider) Delete(urn resource.URN,
	id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	return resource.StatusOK, errors.New("unsupported")
}
func (prov *testProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return nil, nil, errors.New("unsupported")
}
func (prov *testProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{
		Name:    "testProvider",
		Version: &prov.version,
	}, nil
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
	load func(tokens.Package, semver.Version) (plugin.Provider, error)) *providerLoader {

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
			checkConfig: func(olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
				return news, nil, nil
			},
			diffConfig: func(olds, news resource.PropertyMap) (plugin.DiffResult, error) {
				return plugin.DiffResult{}, nil
			},
			config: config,
		}, nil
	})
}

func newProviderState(pkg, name, id string, delete bool, inputs resource.PropertyMap) *resource.State {
	typ := MakeProviderType(tokens.Package(pkg))
	urn := resource.NewURN("test", "test", "", typ, tokens.QName(name))
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
	r, err := NewRegistry(&testPluginHost{}, nil, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	r, err = NewRegistry(&testPluginHost{}, nil, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestNewRegistryOldState(t *testing.T) {
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

	r, err := NewRegistry(host, olds, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	assert.Equal(t, len(olds), len(r.providers))

	for _, old := range olds {
		ref, err := NewReference(old.URN, old.ID)
		assert.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.True(t, ok)
		assert.NotNil(t, p)

		assert.True(t, p.(*testProvider).configured)

		assert.Equal(t, getProviderPackage(old.Type), p.Pkg())

		ver, err := getProviderVersion(old.Inputs)
		assert.NoError(t, err)
		if ver != nil {
			info, err := p.GetPluginInfo()
			assert.NoError(t, err)
			assert.True(t, info.Version.GTE(*ver))
		}
	}
}

func TestNewRegistryOldStateNoProviders(t *testing.T) {
	olds := []*resource.State{
		newProviderState("pkgA", "a", "id1", false, nil),
	}
	host := newPluginHost(t, []*providerLoader{})

	r, err := NewRegistry(host, olds, false, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewRegistryOldStateWrongPackage(t *testing.T) {
	olds := []*resource.State{
		newProviderState("pkgA", "a", "id1", false, nil),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgB", "", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, olds, false, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewRegistryOldStateWrongVersion(t *testing.T) {
	olds := []*resource.State{
		newProviderState("pkgA", "a", "id1", false, resource.PropertyMap{
			"version": resource.NewStringProperty("1.0.0"),
		}),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "0.5.0", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, olds, false, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewRegistryOldStateNoID(t *testing.T) {
	olds := []*resource.State{
		newProviderState("pkgA", "a", "", false, nil),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, olds, false, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewRegistryOldStateUnknownID(t *testing.T) {
	olds := []*resource.State{
		newProviderState("pkgA", "a", UnknownID, false, nil),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, olds, false, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestNewRegistryOldStateDuplicates(t *testing.T) {
	olds := []*resource.State{
		newProviderState("pkgA", "a", "id1", false, nil),
		newProviderState("pkgA", "a", "id1", false, nil),
	}
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, olds, false, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestCRUD(t *testing.T) {
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

	r, err := NewRegistry(host, olds, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	assert.Equal(t, len(olds), len(r.providers))

	for _, old := range olds {
		ref, err := NewReference(old.URN, old.ID)
		assert.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.True(t, ok)
		assert.NotNil(t, p)

		assert.Equal(t, getProviderPackage(old.Type), p.Pkg())
	}

	// Create a new provider for each package.
	for _, l := range loaders {
		typ := MakeProviderType(l.pkg)
		urn := resource.NewURN("test", "test", "", typ, "b")
		olds, news := resource.PropertyMap{}, resource.PropertyMap{}

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is not a preview, the provider should not yet be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnknownID})
		assert.True(t, ok)
		assert.False(t, p.(*testProvider).configured)

		// Create
		id, outs, status, err := r.Create(urn, inputs)
		assert.NoError(t, err)
		assert.NotEqual(t, "", id)
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

		// Fetch the old provider instance.
		old, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is not a preview, the provider should not yet be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnknownID})
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.False(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(urn, id, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, plugin.DiffResult{}, diff)

		// The old provider should still be registered.
		p2, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.Equal(t, old, p2)

		// Update
		outs, status, err := r.Update(urn, id, olds, inputs)
		assert.NoError(t, err)
		assert.Equal(t, resource.PropertyMap{}, outs)
		assert.Equal(t, resource.StatusOK, status)

		p3, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.True(t, p3 == p)
		assert.True(t, p3.(*testProvider).configured)
	}

	// Delete the existingv provider for the last entry in olds.
	{
		urn, id := olds[len(olds)-1].URN, olds[len(olds)-1].ID

		// Fetch the old provider instance.
		_, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Delete
		status, err := r.Delete(urn, id, resource.PropertyMap{})
		assert.NoError(t, err)
		assert.Equal(t, resource.StatusOK, status)

		_, ok = r.GetProvider(Reference{urn: urn, id: id})
		assert.False(t, ok)
	}
}

func TestCRUDPreview(t *testing.T) {
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
				checkConfig: func(olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return news, nil, nil
				},
				diffConfig: func(olds, news resource.PropertyMap) (plugin.DiffResult, error) {
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

	r, err := NewRegistry(host, olds, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	assert.Equal(t, len(olds), len(r.providers))

	for _, old := range olds {
		ref, err := NewReference(old.URN, old.ID)
		assert.NoError(t, err)

		p, ok := r.GetProvider(ref)
		assert.True(t, ok)
		assert.NotNil(t, p)

		assert.Equal(t, getProviderPackage(old.Type), p.Pkg())
	}

	// Create a new provider for each package.
	for _, l := range loaders {
		typ := MakeProviderType(l.pkg)
		urn := resource.NewURN("test", "test", "", typ, "b")
		olds, news := resource.PropertyMap{}, resource.PropertyMap{}

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is a preview, the provider should be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnknownID})
		assert.True(t, ok)
		assert.True(t, p.(*testProvider).configured)
	}

	// Update the existing provider for the first entry in olds.
	{
		urn, id := olds[0].URN, olds[0].ID
		olds, news := olds[0].Inputs, olds[0].Inputs

		// Fetch the old provider instance.
		old, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is a preview, the provider should be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnknownID})
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.True(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(urn, id, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, plugin.DiffResult{}, diff)

		// The new provider should be registered.
		p2, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)
		assert.False(t, p2 == old)
		assert.True(t, p2 == p)
	}

	// Replace the existing provider for the last entry in olds.
	{
		urn, id := olds[len(olds)-1].URN, olds[len(olds)-1].ID
		olds, news := olds[len(olds)-1].Inputs, olds[len(olds)-1].Inputs

		// Fetch the old provider instance.
		old, ok := r.GetProvider(Reference{urn: urn, id: id})
		assert.True(t, ok)

		// Check
		inputs, failures, err := r.Check(urn, olds, news, false)
		assert.NoError(t, err)
		assert.Equal(t, news, inputs)
		assert.Empty(t, failures)

		// Since this is a preview, the provider should be configured.
		p, ok := r.GetProvider(Reference{urn: urn, id: UnknownID})
		assert.True(t, ok)
		assert.False(t, p == old)
		assert.True(t, p.(*testProvider).configured)

		// Diff
		diff, err := r.Diff(urn, id, olds, news, false)
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
	host := newPluginHost(t, []*providerLoader{})

	r, err := NewRegistry(host, []*resource.State{}, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false)
	assert.Error(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, inputs)
}

func TestCRUDWrongPackage(t *testing.T) {
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgB", "", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, []*resource.State{}, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false)
	assert.Error(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, inputs)
}

func TestCRUDWrongVersion(t *testing.T) {
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "0.5.0", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, []*resource.State{}, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewStringProperty("1.0.0")}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false)
	assert.Error(t, err)
	assert.Empty(t, failures)
	assert.Nil(t, inputs)
}

func TestCRUDBadVersionNotString(t *testing.T) {
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, []*resource.State{}, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewBoolProperty(true)}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false)
	assert.NoError(t, err)
	assert.Len(t, failures, 1)
	assert.Equal(t, "version", string(failures[0].Property))
	assert.Nil(t, inputs)
}

func TestCRUDBadVersion(t *testing.T) {
	loaders := []*providerLoader{
		newSimpleLoader(t, "pkgA", "1.0.0", nil),
	}
	host := newPluginHost(t, loaders)

	r, err := NewRegistry(host, []*resource.State{}, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, r)

	typ := MakeProviderType("pkgA")
	urn := resource.NewURN("test", "test", "", typ, "b")
	olds, news := resource.PropertyMap{}, resource.PropertyMap{"version": resource.NewStringProperty("foo")}

	// Check
	inputs, failures, err := r.Check(urn, olds, news, false)
	assert.NoError(t, err)
	assert.Len(t, failures, 1)
	assert.Equal(t, "version", string(failures[0].Property))
	assert.Nil(t, inputs)
}
