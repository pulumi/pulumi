// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package errors

// Plan and apply errors are in the [2000,3000) range.
var (
	ErrorCantCreateCompiler     = newError(2000, "An error occurred during compiler construction: %v")
	ErrorCantReadPackage        = newError(2001, "An error occurred while reading the package '%v': %v")
	ErrorCantCreateSnapshot     = newError(2002, "A problem was found during planning: %v")
	ErrorPlanApplyFailed        = newError(2003, "Plan apply failed: %v")
	ErrorIllegalMarkupExtension = newError(2004, "Resource serialization failed; illegal markup extension '%v'")
	ErrorCantReadDeployment     = newError(2005, "Could not read deployment file '%v': %v")
	ErrorDuplicateURNNames      = newError(2006, "Duplicate objects with the same URN: %v")
	ErrorInvalidEnvName         = newError(2007, "Environment '%v' could not be found in the current workspace")
	ErrorIllegalConfigToken     = newError(2008,
		"Configs may only target module properties and class static properties; '%v' is neither")
	ErrorConfigApplyFailure           = newError(2009, "One or more errors occurred while applying '%v's configuration")
	ErrorDuplicateResourceURN         = newError(2010, "Duplicate resource URN '%v'; try giving it a unique name")
	ErrorResourceInvalid              = newError(2012, "%v resource '%v' has a problem: %v")
	ErrorResourcePropertyInvalidValue = newError(2013, "%v resource '%v's property '%v' value %v has a problem: %v")
	ErrorAnalyzeFailure               = newError(2014, "Analyzer '%v' reported an error: %v")
	ErrorAnalyzeResourceFailure       = newError(2015,
		"Analyzer '%v' reported a resource error:\n"+
			"\tResource: %v\n"+
			"\tProperty: %v\n"+
			"\tReason: %v")
)
