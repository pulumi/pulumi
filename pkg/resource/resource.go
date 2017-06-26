// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"github.com/pulumi/lumi/pkg/tokens"
)

// Resource is an instance of a resource with an ID, type, and bag of state.
type Resource interface {
	URN() URN          // the resource's object URN: a human-friendly, unique name for the resource.
	Type() tokens.Type // the resource's type.
}

// Status is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type Status int

const (
	StatusOK Status = iota
	StatusUnknown
)

// HasURN returns true if the resource has been assigned a universal resource name (URN).
func HasURN(r Resource) bool {
	return r.URN() != ""
}
