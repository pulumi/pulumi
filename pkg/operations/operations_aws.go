package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/operations"

// AWSOperationsProvider creates an OperationsProvider capable of answering operational queries based on the
// underlying resources of the `@pulumi/aws` implementation.
func AWSOperationsProvider(config map[config.Key]string, component *Resource) (Provider, error) {
	return operations.AWSOperationsProvider(config, component)
}

