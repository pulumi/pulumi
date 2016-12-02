// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorUnrecognizedExtensionProvider = &diag.Diag{
	ID:      1000,
	Message: "Extension type '%v' was not recognized",
}
