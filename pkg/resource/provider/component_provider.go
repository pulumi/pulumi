package provider

import provider "github.com/pulumi/pulumi/sdk/v3/pkg/resource/provider"

type Options = provider.Options

// MainWithOptions is an entrypoint for a resource provider plugin that implements `Construct` and optionally also
// `Call` for component resources.
// 
// Using it isn't required but can cut down significantly on the amount of boilerplate necessary to fire up a new
// resource provider for components.
func MainWithOptions(opts Options) error {
	return provider.MainWithOptions(opts)
}

// ComponentMain is an entrypoint for a resource provider plugin that implements `Construct` for component resources.
// Using it isn't required but can cut down significantly on the amount of boilerplate necessary to fire up a new
// resource provider for components.
func ComponentMain(name, version string, schema []byte, construct provider.ConstructFunc) error {
	return provider.ComponentMain(name, version, schema, construct)
}

