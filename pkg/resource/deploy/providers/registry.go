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
	"fmt"
	"sync"

	"github.com/blang/semver"
	uuid "github.com/gofrs/uuid"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// GetProviderVersion fetches and parses a provider version from the given property map. If the version property is not
// present, this function returns nil.
func GetProviderVersion(inputs resource.PropertyMap) (*semver.Version, error) {
	versionProp, ok := inputs["version"]
	if !ok {
		return nil, nil
	}

	if !versionProp.IsString() {
		return nil, errors.New("'version' must be a string")
	}

	sv, err := semver.ParseTolerant(versionProp.StringValue())
	if err != nil {
		return nil, errors.Errorf("could not parse provider version: %v", err)
	}
	return &sv, nil
}

// Registry manages the lifecylce of provider resources and their plugins and handles the resolution of provider
// references to loaded plugins.
//
// When a registry is created, it is handed the set of old provider resources that it will manage. Each provider
// resource in this set is loaded and configured as per its recorded inputs and registered under the provider
// reference that corresponds to its URN and ID, both of which must be known. At this point, the created registry is
// prepared to be used to manage the lifecycle of these providers as well as any new provider resources requested by
// invoking the registry's CRUD operations.
//
// In order to fit neatly in to the existing infrastructure for managing resources using Pulumi, a provider regidstry
// itself implements the plugin.Provider interface.
type Registry struct {
	host      plugin.Host
	isPreview bool
	providers map[Reference]plugin.Provider
	builtins  plugin.Provider
	m         sync.RWMutex
}

var _ plugin.Provider = (*Registry)(nil)

func loadProvider(pkg tokens.Package, version *semver.Version, host plugin.Host,
	builtins plugin.Provider) (plugin.Provider, error) {

	if builtins != nil && pkg == builtins.Pkg() {
		return builtins, nil
	}

	return host.Provider(pkg, version)
}

// NewRegistry creates a new provider registry using the given host and old resources. Each provider present in the old
// resources will be loaded, configured, and added to the returned registry under its reference. If any provider is not
// loadable/configurable or has an invalid ID, this function returns an error.
func NewRegistry(host plugin.Host, prev []*resource.State, isPreview bool,
	builtins plugin.Provider) (*Registry, error) {

	r := &Registry{
		host:      host,
		isPreview: isPreview,
		providers: make(map[Reference]plugin.Provider),
		builtins:  builtins,
	}

	for _, res := range prev {
		urn := res.URN
		if !IsProviderType(urn.Type()) {
			logging.V(7).Infof("provider(%v): %v", urn, res.Provider)
			continue
		}

		// Ensure that this provider has a known ID.
		if res.ID == "" || res.ID == UnknownID {
			return nil, errors.Errorf("provider '%v' has an unknown ID", urn)
		}

		// Ensure that we have no duplicates.
		ref := mustNewReference(urn, res.ID)
		if _, ok := r.providers[ref]; ok {
			return nil, errors.Errorf("duplicate provider found in old state: '%v'", ref)
		}

		providerPkg := GetProviderPackage(urn.Type())

		// Parse the provider version, then load, configure, and register the provider.
		version, err := GetProviderVersion(res.Inputs)
		if err != nil {
			return nil, errors.Errorf("could not parse version for %v provider '%v': %v", providerPkg, urn, err)
		}
		provider, err := loadProvider(providerPkg, version, host, builtins)
		if err != nil {
			return nil, errors.Errorf("could not load plugin for %v provider '%v': %v", providerPkg, urn, err)
		}
		if provider == nil {
			return nil, errors.Errorf("could not find plugin for %v provider '%v' at version %v", providerPkg, urn, version)
		}
		if err := provider.Configure(res.Inputs); err != nil {
			closeErr := host.CloseProvider(provider)
			contract.IgnoreError(closeErr)
			return nil, errors.Errorf("could not configure provider '%v': %v", urn, err)
		}

		logging.V(7).Infof("loaded provider %v", ref)
		r.providers[ref] = provider
	}

	return r, nil
}

// GetProvider returns the provider plugin that is currently registered under the given reference, if any.
func (r *Registry) GetProvider(ref Reference) (plugin.Provider, bool) {
	r.m.RLock()
	defer r.m.RUnlock()

	logging.V(7).Infof("GetProvider(%v)", ref)

	provider, ok := r.providers[ref]
	return provider, ok
}

func (r *Registry) setProvider(ref Reference, provider plugin.Provider) {
	r.m.Lock()
	defer r.m.Unlock()

	logging.V(7).Infof("setProvider(%v)", ref)

	r.providers[ref] = provider
}

func (r *Registry) deleteProvider(ref Reference) (plugin.Provider, bool) {
	r.m.Lock()
	defer r.m.Unlock()

	provider, ok := r.providers[ref]
	if !ok {
		return nil, false
	}
	delete(r.providers, ref)
	return provider, true
}

// The rest of the methods below are the implementation of the plugin.Provider interface methods.

func (r *Registry) Close() error {
	return nil
}

func (r *Registry) Pkg() tokens.Package {
	return "pulumi"
}

func (r *Registry) label() string {
	return "ProviderRegistry"
}

// GetSchema returns the JSON-serialized schema for the provider.
func (r *Registry) GetSchema(version int) ([]byte, error) {
	contract.Fail()

	return nil, errors.New("the provider registry has no schema")
}

// CheckConfig validates the configuration for this resource provider.
func (r *Registry) CheckConfig(urn resource.URN, olds,
	news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	contract.Fail()
	return nil, nil, errors.New("the provider registry is not configurable")
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (r *Registry) DiffConfig(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	contract.Fail()
	return plugin.DiffResult{}, errors.New("the provider registry is not configurable")
}

func (r *Registry) Configure(props resource.PropertyMap) error {
	contract.Fail()
	return errors.New("the provider registry is not configurable")
}

// Check validates the configuration for a particular provider resource.
//
// The particulars of Check are a bit subtle for a few reasons:
// - we need to load the provider for the package indicated by the type name portion provider resource's URN in order
//   to check its config
// - we need to keep the newly-loaded provider around in case we need to diff its config
// - if we are running a preview, we need to configure the provider, as its corresponding CRUD operations will not run
//   (we would normally configure the provider in Create or Update).
func (r *Registry) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	contract.Require(IsProviderType(urn.Type()), "urn")

	label := fmt.Sprintf("%s.Check(%s)", r.label(), urn)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	// Parse the version from the provider properties and load the provider.
	version, err := GetProviderVersion(news)
	if err != nil {
		return nil, []plugin.CheckFailure{{Property: "version", Reason: err.Error()}}, nil
	}
	provider, err := loadProvider(GetProviderPackage(urn.Type()), version, r.host, r.builtins)
	if err != nil {
		return nil, nil, err
	}
	if provider == nil {
		return nil, nil, errors.New("could not find plugin")
	}

	// Check the provider's config. If the check fails, unload the provider.
	inputs, failures, err := provider.CheckConfig(urn, olds, news, allowUnknowns)
	if len(failures) != 0 || err != nil {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
		return nil, failures, err
	}

	// Create a provider reference using the URN and the unknown ID and register the provider.
	r.setProvider(mustNewReference(urn, UnknownID), provider)

	return inputs, nil, nil
}

// Diff diffs the configuration of the indicated provider. The provider corresponding to the given URN must have
// previously been loaded by a call to Check.
func (r *Registry) Diff(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	contract.Require(id != "", "id")

	label := fmt.Sprintf("%s.Diff(%s,%s)", r.label(), urn, id)
	logging.V(7).Infof("%s: executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	// Create a reference using the URN and the unknown ID and fetch the provider.
	provider, ok := r.GetProvider(mustNewReference(urn, UnknownID))
	if !ok {
		// If the provider was not found in the registry under its URN and the Unknown ID, then it must have not have
		// been subject to a call to `Check`. This can happen when we are diffing a provider's inputs as part of
		// evaluating the fanout of a delete-before-replace operation. In this case, we can just use the old provider
		// (which must have been loaded when the registry was created), and we will not unload it.
		provider, ok = r.GetProvider(mustNewReference(urn, id))
		contract.Assertf(ok, "Provider must have been registered by NewRegistry for DBR Diff (%v::%v)", urn, id)

		diff, err := provider.DiffConfig(urn, olds, news, allowUnknowns, ignoreChanges)
		if err != nil {
			return plugin.DiffResult{Changes: plugin.DiffUnknown}, err
		}
		return diff, nil
	}

	// Diff the properties.
	diff, err := provider.DiffConfig(urn, olds, news, allowUnknowns, ignoreChanges)
	if err != nil {
		return plugin.DiffResult{Changes: plugin.DiffUnknown}, err
	}
	if diff.Changes == plugin.DiffUnknown {
		if olds.DeepEquals(news) {
			diff.Changes = plugin.DiffNone
		} else {
			diff.Changes = plugin.DiffSome
		}
	}

	// If the diff requires replacement, unload the provider: the engine will reload it during its replacememnt Check.
	if diff.Replace() {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
	}

	return diff, nil
}

// Create coonfigures the provider with the given URN using the indicated configuration, assigns it an ID, and
// registers it under the assigned (URN, ID).
//
// The provider must have been loaded by a prior call to Check.
func (r *Registry) Create(urn resource.URN, news resource.PropertyMap, timeout float64,
	preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

	label := fmt.Sprintf("%s.Create(%s)", r.label(), urn)
	logging.V(7).Infof("%s executing (#news=%v)", label, len(news))

	// Fetch the unconfigured provider, configure it, and register it under a new ID.
	provider, ok := r.GetProvider(mustNewReference(urn, UnknownID))
	contract.Assertf(ok, "'Check' must be called before 'Create' (%v)", urn)

	if err := provider.Configure(news); err != nil {
		return "", nil, resource.StatusOK, err
	}

	var id resource.ID
	if !preview {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return "", nil, resource.StatusOK, err
		}
		id = resource.ID(uuid.String())
		contract.Assert(id != UnknownID)
	}

	r.setProvider(mustNewReference(urn, id), provider)
	return id, news, resource.StatusOK, nil
}

// Update configures the provider with the given URN and ID using the indicated configuration and registers it at the
// reference indicated by the (URN, ID) pair.
//
// THe provider must have been loaded by a prior call to Check.
func (r *Registry) Update(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
	ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

	label := fmt.Sprintf("%s.Update(%s,%s)", r.label(), id, urn)
	logging.V(7).Infof("%s executing (#olds=%v,#news=%v)", label, len(olds), len(news))

	// Fetch the unconfigured provider and configure it.
	provider, ok := r.GetProvider(mustNewReference(urn, UnknownID))
	contract.Assertf(ok, "'Check' and 'Diff' must be called before 'Update' (%v)", urn)

	if err := provider.Configure(news); err != nil {
		return nil, resource.StatusUnknown, err
	}

	// Publish the configured provider.
	r.setProvider(mustNewReference(urn, id), provider)
	return news, resource.StatusOK, nil
}

// Delete unregisters and unloads the provider with the given URN and ID. The provider must have been loaded when the
// registry was created (i.e. it must have been present in the state handed to NewRegistry).
func (r *Registry) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap,
	timeout float64) (resource.Status, error) {
	contract.Assert(!r.isPreview)

	ref := mustNewReference(urn, id)
	provider, has := r.deleteProvider(ref)
	contract.Assert(has)

	closeErr := r.host.CloseProvider(provider)
	contract.IgnoreError(closeErr)
	return resource.StatusOK, nil
}

func (r *Registry) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
	return plugin.ReadResult{}, resource.StatusUnknown, errors.New("provider resources may not be read")
}

func (r *Registry) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{}, errors.New("provider resources may not be constructed")
}

func (r *Registry) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {

	// It is the responsibility of the eval source to ensure that we never attempt an invoke using the provider
	// registry.
	contract.Fail()
	return nil, nil, errors.New("the provider registry is not invokable")
}

func (r *Registry) StreamInvoke(
	tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(resource.PropertyMap) error) ([]plugin.CheckFailure, error) {

	return nil, fmt.Errorf("the provider registry does not implement streaming invokes")
}

func (r *Registry) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions) (plugin.CallResult, error) {

	// It is the responsibility of the eval source to ensure that we never attempt an call using the provider
	// registry.
	contract.Fail()
	return plugin.CallResult{}, errors.New("the provider registry is not callable")
}

func (r *Registry) GetPluginInfo() (workspace.PluginInfo, error) {
	// return an error: this should not be called for the provider registry
	return workspace.PluginInfo{}, errors.New("the provider registry does not report plugin info")
}

func (r *Registry) SignalCancellation() error {
	// At the moment there isn't anything reasonable we can do here. In the future, it might be nice to plumb
	// cancellation through the plugin loader and cancel any outstanding load requests here.
	return nil
}
