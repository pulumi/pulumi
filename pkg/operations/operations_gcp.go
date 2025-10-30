package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/operations"

// GCPOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/gcp` implementation.
func GCPOperationsProvider(config map[config.Key]string, component *Resource) (Provider, error) {
	return operations.GCPOperationsProvider(config, component)
}

