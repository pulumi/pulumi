// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorMissingMufile = &diag.Diag{
	ID:      100,
	Message: "No Mufile was found in the given path or any of its parents (%v)",
}

var WarningIllegalMufileCasing = &diag.Diag{
	ID:      101,
	Message: "A Mufile-like file was located, but it has incorrect casing (expected Mufile.*)",
}

var WarningIllegalMufileExt = &diag.Diag{
	ID:      102,
	Message: "A file named `Mufile` was located, but '%v' isn't a valid file extension (expected .json or .yaml)",
}

var ErrorIO = &diag.Diag{
	ID:      103,
	Message: "An IO error occurred during the current operation: %v",
}

var ErrorMissingDependency = &diag.Diag{
	ID:      104,
	Message: "The dependency '%v' could not be found; has it been installed?",
}

var ErrorUnrecognizedCloudArch = &diag.Diag{
	ID:      120,
	Message: "The cloud architecture '%v' was not recognized",
}

var ErrorUnrecognizedSchedulerArch = &diag.Diag{
	ID:      121,
	Message: "The cloud scheduler architecture '%v' was not recognized",
}

var ErrorIllegalCloudSchedulerCombination = &diag.Diag{
	ID:      122,
	Message: "The cloud architecture '%v' is incompatible with scheduler '%v'",
}

var ErrorConflictingClusterArchSelection = &diag.Diag{
	ID:      123,
	Message: "The cloud architecture specification '%v' conflicts with cluster '%v's setting of '%v'",
}

var ErrorClusterNotFound = &diag.Diag{
	ID:      124,
	Message: "A cloud target '%v' was not found in the stack or cluster definition",
}

var ErrorMissingTarget = &diag.Diag{
	ID:      125,
	Message: "Neither a target nor cloud architecture was provided, and no defaults were found",
}
