// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorMissingStackName = &diag.Diag{
	ID:      500,
	Message: "This Stack is missing a `name` property (or it is empty)",
}

var ErrorIllegalStackVersion = &diag.Diag{
	ID:      501,
	Message: "This Stack's version '%v' is invalid: %v",
}

var ErrorSymbolAlreadyExists = &diag.Diag{
	ID:      502,
	Message: "A symbol already exists with the name '%v'",
}

var ErrorTypeNotFound = &diag.Diag{
	ID:      503,
	Message: "Type '%v' was not found",
}

var ErrorNonAbstractStacksMustDefineServices = &diag.Diag{
	ID:      504,
	Message: "Non-abstract stacks must declare at least one private or public service",
}

var ErrorMalformedStackReference = &diag.Diag{
	ID: 505,
	Message: "The stack reference '%v' is malformed; " +
		"expected format is '[[proto://]base.url/]stack/../name[@version]': %v",
}
