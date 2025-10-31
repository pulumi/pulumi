package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"

// ProviderFactory functions accept a PackageDescriptor and return a Provider. If the PackageDescriptor specifies a
// parameterization, the factory is responsible for returning a provider that has already been appropriately
// parameterized.
type ProviderFactory = convert.ProviderFactory

// ProviderFactoryFromHost builds a ProviderFactory that uses the given plugin host to create providers and manage their
// lifecycles.
func ProviderFactoryFromHost(ctx context.Context, host plugin.Host) ProviderFactory {
	return convert.ProviderFactoryFromHost(ctx, host)
}

