// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package deploy

import (
	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

// Target represents information about a deployment target.
type Target struct {
	Name   tokens.QName       // the target environment name.
	Config resource.ConfigMap // optional configuration key/values.
}
