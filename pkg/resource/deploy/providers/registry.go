// Copyright 2016-2021, Pulumi Corporation.
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
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/blang/semver"
	uuid "github.com/gofrs/uuid"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	versionKey         resource.PropertyKey = "version"
	pluginDownloadKey  resource.PropertyKey = "pluginDownloadURL"
	pluginChecksumsKey resource.PropertyKey = "pluginChecksums"
)

// SetProviderChecksums sets the provider plugin checksums in the given property map.
func SetProviderChecksums(inputs resource.PropertyMap, value map[string][]byte) {
	propMap := make(resource.PropertyMap)
	for key, checksum := range value {
		hex := hex.EncodeToString(checksum)
		propMap[resource.PropertyKey(key)] = resource.NewStringProperty(hex)
	}

	inputs[pluginChecksumsKey] = resource.NewObjectProperty(propMap)
}

// GetProviderChecksums fetches a provider plugin checksums from the given property map.
// If the checksums is not set, this function returns nil.
func GetProviderChecksums(inputs resource.PropertyMap) (map[string][]byte, error) {
	checksums, ok := inputs[pluginChecksumsKey]
	if !ok {
		return nil, nil
	}
	if !checksums.IsObject() {
		return nil, fmt.Errorf("'%s' must be an object", pluginChecksumsKey)
	}
	result := make(map[string][]byte)
	for key, value := range checksums.ObjectValue() {
		if !value.IsString() {
			return nil, fmt.Errorf("'%s[%s]' must be a string", pluginChecksumsKey, key)
		}

		bytes, err := hex.DecodeString(value.StringValue())
		if err != nil {
			return nil, fmt.Errorf("'%s[%s]' must be a hexadecimal string", pluginChecksumsKey, key)
		}

		result[string(key)] = bytes
	}

	return result, nil
}

// SetProviderURL sets the provider plugin download server URL in the given property map.
func SetProviderURL(inputs resource.PropertyMap, value string) {
	inputs[pluginDownloadKey] = resource.NewStringProperty(value)
}

// GetProviderDownloadURL fetches a provider plugin download server URL from the given property map.
// If the server URL is not set, this function returns "".
func GetProviderDownloadURL(inputs resource.PropertyMap) (string, error) {
	url, ok := inputs[pluginDownloadKey]
	if !ok {
		return "", nil
	}
	if !url.IsString() {
		return "", fmt.Errorf("'%s' must be a string", pluginDownloadKey)
	}
	return url.StringValue(), nil
}

// Sets the provider version in the given property map.
func SetProviderVersion(inputs resource.PropertyMap, value *semver.Version) {
	inputs[versionKey] = resource.NewStringProperty(value.String())
}

// GetProviderVersion fetches and parses a provider version from the given property map. If the
// version property is not present, this function returns nil.
func GetProviderVersion(inputs resource.PropertyMap) (*semver.Version, error) {
	version, ok := inputs[versionKey]
	if !ok {
		return nil, nil
	}

	if !version.IsString() {
		return nil, fmt.Errorf("'%s' must be a string", versionKey)
	}

	sv, err := semver.ParseTolerant(version.StringValue())
	if err != nil {
		return nil, fmt.Errorf("could not parse provider version: %v", err)
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
	aliases   map[resource.URN]resource.URN
	m         sync.RWMutex
}

var _ plugin.Provider = (*Registry)(nil)

func loadProvider(pkg tokens.Package, version *semver.Version, downloadURL string, checksums map[string][]byte,
	host plugin.Host, builtins plugin.Provider,
) (plugin.Provider, error) {
	if builtins != nil && pkg == builtins.Pkg() {
		return builtins, nil
	}

	provider, err := host.Provider(pkg, version)
	if err == nil {
		return provider, nil
	}

	// host.Provider _might_ return MissingError,  this could be due to the fact that a transitive
	// version of a plugin is required which are not picked up by initial pass of required plugin
	// installations or because of bugs in GetRequiredPlugins. Instead of reporting an error, we first try to
	// install the plugin now, and only error if we can't do that.
	var me *workspace.MissingError
	if !errors.As(err, &me) {
		// Not a MissingError, return the original error.
		return nil, err
	}

	// Try to install the plugin, unless auto plugin installs are turned off, we have all the specific information we
	// need to do so here while once we call into `host.Provider` we no longer have the download URL or checksums.
	if env.DisableAutomaticPluginAcquisition.Value() {
		return nil, err
	}

	pluginSpec := workspace.PluginSpec{
		Kind:              workspace.ResourcePlugin,
		Name:              string(pkg),
		Version:           version,
		PluginDownloadURL: downloadURL,
		Checksums:         checksums,
	}

	log := func(sev diag.Severity, msg string) {
		host.Log(sev, "", msg, 0)
	}

	_, err = pkgWorkspace.InstallPlugin(pluginSpec, log)
	if err != nil {
		return nil, err
	}

	// Try to load the provider again, this time it should succeed.
	return host.Provider(pkg, version)
}

// NewRegistry creates a new provider registry using the given host.
func NewRegistry(host plugin.Host, isPreview bool, builtins plugin.Provider) *Registry {
	return &Registry{
		host:      host,
		isPreview: isPreview,
		providers: make(map[Reference]plugin.Provider),
		builtins:  builtins,
		aliases:   make(map[resource.URN]resource.URN),
	}
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

	if alias, ok := r.aliases[ref.URN()]; ok {
		r.providers[mustNewReference(alias, ref.ID())] = provider
	}
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
	contract.Failf("GetSchema must not be called on the provider registry")

	return nil, errors.New("the provider registry has no schema")
}

func (r *Registry) GetMapping(key, provider string) ([]byte, string, error) {
	contract.Failf("GetMapping must not be called on the provider registry")

	return nil, "", errors.New("the provider registry has no mappings")
}

func (r *Registry) GetMappings(key string) ([]string, error) {
	contract.Failf("GetMappings must not be called on the provider registry")

	return nil, errors.New("the provider registry has no mappings")
}

// CheckConfig validates the configuration for this resource provider.
func (r *Registry) CheckConfig(urn resource.URN, olds,
	news resource.PropertyMap, allowUnknowns bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	contract.Failf("CheckConfig must not be called on the provider registry")
	return nil, nil, errors.New("the provider registry is not configurable")
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (r *Registry) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	contract.Failf("DiffConfig must not be called on the provider registry")
	return plugin.DiffResult{}, errors.New("the provider registry is not configurable")
}

func (r *Registry) Configure(props resource.PropertyMap) error {
	contract.Failf("Configure must not be called on the provider registry")
	return errors.New("the provider registry is not configurable")
}

// Check validates the configuration for a particular provider resource.
//
// The particulars of Check are a bit subtle for a few reasons:
//   - we need to load the provider for the package indicated by the type name portion provider resource's URN in order
//     to check its config
//   - we need to keep the newly-loaded provider around in case we need to diff its config
//   - if we are running a preview, we need to configure the provider, as its corresponding CRUD operations will not run
//     (we would normally configure the provider in Create or Update).
func (r *Registry) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	contract.Requiref(IsProviderType(urn.Type()), "urn", "must be a provider type, got %v", urn.Type())

	label := fmt.Sprintf("%s.Check(%s)", r.label(), urn)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	// Parse the version from the provider properties and load the provider.
	version, err := GetProviderVersion(news)
	if err != nil {
		return nil, []plugin.CheckFailure{{Property: "version", Reason: err.Error()}}, nil
	}
	downloadURL, err := GetProviderDownloadURL(news)
	if err != nil {
		return nil, []plugin.CheckFailure{{Property: "pluginDownloadURL", Reason: err.Error()}}, nil
	}
	// TODO: We should thread checksums through here.
	provider, err := loadProvider(GetProviderPackage(urn.Type()), version, downloadURL, nil, r.host, r.builtins)
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

	// Create a provider reference using the URN and the unconfigured ID and register the provider.
	r.setProvider(mustNewReference(urn, UnconfiguredID), provider)

	return inputs, nil, nil
}

// RegisterAliases informs the registry that the new provider object with the given URN is aliased to the given list
// of URNs.
func (r *Registry) RegisterAlias(providerURN, alias resource.URN) {
	r.m.Lock()
	defer r.m.Unlock()

	if providerURN != alias {
		r.aliases[providerURN] = alias
	}
}

// Diff diffs the configuration of the indicated provider. The provider corresponding to the given URN must have
// previously been loaded by a call to Check.
func (r *Registry) Diff(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	contract.Requiref(id != "", "id", "must not be empty")

	label := fmt.Sprintf("%s.Diff(%s,%s)", r.label(), urn, id)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(oldInputs), len(oldOutputs), len(newInputs))

	// Create a reference using the URN and the unconfigured ID and fetch the provider.
	provider, ok := r.GetProvider(mustNewReference(urn, UnconfiguredID))
	if !ok {
		// If the provider was not found in the registry under its URN and the unconfigured ID, then it must have not have
		// been subject to a call to `Check`. This can happen when we are diffing a provider's inputs as part of
		// evaluating the fanout of a delete-before-replace operation. In this case, we can just use the old provider
		// (which we should have loaded during diff search), and we will not unload it.
		provider, ok = r.GetProvider(mustNewReference(urn, id))
		contract.Assertf(ok, "Provider must have been registered at some point for DBR Diff (%v::%v)", urn, id)
	}

	// Diff the properties.
	diff, err := provider.DiffConfig(urn, oldInputs, oldOutputs, newInputs, allowUnknowns, ignoreChanges)
	if err != nil {
		return plugin.DiffResult{Changes: plugin.DiffUnknown}, err
	}
	if diff.Changes == plugin.DiffUnknown {
		if oldOutputs.DeepEquals(newInputs) {
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

	logging.V(7).Infof("%s: executed (%#v, %#v)", label, diff.Changes, diff.ReplaceKeys)

	return diff, nil
}

// Same executes as part of the "Same" step for a provider that has not changed. It configures the provider
// instance with the given state and fixes up aliases.
func (r *Registry) Same(res *resource.State) error {
	urn := res.URN
	if !IsProviderType(urn.Type()) {
		return fmt.Errorf("urn %v is not a provider type", urn)
	}

	// Ensure that this provider has a known ID.
	if res.ID == "" || res.ID == UnknownID {
		return fmt.Errorf("provider '%v' has an unknown ID", urn)
	}

	ref := mustNewReference(urn, res.ID)
	logging.V(7).Infof("Same(%v)", ref)

	// If this provider is already configured, then we're done.
	_, ok := r.GetProvider(ref)
	if ok {
		return nil
	}

	// We may have started this provider up for Check/Diff, but then decided to Same it, if so we can just
	// reuse that instance, but as we're now configuring it remove the unconfigured ID from the provider map
	// so nothing else tries to use it.
	provider, ok := r.deleteProvider(mustNewReference(urn, UnconfiguredID))
	if !ok {
		// Else we need to load it fresh
		providerPkg := GetProviderPackage(urn.Type())

		// Parse the provider version, then load, configure, and register the provider.
		version, err := GetProviderVersion(res.Inputs)
		if err != nil {
			return fmt.Errorf("parse version for %v provider '%v': %v", providerPkg, urn, err)
		}
		downloadURL, err := GetProviderDownloadURL(res.Inputs)
		if err != nil {
			return fmt.Errorf("parse download URL for %v provider '%v': %v", providerPkg, urn, err)
		}
		// TODO: We should thread checksums through here.
		provider, err = loadProvider(providerPkg, version, downloadURL, nil, r.host, r.builtins)
		if err != nil {
			return fmt.Errorf("load plugin for %v provider '%v': %v", providerPkg, urn, err)
		}
		if provider == nil {
			return fmt.Errorf("find plugin for %v provider '%v' at version %v", providerPkg, urn, version)
		}
	}
	contract.Assertf(provider != nil, "provider must not be nil")

	if err := provider.Configure(res.Inputs); err != nil {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
		return fmt.Errorf("configure provider '%v': %v", urn, err)
	}

	logging.V(7).Infof("loaded provider %v", ref)

	r.setProvider(ref, provider)

	return nil
}

// Create configures the provider with the given URN using the indicated configuration, assigns it an ID, and
// registers it under the assigned (URN, ID).
//
// The provider must have been loaded by a prior call to Check.
func (r *Registry) Create(urn resource.URN, news resource.PropertyMap, timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	label := fmt.Sprintf("%s.Create(%s)", r.label(), urn)
	logging.V(7).Infof("%s executing (#news=%v)", label, len(news))

	// Fetch the unconfigured provider, configure it, and register it under a new ID. We remove the
	// unconfigured ID from the provider map so nothing else tries to use and re-configure this instance.
	provider, ok := r.deleteProvider(mustNewReference(urn, UnconfiguredID))
	if !ok {
		// The unconfigured provider may have been Same'd after Check and this provider could be a replacement create.
		// In which case we need to start up a fresh copy.

		providerPkg := GetProviderPackage(urn.Type())

		// Parse the provider version, then load, configure, and register the provider.
		version, err := GetProviderVersion(news)
		if err != nil {
			return "", nil, resource.StatusUnknown,
				fmt.Errorf("parse version for %v provider '%v': %v", providerPkg, urn, err)
		}
		downloadURL, err := GetProviderDownloadURL(news)
		if err != nil {
			return "", nil, resource.StatusUnknown,
				fmt.Errorf("parse download URL for %v provider '%v': %v", providerPkg, urn, err)
		}
		// TODO: We should thread checksums through here.
		provider, err = loadProvider(providerPkg, version, downloadURL, nil, r.host, r.builtins)
		if err != nil {
			return "", nil, resource.StatusUnknown,
				fmt.Errorf("load plugin for %v provider '%v': %v", providerPkg, urn, err)
		}
		if provider == nil {
			return "", nil, resource.StatusUnknown,
				fmt.Errorf("find plugin for %v provider '%v' at version %v", providerPkg, urn, version)
		}
	}

	if err := provider.Configure(news); err != nil {
		return "", nil, resource.StatusOK, err
	}

	id := resource.ID(UnknownID)
	if !preview {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return "", nil, resource.StatusOK, err
		}
		id = resource.ID(uuid.String())
		contract.Assertf(id != UnknownID, "resource ID must not be unknown")
	}

	r.setProvider(mustNewReference(urn, id), provider)
	return id, news, resource.StatusOK, nil
}

// Update configures the provider with the given URN and ID using the indicated configuration and registers it at the
// reference indicated by the (URN, ID) pair.
//
// THe provider must have been loaded by a prior call to Check.
func (r *Registry) Update(urn resource.URN, id resource.ID,
	oldInputs, oldOutputs, newInputs resource.PropertyMap, timeout float64,
	ignoreChanges []string, preview bool,
) (resource.PropertyMap, resource.Status, error) {
	label := fmt.Sprintf("%s.Update(%s,%s)", r.label(), id, urn)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(oldInputs), len(oldOutputs), len(newInputs))

	// Fetch the unconfigured provider, configure it, and register it under a new ID. We remove the
	// unconfigured ID from the provider map so nothing else tries to use and re-configure this instance.
	provider, ok := r.deleteProvider(mustNewReference(urn, UnconfiguredID))
	contract.Assertf(ok, "'Check' and 'Diff' must be called before 'Update' (%v)", urn)

	if err := provider.Configure(newInputs); err != nil {
		return nil, resource.StatusUnknown, err
	}

	// Publish the configured provider.
	r.setProvider(mustNewReference(urn, id), provider)
	return newInputs, resource.StatusOK, nil
}

// Delete unregisters and unloads the provider with the given URN and ID. If the provider was never loaded
// this is a no-op.
func (r *Registry) Delete(urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	contract.Assertf(!r.isPreview, "Delete must not be called during preview")

	ref := mustNewReference(urn, id)
	provider, has := r.deleteProvider(ref)
	if !has {
		return resource.StatusOK, nil
	}

	closeErr := r.host.CloseProvider(provider)
	contract.IgnoreError(closeErr)
	return resource.StatusOK, nil
}

func (r *Registry) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	return plugin.ReadResult{}, resource.StatusUnknown, errors.New("provider resources may not be read")
}

func (r *Registry) Construct(info plugin.ConstructInfo, typ tokens.Type, name string, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions,
) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{}, errors.New("provider resources may not be constructed")
}

func (r *Registry) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	// It is the responsibility of the eval source to ensure that we never attempt an invoke using the provider
	// registry.
	contract.Failf("Invoke must not be called on the provider registry")
	return nil, nil, errors.New("the provider registry is not invokable")
}

func (r *Registry) StreamInvoke(
	tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(resource.PropertyMap) error,
) ([]plugin.CheckFailure, error) {
	return nil, fmt.Errorf("the provider registry does not implement streaming invokes")
}

func (r *Registry) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions,
) (plugin.CallResult, error) {
	// It is the responsibility of the eval source to ensure that we never attempt an call using the provider
	// registry.
	contract.Failf("Call must not be called on the provider registry")
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
