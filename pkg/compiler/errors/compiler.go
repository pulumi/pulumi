// Copyright 2016 Pulumi, Inc. All rights reserved.

package errors

// Compiler errors are in the [100-200) range.
var (
	ErrorIO                        = newError(100, "An IO error occurred during the current operation: %v")
	ErrorMissingNutfile            = newError(101, "No Nutfile was found underneath the given path: %v")
	ErrorCouldNotReadNutfile       = newError(102, "An IO error occurred while reading the Nutfile: %v")
	ErrorIllegalNutfileSyntax      = newError(103, "A syntax error was detected while parsing the Nutfile: %v")
	ErrorIllegalWorkspaceSyntax    = newError(104, "A syntax error was detected while parsing workspace settings: %v")
	WarningIllegalMarkupFileCasing = newWarning(105, "A %v-like file was located, but it has incorrect casing")
	WarningIllegalMarkupFileExt    = newWarning(
		106, "A %v-like file was located, but %v isn't a valid file extension (expected .json or .yaml)")
)
