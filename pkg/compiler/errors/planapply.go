// Copyright 2016 Pulumi, Inc. All rights reserved.

package errors

// Plan and apply errors are in the [2000,3000) range.
var (
	ErrorCantCreateCompiler     = newError(2000, "An error occurred during compiler construction: %v")
	ErrorCantReadPackage        = newError(2001, "An error occurred while reading the package '%v': %v")
	ErrorCantCreateSnapshot     = newError(2002, "A problem was encountered creating a snapshot: %v")
	ErrorPlanApplyFailed        = newError(2003, "Plan apply failed: %v")
	ErrorIllegalMarkupExtension = newError(2004, "Resource serialization failed; illegal markup extension '%v'")
	ErrorCantReadDeployment     = newError(2005, "Could not read deployment file '%v': %v")
	ErrorDuplicateURNNames      = newError(2006, "Duplicate objects with the same URN: %v")
	ErrorInvalidEnvName         = newError(2007, "Environment '%v' could not be found in the current workspace")
	ErrorIllegalConfigToken     = newError(2008,
		"Configs may only target module properties and class static properties; %v is neither")
	ErrorConfigApplyFailure           = newError(2009, "One or more errors occurred while applying '%v's configuration")
	ErrorResourcePropertyValueInvalid = newError(2010, "Resource '%v's property '%v' value is invalid: %v")
)
