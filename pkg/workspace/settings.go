// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

// Settings defines workspace settings shared amongst many related projects.
type Settings struct {
	Namespace string       `json:"namespace,omitempty"` // an optional namespace.
	Env       tokens.QName `json:"env,omitempty"`       // an optional default environment to use.
}
