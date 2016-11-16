// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var MissingMetadataName = &diag.Diag{
	ID:      200,
	Message: "This %v is missing a `name` property (or it is empty)",
}

var IllegalMetadataSemVer = &diag.Diag{
	ID:      201,
	Message: "This %v's version '%v' is not a valid semantic version number (note: it may not be a range)",
}

var IllegalDependencySemVer = &diag.Diag{
	ID:      202,
	Message: "Dependency '%v's version '%v' is not a valid semantic version number or range",
}
