// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package diag

import (
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// errors tracks all existing errors, keyed by their unique ID.
var errors = make(map[ID]*Diag)

// newError registers a new error message underneath the given unique ID.
func newError(id ID, message string) *Diag {
	contract.Assert(errors[id] == nil)
	e := &Diag{ID: id, Message: message}
	errors[id] = e
	return e
}

// Plan and apply errors are in the [2000,3000) range.
var (
	ErrorPlanApplyFailed              = newError(2000, "Plan apply failed: %v")
	ErrorDuplicateResourceURN         = newError(2001, "Duplicate resource URN '%v'; try giving it a unique name")
	ErrorResourceInvalid              = newError(2002, "%v resource '%v' has a problem: %v")
	ErrorResourcePropertyInvalidValue = newError(2003, "%v resource '%v's property '%v' value %v has a problem: %v")
	ErrorAnalyzeResourceFailure       = newError(2004,
		"Analyzer '%v' reported a resource error:\n"+
			"\tResource: %v\n"+
			"\tProperty: %v\n"+
			"\tReason: %v")
)
