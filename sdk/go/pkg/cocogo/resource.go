// Copyright 2017 Pulumi, Inc. All rights reserved.

package cocogo

// Resource is the base struct for all Coconut Go resource type definitions.
type Resource struct {
	Name string `json:"name"` // a unique friendly name for this resource.
}
