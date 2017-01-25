// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorUnhandledException = &diag.Diag{
	ID:      1000,
	Message: "An unhandled exception terminated the program: %v",
}
