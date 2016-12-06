// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/ast"
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

var ErrorStackTypeNotFound = &diag.Diag{
	ID:      503,
	Message: "Stack type '%v' was not found; has it been installed?",
}

var ErrorNonAbstractStacksMustDefineServices = &diag.Diag{
	ID:      504,
	Message: "Non-abstract stacks must declare at least one private or public service",
}

var ErrorCannotCreateAbstractStack = &diag.Diag{
	ID:      505,
	Message: "Service '%v' cannot create abstract stack '%v'; only concrete stacks may be created",
}

var ErrorMissingRequiredProperty = &diag.Diag{
	ID:      506,
	Message: "Missing required property '%v' on '%v'",
}

var ErrorUnrecognizedProperty = &diag.Diag{
	ID:      507,
	Message: "Unrecognized property '%v' on '%v'",
}

var ErrorIncorrectType = &diag.Diag{
	ID:      508,
	Message: "Incorrect type; expected '%v', got '%v'",
}

var ErrorServiceNotFound = &diag.Diag{
	ID:      509,
	Message: "A service named '%v' was not found",
}

var ErrorServiceHasNoPublics = &diag.Diag{
	ID:      510,
	Message: "The service '%v' of type '%v' has no public entrypoint; it cannot be referenced",
}

var ErrorServiceHasManyPublics = &diag.Diag{
	ID:      511,
	Message: "The service '%v' of type '%v' has multiple public entrypoints; please choose one using a selector",
}

var ErrorServiceSelectorNotFound = &diag.Diag{
	ID:      512,
	Message: "No public by the given selector '%v' was found in service '%v' of type '%v'",
}

var ErrorServiceSelectorIsPrivate = &diag.Diag{
	ID:      513,
	Message: "The given selector '%v' references a private service in '%v' of type '%v'; it must be public",
}

var ErrorNotAName = &diag.Diag{
	ID:      514,
	Message: "The string '%v' is not a valid name (expected: " + ast.NamePartRegexps + ")",
}
