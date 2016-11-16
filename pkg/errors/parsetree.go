// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var MissingStackName = &diag.Diag{
	ID:      200,
	Message: "This Stack is missing a `name` property (or it is empty)",
}

var IllegalStackSemVer = &diag.Diag{
	ID:      1502,
	Message: "This Stack's version '%v' is not a valid semantic version number (note: it may not be a range)",
}
