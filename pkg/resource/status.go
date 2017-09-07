// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

// Status is returned when an error has occurred during a resource provider operation.  It indicates whether the
// operation could be rolled back cleanly (OK).  If not, it means the resource was left in an indeterminate state.
type Status int

const (
	StatusOK Status = iota
	StatusUnknown
)
