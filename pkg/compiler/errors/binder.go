// Copyright 2017 Pulumi, Inc. All rights reserved.

package errors

// Binder errors are in the [500-600) range.
var (
	ErrorInvalidPackageName       = newError(500, "The package name must be a valid identifier")
	ErrorMalformedToken           = newError(501, "%v token '%v' is malformed: %v")
	ErrorMalformedPackageURL      = newError(502, "Package URL '%v' is malformed: %v")
	ErrorImportNotFound           = newError(503, "The imported package '%v' was not found; has it been installed?%v")
	ErrorTypeNotFound             = newError(504, "Type '%v' could not be found: %v")
	ErrorSymbolNotFound           = newError(505, "Symbol '%v' could not be found: %v")
	ErrorSymbolAlreadyExists      = newError(506, "A symbol already exists with the name '%v'")
	ErrorIncorrectExprType        = newError(507, "Expression has the wrong type; expected '%v', got '%v'")
	ErrorMemberNotAccessible      = newError(508, "Member '%v' is not accessible (it is %v)")
	ErrorExpectedReturnExpr       = newError(509, "Expected a return expression of type '%v'")
	ErrorUnexpectedReturnExpr     = newError(510, "Unexpected return expression; function has no return type (void)")
	ErrorDuplicateLabel           = newError(511, "Duplicate label '%v': %v")
	ErrorUnknownJumpLabel         = newError(512, "Unknown label '%v' used in the %v statement")
	ErrorCannotNewAbstractClass   = newError(513, "Cannot `new` an abstract class '%v'")
	ErrorIllegalObjectLiteralType = newError(514,
		"The type '%v' may not be used as an object literal type; only records and interfaces are permitted")
	ErrorMissingRequiredProperty     = newError(515, "Missing required property '%v'")
	ErrorUnaryOperatorInvalidForType = newError(516,
		"The operator %v is invalid on operand type '%v'; expected '%v'")
	ErrorUnaryOperatorInvalidForOperand = newError(517, "The operator %v is invalid on operand %v; expected %v")
	ErrorUnaryOperatorMustBePrefix      = newError(518, "The operator %v must be in a prefix position (not postfix)")
	ErrorBinaryOperatorInvalidForType   = newError(519,
		"The operator %v is invalid on %v operand type '%v'; expected '%v'")
	ErrorIllegalAssignmentLValue        = newError(520, "Cannot assign to the target LHS expression (not an l-value)")
	ErrorIllegalNumericAssignmentLValue = newError(521, "Cannot perform numeric assignment %v on a non-numeric LHS")
	ErrorIllegalAssignmentTypes         = newError(522, "Cannot assign a value of type '%v' to target of type '%v'")
	ErrorCannotInvokeNonFunction        = newError(523, "Cannot invoke a non-function; type '%v' is not a function")
	ErrorArgumentCountMismatch          = newError(524, "Function expects %v arguments; got %v instead")
	ErrorConstructorReturnType          = newError(525, "Constructor '%v' has a return type '%v'; should be nil (void)")
	ErrorConstructorNotMethod           = newError(526, "Constructor '%v' is not a method; got %v instead")
	ErrorExpectedObject                 = newError(527,
		"Expected an object target for this instance member load operation")
	ErrorUnexpectedObject = newError(528,
		"Unexpected object target for this static or module load operation")
	ErrorInvalidCast               = newError(529, "Illegal cast from '%v' to '%v'; this can never succeed")
	ErrorModuleAliasTargetNotFound = newError(530, "Module alias target '%v' was not found (from '%v')")
)
