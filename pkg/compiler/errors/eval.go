// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

// Eval errors are in the [1000,2000) range.
var (
	ErrorUnhandledException        = newError(1000, "An unhandled exception terminated the program: %v")
	ErrorPackageHasNoDefaultModule = newError(1001, "Package '%v' is missing a default module")
	ErrorModuleHasNoEntryPoint     = newError(1002, "Module '%v' is missing an entrypoint function")
	ErrorFunctionArgMismatch       = newError(1003, "Function expected %v arguments, but only got %v")
	ErrorFunctionArgIncorrectType  = newError(1004, "Function argument has an incorrect type; expected %v, got %v")
	ErrorFunctionArgNotFound       = newError(1005, "Function argument '%v' was not supplied")
	ErrorFunctionArgUnknown        = newError(1006, "Function argument '%v' was not recognized")
)
