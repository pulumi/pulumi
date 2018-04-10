// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package diag

import (
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// errors tracks all existing errors, keyed by their unique ID.
var errors = make(map[ID]*Diag)

// newError registers a new error message underneath the given unique ID.
func newError(urn resource.URN, id ID, message string) *Diag {
	contract.Assert(errors[id] == nil)
	e := &Diag{URN: urn, ID: id, Message: message}
	errors[id] = e
	return e
}

// Plan and apply errors are in the [2000,3000) range.

func GetPlanApplyFailedError(urn resource.URN) *Diag {
	return newError(urn, 2000, "Plan apply failed: %v")
}

func GetDuplicateResourceURNError(urn resource.URN) *Diag {
	return newError(urn, 2001, "Duplicate resource URN '%v'; try giving it a unique name")
}

func GetResourceInvalidError(urn resource.URN) *Diag {
	return newError(urn, 2002, "%v resource '%v' has a problem: %v")
}

func GetResourcePropertyInvalidValueError(urn resource.URN) *Diag {
	return newError(urn, 2003, "%v resource '%v's property '%v' value %v has a problem: %v")
}

func GetAnalyzeResourceFailureError(urn resource.URN) *Diag {
	return newError(urn, 2004,
		"Analyzer '%v' reported a resource error:\n"+
			"\tResource: %v\n"+
			"\tProperty: %v\n"+
			"\tReason: %v")
}
