// Copyright 2016 Marapongo, Inc. All rights reserved.

package errors

import (
	"github.com/marapongo/mu/pkg/diag"
)

var ErrorUnrecognizedIntrinsic = &diag.Diag{
	ID:      1000,
	Message: "Intrinsic '%v' was not recognized; it may be unsupported for the target cloud architecture",
}
