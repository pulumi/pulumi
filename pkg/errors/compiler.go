// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var MissingMufile = &diag.Diag{
	ID:      100,
	Message: "No Mufile was found in the given path or any of its parents (%v)",
}

var WarnIllegalMufileCasing = &diag.Diag{
	ID:      101,
	Message: "A Mufile-like file was located, but it has incorrect casing (expected Mufile.*)",
}

var WarnIllegalMufileExt = &diag.Diag{
	ID:      102,
	Message: "A file named `Mufile` was located, but '%v' isn't a valid file extension (expected .json or .yaml)",
}
