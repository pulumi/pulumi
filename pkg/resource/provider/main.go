package provider

import provider "github.com/pulumi/pulumi/sdk/v3/pkg/resource/provider"

// Main is the typical entrypoint for a resource provider plugin.  Using it isn't required but can cut down
// significantly on the amount of boilerplate necessary to fire up a new resource provider.
func Main(name string, provMaker func(*HostClient) (pulumirpc.ResourceProviderServer, error)) error {
	return provider.Main(name, provMaker)
}

// MainContext is the same as Main but it accepts a context so it can be cancelled.
func MainContext(ctx context.Context, name string, provMaker func(*HostClient) (pulumirpc.ResourceProviderServer, error)) error {
	return provider.MainContext(ctx, name, provMaker)
}

