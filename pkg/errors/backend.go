// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorUnrecognizedExtensionProvider = &diag.Diag{
	ID:      1000,
	Message: "Extension type '%v' was not recognized",
}

var ErrorMissingExtensionProperty = &diag.Diag{
	ID:      1001,
	Message: "Missing required property '%v'",
}

var ErrorIncorrectExtensionPropertyType = &diag.Diag{
	ID:      1001,
	Message: "Property '%v' has the wrong type; got '%v', expected '%v'",
}
