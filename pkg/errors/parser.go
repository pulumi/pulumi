// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var CouldNotReadMufile = &diag.Diag{
	ID:      1500,
	Message: "An IO error occurred while reading the Mufile: %v",
}

var IllegalMufileSyntax = &diag.Diag{
	ID:      1501,
	Message: "A syntax error was detected while parsing the Mufile: %v",
}
