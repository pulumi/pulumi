// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

// Binder errors are in the [500-600) range.
var (
	ErrorInvalidPackageName  = newError(500, "The package name must be a valid identifier")
	ErrorMalformedPackageURL = newError(501, "Package URL '%v' is malformed: %v")
	ErrorImportNotFound      = newError(502, "The imported package '%v' was not found; has it been installed?")
	ErrorImportCycle         = newError(
		503, "An import cycle was found in %v's transitive closure of package imports")
	ErrorTypeNotFound             = newError(504, "Type '%v' could not be found: %v")
	ErrorSymbolNotFound           = newError(505, "Symbol '%v' could not be found: %v")
	ErrorSymbolAlreadyExists      = newError(506, "A symbol already exists with the name '%v'")
	ErrorIncorrectExprType        = newError(507, "Expression has an incorrect type; expected '%v', got '%v'")
	ErrorMemberNotAccessible      = newError(508, "Member '%v' is not accessible (it is %v)")
	ErrorExpectedReturnExpr       = newError(509, "Expected a return expression of type %v")
	ErrorUnexpectedReturnExpr     = newError(510, "Unexpected return expression; function has no return type (void)")
	ErrorDuplicateLabel           = newError(511, "Duplicate label '%v': %v")
	ErrorUnknownJumpLabel         = newError(512, "Unknown label '%v' used in the %v statement")
	ErrorCannotNewAbstractClass   = newError(513, "Cannot `new` an abstract class '%v'")
	ErrorIllegalObjectLiteralType = newError(514,
		"The type '%v' may not be used as an object literal type; only records and interfaces are permitted")
	ErrorMissingRequiredProperty = newError(515, "Missing required property '%v'")
)
