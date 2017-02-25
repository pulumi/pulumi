// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

// Plan and apply errors are in the [2000,3000) range.
var (
	ErrorCantCreateCompiler     = newError(2000, "An error occurred during compiler construction: %v")
	ErrorCantReadPackage        = newError(2001, "An error occurred while reading the package '%v': %v")
	ErrorCantCreateSnapshot     = newError(2002, "Illegal MuGL structure detected; cannot create a snapshot: %v")
	ErrorPlanApplyFailed        = newError(2003, "Plan apply failed: %v")
	ErrorIllegalMarkupExtension = newError(2004, "Resource serialization failed; illegal markup extension '%v'")
	ErrorCantReadSnapshot       = newError(2005, "Could not read snapshot file '%v': %v")
	ErrorDuplicateMonikerNames  = newError(2006, "Duplicate objects with the same name: %v")
)
