// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi/pkg/tokens"
)

// Target represents information about a deployment target.
type Target struct {
	Name   tokens.QName                   // the target environment name.
	Config map[tokens.ModuleMember]string // optional configuration key/values.
}
