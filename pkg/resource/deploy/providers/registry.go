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
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/blang/semver"
	uuid "github.com/gofrs/uuid"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
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
	parameterKey       resource.PropertyKey = "parameter"
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
		return nil, fmt.Errorf("could not parse provider version: %w", err)
	}
	return &sv, nil
}

// Sets the provider parameter in the given property map.
func SetProviderParameter(inputs resource.PropertyMap, value *ProviderParameter) {
	inputs[parameterKey] = resource.NewObjectProperty(resource.PropertyMap{
		"pkg":     resource.NewStringProperty(string(value.pkg)),
		"version": resource.NewStringProperty(value.version.String()),
		"value":   resource.NewPropertyValue(value.value),
	})
}

// GetProviderParameter fetches and parses a provider parameter from the given property map. If the
// parameter property is not present, this function returns nil.
func GetProviderParameter(inputs resource.PropertyMap) (*ProviderParameter, error) {
	parameter, ok := inputs[parameterKey]
	if !ok {
		return nil, nil
	}

	if !parameter.IsObject() {
		return nil, fmt.Errorf("'%s' must be an object", parameterKey)
	}
	obj := parameter.ObjectValue()

	pkg, ok := obj["pkg"]
	if !ok {
		return nil, fmt.Errorf("'%s' must have a 'pkg' field", parameterKey)
	}
	if !pkg.IsString() {
		return nil, fmt.Errorf("'%s' must have a 'pkg' field of type string", parameterKey)
	}

	version, ok := obj["version"]
	if !ok {
		return nil, fmt.Errorf("'%s' must have a 'version' field", parameterKey)
	}
	if !version.IsString() {
		return nil, fmt.Errorf("'%s' must have a 'version' field of type string", parameterKey)
	}
	sv, err := semver.Parse(version.StringValue())
	if err != nil {
		return nil, fmt.Errorf("could not parse provider version: %w", err)
	}

	value, ok := obj["value"]
	if !ok {
		return nil, fmt.Errorf("'%s' must have a 'value' field", parameterKey)
	}

	return &ProviderParameter{
		pkg:     tokens.Package(pkg.StringValue()),
		version: &sv,
		value:   value.V,
	}, nil
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
	plugin.NotForwardCompatibleProvider

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
		Kind:              apitype.ResourcePlugin,
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

// loadParameterizedProvider wraps loadProvider to also support loading parameterized providers.
func loadParameterizedProvider(
	pkg tokens.Package, version *semver.Version, downloadURL string, checksums map[string][]byte,
	parameter *ProviderParameter,
	host plugin.Host, builtins plugin.Provider,
) (plugin.Provider, error) {
	basePkg := pkg
	baseVersion := version
	if parameter != nil {
		basePkg = parameter.pkg
		baseVersion = parameter.version
	}

	provider, err := loadProvider(basePkg, baseVersion, downloadURL, checksums, host, builtins)
	if err != nil {
		return nil, err
	}

	if parameter != nil {
		resp, err := provider.Parameterize(context.TODO(), plugin.ParameterizeRequest{
			Parameters: &plugin.ParameterizeValue{
				Name:    string(pkg),
				Version: version,
				Value:   parameter.value,
			},
		})
		if err != nil {
			return nil, err
		}
		if resp.Name != string(pkg) {
			return nil, fmt.Errorf("parameterize response name %q does not match expected package %q", resp.Name, pkg)
		}
	}
	return provider, nil
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

func (r *Registry) Parameterize(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
	contract.Failf("Parameterize must not be called on the provider registry")

	return plugin.ParameterizeResponse{}, errors.New("the provider registry has no parameters")
}

// GetSchema returns the JSON-serialized schema for the provider.
func (r *Registry) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	contract.Failf("GetSchema must not be called on the provider registry")

	return plugin.GetSchemaResponse{}, errors.New("the provider registry has no schema")
}

func (r *Registry) GetMapping(context.Context, plugin.GetMappingRequest) (plugin.GetMappingResponse, error) {
	contract.Failf("GetMapping must not be called on the provider registry")

	return plugin.GetMappingResponse{}, errors.New("the provider registry has no mappings")
}

func (r *Registry) GetMappings(context.Context, plugin.GetMappingsRequest) (plugin.GetMappingsResponse, error) {
	contract.Failf("GetMappings must not be called on the provider registry")

	return plugin.GetMappingsResponse{}, errors.New("the provider registry has no mappings")
}

// CheckConfig validates the configuration for this resource provider.
func (r *Registry) CheckConfig(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
	contract.Failf("CheckConfig must not be called on the provider registry")
	return plugin.CheckConfigResponse{}, errors.New("the provider registry is not configurable")
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (r *Registry) DiffConfig(context.Context, plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error) {
	contract.Failf("DiffConfig must not be called on the provider registry")
	return plugin.DiffResult{}, errors.New("the provider registry is not configurable")
}

func (r *Registry) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	contract.Failf("Configure must not be called on the provider registry")
	return plugin.ConfigureResponse{}, errors.New("the provider registry is not configurable")
}

// Check validates the configuration for a particular provider resource.
//
// The particulars of Check are a bit subtle for a few reasons:
//   - we need to load the provider for the package indicated by the type name portion provider resource's URN in order
//     to check its config
//   - we need to keep the newly-loaded provider around in case we need to diff its config
//   - if we are running a preview, we need to configure the provider, as its corresponding CRUD operations will not run
//     (we would normally configure the provider in Create or Update).
func (r *Registry) Check(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	contract.Requiref(IsProviderType(req.URN.Type()), "urn", "must be a provider type, got %v", req.URN.Type())

	label := fmt.Sprintf("%s.Check(%s)", r.label(), req.URN)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(req.Olds), len(req.News))

	// Parse the version from the provider properties and load the provider.
	version, err := GetProviderVersion(req.News)
	if err != nil {
		return plugin.CheckResponse{Failures: []plugin.CheckFailure{{
			Property: "version", Reason: err.Error(),
		}}}, nil
	}
	downloadURL, err := GetProviderDownloadURL(req.News)
	if err != nil {
		return plugin.CheckResponse{Failures: []plugin.CheckFailure{{
			Property: "pluginDownloadURL", Reason: err.Error(),
		}}}, nil
	}
	parameter, err := GetProviderParameter(req.News)
	if err != nil {
		return plugin.CheckResponse{Failures: []plugin.CheckFailure{{
			Property: "parameter", Reason: err.Error(),
		}}}, nil
	}
	// TODO: We should thread checksums through here.
	provider, err := loadParameterizedProvider(
		GetProviderPackage(req.URN.Type()), version, downloadURL, nil, parameter, r.host, r.builtins)
	if err != nil {
		return plugin.CheckResponse{}, err
	}
	if provider == nil {
		return plugin.CheckResponse{}, errors.New("could not find plugin")
	}

	// Check the provider's config. If the check fails, unload the provider.
	resp, err := provider.CheckConfig(ctx, plugin.CheckConfigRequest{
		URN:           req.URN,
		Olds:          req.Olds,
		News:          req.News,
		AllowUnknowns: true,
	})
	if len(resp.Failures) != 0 || err != nil {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
		return plugin.CheckResponse{Failures: resp.Failures}, err
	}

	// Create a provider reference using the URN and the unconfigured ID and register the provider.
	r.setProvider(mustNewReference(req.URN, UnconfiguredID), provider)

	return plugin.CheckResponse{Properties: resp.Properties}, nil
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
func (r *Registry) Diff(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
	contract.Requiref(req.ID != "", "id", "must not be empty")

	label := fmt.Sprintf("%s.Diff(%s,%s)", r.label(), req.URN, req.ID)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(req.OldInputs), len(req.OldOutputs), len(req.NewInputs))

	// Create a reference using the URN and the unconfigured ID and fetch the provider.
	provider, ok := r.GetProvider(mustNewReference(req.URN, UnconfiguredID))
	if !ok {
		// If the provider was not found in the registry under its URN and the unconfigured ID, then it must have not have
		// been subject to a call to `Check`. This can happen when we are diffing a provider's inputs as part of
		// evaluating the fanout of a delete-before-replace operation. In this case, we can just use the old provider
		// (which we should have loaded during diff search), and we will not unload it.
		provider, ok = r.GetProvider(mustNewReference(req.URN, req.ID))
		contract.Assertf(ok, "Provider must have been registered at some point for DBR Diff (%v::%v)", req.URN, req.ID)
	}

	// Diff the properties.
	diff, err := provider.DiffConfig(context.Background(), plugin.DiffConfigRequest{
		URN:           req.URN,
		OldInputs:     req.OldInputs,
		OldOutputs:    req.OldOutputs,
		NewInputs:     req.NewInputs,
		AllowUnknowns: req.AllowUnknowns,
		IgnoreChanges: req.IgnoreChanges,
	})
	if err != nil {
		return plugin.DiffResult{Changes: plugin.DiffUnknown}, err
	}
	if diff.Changes == plugin.DiffUnknown {
		if req.OldOutputs.DeepEquals(req.NewInputs) {
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
			return fmt.Errorf("parse version for %v provider '%v': %w", providerPkg, urn, err)
		}
		downloadURL, err := GetProviderDownloadURL(res.Inputs)
		if err != nil {
			return fmt.Errorf("parse download URL for %v provider '%v': %w", providerPkg, urn, err)
		}
		parameter, err := GetProviderParameter(res.Inputs)
		if err != nil {
			return fmt.Errorf("parse parameter for %v provider '%v': %w", providerPkg, urn, err)
		}
		// TODO: We should thread checksums through here.
		provider, err = loadParameterizedProvider(providerPkg, version, downloadURL, nil, parameter, r.host, r.builtins)
		if err != nil {
			return fmt.Errorf("load plugin for %v provider '%v': %w", providerPkg, urn, err)
		}
		if provider == nil {
			return fmt.Errorf("find plugin for %v provider '%v' at version %v", providerPkg, urn, version)
		}
	}
	contract.Assertf(provider != nil, "provider must not be nil")

	if _, err := provider.Configure(context.Background(), plugin.ConfigureRequest{
		Inputs: res.Inputs,
	}); err != nil {
		closeErr := r.host.CloseProvider(provider)
		contract.IgnoreError(closeErr)
		return fmt.Errorf("configure provider '%v': %w", urn, err)
	}

	logging.V(7).Infof("loaded provider %v", ref)

	r.setProvider(ref, provider)

	return nil
}

// Create configures the provider with the given URN using the indicated configuration, assigns it an ID, and
// registers it under the assigned (URN, ID).
//
// The provider must have been loaded by a prior call to Check.
func (r *Registry) Create(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	label := fmt.Sprintf("%s.Create(%s)", r.label(), req.URN)
	logging.V(7).Infof("%s executing (#news=%v)", label, len(req.Properties))

	// Fetch the unconfigured provider, configure it, and register it under a new ID. We remove the
	// unconfigured ID from the provider map so nothing else tries to use and re-configure this instance.
	provider, ok := r.deleteProvider(mustNewReference(req.URN, UnconfiguredID))
	if !ok {
		// The unconfigured provider may have been Same'd after Check and this provider could be a replacement create.
		// In which case we need to start up a fresh copy.

		providerPkg := GetProviderPackage(req.URN.Type())

		// Parse the provider version, then load, configure, and register the provider.
		version, err := GetProviderVersion(req.Properties)
		if err != nil {
			return plugin.CreateResponse{Status: resource.StatusUnknown},
				fmt.Errorf("parse version for %v provider '%v': %w", providerPkg, req.URN, err)
		}
		downloadURL, err := GetProviderDownloadURL(req.Properties)
		if err != nil {
			return plugin.CreateResponse{Status: resource.StatusUnknown},
				fmt.Errorf("parse download URL for %v provider '%v': %w", providerPkg, req.URN, err)
		}
		parameter, err := GetProviderParameter(req.Properties)
		if err != nil {
			return plugin.CreateResponse{Status: resource.StatusUnknown},
				fmt.Errorf("parse parameter for %v provider '%v': %w", providerPkg, req.URN, err)
		}
		// TODO: We should thread checksums through here.
		provider, err = loadParameterizedProvider(providerPkg, version, downloadURL, nil, parameter, r.host, r.builtins)
		if err != nil {
			return plugin.CreateResponse{Status: resource.StatusUnknown},
				fmt.Errorf("load plugin for %v provider '%v': %w", providerPkg, req.URN, err)
		}
		if provider == nil {
			return plugin.CreateResponse{Status: resource.StatusUnknown},
				fmt.Errorf("find plugin for %v provider '%v' at version %v", providerPkg, req.URN, version)
		}
	}

	if _, err := provider.Configure(context.Background(), plugin.ConfigureRequest{
		Inputs: req.Properties,
	}); err != nil {
		return plugin.CreateResponse{Status: resource.StatusOK}, err
	}

	id := resource.ID(UnknownID)
	if !req.Preview {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return plugin.CreateResponse{Status: resource.StatusOK}, err
		}
		id = resource.ID(uuid.String())
		contract.Assertf(id != UnknownID, "resource ID must not be unknown")
	}

	r.setProvider(mustNewReference(req.URN, id), provider)
	return plugin.CreateResponse{
		ID:         id,
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

// Update configures the provider with the given URN and ID using the indicated configuration and registers it at the
// reference indicated by the (URN, ID) pair.
//
// The provider must have been loaded by a prior call to Check.
func (r *Registry) Update(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	label := fmt.Sprintf("%s.Update(%s,%s)", r.label(), req.ID, req.URN)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(req.OldInputs), len(req.OldOutputs), len(req.NewInputs))

	// Fetch the unconfigured provider, configure it, and register it under a new ID. We remove the
	// unconfigured ID from the provider map so nothing else tries to use and re-configure this instance.
	provider, ok := r.deleteProvider(mustNewReference(req.URN, UnconfiguredID))
	contract.Assertf(ok, "'Check' and 'Diff' must be called before 'Update' (%v)", req.URN)

	if _, err := provider.Configure(ctx, plugin.ConfigureRequest{Inputs: req.NewInputs}); err != nil {
		return plugin.UpdateResponse{Status: resource.StatusUnknown}, err
	}

	// Publish the configured provider.
	r.setProvider(mustNewReference(req.URN, req.ID), provider)
	return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
}

// Delete unregisters and unloads the provider with the given URN and ID. If the provider was never loaded
// this is a no-op.
func (r *Registry) Delete(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	contract.Assertf(!r.isPreview, "Delete must not be called during preview")

	ref := mustNewReference(req.URN, req.ID)
	provider, has := r.deleteProvider(ref)
	if !has {
		return plugin.DeleteResponse{}, nil
	}

	closeErr := r.host.CloseProvider(provider)
	contract.IgnoreError(closeErr)
	return plugin.DeleteResponse{}, nil
}

func (r *Registry) Read(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{}, errors.New("provider resources may not be read")
}

func (r *Registry) Construct(context.Context, plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	return plugin.ConstructResult{}, errors.New("provider resources may not be constructed")
}

func (r *Registry) Invoke(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	// It is the responsibility of the eval source to ensure that we never attempt an invoke using the provider
	// registry.
	contract.Failf("Invoke must not be called on the provider registry")
	return plugin.InvokeResponse{}, errors.New("the provider registry is not invokable")
}

func (r *Registry) StreamInvoke(context.Context, plugin.StreamInvokeRequest) (plugin.StreamInvokeResponse, error) {
	return plugin.StreamInvokeResponse{}, errors.New("the provider registry does not implement streaming invokes")
}

func (r *Registry) Call(context.Context, plugin.CallRequest) (plugin.CallResponse, error) {
	// It is the responsibility of the eval source to ensure that we never attempt an call using the provider
	// registry.
	contract.Failf("Call must not be called on the provider registry")
	return plugin.CallResult{}, errors.New("the provider registry is not callable")
}

func (r *Registry) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	// return an error: this should not be called for the provider registry
	return workspace.PluginInfo{}, errors.New("the provider registry does not report plugin info")
}

func (r *Registry) SignalCancellation(context.Context) error {
	// At the moment there isn't anything reasonable we can do here. In the future, it might be nice to plumb
	// cancellation through the plugin loader and cancel any outstanding load requests here.
	return nil
}
