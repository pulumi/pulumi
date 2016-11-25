// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorCouldNotReadMufile = &diag.Diag{
	ID:      150,
	Message: "An IO error occurred while reading the Mufile: %v",
}

var ErrorIllegalMufileSyntax = &diag.Diag{
	ID:      151,
	Message: "A syntax error was detected while parsing the Mufile: %v",
}

var ErrorIllegalWorkspaceSyntax = &diag.Diag{
	ID:      152,
	Message: "A syntax error was detected while parsing workspace settings: %v",
}

var ErrorBadTemplate = &diag.Diag{
	ID:      153,
	Message: "A template error occurred: %v",
}
