package providers

import providers "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/providers"

// A ProviderRequest is a tuple of an optional semantic version, download server url, parameter, and a package name.
// Whenever the engine receives a registration for a resource that doesn't explicitly specify a provider, the engine
// creates a ProviderRequest for that resource's provider, using the version passed to the engine as part of
// RegisterResource and the package derived from the resource's token.
// 
// The source evaluator (source_eval.go) is responsible for servicing provider requests. It does this by interpreting
// these provider requests and sending resource registrations to the engine for the providers themselves. These are
// called "default providers".
// 
// ProviderRequest is useful as a hash key. The engine is free to instantiate any number of provider requests, but it is
// free to cache requests for a provider request that is equal to one that has already been serviced. If you do use
// ProviderRequest as a hash key, you should call String() to get a usable key for string-based hash maps.
// ProviderRequests only hash by their package name, version and download URL. The checksums and parameterization are
// not used in the hash.
type ProviderRequest = providers.ProviderRequest

// NewProviderRequest constructs a new provider request from an optional version, optional
// pluginDownloadURL, optional parameter, and package.
func NewProviderRequest(name tokens.Package, version *semver.Version, pluginDownloadURL string, checksums map[string][]byte, parameterization *workspace.Parameterization) ProviderRequest {
	return providers.NewProviderRequest(name, version, pluginDownloadURL, checksums, parameterization)
}

