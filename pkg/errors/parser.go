// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"fmt"

	"github.com/marapongo/mu/pkg/ast"
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

var ErrorIllegalMapLikeSyntax = &diag.Diag{
	ID: 154,
	Message: "The map type '%v' is malformed (expected syntax is '" +
		fmt.Sprintf(string(ast.TypeDecorsMap), "key", "value") + "')",
}

var ErrorIllegalArrayLikeSyntax = &diag.Diag{
	ID: 155,
	Message: "The array type '%v' is malformed (expected syntax is '" +
		fmt.Sprintf(string(ast.TypeDecorsArray), "element") + "')",
}

var ErrorIllegalNameLikeSyntax = &diag.Diag{
	ID: 156,
	Message: "The named type '%v' is malformed; " +
		"expected format is '[[proto://]base.url/]stack/../name[@version]': %v",
}
