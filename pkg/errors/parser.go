// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var IllegalMufileExt = &diag.Diag{
	ID:      1500,
	Message: "A file named `Mufile` was located, but '%v' isn't a valid file extension (must be .json or .yaml)",
}

var CouldNotReadMufile = &diag.Diag{
	ID:      1501,
	Message: "An IO error occurred while reading the Mufile: %v",
}

var IllegalMufileSyntax = &diag.Diag{
	ID:      1502,
	Message: "A syntax error was detected while parsing the Mufile: %v",
}
