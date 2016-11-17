// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var SymbolAlreadyExists = &diag.Diag{
	ID:      500,
	Message: "A symbol already exists with the name '%v'",
}

var TypeNotFound = &diag.Diag{
	ID:      501,
	Message: "Type '%v' was not found",
}
