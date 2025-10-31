package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/operations"

// CloudOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/cloud-aws` implementation.
func CloudOperationsProvider(config map[config.Key]string, component *Resource) (Provider, error) {
	return operations.CloudOperationsProvider(config, component)
}

