// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi-fabric/pkg/resource"
)

// Progress can be used for progress reporting.
type Progress interface {
	// Before is invoked prior to a step executing.
	Before(step Step)
	// After is invoked after a step executes, and is given access to the error, if any, that occurred.
	After(step Step, state resource.Status, err error)
}
