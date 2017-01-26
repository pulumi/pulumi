// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

// Eval errors are in the [1000,2000) range.
var (
	ErrorUnhandledException = newError(1000, "An unhandled exception terminated the program: %v")
)
