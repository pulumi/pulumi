// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package errors

// Compiler errors are in the [100-200) range.
var (
	ErrorIO                        = newError(100, "An IO error occurred during the current operation: %v")
	ErrorMissingProject            = newError(101, "No project was found underneath the given path: %v")
	ErrorCouldNotReadProject       = newError(102, "An IO error occurred while reading the project: %v")
	ErrorCouldNotReadPackage       = newError(103, "An IO error occurred while reading the package: %v")
	ErrorIllegalProjectSyntax      = newError(104, "A syntax error was detected while parsing the project: %v")
	ErrorIllegalWorkspaceSyntax    = newError(105, "A syntax error was detected while parsing workspace settings: %v")
	WarningIllegalMarkupFileCasing = newWarning(106, "A %v-like file was located, but it has incorrect casing")
	WarningIllegalMarkupFileExt    = newWarning(
		107, "A %v-like file was located, but %v isn't a valid file extension (expected .json or .yaml)")
)
