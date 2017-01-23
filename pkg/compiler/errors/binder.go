// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/tokens"
)

var ErrorMissingPackageName = &diag.Diag{
	ID:      500,
	Message: "This package is missing a `name` property (or it is empty)",
}

var ErrorIllegalStackVersion = &diag.Diag{
	ID:      501,
	Message: "This Stack's version '%v' is invalid: %v",
}

var ErrorSymbolAlreadyExists = &diag.Diag{
	ID:      502,
	Message: "A symbol already exists with the name '%v'",
}

var ErrorPackageNotFound = &diag.Diag{
	ID:      503,
	Message: "Package '%v' was not found; has it been installed?",
}

var ErrorTypeNotFound = &diag.Diag{
	ID:      504,
	Message: "Type '%v' could not be found: %v",
}

var ErrorSymbolNotFound = &diag.Diag{
	ID:      504,
	Message: "Symbol '%v' could not be found: %v",
}

var ErrorMemberNotAccessible = &diag.Diag{
	ID:      505,
	Message: "Member '%v' is not accessible (it is %v)",
}

var ErrorNonAbstractStacksMustDefineServices = &diag.Diag{
	ID:      504,
	Message: "Non-abstract stacks must declare at least one private or public service",
}

var ErrorCannotNewAbstractClass = &diag.Diag{
	ID:      505,
	Message: "Cannot `new` an abstract class '%v'",
}

var ErrorMissingRequiredProperty = &diag.Diag{
	ID:      506,
	Message: "Missing required property '%v'",
}

var ErrorUnrecognizedProperty = &diag.Diag{
	ID:      507,
	Message: "Unrecognized property '%v'",
}

var ErrorIncorrectExprType = &diag.Diag{
	ID:      508,
	Message: "Expression has an incorrect type; expected '%v', got '%v'",
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
	Message: "The string '%v' is not a valid name (expected: " + tokens.NameRegexpPattern + ")",
}

var ErrorStackTypeExpected = &diag.Diag{
	ID:      515,
	Message: "A stack type was expected here; '%v' did not resolve to a stack ('%v')",
}

var ErrorSchemaTypeExpected = &diag.Diag{
	ID:      516,
	Message: "A schema type was expected here; '%v' did not resolve to a schema ('%v')",
}

var ErrorSchemaConstraintUnmet = &diag.Diag{
	ID:      517,
	Message: "Schema constraint %v unmet; expected %v, got %v",
}

var ErrorSchemaConstraintType = &diag.Diag{
	ID:      518,
	Message: "Unexpected type conflict with constraint %v; expected %v, got %v",
}

var ErrorImportCycle = &diag.Diag{
	ID:      520,
	Message: "An import cycle was found in %v's transitive closure of package imports",
}

var ErrorPackageURLMalformed = &diag.Diag{
	ID:      521,
	Message: "Package URL '%v' is malformed: %v",
}

var ErrorExpectedReturnExpr = &diag.Diag{
	ID:      522,
	Message: "Expected a return expression of type %v",
}

var ErrorUnexpectedReturnExpr = &diag.Diag{
	ID:      523,
	Message: "Unexpected return expression; function has no return type (void)",
}

var ErrorDuplicateLabel = &diag.Diag{
	ID:      524,
	Message: "Duplicate label '%v': %v",
}

var ErrorUnknownJumpLabel = &diag.Diag{
	ID:      525,
	Message: "Unknown label '%v' used in the %v statement",
}

var ErrorIllegalObjectLiteralType = &diag.Diag{
	ID:      526,
	Message: "The type '%v' may not be used as an object literal type; only records and interfaces are permitted",
}
