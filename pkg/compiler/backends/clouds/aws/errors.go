// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"github.com/marapongo/mu/pkg/diag"
)

// TODO: we need a strategy for error message numbering, perhaps even using distinct prefixes (e.g., AWS vs. MU).

var ErrorMarshalingCloudFormationTemplate = &diag.Diag{
	ID:      10000,
	Message: "An error occurred when marshaling the output AWS CloudFormation template: %v",
}
