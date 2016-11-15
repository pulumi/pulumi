// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var MissingMufile = &diag.Diag{
	ID:      100,
	Message: "No Mufile was found in the given path or any of its parents (%v)",
}
