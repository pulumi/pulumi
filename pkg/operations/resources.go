package operations

import operations "github.com/pulumi/pulumi/sdk/v3/pkg/operations"

// Resource is a tree representation of a resource/component hierarchy
type Resource = operations.Resource

// NewResourceMap constructs a map of resources with parent/child relations, indexed by URN.
func NewResourceMap(source []*resource.State) map[resource.URN]*Resource {
	return operations.NewResourceMap(source)
}

// NewResourceTree constructs a tree representation of a resource/component hierarchy
func NewResourceTree(source []*resource.State) *Resource {
	return operations.NewResourceTree(source)
}

