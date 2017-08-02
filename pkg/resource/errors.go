// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/pulumi-fabric/pkg/util/mapper"
)

// NewErrors creates a new error list pertaining to a resource.  Note that it just turns around and defers to
// the same mapping infrastructure used for serialization and deserialization, but it presents a nicer interface.
func NewErrors(errs []error) error {
	return mapper.NewMappingError(errs)
}

// NewPropertyError creates a new error pertaining to a resource's property.  Note that it just turns around and defers
// to the same mapping infrastructure used for serialization and deserialization, but it presents a nicer interface.
func NewPropertyError(typ string, property string, err error) error {
	return mapper.NewFieldError(typ, property, err)
}
