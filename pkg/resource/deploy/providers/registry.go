package providers

import providers "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/providers"

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
type Registry = providers.Registry

// SetProviderChecksums sets the provider plugin checksums in the given property map.
func SetProviderChecksums(inputs resource.PropertyMap, value map[string][]byte) {
	providers.SetProviderChecksums(inputs, value)
}

// GetProviderChecksums fetches a provider plugin checksums from the given property map.
// If the checksums is not set, this function returns nil.
func GetProviderChecksums(inputs resource.PropertyMap) (map[string][]byte, error) {
	return providers.GetProviderChecksums(inputs)
}

// SetProviderURL sets the provider plugin download server URL in the given property map.
func SetProviderURL(inputs resource.PropertyMap, value string) {
	providers.SetProviderURL(inputs, value)
}

// GetProviderDownloadURL fetches a provider plugin download server URL from the given property map.
// If the server URL is not set, this function returns "".
func GetProviderDownloadURL(inputs resource.PropertyMap) (string, error) {
	return providers.GetProviderDownloadURL(inputs)
}

// Sets the provider version in the given property map.
func SetProviderVersion(inputs resource.PropertyMap, value *semver.Version) {
	providers.SetProviderVersion(inputs, value)
}

// GetProviderVersion fetches and parses a provider version from the given property map. If the
// version property is not present, this function returns nil.
func GetProviderVersion(inputs resource.PropertyMap) (*semver.Version, error) {
	return providers.GetProviderVersion(inputs)
}

// Sets the provider name in the given property map.
func SetProviderName(inputs resource.PropertyMap, name tokens.Package) {
	providers.SetProviderName(inputs, name)
}

// GetProviderName fetches and parses a provider name from the given property map. If the
// name property is not present, this function returns the passed in name.
func GetProviderName(name tokens.Package, inputs resource.PropertyMap) (tokens.Package, error) {
	return providers.GetProviderName(name, inputs)
}

// Sets the provider parameterization in the given property map, this should be called _after_ SetVersion.
func SetProviderParameterization(inputs resource.PropertyMap, value *workspace.Parameterization) {
	providers.SetProviderParameterization(inputs, value)
}

// GetProviderParameterization fetches and parses a provider parameterization from the given property map. If the
// parameterization property is not present, this function returns nil.
func GetProviderParameterization(name tokens.Package, inputs resource.PropertyMap) (*workspace.Parameterization, error) {
	return providers.GetProviderParameterization(name, inputs)
}

// FilterProviderConfig filters out the __internal key from provider state so the resulting map can be passed to
// provider plugins.
func FilterProviderConfig(inputs resource.PropertyMap) resource.PropertyMap {
	return providers.FilterProviderConfig(inputs)
}

// NewRegistry creates a new provider registry using the given host.
func NewRegistry(host plugin.Host, isPreview bool, builtins plugin.Provider) *Registry {
	return providers.NewRegistry(host, isPreview, builtins)
}

