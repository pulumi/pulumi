// Copyright 2017 Pulumi, Inc. All rights reserved.

package errors

import (
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// errors tracks all existing errors, keyed by their unique ID.
var errors = make(map[diag.ID]*diag.Diag)

// newError registers a new error message underneath the given unique ID.
func newError(id diag.ID, message string) *diag.Diag {
	contract.Assert(errors[id] == nil)
	e := &diag.Diag{ID: id, Message: message}
	errors[id] = e
	return e
}

// newWarning registers a new warning message underneath the given unique ID.
func newWarning(id diag.ID, message string) *diag.Diag {
	// At the moment, there isn't a distinction between errors and warnings; however, we use different functions just in
	// case someday down the road there is, and so we don't have to go audit all callsites.
	return newError(id, message)
}
